//go:build windows

package main

import "fmt"

// laboratorySixDirectionQAIssues is a structural audit for the six laboratory
// directions. It intentionally checks wiring, not scientific validity: every
// visible direction must have real fields, baseline inputs and an assessment
// path, so a polished UI cannot silently exist without working logic behind it.
func laboratorySixDirectionQAIssues() []string {
	expected := []string{"corrosion", "scale", "aspo", "solids", "microbiology", "gases"}
	groups := map[string]LabGroupDef{}
	for _, group := range labGroups {
		if group.Code != "overall" {
			groups[group.Code] = group
		}
	}
	var issues []string
	for _, code := range expected {
		group, ok := groups[code]
		if !ok {
			issues = append(issues, "нет направления "+code)
			continue
		}
		if group.Name == "" || len(group.Fields) == 0 {
			issues = append(issues, fmt.Sprintf("%s: пустое имя или нет полей", code))
			continue
		}
		fieldKeys := map[string]bool{}
		for _, field := range group.Fields {
			if field.Key == "" || field.Label == "" || field.Unit == "" {
				issues = append(issues, fmt.Sprintf("%s: неполное описание поля %q", code, field.Key))
			}
			if fieldKeys[field.Key] {
				issues = append(issues, fmt.Sprintf("%s: дублирован ключ %s", code, field.Key))
			}
			fieldKeys[field.Key] = true
		}
		baseline := laboratoryBaselineKeys(code)
		if len(baseline) == 0 {
			issues = append(issues, fmt.Sprintf("%s: не определён базовый набор", code))
		}
		for key := range baseline {
			if !fieldKeys[key] {
				issues = append(issues, fmt.Sprintf("%s: базовый показатель %s отсутствует в форме", code, key))
			}
		}
		if code != "scale" && len(laboratoryScreeningChecks()[code]) == 0 {
			issues = append(issues, fmt.Sprintf("%s: нет пути предварительной оценки", code))
		}
		// One baseline value must at least turn the block from a decorative empty
		// state into a real, explicitly limited assessment state.
		for key := range baseline {
			values := map[string]string{key: "1"}
			result := laboratoryGroupSummary(group, values, "davies")
			if !result.HasData {
				issues = append(issues, fmt.Sprintf("%s: заполнение %s не распознаётся как данные", code, key))
			}
			break
		}
	}
	return issues
}
