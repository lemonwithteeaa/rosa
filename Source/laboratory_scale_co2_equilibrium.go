//go:build windows

package main

import (
	"fmt"
	"math"
)

// Reduced gas/water equilibrium for the verified NaCl/CO2 reference domain.
// Scope: 25 C, pure CO2, 0 < P <= 1 atm, equal Na/Cl background <= 0.1 mol/kgw,
// no minerals, Ca/Mg, precipitation, kinetics or other acid systems.
type laboratoryCO2AqueousInput struct {
	Gas                        laboratoryCO2GasInput
	SodiumMolal, ChlorideMolal float64
}

type laboratoryCO2Equilibrium struct {
	Status                                   string
	PH, IonicStrength, WaterActivity         float64
	PressureATM, FugacityATM                 float64
	DissolvedCO2Activity                     float64
	CO2Molal, HCO3Molal, CO3Molal            float64
	CarbonWaterMol, GasMoles, TotalCarbonMol float64
}

func laboratoryCO2AqueousConstants25() (logHCO3, logCO2, logKW float64) {
	t := 298.15
	logHCO3 = 107.8871 + 0.03252849*t - 5151.79/t - 38.92561*math.Log10(t) + 563713.9/(t*t)
	logCO2 = 464.1965 + 0.09344813*t - 26986.16/t - 165.75951*math.Log10(t) + 2248628.9/(t*t)
	logKW = 293.29227 + 0.1360833*t - 10576.913/t - 123.73158*math.Log10(t) - 6.996455e-5*t*t
	return
}

func laboratoryCO2ReducedValidate(input laboratoryCO2AqueousInput) (laboratoryCO2GasNormalized, error) {
	g, err := laboratoryNormalizeCO2GasInput(input.Gas)
	if err != nil {
		return g, err
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if !finite(input.SodiumMolal) || !finite(input.ChlorideMolal) || input.SodiumMolal < 0 || input.ChlorideMolal < 0 {
		return g, fmt.Errorf("Фоновая моляльность Na/Cl должна быть конечной и неотрицательной")
	}
	if math.Abs(input.SodiumMolal-input.ChlorideMolal) > 1e-9 {
		return g, fmt.Errorf("Первая проверяемая CO₂-задача поддерживает только электронейтральный фон NaCl: Na и Cl должны совпадать")
	}
	if input.SodiumMolal > .1 {
		return g, fmt.Errorf("Первая проверяемая CO₂-задача ограничена фоном NaCl не выше 0,1 моль/кг воды")
	}
	return g, nil
}

type laboratoryCO2ReducedSpecies struct {
	h, oh, co2, hco3, co3, ionic, aw float64
}

func laboratoryCO2ReducedSpeciesAtPH(ph, co2Activity, background float64) (laboratoryCO2ReducedSpecies, error) {
	if ph < 0 || ph > 14 || co2Activity <= 0 || math.IsNaN(co2Activity) || math.IsInf(co2Activity, 0) {
		return laboratoryCO2ReducedSpecies{}, fmt.Errorf("Некорректное состояние водной CO₂-задачи")
	}
	logHCO3, logCO2, logKW := laboratoryCO2AqueousConstants25()
	kHCO3, kCO2, kw := math.Pow(10, logHCO3), math.Pow(10, logCO2), math.Pow(10, logKW)
	ionic, aw := background, 1-0.017*(2*background)
	if aw <= 0 {
		return laboratoryCO2ReducedSpecies{}, fmt.Errorf("Активность воды вышла за физический диапазон")
	}
	out := laboratoryCO2ReducedSpecies{}
	for n := 0; n < 100; n++ {
		g1 := laboratoryCarbonDaviesGamma(1, ionic)
		g2 := laboratoryCarbonDaviesGamma(2, ionic)
		// phreeqc.dat: CO2 has -gamma 0 0.066, hence log10(gamma)=0.066*I.
		g0 := math.Pow(10, 0.066*ionic)
		aH := math.Pow(10, -ph)
		aCO3 := co2Activity * aw / (kCO2 * aH * aH)
		aHCO3 := kHCO3 * aCO3 * aH
		aOH := kw * aw / aH
		out = laboratoryCO2ReducedSpecies{h: aH / g1, oh: aOH / g1, co2: co2Activity / g0, hco3: aHCO3 / g1, co3: aCO3 / g2}
		newI := .5 * (2*background + out.h + out.oh + out.hco3 + 4*out.co3)
		sumMolal := 2*background + out.h + out.oh + out.co2 + out.hco3 + out.co3
		newAw := 1 - .017*sumMolal
		if newAw <= 0 || newI > .1 {
			return laboratoryCO2ReducedSpecies{}, fmt.Errorf("Водная CO₂-задача вышла за область Дэвиса I≤0,1")
		}
		if math.Abs(newI-ionic) < 1e-14 && math.Abs(newAw-aw) < 1e-14 {
			out.ionic, out.aw = newI, newAw
			return out, nil
		}
		ionic, aw = newI, newAw
	}
	return laboratoryCO2ReducedSpecies{}, fmt.Errorf("Водная CO₂-задача не сошлась")
}

func laboratoryCO2ReducedPHForActivity(co2Activity, background float64) (float64, laboratoryCO2ReducedSpecies, error) {
	charge := func(ph float64) (float64, laboratoryCO2ReducedSpecies, error) {
		s, err := laboratoryCO2ReducedSpeciesAtPH(ph, co2Activity, background)
		if err != nil {
			return 0, s, err
		}
		return s.h - s.oh - s.hco3 - 2*s.co3, s, nil
	}
	lo, hi := 3.0, 7.5
	flo, _, err := charge(lo)
	if err != nil {
		return 0, laboratoryCO2ReducedSpecies{}, err
	}
	fhi, _, err := charge(hi)
	if err != nil {
		return 0, laboratoryCO2ReducedSpecies{}, err
	}
	if flo*fhi > 0 {
		return 0, laboratoryCO2ReducedSpecies{}, fmt.Errorf("Не удалось ограничить корень pH")
	}
	for i := 0; i < 100; i++ {
		mid := .5 * (lo + hi)
		fm, s, err := charge(mid)
		if err != nil {
			return 0, s, err
		}
		if math.Abs(fm) < 1e-14 || hi-lo < 1e-12 {
			return mid, s, nil
		}
		if flo*fm <= 0 {
			hi = mid
			fhi = fm
		} else {
			lo = mid
			flo = fm
		}
		_ = fhi
	}
	ph := .5 * (lo + hi)
	_, s, err := charge(ph)
	return ph, s, err
}

func laboratoryCO2ReducedAtPressure(pressureATM, background, waterKG float64) (laboratoryCO2Equilibrium, error) {
	gas, err := laboratoryCO2PureGasState(pressureATM, 25)
	if err != nil {
		return laboratoryCO2Equilibrium{}, err
	}
	aCO2, err := laboratoryCO2ActivityFromFugacity(gas.FugacityATM, 25)
	if err != nil {
		return laboratoryCO2Equilibrium{}, err
	}
	ph, s, err := laboratoryCO2ReducedPHForActivity(aCO2, background)
	if err != nil {
		return laboratoryCO2Equilibrium{}, err
	}
	carbon := (s.co2 + s.hco3 + s.co3) * waterKG
	return laboratoryCO2Equilibrium{Status: "equilibrium", PH: ph, IonicStrength: s.ionic, WaterActivity: s.aw, PressureATM: pressureATM, FugacityATM: gas.FugacityATM, DissolvedCO2Activity: aCO2, CO2Molal: s.co2, HCO3Molal: s.hco3, CO3Molal: s.co3, CarbonWaterMol: carbon, TotalCarbonMol: carbon}, nil
}

func laboratoryCO2ReducedFromTotalCarbon(totalMol, background, waterKG float64) (laboratoryCO2Equilibrium, error) {
	if totalMol <= 0 || waterKG <= 0 {
		return laboratoryCO2Equilibrium{}, fmt.Errorf("Запас углерода и масса воды должны быть положительными")
	}
	lo, hi := 1e-12, 1.0
	var last laboratoryCO2Equilibrium
	for i := 0; i < 120; i++ {
		p := math.Sqrt(lo * hi)
		r, err := laboratoryCO2ReducedAtPressure(p, background, waterKG)
		if err != nil {
			return r, err
		}
		last = r
		if math.Abs(r.CarbonWaterMol/totalMol-1) < 1e-9 {
			r.Status = "stock_exhausted"
			r.PressureATM = 0
			r.FugacityATM = 0
			r.GasMoles = 0
			r.TotalCarbonMol = totalMol
			return r, nil
		}
		if r.CarbonWaterMol > totalMol {
			hi = p
		} else {
			lo = p
		}
	}
	last.Status = "stock_exhausted"
	last.PressureATM = 0
	last.FugacityATM = 0
	last.GasMoles = 0
	last.TotalCarbonMol = totalMol
	return last, nil
}

func laboratoryCO2SolveReduced(input laboratoryCO2AqueousInput) (laboratoryCO2Equilibrium, error) {
	g, err := laboratoryCO2ReducedValidate(input)
	if err != nil {
		return laboratoryCO2Equilibrium{}, err
	}
	background := input.SodiumMolal
	if g.Mode == "external_reservoir" {
		target, err := laboratoryCO2ReducedAtPressure(g.PressureATM, background, g.WaterKG)
		if err != nil {
			return target, err
		}
		if target.CarbonWaterMol <= g.ReservoirMoles*(1+1e-10) {
			target.GasMoles = g.ReservoirMoles - target.CarbonWaterMol
			target.TotalCarbonMol = g.ReservoirMoles
			return target, nil
		}
		return laboratoryCO2ReducedFromTotalCarbon(g.ReservoirMoles, background, g.WaterKG)
	}
	initial, err := laboratoryCO2InitialGasMoles(input.Gas)
	if err != nil {
		return laboratoryCO2Equilibrium{}, err
	}
	lo, hi := 1e-12, g.PressureATM
	var result laboratoryCO2Equilibrium
	for i := 0; i < 120; i++ {
		p := math.Sqrt(lo * hi)
		r, err := laboratoryCO2ReducedAtPressure(p, background, g.WaterKG)
		if err != nil {
			return r, err
		}
		gas, err := laboratoryCO2PureGasState(p, 25)
		if err != nil {
			return r, err
		}
		gasMol := g.GasVolumeL / gas.MolarVolumeL
		sum := r.CarbonWaterMol + gasMol
		r.GasMoles, r.TotalCarbonMol = gasMol, sum
		result = r
		if math.Abs(sum/initial-1) < 1e-9 {
			result.TotalCarbonMol = initial
			return result, nil
		}
		if sum > initial {
			hi = p
		} else {
			lo = p
		}
	}
	result.TotalCarbonMol = initial
	return result, nil
}
