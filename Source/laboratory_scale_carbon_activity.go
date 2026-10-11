//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

// A(25 C) determined from the actual PHREEQC 3.8.6-17100 reduced reference.
// Fixed version-specific coefficient; the legacy Davies route remains unchanged.
const laboratoryCarbonDaviesA25 = 0.5100247900260674

func laboratoryCarbonDaviesGamma(z, i float64) float64 {
	return math.Pow(10, -laboratoryCarbonDaviesA25*z*z*(math.Sqrt(i)/(1+math.Sqrt(i))-0.3*i))
}

type laboratoryCarbonActivitySolution struct {
	Forms                             laboratoryCarbonForms
	IonicStrength, Gamma1, Gamma2, SI float64
	Iterations                        int
}

func laboratoryCarbonDaviesSolve(ph, ca, hc, backgroundI float64) (laboratoryCarbonActivitySolution, error) {
	if math.IsNaN(ph) || math.IsInf(ph, 0) || ph < 0 || ph > 14 || math.IsNaN(ca) || math.IsInf(ca, 0) || ca <= 0 || math.IsNaN(hc) || math.IsInf(hc, 0) || hc <= 0 || math.IsNaN(backgroundI) || math.IsInf(backgroundI, 0) || backgroundI < 0 {
		return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для равновесия Дэвиса нужны корректные pH, Ca²⁺, HCO₃⁻ и ионный состав")
	}
	ideal, err := laboratoryCarbonFormsFromPH(25, ph, hc)
	if err != nil {
		return laboratoryCarbonActivitySolution{}, err
	}
	i := backgroundI + 0.5*hc
	for step := 1; step <= 100; step++ {
		if i > 0.1 {
			return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия Дэвиса ионная сила должна быть не выше 0,1 моль/л; нужен метод для рассола")
		}
		g1 := laboratoryCarbonDaviesGamma(1, i)
		g2 := laboratoryCarbonDaviesGamma(2, i)
		f := ideal
		f.CO3 = ideal.CO3 * g1 / g2
		f.CO2 = ideal.CO2 * g1
		f.Hydrogen = ideal.Hydrogen / g1
		f.Hydroxide = ideal.Hydroxide / g1
		f.TotalCarbon = f.CO2 + f.HCO3 + f.CO3
		f.Alkalinity = f.HCO3 + 2*f.CO3 + f.Hydroxide - f.Hydrogen
		target := backgroundI + 0.5*(hc+4*f.CO3+f.Hydrogen+f.Hydroxide)
		if math.IsNaN(target) || math.IsInf(target, 0) {
			return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия произошло переполнение")
		}
		if target > 0.1 {
			return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия Дэвиса ионная сила должна быть не выше 0,1 моль/л; нужен метод для рассола")
		}
		if math.Abs(target-i) <= 1e-12 {
			k, _ := laboratoryTemperatureLogK("Кальцит", 25)
			si := math.Log10(ca*g2) + math.Log10(f.CO3*g2) - k
			return laboratoryCarbonActivitySolution{f, i, g1, g2, si, step}, nil
		}
		i = 0.5 * (i + target)
	}
	return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия не достигнута сходимость; результат не выдан")
}
func laboratoryCarbonDaviesFromValues(v map[string]string) (laboratoryCarbonActivitySolution, error) {
	t, ok := parseLabNumber(v["sample_temp"])
	if !ok || math.Abs(t-25) > 0.001 {
		return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия Дэвиса требуется температура пробы 25 °C; температурное изменение коэффициента Дэвиса не реализовано")
	}
	if strings.TrimSpace(v["ph_measurement_temp"]) != "" {
		mt, ok := parseLabNumber(v["ph_measurement_temp"])
		if !ok || math.Abs(mt-t) > 0.5 {
			return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия температура измерения pH должна совпадать с температурой пробы")
		}
	}
	background := 0.0
	for _, ion := range []struct {
		key     string
		mass, z float64
	}{{"ca", 40.078, 2}, {"mg", 24.305, 2}, {"na", 22.990, 1}, {"k", 39.0983, 1}, {"cl", 35.45, 1}, {"so4", 96.06, 2}} {
		c, ok := scaleIonMolar(v, ion.key, ion.mass)
		if !ok {
			return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия нужен отдельный ион %s (нуль допустим); Na⁺ + K⁺ не подставляется вместо отдельных анализов", ion.key)
		}
		background += 0.5 * c * ion.z * ion.z
	}
	for _, ion := range []struct {
		key  string
		mass float64
	}{{"ba", 137.327}, {"sr", 87.62}} {
		if strings.TrimSpace(v[ion.key]) != "" {
			c, ok := scaleIonMolar(v, ion.key, ion.mass)
			if !ok {
				return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия некорректен %s", ion.key)
			}
			background += 2 * c
		}
	}
	ca, _ := scaleIonMolar(v, "ca", 40.078)
	hc, ok := scaleIonMolar(v, "hco3", 61.016)
	ph, pok := parseLabNumber(v["ph"])
	if !ok || !pok {
		return laboratoryCarbonActivitySolution{}, fmt.Errorf("Для карбонатного равновесия нужны pH и HCO₃⁻")
	}
	return laboratoryCarbonDaviesSolve(ph, ca, hc, background)
}
func laboratoryCarbonDaviesResults(v map[string]string) ([]laboratorySaturationResult, float64, []string) {
	s, e := laboratoryCarbonDaviesFromValues(v)
	if e != nil {
		return nil, 0, []string{e.Error()}
	}
	warnings := []string{"Дэвис при 25 °C, активность воды 1; введённые Ca²⁺/HCO₃⁻ и прочие ионы приняты свободными концентрациями. Комплексы, плотность и давление не учтены. Не является промысловой моделью или полным PHREEQC."}
	if strings.TrimSpace(v["ph_measurement_temp"]) == "" {
		warnings = append(warnings, "Температура измерения pH не указана: подтвердите измерение при 25 °C.")
	}
	warnings = append(warnings, laboratoryCarbonMeasuredChecks(v, s.Forms)...)
	if balance := laboratoryCarbonDaviesChargeBalance(v, s); math.Abs(balance) > 5 {
		warnings = append(warnings, fmt.Sprintf("Невязка баланса зарядов %+.2f %% выше рабочего контроля 5 %%; сверить ионный анализ, результат ограничен.", balance))
	}
	k, _ := laboratoryTemperatureLogK("Кальцит", 25)
	return []laboratorySaturationResult{{Mineral: "Кальцит", Formula: "CaCO₃", SI: s.SI, Ratio: math.Pow(10, s.SI), TemperatureC: 25, LogK: k}}, s.IonicStrength, warnings
}
func laboratoryCarbonDaviesText(v map[string]string) string {
	s, e := laboratoryCarbonDaviesFromValues(v)
	if e != nil {
		return e.Error()
	}
	f := s.Forms
	return fmt.Sprintf("Равновесие Дэвиса: I %.8g моль/л; γ₁ %.8g; γ₂ %.8g; итераций %d. Свободные формы: CO₂ %.6g мг/л; HCO₃⁻ %.6g мг/л; CO₃²⁻ %.6g мг/л. Сумма форм %.6g мг C/л; щёлочность %.6g мг/л как CaCO₃. Исходные поля не изменены.", s.IonicStrength, s.Gamma1, s.Gamma2, s.Iterations, f.CO2*44010, f.HCO3*61016, f.CO3*60010, f.TotalCarbon*12011.1, f.Alkalinity*50043.45)
}

func laboratoryCarbonDaviesChargeBalance(v map[string]string, s laboratoryCarbonActivitySolution) float64 {
	cations := s.Forms.Hydrogen
	anions := s.Forms.HCO3 + 2*s.Forms.CO3 + s.Forms.Hydroxide
	for _, ion := range []struct {
		key     string
		mass, z float64
	}{{"ca", 40.078, 2}, {"mg", 24.305, 2}, {"na", 22.990, 1}, {"k", 39.0983, 1}, {"ba", 137.327, 2}, {"sr", 87.62, 2}} {
		c, _ := scaleIonMolar(v, ion.key, ion.mass)
		cations += c * ion.z
	}
	cl, _ := scaleIonMolar(v, "cl", 35.45)
	so4, _ := scaleIonMolar(v, "so4", 96.06)
	anions += cl + 2*so4
	if cations+anions == 0 {
		return 0
	}
	return 100 * (cations - anions) / (cations + anions)
}
func laboratoryScaleModelIonBalance(v map[string]string, code string) (float64, bool) {
	if code == "water_mixing" {
		m, e := laboratoryWaterMixingFromValues(v)
		if e != nil {
			return 0, false
		}
		return m.ChargeBalance, true
	}
	if code == "multicomplex_molal" {
		normalized, _, e := laboratoryMolalityValues(v)
		if e != nil {
			return 0, false
		}
		return laboratoryScaleModelIonBalance(normalized, "multicomplex")
	}
	if code == "multicomplex" {
		s, e := laboratoryMultiComplexFromValues(v)
		if e != nil {
			return 0, false
		}
		na, _ := scaleIonMolar(v, "na", 22.99)
		k, _ := scaleIonMolar(v, "k", 39.0983)
		cl, _ := scaleIonMolar(v, "cl", 35.45)
		c := 2*(s.Ca+s.Mg+s.Ba+s.Sr) + na + k + s.Forms.Hydrogen + s.CaHCO3 + s.MgHCO3 + s.CaOH + s.MgOH
		a := 2*s.SO4 + cl + s.Forms.HCO3 + 2*s.Forms.CO3 + s.Forms.Hydroxide
		return 100 * (c - a) / (c + a), true
	}
	if code == "carbonate_complex" {
		s, e := laboratoryCarbonComplexFromValues(v)
		if e != nil {
			return 0, false
		}
		copy := map[string]string{}
		for k, x := range v {
			copy[k] = x
		}
		copy["ca"] = fmt.Sprint(s.FreeCalcium * 40078)
		balanceCations := s.Forms.Hydrogen + s.CaHCO3
		anions := s.Forms.HCO3 + 2*s.Forms.CO3 + s.Forms.Hydroxide
		for _, ion := range []struct {
			key     string
			mass, z float64
		}{{"ca", 40.078, 2}, {"mg", 24.305, 2}, {"na", 22.99, 1}, {"k", 39.0983, 1}, {"ba", 137.327, 2}, {"sr", 87.62, 2}} {
			x, _ := scaleIonMolar(copy, ion.key, ion.mass)
			balanceCations += x * ion.z
		}
		cl, _ := scaleIonMolar(v, "cl", 35.45)
		so4, _ := scaleIonMolar(v, "so4", 96.06)
		anions += cl + 2*so4
		return 100 * (balanceCations - anions) / (balanceCations + anions), true
	}
	if code != "carbonate_davies" {
		return laboratoryIonBalance(v)
	}
	s, e := laboratoryCarbonDaviesFromValues(v)
	if e != nil {
		return 0, false
	}
	return laboratoryCarbonDaviesChargeBalance(v, s), true
}
