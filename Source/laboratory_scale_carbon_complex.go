//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

type laboratoryCarbonComplexSolution struct {
	Forms                                                                    laboratoryCarbonForms
	FreeCalcium, CaCO3, CaHCO3, TotalCalcium, TotalCarbon, IonicStrength, SI float64
	Iterations                                                               int
}

// Reduced closed system: analytical Ca and C, CaCO3(aq)/CaHCO3+ only.
func laboratoryCarbonComplexSolve(ph, caTotal, cTotal, background float64) (laboratoryCarbonComplexSolution, error) {
	for _, x := range []float64{ph, caTotal, cTotal, background} {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов нужны конечные исходные числа")
		}
	}
	if ph < 0 || ph > 14 || caTotal <= 0 || cTotal <= 0 || caTotal > 10 || cTotal > 10 || background < 0 {
		return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов нужны pH 0–14, положительные общие Ca/C не выше 10 моль/л и корректный фон")
	}
	t := 298.15
	// Analytical coefficients in unchanged PHREEQC 3.8.6-17100 phreeqc.dat.
	k0 := math.Pow(10, -1228.732-0.29944*t+35512.75/t+485.818*math.Log10(t))
	k1 := math.Pow(10, -6.009+0.03377*t+2044/t)
	assoc, _ := laboratoryCarbonateAssociationLogK(25)
	ideal, _ := laboratoryCarbonFormsFromPH(25, ph, 1)
	i := math.Min(0.1, background+2*caTotal+0.5*cTotal)
	for step := 1; step <= 150; step++ {
		g1 := laboratoryCarbonDaviesGamma(1, i)
		g2 := laboratoryCarbonDaviesGamma(2, i)
		q0 := k0 * g2 * g1 * math.Pow(10, ph-assoc)
		q1 := k1 * g2 * math.Pow(10, -assoc)
		q := q0 + q1
		d := ideal.CO2*g1 + 1 + ideal.CO3*g1/g2
		b := d + q*(caTotal-cTotal)
		disc := math.Sqrt(b*b + 4*d*q*cTotal)
		hc := 0.0
		if b >= 0 {
			hc = 2 * cTotal / (b + disc)
		} else {
			hc = (-b + disc) / (2 * d * q)
		}
		ca := caTotal / (1 + hc*q)
		x := ca * hc * q0
		y := ca * hc * q1
		f := ideal
		f.HCO3 = hc
		f.CO3 = hc * ideal.CO3 * g1 / g2
		f.CO2 = hc * ideal.CO2 * g1
		f.Hydrogen = ideal.Hydrogen / g1
		f.Hydroxide = ideal.Hydroxide / g1
		f.TotalCarbon = f.CO2 + f.HCO3 + f.CO3
		f.Alkalinity = f.HCO3 + 2*f.CO3 + f.Hydroxide - f.Hydrogen + 2*x + y
		target := background + 0.5*(4*ca+hc+4*f.CO3+f.Hydrogen+f.Hydroxide+y)
		if math.IsNaN(target) || math.IsInf(target, 0) {
			return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов произошло переполнение")
		}
		if target > 0.1 {
			return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов I выше 0,1 моль/л; результат не выдан")
		}
		if math.Abs(target-i) <= 1e-12 {
			k, _ := laboratoryTemperatureLogK("Кальцит", 25)
			si := math.Log10(ca*g2) + math.Log10(f.CO3*g2) - k
			s := laboratoryCarbonComplexSolution{f, ca, x, y, ca + x + y, f.TotalCarbon + x + y, i, si, step}
			if math.Abs(s.TotalCalcium/caTotal-1) > 1e-10 || math.Abs(s.TotalCarbon/cTotal-1) > 1e-10 {
				return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов не замкнуты балансы Ca/C")
			}
			return s, nil
		}
		i = 0.5 * (i + target)
	}
	return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов не достигнута сходимость")
}
func laboratoryCarbonComplexFromValues(v map[string]string) (laboratoryCarbonComplexSolution, error) {
	t, ok := parseLabNumber(v["sample_temp"])
	if !ok || math.Abs(t-25) > 0.001 {
		return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов требуется температура пробы 25 °C")
	}
	if strings.TrimSpace(v["ph_measurement_temp"]) != "" {
		mt, ok := parseLabNumber(v["ph_measurement_temp"])
		if !ok || math.Abs(mt-t) > 0.5 {
			return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов температура измерения pH не согласована")
		}
	}
	ca, cok := scaleIonMolar(v, "ca_total", 40.078)
	c, ok := scaleIonMolar(v, "tic", 12.0111)
	ph, pok := parseLabNumber(v["ph"])
	if !cok || !ok || !pok {
		return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов нужны общий Ca (как Ca), общий неорганический C (как C) и pH")
	}
	background := 0.0
	for _, ion := range []struct {
		key     string
		mass, z float64
	}{{"mg", 24.305, 2}, {"na", 22.99, 1}, {"k", 39.0983, 1}, {"cl", 35.45, 1}, {"so4", 96.06, 2}} {
		x, ok := scaleIonMolar(v, ion.key, ion.mass)
		if !ok {
			return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов нужен отдельный %s; явный нуль допустим", ion.key)
		}
		background += 0.5 * x * ion.z * ion.z
	}
	for _, ion := range []struct {
		key  string
		mass float64
	}{{"ba", 137.327}, {"sr", 87.62}} {
		if strings.TrimSpace(v[ion.key]) != "" {
			x, ok := scaleIonMolar(v, ion.key, ion.mass)
			if !ok {
				return laboratoryCarbonComplexSolution{}, fmt.Errorf("Для карбонатных комплексов некорректен %s", ion.key)
			}
			background += 2 * x
		}
	}
	return laboratoryCarbonComplexSolve(ph, ca, c, background)
}
func laboratoryCarbonComplexText(v map[string]string) string {
	s, e := laboratoryCarbonComplexFromValues(v)
	if e != nil {
		return e.Error()
	}
	ca, _ := scaleIonMolar(v, "ca_total", 40.078)
	c, _ := scaleIonMolar(v, "tic", 12.0111)
	return fmt.Sprintf("Кальций: общий %.6g, свободный %.6g мг Ca/л. Комплексы: CaCO₃(aq) %.6g, CaHCO₃⁺ %.6g моль/л. Свободные формы: CO₂ %.6g, HCO₃⁻ %.6g, CO₃²⁻ %.6g мг/л. Баланс Ca: остаток %+.3g моль/л; баланс C: остаток %+.3g моль/л. Общий C использован как вход, это проверка замыкания расчёта, а не независимая сверка. Щёлочность %.6g мг/л как CaCO₃; I %.8g моль/л; итераций %d. Исходные значения сохранены.", ca*40078, s.FreeCalcium*40078, s.CaCO3, s.CaHCO3, s.Forms.CO2*44010, s.Forms.HCO3*61016, s.Forms.CO3*60010, s.TotalCalcium-ca, s.TotalCarbon-c, s.Forms.Alkalinity*50043.45, s.IonicStrength, s.Iterations)
}
func laboratoryCarbonComplexResults(v map[string]string) ([]laboratorySaturationResult, float64, []string) {
	s, e := laboratoryCarbonComplexFromValues(v)
	if e != nil {
		return nil, 0, []string{e.Error()}
	}
	w := []string{"Сокращённая модель при 25 °C: только CaCO₃(aq) и CaHCO₃⁺, Дэвис, активность воды и нейтральные коэффициенты 1. Комплексы Mg/сульфатов, CaOH⁺, другие кислотные системы, плотность и давление не рассчитаны; это не полное промысловое ядро."}
	if strings.TrimSpace(v["ph_measurement_temp"]) == "" {
		w = append(w, "Температура измерения pH не указана; подтвердите измерение при 25 °C.")
	}
	for _, check := range []struct {
		key, unit string
		calc      float64
	}{{"hco3", "мг HCO₃⁻/л", s.Forms.HCO3 * 61016}, {"alkalinity", "мг/л как CaCO₃", s.Forms.Alkalinity * 50043.45}} {
		m, ok := parseLabNumber(v[check.key])
		if !ok || m < 0 {
			w = append(w, "Независимая сверка "+check.key+" не выполнена: анализ отсутствует или некорректен.")
			continue
		}
		den := math.Max(math.Abs(check.calc), math.Abs(m))
		if den > 0 && math.Abs(check.calc-m)/den > 0.1 {
			w = append(w, fmt.Sprintf("Независимая сверка %s: расчёт %.6g, анализ %.6g %s; разность выше рабочего уровня 10 %%, не нормативного.", check.key, check.calc, m, check.unit))
		}
	}
	if b, ok := laboratoryScaleModelIonBalance(v, "carbonate_complex"); ok && math.Abs(b) > 5 {
		w = append(w, fmt.Sprintf("Невязка зарядов %+.2f %% выше рабочего контроля 5 %%.", b))
	}
	k, _ := laboratoryTemperatureLogK("Кальцит", 25)
	return []laboratorySaturationResult{{Mineral: "Кальцит", Formula: "CaCO₃", SI: s.SI, Ratio: math.Pow(10, s.SI), TemperatureC: 25, LogK: k}}, s.IonicStrength, w
}
