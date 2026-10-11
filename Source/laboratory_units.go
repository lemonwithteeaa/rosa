//go:build windows

package main

import (
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func laboratoryUnitOptions(field LabFieldDef) []string {
	options := map[string][]string{
		"мг/л":     {"мг/л", "г/л", "мкг/л"},
		"г/л":      {"г/л", "мг/л"},
		"МПа":      {"МПа", "бар", "кПа"},
		"МПа abs":  {"МПа abs", "бар abs", "кПа abs"},
		"°C":       {"°C", "K"},
		"мм/год":   {"мм/год", "мкм/год"},
		"мПа·с":    {"мПа·с", "сП"},
		"кг/м³":    {"кг/м³", "г/см³"},
		"КОЕ/мл":   {"КОЕ/мл", "КОЕ/л"},
		"копий/мл": {"копий/мл", "копий/л"},
		"мм":       {"мм", "мкм"},
	}
	if result := options[field.Unit]; len(result) > 0 {
		return result
	}
	return []string{field.Unit}
}

func setLaboratoryUnitCombo(combo syscall.Handle, options []string, selected string) {
	pSendMessageW.Call(uintptr(combo), CB_RESETCONTENT, 0, 0)
	selectedIndex := 0
	for index, option := range options {
		pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(option))))
		if strings.EqualFold(strings.TrimSpace(option), strings.TrimSpace(selected)) {
			selectedIndex = index
		}
	}
	pSendMessageW.Call(uintptr(combo), CB_SETCURSEL, uintptr(selectedIndex), 0)
	// The native collapsed combo height must fit the common font before the next row.
	pSendMessageW.Call(uintptr(combo), 0x0153, ^uintptr(0), 24) // CB_SETITEMHEIGHT, selection
	pSendMessageW.Call(uintptr(combo), 0x0153, 0, 24)           // list items
}

func currentLaboratoryUnits() map[string]string {
	group := currentLabGroup()
	units := map[string]string{}
	for key, unit := range labUnitDrafts[group.Code] {
		units[key] = unit
	}
	start := labFieldsPage * 10
	for index := 0; index < 10 && start+index < len(group.Fields); index++ {
		field := group.Fields[start+index]
		unit := strings.TrimSpace(getText(labUnitLabels[index]))
		if unit == "" {
			unit = field.Unit
		}
		units[field.Key] = unit
	}
	return units
}

func laboratoryCanonicalValue(field LabFieldDef, raw, unit string) string {
	value, ok := parseLabNumber(raw)
	if !ok || strings.TrimSpace(unit) == "" || strings.EqualFold(strings.TrimSpace(unit), field.Unit) {
		return raw
	}
	switch field.Unit + "|" + strings.TrimSpace(unit) {
	case "мг/л|г/л":
		value *= 1000
	case "мг/л|мкг/л":
		value /= 1000
	case "г/л|мг/л":
		value /= 1000
	case "МПа|бар", "МПа abs|бар abs":
		value *= 0.1
	case "МПа|кПа", "МПа abs|кПа abs":
		value /= 1000
	case "°C|K":
		value -= 273.15
	case "мм/год|мкм/год", "мм|мкм":
		value /= 1000
	case "кг/м³|г/см³":
		value *= 1000
	case "КОЕ/мл|КОЕ/л", "копий/мл|копий/л":
		value /= 1000
	case "мПа·с|сП":
		// Numerically equivalent.
	default:
		return raw
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func laboratoryDisplayValue(field LabFieldDef, canonical, unit string) string {
	value, ok := parseLabNumber(canonical)
	unit = strings.TrimSpace(unit)
	if !ok || unit == "" || strings.EqualFold(unit, field.Unit) {
		return canonical
	}
	switch field.Unit + "|" + unit {
	case "мг/л|г/л":
		value /= 1000
	case "мг/л|мкг/л":
		value *= 1000
	case "г/л|мг/л":
		value *= 1000
	case "МПа|бар", "МПа abs|бар abs":
		value /= 0.1
	case "МПа|кПа", "МПа abs|кПа abs":
		value *= 1000
	case "°C|K":
		value += 273.15
	case "мм/год|мкм/год", "мм|мкм":
		value *= 1000
	case "кг/м³|г/см³":
		value /= 1000
	case "КОЕ/мл|КОЕ/л", "копий/мл|копий/л":
		value *= 1000
	case "мПа·с|сП":
		// Numerically equivalent.
	default:
		return canonical
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func canonicalizeLaboratoryValues(group LabGroupDef, values, units map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	for _, field := range group.Fields {
		if laboratoryCalculatedField(group.Code, field.Key) {
			continue
		}
		result[field.Key] = laboratoryCanonicalValue(field, values[field.Key], units[field.Key])
	}
	for _, field := range group.Fields {
		if laboratoryCalculatedField(group.Code, field.Key) {
			result[field.Key] = laboratoryCalculatedFieldValue(group.Code, field.Key, result)
		}
	}
	return result
}

func handleLaboratoryUnitChange(id int) bool {
	index := id - ID_LAB_UNIT_BASE
	if index < 0 || index >= 10 || labActiveGroup == "overall" {
		return false
	}
	group := currentLabGroup()
	fieldIndex := labFieldsPage*10 + index
	if fieldIndex >= len(group.Fields) {
		return false
	}
	if labUnitDrafts[group.Code] == nil {
		labUnitDrafts[group.Code] = map[string]string{}
	}
	field := group.Fields[fieldIndex]
	unit := strings.TrimSpace(getText(labUnitLabels[index]))
	previousUnit := strings.TrimSpace(labUnitDrafts[group.Code][field.Key])
	if previousUnit == "" {
		previousUnit = field.Unit
	}
	if unit == "" {
		unit = field.Unit
	}
	// RC151: changing only the unit must not silently change the physical value.
	// Convert the current display through the field's canonical unit first.
	if raw := strings.TrimSpace(labDrafts[group.Code][field.Key]); raw != "" && !strings.EqualFold(previousUnit, unit) {
		canonical := laboratoryCanonicalValue(field, raw, previousUnit)
		converted := laboratoryDisplayValue(field, canonical, unit)
		labDrafts[group.Code][field.Key] = converted
		setText(labFieldEdits[index], converted)
	}
	labUnitDrafts[group.Code][field.Key] = unit
	shared := laboratorySharedKey(group.Code, field.Key)
	if shared != "" {
		for _, candidate := range labGroups {
			for _, candidateField := range candidate.Fields {
				if laboratorySharedKey(candidate.Code, candidateField.Key) != shared {
					continue
				}
				if labUnitDrafts[candidate.Code] == nil {
					labUnitDrafts[candidate.Code] = map[string]string{}
				}
				if unitSupported(candidateField, unit) {
					labUnitDrafts[candidate.Code][candidateField.Key] = unit
				}
			}
		}
	}
	if shared != "" {
		synchronizeLaboratorySharedValue(group.Code, field.Key, labDrafts[group.Code][field.Key])
	}
	updateLaboratoryFieldPassport(index)
	updateLaboratoryReadinessPanel()
	updateLaboratoryScaleModelInfo()
	return true
}

func unitSupported(field LabFieldDef, unit string) bool {
	for _, option := range laboratoryUnitOptions(field) {
		if strings.EqualFold(option, unit) {
			return true
		}
	}
	return false
}

func validateLaboratoryPassWithUnits(pass *LabPassRecord) ([]string, []string) {
	canonical := map[string]map[string]string{}
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		canonical[group.Code] = canonicalizeLaboratoryValues(group, pass.Groups[group.Code], pass.Units[group.Code])
	}
	return validateLaboratoryPassGroups(canonical)
}
