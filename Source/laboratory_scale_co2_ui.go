//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

var laboratoryCO2EquilibriumKeys = []string{
	"co2_eq_mode", "co2_eq_pressure_atm", "co2_eq_water_kg", "co2_eq_na_molal", "co2_eq_cl_molal",
	"co2_eq_reservoir_mol", "co2_eq_gas_volume_l", "co2_eq_gas_percent", "co2_eq_temp",
}

func laboratoryIsCO2EquilibriumField(key string) bool {
	for _, candidate := range laboratoryCO2EquilibriumKeys {
		if key == candidate {
			return true
		}
	}
	return false
}

func laboratoryCO2EquilibriumMode(v map[string]string) string {
	raw := strings.ToLower(strings.TrimSpace(v["co2_eq_mode"]))
	raw = strings.ReplaceAll(raw, "ё", "е")
	switch raw {
	case "резервуар", "внешний резервуар", "external_reservoir", "external", "1":
		return "external_reservoir"
	case "замкнутый", "замкнутый объем", "замкнутый объём", "closed_volume", "closed", "2":
		return "closed_volume"
	default:
		return ""
	}
}

func laboratoryCO2EquilibriumRequiredKey(key string) bool {
	switch key {
	case "co2_eq_mode", "co2_eq_pressure_atm", "co2_eq_water_kg", "co2_eq_na_molal", "co2_eq_cl_molal", "co2_eq_temp":
		return true
	}
	return false
}

func laboratoryCO2EquilibriumRequiredKeys(v map[string]string) map[string]bool {
	required := map[string]bool{}
	for _, key := range laboratoryCO2EquilibriumKeys {
		if laboratoryCO2EquilibriumRequiredKey(key) {
			required[key] = true
		}
	}
	switch laboratoryCO2EquilibriumMode(v) {
	case "external_reservoir":
		required["co2_eq_reservoir_mol"] = true
	case "closed_volume":
		required["co2_eq_gas_volume_l"] = true
		required["co2_eq_gas_percent"] = true
	}
	return required
}

func laboratoryCO2EquilibriumCoverage(group LabGroupDef, v map[string]string) (int, int, int, int) {
	required := laboratoryCO2EquilibriumRequiredKeys(v)
	filled, optional, requiredTotal, optionalTotal := 0, 0, 0, 0
	for _, field := range group.Fields {
		if !laboratoryInputFieldEligible(group.Code, field.Key) {
			continue
		}
		if required[field.Key] {
			requiredTotal++
			if strings.TrimSpace(v[field.Key]) != "" {
				filled++
			}
		} else {
			optionalTotal++
			if strings.TrimSpace(v[field.Key]) != "" {
				optional++
			}
		}
	}
	return filled, requiredTotal, optional, optionalTotal
}

func laboratoryCO2EquilibriumCoverageText(group LabGroupDef, v map[string]string) string {
	group = laboratoryScaleInputGroup(group, "co2_gas_water")
	filled, total, optional, optionalTotal := laboratoryCO2EquilibriumCoverage(group, v)
	required := laboratoryCO2EquilibriumRequiredKeys(v)
	var missing []string
	for _, field := range group.Fields {
		if required[field.Key] && strings.TrimSpace(v[field.Key]) == "" {
			missing = append(missing, field.Label)
		}
	}
	text := fmt.Sprintf("Обязательные поля CO₂-равновесия: %d из %d. Дополнительные / другой постановки: %d из %d.", filled, total, optional, optionalTotal)
	if laboratoryCO2EquilibriumMode(v) == "" {
		text += " Укажите постановку словами «резервуар» или «замкнутый»."
	}
	if len(missing) > 0 {
		text += " Нужно заполнить: " + strings.Join(missing, ", ") + "."
	}
	return text
}

func laboratoryCO2EquilibriumInputFromValues(v map[string]string) (laboratoryCO2AqueousInput, error) {
	var out laboratoryCO2AqueousInput
	mode := laboratoryCO2EquilibriumMode(v)
	if mode == "" {
		return out, fmt.Errorf("укажите постановку CO₂: «резервуар» или «замкнутый»")
	}
	read := func(key, label string) (float64, error) {
		n, ok := parseLabNumber(v[key])
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, fmt.Errorf("поле «%s» должно содержать число", label)
		}
		return n, nil
	}
	pressure, err := read("co2_eq_pressure_atm", "Давление CO₂")
	if err != nil {
		return out, err
	}
	water, err := read("co2_eq_water_kg", "Масса воды-растворителя")
	if err != nil {
		return out, err
	}
	na, err := read("co2_eq_na_molal", "Фоновый Na")
	if err != nil {
		return out, err
	}
	cl, err := read("co2_eq_cl_molal", "Фоновый Cl")
	if err != nil {
		return out, err
	}
	temp, err := read("co2_eq_temp", "Температура CO₂-равновесия")
	if err != nil {
		return out, err
	}
	gas := laboratoryCO2GasInput{Mode: mode, Pressure: pressure, PressureUnit: "атм", AbsolutePressure: true, TemperatureC: temp, WaterKG: water}
	if mode == "external_reservoir" {
		gas.ReservoirMoles, err = read("co2_eq_reservoir_mol", "Запас CO₂ резервуара")
		if err != nil {
			return out, err
		}
	} else {
		gas.GasVolumeL, err = read("co2_eq_gas_volume_l", "Замкнутый объём газа")
		if err != nil {
			return out, err
		}
		gas.CO2Percent, err = read("co2_eq_gas_percent", "CO₂ в замкнутом газе")
		if err != nil {
			return out, err
		}
	}
	out = laboratoryCO2AqueousInput{Gas: gas, SodiumMolal: na, ChlorideMolal: cl}
	return out, nil
}

func laboratoryCO2EquilibriumFromValues(v map[string]string) (laboratoryCO2Equilibrium, error) {
	input, err := laboratoryCO2EquilibriumInputFromValues(v)
	if err != nil {
		return laboratoryCO2Equilibrium{}, err
	}
	return laboratoryCO2SolveReduced(input)
}

func laboratoryCO2EquilibriumStatusLabel(status string) string {
	switch status {
	case "equilibrium":
		return "РАВНОВЕСИЕ ДОСТИГНУТО"
	case "stock_exhausted":
		return "ЗАПАС CO₂ ИСЧЕРПАН ДО ЦЕЛЕВОГО РАВНОВЕСИЯ"
	default:
		return "НЕ РАССЧИТАНО"
	}
}

func laboratoryCO2EquilibriumResultText(v map[string]string) string {
	result, err := laboratoryCO2EquilibriumFromValues(v)
	if err != nil {
		return "Статус расчёта: НЕ РАССЧИТАНО. Причина: " + err.Error() + "."
	}
	lines := []string{
		"Статус расчёта: " + laboratoryCO2EquilibriumStatusLabel(result.Status) + ".",
		fmt.Sprintf("Конечный pH: %.4f.", result.PH),
		fmt.Sprintf("Ионная сила: %.6g моль/кг воды; активность воды: %.6f.", result.IonicStrength, result.WaterActivity),
		fmt.Sprintf("Растворённый углерод: %.8g моль C; CO₂(aq)=%.8g, HCO₃⁻=%.8g, CO₃²⁻=%.8g моль/кг воды.", result.CarbonWaterMol, result.CO2Molal, result.HCO3Molal, result.CO3Molal),
	}
	if result.Status == "equilibrium" {
		lines = append(lines, fmt.Sprintf("Конечное давление CO₂: %.8g атм abs; фугитивность: %.8g атм; остаток газа: %.8g моль.", result.PressureATM, result.FugacityATM, result.GasMoles))
	} else {
		lines = append(lines, "Заданное давление не считается достигнутым: конечного газового резервуара недостаточно, весь доступный углерод передан водной задаче.")
	}
	return strings.Join(lines, "\r\n")
}

func laboratoryCO2EquilibriumPanelText(v map[string]string) string {
	lines := []string{"МЕТОД", "CO₂ ↔ вода: проверенный учебный режим"}
	result, err := laboratoryCO2EquilibriumFromValues(v)
	if err != nil {
		lines = append(lines, "СТАТУС", "НЕ РАССЧИТАНО", "ОГРАНИЧЕНИЕ", err.Error(), "УСЛОВИЯ РАСЧЁТА", "Только 25 °C; P≤1 атм; чистый CO₂; Na=Cl≤0,1 моль/кг воды.")
		return strings.Join(lines, "\r\n")
	}
	lines = append(lines, "СТАТУС", laboratoryCO2EquilibriumStatusLabel(result.Status), "УСЛОВИЯ РАСЧЁТА", "25 °C; чистый CO₂; электронейтральный NaCl-фон; сокращённая химия phreeqc.dat.", "ДОСТУПНЫЙ РЕЗУЛЬТАТ", fmt.Sprintf("pH %.4f; C(вода) %.6g моль; I %.5g моль/кг воды.", result.PH, result.CarbonWaterMol, result.IonicStrength))
	if result.Status == "equilibrium" {
		lines = append(lines, fmt.Sprintf("Pкон %.6g атм abs; fCO₂ %.6g атм; газ %.6g моль.", result.PressureATM, result.FugacityATM, result.GasMoles))
	} else {
		lines = append(lines, "Заданное давление не достигнуто: конечный запас CO₂ исчерпан.")
	}
	lines = append(lines, "ОГРАНИЧЕНИЕ", "Без Ca/Mg/Ba/Sr, минералов, осаждения, кинетики, других кислотных систем и T/P-траектории. Это не прогноз солеотложения.")
	return strings.Join(lines, "\r\n")
}

func laboratoryCO2EquilibriumDecisionText(v map[string]string) string {
	result, err := laboratoryCO2EquilibriumFromValues(v)
	lines := []string{"ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО CO₂-РАВНОВЕСИЮ"}
	if err != nil {
		return strings.Join(append(lines,
			"Статус: НЕ ОПРЕДЕЛЁН — расчёт не выполнен.",
			"Причина: "+err.Error()+".",
			"Следующее действие: исправить постановку или входные данные; не переносить этот режим на промысловую T/P-траекторию."), "\n")
	}
	if result.Status == "stock_exhausted" {
		lines = append(lines,
			"Статус: запас CO₂ исчерпан раньше достижения заданного газового равновесия.",
			fmt.Sprintf("Расчётное состояние воды после расходования доступного углерода: pH %.4f; C(вода) %.6g моль.", result.PH, result.CarbonWaterMol),
			"Заданное пользователем давление не выдаётся за достигнутое.")
	} else {
		lines = append(lines,
			"Статус: связанное равновесие газа и воды достигнуто в проверенной учебной области.",
			fmt.Sprintf("Конечное состояние: pH %.4f; PCO₂ %.6g атм abs; C(вода) %.6g моль.", result.PH, result.PressureATM, result.CarbonWaterMol))
	}
	lines = append(lines,
		"Что можно заключить: рассчитано только распределение CO₂/карбонатных форм в сокращённой NaCl-системе.",
		"Что нельзя заключить: вероятность выпадения CaCO₃/BaSO₄/SrSO₄, масса осадка, коррозия, МРП и поведение по стволу не определяются этим режимом.")
	return strings.Join(lines, "\n")
}
