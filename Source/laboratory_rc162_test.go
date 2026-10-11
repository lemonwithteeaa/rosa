//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestLaboratoryTrendRC162UsesSavedComparablePasses(t *testing.T) {
	group, _ := labGroupByCode("scale")
	passes := []LabPassRecord{
		{PassID: "a", ObjectKey: "X", Date: "01.01.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"scale": {"mineralization": "100"}}},
		{PassID: "b", ObjectKey: "X", Date: "01.02.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"scale": {"mineralization": "120"}}},
	}
	text := laboratoryGroupTrendShort(group, "X", "02.02.2026", passes)
	if !strings.Contains(text, "ТРЕНД ИСТОРИИ") || !strings.Contains(text, "+20") {
		t.Fatalf("unexpected trend: %s", text)
	}
}

func TestLaboratoryExportQCRC162KeepsLimitations(t *testing.T) {
	pass := LabPassRecord{ObjectKey: "X", Date: "01.01.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{}, Units: map[string]map[string]string{}}
	text := laboratoryExportQCText(pass, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	for _, want := range []string{"QC-ГЕЙТ ЭКСПОРТА", "не указан источник / протокол", "QC не заменяет лабораторный протокол"} {
		if !strings.Contains(text, want) {
			t.Fatalf("export QC misses %q: %s", want, text)
		}
	}
}
