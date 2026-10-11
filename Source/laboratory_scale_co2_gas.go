//go:build windows

package main

import (
	"fmt"
	"math"
)

// Constants match the archived USGS PHREEQC 3.8.9 source and phreeqc.dat.
// Only pure CO2, 25 C and 0 < P <= 1 atm. Not a coupled gas/water solver.
const laboratoryCO2GasR = 0.0820597 // litre-atm/(mol K), USGS source

type laboratoryCO2GasState struct {
	Z, Phi, FugacityATM, MolarVolumeL float64
}

func laboratoryCO2PureGasState(pressureATM, temperatureC float64) (laboratoryCO2GasState, error) {
	out := laboratoryCO2GasState{}
	if math.IsNaN(pressureATM) || math.IsInf(pressureATM, 0) || pressureATM <= 0 || pressureATM > 1 {
		return out, fmt.Errorf("Чистый CO₂: давление должно быть больше нуля и не выше 1 атм")
	}
	if math.IsNaN(temperatureC) || math.IsInf(temperatureC, 0) || math.Abs(temperatureC-25) > .001 {
		return out, fmt.Errorf("Газовая модель CO₂ ограничена 25 °C")
	}
	t := temperatureC + 273.15
	tc, pc, omega := 304.2, 72.86, .225
	kappa := .37464 + 1.54226*omega - .26992*omega*omega
	alpha := math.Pow(1+kappa*(1-math.Sqrt(t/tc)), 2)
	a := .457235 * alpha * math.Pow(tc/t, 2) * pressureATM / pc
	b := .077796 * (tc / t) * pressureATM / pc
	z := 1.0
	converged := false
	for i := 0; i < 100; i++ {
		f := z*z*z - (1-b)*z*z + (a-3*b*b-2*b)*z - (a*b - b*b - b*b*b)
		derivative := 3*z*z - 2*(1-b)*z + a - 3*b*b - 2*b
		next := z - f/derivative
		if math.IsNaN(next) || math.IsInf(next, 0) || next <= b {
			return out, fmt.Errorf("Не найден газовый корень уравнения CO₂")
		}
		if math.Abs(next-z) < 1e-14 {
			z = next
			converged = true
			break
		}
		z = next
	}
	if !converged {
		return out, fmt.Errorf("Газовое уравнение CO₂ не сошлось")
	}
	// log1p avoids loss of precision near the ideal-gas limit.
	sqrt2 := math.Sqrt(2)
	lnphi := z - 1 - math.Log(z-b) - a/(2*sqrt2*b)*math.Log1p(2*sqrt2*b/(z+(1-sqrt2)*b))
	phi := math.Exp(lnphi)
	out = laboratoryCO2GasState{Z: z, Phi: phi, FugacityATM: phi * pressureATM, MolarVolumeL: z * laboratoryCO2GasR * t / pressureATM}
	if math.IsInf(out.MolarVolumeL, 0) || math.IsNaN(phi) || phi <= 0 || out.FugacityATM <= 0 {
		return laboratoryCO2GasState{}, fmt.Errorf("Газовый расчёт CO₂ вышел за числовой диапазон")
	}
	return out, nil
}

func laboratoryCO2InitialGasMoles(input laboratoryCO2GasInput) (float64, error) {
	normalized, err := laboratoryNormalizeCO2GasInput(input)
	if err != nil {
		return 0, err
	}
	if normalized.Mode != "closed_volume" {
		return 0, fmt.Errorf("Начальный запас замкнутого газа не рассчитывается для внешнего резервуара")
	}
	state, err := laboratoryCO2PureGasState(normalized.PressureATM, normalized.TemperatureC)
	if err != nil {
		return 0, err
	}
	moles := normalized.GasVolumeL / state.MolarVolumeL
	if math.IsInf(moles, 0) || math.IsNaN(moles) || moles <= 0 {
		return 0, fmt.Errorf("Начальный запас CO₂ вышел за числовой диапазон")
	}
	return moles, nil
}

// Activity, not concentration or total carbon. Apply only when actual
// gas/water equilibrium is established; no assumption of sufficient supply.
func laboratoryCO2ActivityFromFugacity(fugacityATM, temperatureC float64) (float64, error) {
	if math.IsNaN(fugacityATM) || math.IsInf(fugacityATM, 0) || fugacityATM <= 0 || fugacityATM > 1 {
		return 0, fmt.Errorf("Фугитивность CO₂ должна быть положительной и не выше 1 атм")
	}
	if math.IsNaN(temperatureC) || math.IsInf(temperatureC, 0) || math.Abs(temperatureC-25) > .001 {
		return 0, fmt.Errorf("Связь растворённого CO₂ ограничена 25 °C")
	}
	t := temperatureC + 273.15
	logk := 10.5624 - .023547*t - 3972.8/t + 587460/(t*t) + .000019194*t*t
	return math.Pow(10, logk) * fugacityATM, nil
}
