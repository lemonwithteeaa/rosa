//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

// USGS PHREEQC 3.8.6-17100 phreeqc.dat, CO3-2 + H+ = HCO3-.
// The analytical expression supersedes the rounded 25 C log_k.
func laboratoryCarbonateAssociationLogK(temperatureC float64) (float64, error) {
	if _, err := laboratoryTemperatureLogK("Кальцит", temperatureC); err != nil {
		return 0, err
	}
	t := temperatureC + 273.15
	return 107.8871 + 0.03252849*t - 5151.79/t - 38.92561*math.Log10(t) + 563713.9/(t*t), nil
}

func laboratoryCarbonatePHResults(values map[string]string) ([]laboratorySaturationResult, float64, []string) {
	strength, _ := laboratoryScaleIonicStrength(values)
	temp, ok := parseLabNumber(values["sample_temp"])
	if !ok {
		return nil, strength, []string{"Температура пробы обязательна для карбонатного скрининга."}
	}
	association, err := laboratoryCarbonateAssociationLogK(temp)
	if err != nil {
		return nil, strength, []string{err.Error()}
	}
	measuredTemp, hasMeasuredTemp := parseLabNumber(values["ph_measurement_temp"])
	if strings.TrimSpace(values["ph_measurement_temp"]) != "" && (!hasMeasuredTemp || math.Abs(measuredTemp-temp) > 0.5) {
		return nil, strength, []string{"Для карбонатного скрининга температура измерения pH должна совпадать с температурой пробы в пределах 0,5 °C; pH автоматически не пересчитывается."}
	}
	ph, phOK := parseLabNumber(values["ph"])
	ca, caOK := scaleIonMolar(values, "ca", 40.078)
	hco3, hcOK := scaleIonMolar(values, "hco3", 61.016)
	if !phOK || ph < 0 || ph > 14 || !caOK || ca <= 0 || !hcOK || hco3 <= 0 {
		return nil, strength, []string{"Для карбонатного скрининга нужны pH 0–14, положительные Ca²⁺ и HCO₃⁻ одной пробы."}
	}
	logK, _ := laboratoryTemperatureLogK("Кальцит", temp)
	// a(CO3) = a(HCO3) / (K_association * a(H)); gamma=1 screening.
	si := math.Log10(ca) + math.Log10(hco3) + ph - association - logK
	warnings := []string{"Расчётный CO₃²⁻ получен из pH и HCO₃⁻; измеренный CO₃²⁻ и исходные поля не изменены. Концентрации приняты за активности свободных ионов; комплексы и давление не учтены."}
	if !hasMeasuredTemp {
		warnings = append(warnings, "Температура измерения pH не указана: предполагается совпадение с температурой пробы; подтвердите условие измерения.")
	}
	if strength > 0.01 {
		warnings = append(warnings, "Ионная сила выше 0,01 моль/л: концентрационный карбонатный скрининг не применять как прогноз для минерализованной воды; нужен проверенный расчёт активностей.")
	}
	warnings = append(warnings, laboratoryCarbonMeasuredChecks(values, func() laboratoryCarbonForms { f, _ := laboratoryCarbonFormsFromPH(temp, ph, hco3); return f }())...)
	return []laboratorySaturationResult{{Mineral: "Кальцит", Formula: "CaCO₃", SI: si, Ratio: math.Pow(10, si), TemperatureC: temp, LogK: logK}}, strength, warnings
}

func laboratoryCarbonatePHBasis(values map[string]string) string {
	temp, _ := parseLabNumber(values["sample_temp"])
	ph, _ := parseLabNumber(values["ph"])
	hco3, _ := scaleIonMolar(values, "hco3", 61.016)
	association, _ := laboratoryCarbonateAssociationLogK(temp)
	co3 := hco3 * math.Pow(10, ph-association)
	return fmt.Sprintf("Расчётный CO₃²⁻: %.6g мг/л; pH %.3f; T %.2f °C; log K ассоциации %.8f. Лабораторный CO₃²⁻ не заменён.", co3*60.01*1000, ph, temp, association)
}
