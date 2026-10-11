//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLaboratoryPreviousPassUsesSameObjectAndEarlierDate(t *testing.T) {
	current := LabPassRecord{PassID: "CUR", ObjectKey: "Поле 1 / Скв 23", Date: "18.09.2026"}
	passes := []LabPassRecord{
		{PassID: "OTHER", ObjectKey: "Поле 2 / Скв 23", Date: "17.09.2026", Status: labPassSavedStatus},
		{PassID: "NEWER", ObjectKey: "Поле 1 / Скв 23", Date: "19.09.2026", Status: labPassSavedStatus},
		{PassID: "PREV1", ObjectKey: "Поле 1 / Скв 23", Date: "15.09.2026", Status: labPassSavedStatus},
		{PassID: "PREV2", ObjectKey: "поле 1 / скв 23", Date: "17.09.2026", Status: labPassSavedStatus},
	}
	p, ok := laboratoryPreviousSavedPass(current, passes)
	if !ok || p.PassID != "PREV2" {
		t.Fatalf("expected PREV2, got ok=%v pass=%#v", ok, p)
	}
}

func TestLaboratoryComparisonTextShowsDeltasAndSulinChange(t *testing.T) {
	prev := LabPassRecord{
		PassID: "P1", ObjectKey: "Obj", Date: "01.09.2026", Status: labPassSavedStatus,
		Groups: map[string]map[string]string{
			"scale":     {"mineralization": "100", "cl": "10000", "so4": "100", "ca": "500", "ba": "10", "sr": "20", "ph": "7.0", "na_k": "12000", "mg": "1000"},
			"corrosion": {"iron_total": "1", "orp": "50"},
		},
		Units: map[string]map[string]string{"scale": {}, "corrosion": {}},
	}
	cur := LabPassRecord{
		PassID: "P2", ObjectKey: "Obj", Date: "18.09.2026", Status: labPassSavedStatus,
		Groups: map[string]map[string]string{
			"scale":     {"mineralization": "130", "cl": "13000", "so4": "160", "ca": "600", "ba": "15", "sr": "20", "ph": "7.5", "na_k": "5000", "mg": "1000"},
			"corrosion": {"iron_total": "2", "orp": "120"},
		},
		Units: map[string]map[string]string{"scale": {}, "corrosion": {}},
	}
	text := laboratoryPassComparisonText(cur, []LabPassRecord{prev})
	for _, want := range []string{"СРАВНЕНИЕ С ПРЕДЫДУЩИМ", "Общая минерализация", "Железо общее", "ОВП", "заметное изменение"} {
		if !strings.Contains(text, want) {
			t.Fatalf("comparison misses %q:\n%s", want, text)
		}
	}
}

func TestLaboratoryComparisonShortDoesNotOverclaimWithoutPrevious(t *testing.T) {
	group, _ := labGroupByCode("scale")
	text := laboratoryCurrentGroupComparisonShort(group, map[string]string{"mineralization": "120"}, map[string]string{}, "Obj", "18.09.2026", "davies", nil)
	if text != "ДИНАМИКА: предыдущий анализ не найден" {
		t.Fatalf("unexpected short comparison: %s", text)
	}
}
