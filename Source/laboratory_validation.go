//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
)

type LabValidationState int

const (
	labValueEmpty LabValidationState = iota
	labValueValid
	labValueWarning
	labValueError
)

type LabRangeRule struct {
	PhysicalMin, PhysicalMax   float64
	PlausibleMin, PlausibleMax float64
	Expected                   string
}

type LabValidationResult struct {
	State   LabValidationState
	Message string
}

var labFieldValidation = map[syscall.Handle]LabValidationResult{}

func labRangeRule(group, key, unit string) (LabRangeRule, bool) {
	key = strings.TrimPrefix(key, "mix2_")
	if key == "mix_ph" {
		key = "ph"
	}
	if key == "mix_temp" || key == "mix_ph_temp" {
		key = "sample_temp"
	}
	textualKeys := map[string]bool{
		"material_hardness_base": true, "material_hardness_weld": true,
		"hydrogen_permeation": true, "ssc_test": true, "hic_test": true,
		"working_stresses": true, "material_zone_conclusion": true, "plate_comment": true,
	}
	textual := textualKeys[key] || unit == "текст" || unit == "результат" || strings.Contains(unit, "качествен")
	if textual {
		return LabRangeRule{}, false
	}
	specific := map[string]LabRangeRule{
		"ph":                         {0, 14, 3, 10, "0–14 ед.; обычно 3–10 ед."},
		"watercut":                   {0, 100, 0, 100, "0–100 %"},
		"corrosion_rate":             {0, 100, 0, 10, "0–100 мм/год; инженерная проверка выше 10 мм/год"},
		"corrosion_rate_without":     {0, 100, 0, 10, "0–100 мм/год; инженерная проверка выше 10 мм/год"},
		"corrosion_rate_with":        {0, 100, 0, 10, "0–100 мм/год; инженерная проверка выше 10 мм/год"},
		"corrosion_allowance":        {0, 1000, 0, 100, "0–1000 мм; инженерная проверка выше 100 мм"},
		"plate_area":                 {0, 1e7, 0.01, 1e5, "более 0 см²; инженерная проверка выше 100 000 см²"},
		"plate_material_density":     {0, 100, 0.1, 30, "более 0–100 г/см³; обычно 0,1–30 г/см³"},
		"material_yield_strength":    {0, 5000, 50, 2500, "0–5000 МПа; обычно 50–2500 МПа"},
		"wall_thickness":             {0, 1000, 0.1, 200, "0–1000 мм; обычно 0,1–200 мм"},
		"minimum_wall_thickness":     {0, 1000, 0.1, 200, "0–1000 мм; обычно 0,1–200 мм"},
		"pit_depth":                  {0, 1000, 0, 200, "0–1000 мм; инженерная проверка выше 200 мм"},
		"pit_period_years":           {0, 500, 0.01, 100, "более 0–500 лет; обычно 0,01–100 лет"},
		"corrosion_planning_years":   {0, 500, 0.01, 100, "более 0–500 лет; обычно 0,01–100 лет"},
		"mixture_density":            {100, 3000, 500, 1600, "100–3000 кг/м³; обычно 500–1600 кг/м³"},
		"density":                    {100, 2000, 600, 1200, "100–2000 кг/м³; обычно 600–1200 кг/м³"},
		"flow_velocity":              {0, 100, 0, 20, "0–100 м/с; обычно до 20 м/с"},
		"plate_exposure_days":        {0, 36500, 1, 3650, "0–36500 сут; обычно 1–3650 сут"},
		"planning_horizon":           {1, 3650, 30, 1095, "1–3650 сут; обычно 30–1095 сут"},
		"pour_point":                 {-100, 200, -60, 100, "−100…200 °C; обычно −60…100 °C"},
		"wax_onset":                  {-100, 200, -60, 120, "−100…200 °C; обычно −60…120 °C"},
		"hydrate_temp":               {-100, 100, -40, 40, "−100…100 °C; обычно −40…40 °C"},
		"sample_temp":                {-100, 300, -20, 180, "−100…300 °C; обычно −20…180 °C"},
		"formation_temp":             {-100, 350, 0, 220, "−100…350 °C; обычно 0–220 °C"},
		"pump_temp":                  {-100, 350, -20, 220, "−100…350 °C; обычно −20…220 °C"},
		"wellhead_temp":              {-100, 250, -70, 150, "−100…250 °C; обычно −70…150 °C"},
		"profile_temp_min":           {-100, 350, -70, 220, "−100…350 °C; обычно −70…220 °C"},
		"profile_temp_max":           {-100, 350, -20, 250, "−100…350 °C; обычно −20…250 °C"},
		"inhibitor_trial_dose":       {0, 100000, 0, 10000, "0–100 000 мг/л; выше 10 000 мг/л требует проверки протокола"},
		"inhibitor_trial_temp":       {-100, 350, -20, 220, "−100…350 °C; обычно −20…220 °C"},
		"inhibitor_trial_duration_h": {0, 100000, 0.01, 20000, "0–100 000 ч; обычно 0,01–20 000 ч"},
		"inhibitor_trial_efficiency": {0, 100, 0, 100, "0–100 %"},
	}
	if rule, ok := specific[key]; ok {
		return rule, true
	}
	if strings.Contains(key, "pressure") || key == "pco2" || key == "ph2s" {
		return LabRangeRule{0, 250, 0, 100, "0–250 МПа; обычно до 100 МПа"}, true
	}
	if strings.Contains(unit, "%") {
		return LabRangeRule{0, 100, 0, 100, "0–100 %"}, true
	}
	if strings.Contains(unit, "мг/л") || unit == "мг" {
		return LabRangeRule{0, 1e8, 0, 1e6, "неотрицательное значение; инженерная проверка выше 1 000 000"}, true
	}
	if unit == "г/л" || unit == "г/м³" || unit == "г" {
		return LabRangeRule{0, 1e7, 0, 1e5, "неотрицательное значение; инженерная проверка выше 100 000"}, true
	}
	if unit == "мПа·с" {
		return LabRangeRule{0, 1e7, 0, 1e5, "0–10 000 000 мПа·с; инженерная проверка выше 100 000"}, true
	}
	if unit == "мкм" {
		return LabRangeRule{0, 1e7, 0, 1e5, "0–10 000 000 мкм; инженерная проверка выше 100 000"}, true
	}
	if unit == "м³/т" {
		return LabRangeRule{0, 1e6, 0, 1e4, "0–1 000 000 м³/т; обычно до 10 000 м³/т"}, true
	}
	if strings.Contains(unit, "КОЕ") || strings.Contains(unit, "копий") || unit == "отн. ед." || unit == "ед." {
		return LabRangeRule{0, 1e18, 0, 1e15, "неотрицательное числовое значение"}, true
	}
	return LabRangeRule{}, false
}

func validateLaboratoryValue(group string, field LabFieldDef, raw string) LabValidationResult {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return LabValidationResult{State: labValueEmpty}
	}
	if field.Key == "material_hardness_base" || field.Key == "material_hardness_weld" {
		upper := strings.ToUpper(raw)
		if _, numericOnly := parseLabNumber(raw); numericOnly && !strings.Contains(upper, "HV") && !strings.Contains(upper, "HRC") && !strings.Contains(upper, "HB") {
			return LabValidationResult{State: labValueWarning, Message: fmt.Sprintf("%s: число %s неоднозначно без шкалы. Укажите, например, «%s HV» или «%s HRC»; до уточнения значение не подтверждает оценку H₂S-растрескивания", field.Label, raw, raw, raw)}
		}
	}
	rule, numeric := labRangeRule(group, field.Key, field.Unit)
	if !numeric {
		return LabValidationResult{State: labValueValid}
	}
	value, ok := parseLabNumber(raw)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return LabValidationResult{State: labValueError, Message: fmt.Sprintf("%s: требуется число, единица — %s", field.Label, field.Unit)}
	}
	if value < rule.PhysicalMin || value > rule.PhysicalMax {
		return LabValidationResult{State: labValueError, Message: fmt.Sprintf("%s: значение выглядит неадекватным. Скорректируйте введённые данные. Ожидаемый диапазон: %s", field.Label, rule.Expected)}
	}
	if value < rule.PlausibleMin || value > rule.PlausibleMax {
		return LabValidationResult{State: labValueWarning, Message: fmt.Sprintf("%s: значение физически возможно, но выходит за инженерный диапазон (%s). Проверьте источник и единицу", field.Label, rule.Expected)}
	}
	return LabValidationResult{State: labValueValid}
}

func validateLaboratoryGroup(group LabGroupDef, values map[string]string) (map[string]string, []string, []string) {
	clean := make(map[string]string, len(values))
	var warnings, errors []string
	for _, field := range group.Fields {
		if laboratoryCalculatedField(group.Code, field.Key) {
			clean[field.Key] = strings.TrimSpace(values[field.Key])
			continue
		}
		value := strings.TrimSpace(values[field.Key])
		result := validateLaboratoryValue(group.Code, field, value)
		switch result.State {
		case labValueError:
			errors = append(errors, result.Message)
		case labValueWarning:
			warnings = append(warnings, result.Message)
			clean[field.Key] = value
		default:
			clean[field.Key] = value
		}
	}
	if a, okA := parseLabNumber(clean["size_d50"]); okA {
		if b, okB := parseLabNumber(clean["size_max"]); okB && a > b {
			delete(clean, "size_d50")
			delete(clean, "size_max")
			errors = append(errors, "Размер частиц: d50 не может превышать максимальный размер; оба значения исключены из расчёта")
		}
	}
	if a, okA := parseLabNumber(clean["profile_temp_min"]); okA {
		if b, okB := parseLabNumber(clean["profile_temp_max"]); okB && a > b {
			delete(clean, "profile_temp_min")
			delete(clean, "profile_temp_max")
			errors = append(errors, "Температурный профиль: минимальная температура превышает максимальную; оба значения исключены из расчёта")
		}
	}
	if a, okA := parseLabNumber(clean["profile_pressure_min"]); okA {
		if b, okB := parseLabNumber(clean["profile_pressure_max"]); okB && a > b {
			delete(clean, "profile_pressure_min")
			delete(clean, "profile_pressure_max")
			errors = append(errors, "Профиль давления: минимальное давление превышает максимальное; оба значения исключены из расчёта")
		}
	}
	if total, okTotal := parseLabNumber(clean["gas_absolute_pressure"]); okTotal && total > 0 {
		pco2, okCO2 := parseLabNumber(clean["pco2"])
		ph2s, okH2S := parseLabNumber(clean["ph2s"])
		if (okCO2 && pco2 > total) || (okH2S && ph2s > total) || (okCO2 && okH2S && pco2+ph2s > total) {
			delete(clean, "pco2")
			delete(clean, "ph2s")
			errors = append(errors, "Газовая фаза: парциальные давления CO₂/H₂S не могут превышать полное абсолютное давление; парциальные давления исключены из расчёта")
		}
	}
	if initial, okInitial := parseLabNumber(clean["plate_initial_mass"]); okInitial {
		if final, okFinal := parseLabNumber(clean["plate_final_mass"]); okFinal && final > initial {
			delete(clean, "plate_initial_mass")
			delete(clean, "plate_final_mass")
			errors = append(errors, "Коррозионная пластинка: конечная масса превышает начальную; обе массы исключены из расчёта")
		}
	}
	if without, okWithout := parseLabNumber(clean["corrosion_rate_without"]); okWithout {
		if with, okWith := parseLabNumber(clean["corrosion_rate_with"]); okWith && with > without {
			warnings = append(warnings, "Скорость коррозии с ингибитором выше скорости без ингибитора; проверьте режимы испытаний и эффективность защиты")
		}
	}
	for _, keys := range [][]string{{"paraffins", "resins", "asphaltenes"}, {"carbonates", "sulfates", "chloride_salts", "corrosion_products"}} {
		sum, count := 0.0, 0
		for _, key := range keys {
			if value, ok := parseLabNumber(clean[key]); ok {
				sum += value
				count++
			}
		}
		if count >= 2 && sum > 100.000001 {
			for _, key := range keys {
				delete(clean, key)
			}
			errors = append(errors, fmt.Sprintf("Сумма сопоставимых долей состава равна %g %% и превышает 100 %%; значения исключены из расчёта", sum))
		}
	}
	return clean, warnings, errors
}

func validateLaboratoryPassGroups(groups map[string]map[string]string) (warnings, errors []string) {
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		_, groupWarnings, groupErrors := validateLaboratoryGroup(group, groups[group.Code])
		for _, warning := range groupWarnings {
			warnings = append(warnings, group.Name+": "+warning)
		}
		for _, problem := range groupErrors {
			errors = append(errors, group.Name+": "+problem)
		}
	}
	return warnings, errors
}

func validateCurrentLaboratoryPage() (warnings, errors []string) {
	g := currentLabGroup()
	if g.Code == "overall" {
		return nil, nil
	}
	start := labFieldsPage * 10
	units := currentLaboratoryUnits()
	visibleHandles := map[string]syscall.Handle{}
	for i := 0; i < 10; i++ {
		result := LabValidationResult{State: labValueEmpty}
		if start+i < len(g.Fields) {
			field := g.Fields[start+i]
			result = validateLaboratoryValue(g.Code, field, laboratoryCanonicalValue(field, getText(labFieldEdits[i]), units[field.Key]))
			visibleHandles[g.Fields[start+i].Key] = labFieldEdits[i]
		}
		labFieldValidation[labFieldEdits[i]] = result
		pInvalidateRect.Call(uintptr(labFieldEdits[i]), 0, 1)
		if result.State == labValueWarning {
			warnings = append(warnings, result.Message)
		} else if result.State == labValueError {
			errors = append(errors, result.Message)
		}
	}
	_, groupWarnings, groupErrors := validateLaboratoryGroup(g, canonicalizeLaboratoryValues(g, currentLabValues(), units))
	for _, warning := range groupWarnings {
		found := false
		for _, existing := range warnings {
			if existing == warning {
				found = true
				break
			}
		}
		if !found {
			warnings = append(warnings, warning)
		}
	}
	mark := func(keys []string, message string) {
		for _, key := range keys {
			if handle := visibleHandles[key]; handle != 0 {
				labFieldValidation[handle] = LabValidationResult{State: labValueError, Message: message}
				pInvalidateRect.Call(uintptr(handle), 0, 1)
			}
		}
	}
	for _, problem := range groupErrors {
		switch {
		case strings.HasPrefix(problem, "Размер частиц"):
			mark([]string{"size_d50", "size_max"}, problem)
			errors = append(errors, problem)
		case strings.HasPrefix(problem, "Температурный профиль"):
			mark([]string{"profile_temp_min", "profile_temp_max"}, problem)
			errors = append(errors, problem)
		case strings.HasPrefix(problem, "Сумма сопоставимых долей"):
			mark([]string{"paraffins", "resins", "asphaltenes", "carbonates", "sulfates", "chloride_salts", "corrosion_products"}, problem)
			errors = append(errors, problem)
		case strings.HasPrefix(problem, "Коррозионная пластинка"):
			mark([]string{"plate_initial_mass", "plate_final_mass"}, problem)
			errors = append(errors, problem)
		}
	}
	return warnings, errors
}

func laboratoryEditValidation(handle syscall.Handle) LabValidationResult {
	return labFieldValidation[handle]
}

func laboratoryValidationText(warnings, errors []string) string {
	var parts []string
	if len(errors) > 0 {
		parts = append(parts, "ОШИБКИ — ЗНАЧЕНИЯ НЕ ИСПОЛЬЗОВАНЫ:\n• "+strings.Join(errors, "\n• "))
	}
	if len(warnings) > 0 {
		parts = append(parts, "ПРЕДУПРЕЖДЕНИЯ — ПРОВЕРЬТЕ ИСТОЧНИК И ЕДИНИЦЫ:\n• "+strings.Join(warnings, "\n• "))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}
