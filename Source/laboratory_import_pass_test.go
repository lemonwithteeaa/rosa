//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC153ImportedPassIDIsDeterministicAndContentSensitive(t *testing.T) {
	p := LabPassRecord{ObjectKey: "Field 23", Date: "18.09.2026", Source: "Протокол 7", ScaleModel: "davies", Groups: map[string]map[string]string{"corrosion": {"ph": "6.5"}}, Units: map[string]map[string]string{"corrosion": {"ph": "ед."}}}
	a := laboratoryImportedPassID(p)
	b := laboratoryImportedPassID(p)
	if a != b || !strings.HasPrefix(a, "LP-IMP-") {
		t.Fatalf("unexpected deterministic id: %q %q", a, b)
	}
	p.Groups["corrosion"]["ph"] = "6.6"
	if laboratoryImportedPassID(p) == a {
		t.Fatal("changed imported data must produce another pass id")
	}
}

func TestRC153PassCompletenessNamesMissingBaselineFields(t *testing.T) {
	p := LabPassRecord{ScaleModel: "davies", Groups: map[string]map[string]string{"corrosion": {"ph": "6.5"}}}
	text := laboratoryPassCompletenessSummary(p)
	if !strings.Contains(text, "Полнота диагностики") || !strings.Contains(text, "Коррозия") || !strings.Contains(text, "не заполнены базовые показатели") {
		t.Fatalf("unexpected completeness summary: %s", text)
	}
}

func TestRC153SharedConflictRemainsExplicitInPassConclusion(t *testing.T) {
	p := LabPassRecord{PassID: "LP-X", ObjectKey: "OBJ", Date: "18.09.2026", Source: "P", Status: labPassSavedStatus, ScaleModel: "davies",
		Groups: map[string]map[string]string{"corrosion": {"ph": "6.5"}, "scale": {"ph": "7.0"}},
		Units:  map[string]map[string]string{"corrosion": {"ph": "ед."}, "scale": {"ph": "ед."}}}
	p.SharedValues = collectLaboratorySharedValues(p.Groups)
	text := buildLaboratoryPassConclusion(p)
	if !strings.Contains(text, "ОБНАРУЖЕНЫ РАСХОЖДЕНИЯ") {
		t.Fatalf("shared conflict hidden: %s", text)
	}
}

func TestRC153ImportedAtIsShownInProvenance(t *testing.T) {
	p := LabPassRecord{Date: "18.09.2026", Source: "Протокол", ImportedAt: "2026-09-18T10:00:00+03:00", Groups: map[string]map[string]string{}}
	if got := laboratoryPassProvenanceSummary(p); !strings.Contains(got, "импортировано:") {
		t.Fatalf("import timestamp missing: %s", got)
	}
}
