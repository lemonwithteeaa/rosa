//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestLaboratoryPassQCPassportShowsMetadataFreshnessAndNoOpaqueScore(t *testing.T) {
	pass := LabPassRecord{ObjectKey: "OBJ-1", Date: "18.09.2026", Source: "Протокол 12", Groups: map[string]map[string]string{}, Units: map[string]map[string]string{}}
	text := laboratoryPassQCText(pass, time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local))
	for _, want := range []string{"ПАСПОРТ КАЧЕСТВА", "Метаданные:", "Свежесть:", "Покрытие:", "Валидация:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("QC passport missing %q: %s", want, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "балл") || strings.Contains(text, "/100") {
		t.Fatalf("opaque score leaked into QC passport: %s", text)
	}
}

func TestLaboratoryPassQCDetectsSharedConflicts(t *testing.T) {
	pass := LabPassRecord{ObjectKey: "OBJ-1", Date: "18.09.2026", Source: "P", Groups: map[string]map[string]string{
		"scale": {"ph": "6,2"}, "corrosion": {"ph": "7.1"},
	}, Units: map[string]map[string]string{"scale": {}, "corrosion": {}}}
	q := laboratoryPassQCSummaryFor(pass, time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local))
	if len(q.SharedConflicts) == 0 {
		t.Fatal("shared conflict was not detected")
	}
	text := laboratoryPassQCText(pass, time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local))
	if !strings.Contains(text, "расхождения") {
		t.Fatalf("shared conflict hidden from passport: %s", text)
	}
}

func TestLaboratoryExportQCUsesSameAggregatePassportRules(t *testing.T) {
	pass := LabPassRecord{ObjectKey: "OBJ-1", Date: "01.01.2026", Source: "", Groups: map[string]map[string]string{}, Units: map[string]map[string]string{}}
	text := laboratoryExportQCText(pass, time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local))
	for _, want := range []string{"не указан источник / протокол", "Покрытие:", "Валидация:", "Прослеживаемость:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("export QC missing %q: %s", want, text)
		}
	}
}
