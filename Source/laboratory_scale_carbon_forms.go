//go:build windows

package main

import (
	"fmt"
	"math"
)

type laboratoryCarbonForms struct {
	CO2, HCO3, CO3, Hydrogen, Hydroxide, TotalCarbon, Alkalinity float64
}

// Concentration screening: gamma_i=1, a_water=1. No ion pairs or other acid systems.
func laboratoryCarbonFormsFromPH(temp, ph, bicarbonate float64) (laboratoryCarbonForms, error) {
	k, err := laboratoryCarbonateAssociationLogK(temp)
	if err != nil {
		return laboratoryCarbonForms{}, err
	}
	if math.IsNaN(ph) || math.IsInf(ph, 0) || ph < 0 || ph > 14 || math.IsNaN(bicarbonate) || math.IsInf(bicarbonate, 0) || bicarbonate <= 0 {
		return laboratoryCarbonForms{}, fmt.Errorf("Нужны pH 0–14 и положительный гидрокарбонат")
	}
	t := temp + 273.15
	// Unchanged PHREEQC 3.8.6-17100 phreeqc.dat: CO3 + 2H = CO2 + H2O; H2O = OH + H.
	logCO2 := 464.1965 + 0.09344813*t - 26986.16/t - 165.75951*math.Log10(t) + 2248628.9/(t*t)
	logWater := 293.29227 + 0.1360833*t - 10576.913/t - 123.73158*math.Log10(t) - 6.996455e-5*t*t
	f := laboratoryCarbonForms{HCO3: bicarbonate, CO3: bicarbonate * math.Pow(10, ph-k), Hydrogen: math.Pow(10, -ph), Hydroxide: math.Pow(10, logWater+ph)}
	f.CO2 = f.CO3 * math.Pow(10, logCO2-2*ph)
	f.TotalCarbon = f.CO2 + f.HCO3 + f.CO3
	f.Alkalinity = f.HCO3 + 2*f.CO3 + f.Hydroxide - f.Hydrogen
	for _, v := range []float64{f.CO2, f.CO3, f.TotalCarbon, f.Alkalinity} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return laboratoryCarbonForms{}, fmt.Errorf("Переполнение расчёта форм углерода; сверить исходные данные")
		}
	}
	return f, nil
}
func laboratoryCarbonFormsText(values map[string]string) string {
	temp, _ := parseLabNumber(values["sample_temp"])
	ph, _ := parseLabNumber(values["ph"])
	hc, _ := scaleIonMolar(values, "hco3", 61.016)
	f, err := laboratoryCarbonFormsFromPH(temp, ph, hc)
	if err != nil {
		return "Формы углерода не рассчитаны: " + err.Error()
	}
	return laboratoryCarbonIndependentBalanceText(values, f) + "\r\n" + fmt.Sprintf("Расчётные свободные формы: CO₂ %.6g мг/л; HCO₃⁻ %.6g мг/л; CO₃²⁻ %.6g мг/л. Сумма трёх форм: %.6g мг C/л; карбонатная щёлочность с учётом H⁺/OH⁻: %.6g мг/л как CaCO₃. Это концентрационный скрининг без комплексов и других кислотных систем; лабораторные значения не заменены.", f.CO2*44.01*1000, f.HCO3*61.016*1000, f.CO3*60.01*1000, f.TotalCarbon*12.0111*1000, f.Alkalinity*50.04345*1000)
}
func laboratoryCarbonAlkalinityWarnings(values map[string]string) []string {
	temp, _ := parseLabNumber(values["sample_temp"])
	ph, _ := parseLabNumber(values["ph"])
	hc, _ := scaleIonMolar(values, "hco3", 61.016)
	f, err := laboratoryCarbonFormsFromPH(temp, ph, hc)
	if err != nil {
		return []string{err.Error()}
	}
	measured, ok := parseLabNumber(values["alkalinity"])
	if !ok || measured < 0 {
		return []string{"Лабораторная общая щёлочность отсутствует или некорректна: независимая сверка карбонатной системы не выполнена."}
	}
	calculated := f.Alkalinity * 50.04345 * 1000
	denominator := math.Max(math.Abs(calculated), math.Abs(measured))
	if denominator > 0 && math.Abs(calculated-measured)/denominator > 0.1 {
		return []string{fmt.Sprintf("Расчётная карбонатная щёлочность %.6g и лабораторная общая %.6g мг/л как CaCO₃ расходятся более чем на рабочий контрольный уровень 10 %%; сверить pH, формы ионов и вклад других кислотных систем. Не считать этот уровень нормативным.", calculated, measured)}
	}
	return nil
}
