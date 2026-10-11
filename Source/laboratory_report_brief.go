//go:build windows

package main

import (
	"fmt"
	"strings"
)

func laboratoryReportBrief(pass LabPassRecord) string {
	var b strings.Builder
	b.WriteString("ЗАКЛЮЧЕНИЕ\n")
	b.WriteString(laboratoryPassOverallShortConclusion(pass) + "\n\n")
	b.WriteString("ОЦЕНКА ПО НАПРАВЛЕНИЯМ\n")
	var missing []string
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		if !laboratoryPassHasGroupData(pass, group.Code) {
			missing = append(missing, group.Name)
			continue
		}
		b.WriteString("• " + laboratoryPassGroupShortConclusion(pass, group.Code) + "\n")
	}
	if len(missing) > 0 {
		b.WriteString("\nНет данных: " + strings.Join(missing, ", ") + ".\n")
	}
	b.WriteString("\nТЕХНОЛОГУ\nСопоставить результаты с температурой и давлением в зоне отбора, режимом скважины, подачей реагентов и последними отказами.\n")
	b.WriteString("\nХИМИКУ\nСверить протокол, единицы и условия отбора. Для солей проверить баланс ионов; для коррозии — CO₂, H₂S, O₂ и железо; для АСПО и мехпримесей — состав отложений и размер частиц.\n")
	b.WriteString("\nОценка предварительная и не заменяет лабораторный протокол и производственную проверку.\n")
	return b.String()
}

func laboratoryReportSourceRows(pass LabPassRecord) string {
	var b strings.Builder
	b.WriteString("ИСХОДНЫЕ ПОКАЗАТЕЛИ\n")
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		values := pass.Groups[group.Code]
		if len(values) == 0 {
			continue
		}
		b.WriteString("\n" + strings.ToUpper(group.Name) + "\n")
		for _, field := range group.Fields {
			if !laboratoryInputFieldEligible(group.Code, field.Key) {
				continue
			}
			v := strings.TrimSpace(values[field.Key])
			if v == "" {
				continue
			}
			unit := strings.TrimSpace(pass.Units[group.Code][field.Key])
			if unit == "" {
				unit = field.Unit
			}
			fmt.Fprintf(&b, "• %s: %s %s\n", field.Label, v, unit)
		}
	}
	return b.String()
}
