//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestLabPassPrepareAndIsolation(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	pass := LabPassRecord{ObjectKey: " Месторождение Скв-1 ", Groups: map[string]map[string]string{
		"corrosion": {"chlorides": "12500"},
	}}
	if err := pass.prepare(now); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(pass.PassID, "LP-") || pass.Status != labPassDraftStatus {
		t.Fatalf("unexpected prepared pass: %#v", pass)
	}
	copyGroups := cloneLabGroups(pass.Groups)
	copyGroups["corrosion"]["chlorides"] = "1"
	if pass.Groups["corrosion"]["chlorides"] != "12500" {
		t.Fatal("group clone leaked a change into the original pass")
	}
}

func TestLaboratoryPassBuildsOneComplexConclusion(t *testing.T) {
	pass := LabPassRecord{
		PassID: "LP-COMPLEX", ObjectKey: "OBJ-1", Date: "15.09.2026", Source: "Протокол 7",
		Groups: map[string]map[string]string{
			"corrosion": {"ph": "6,1", "chlorides": "12000"},
			"scale":     {"ca": "2300", "so4": "600"},
		},
		SharedValues: map[string]string{"ph": "6,1", "chlorides": "12000"},
	}
	text := buildLaboratoryPassConclusion(pass)
	for _, want := range []string{"ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ", "РАЗДЕЛ: КОРРОЗИЯ", "РАЗДЕЛ: СОЛИ", "LP-COMPLEX", "Заполнено направлений: 2 из 6"} {
		if !strings.Contains(text, want) {
			t.Fatalf("complex conclusion misses %q:\n%s", want, text)
		}
	}
}

func TestLabPassLookupDoesNotMixObjects(t *testing.T) {
	passes := []LabPassRecord{
		{PassID: "LP-1", ObjectKey: "FIELD W1", Groups: map[string]map[string]string{"scale": {"cl": "10"}}},
		{PassID: "LP-1", ObjectKey: "FIELD W2", Groups: map[string]map[string]string{"scale": {"cl": "20"}}},
	}
	p, ok := labPassForObject(passes, "field w2", "LP-1")
	if !ok || p.Groups["scale"]["cl"] != "20" {
		t.Fatalf("wrong object pass selected: %#v, %v", p, ok)
	}
}

func TestLegacyLabRecordRemainsReadableAsPass(t *testing.T) {
	record := LabRecord{ID: "LAB-OLD", ObjectKey: "OBJ", Date: "01.09.2026", GroupCode: "corrosion", Values: map[string]string{"ph": "6,1"}}
	pass := legacyLabPassView(record)
	if pass.PassID != "LEGACY-LAB-OLD" || pass.Groups["corrosion"]["ph"] != "6,1" || pass.Status != labPassSavedStatus {
		t.Fatalf("legacy compatibility failed: %#v", pass)
	}
}

func TestLaboratoryPassConclusionShowsProvenance(t *testing.T) {
	pass := LabPassRecord{
		PassID: "LP-SOURCE", ObjectKey: "OBJ", Date: "15.09.2026", Source: "Протокол 42", Author: "Технолог",
		Groups: map[string]map[string]string{
			"corrosion": {"ph": "6.3", "watercut": "78"},
			"scale":     {"ph": "6.3", "watercut": "78"},
		},
	}
	text := buildLaboratoryPassConclusion(pass)
	for _, want := range []string{"КОНТРОЛЬ ИСТОЧНИКОВ И АКТУАЛЬНОСТИ", "Протокол 42", "Автор: Технолог", "источник: Техрежим", "источник: общее поле диагностики"} {
		if !strings.Contains(text, want) {
			t.Fatalf("provenance conclusion misses %q:\n%s", want, text)
		}
	}
}

func TestLaboratorySharedValueConflictsAreExplicit(t *testing.T) {
	groups := map[string]map[string]string{
		"corrosion": {"ph": "6,2"},
		"scale":     {"ph": "7.1"},
	}
	conflicts := laboratorySharedValueConflicts(groups)
	if len(conflicts) != 1 || !strings.Contains(conflicts[0], "ph") || !strings.Contains(conflicts[0], "6,2") || !strings.Contains(conflicts[0], "7.1") {
		t.Fatalf("shared conflict was not explained: %#v", conflicts)
	}
}

func TestOverallHistorySelectionOpensPassesAndLegacyRows(t *testing.T) {
	passes := []LabPassRecord{{PassID: "LP-1"}, {PassID: "LP-2"}}
	records := []LabRecord{{ID: "OLD-1"}, {ID: "OLD-2"}}
	pass, record, kind := laboratoryOverallHistorySelection(1, passes, records)
	if kind != "pass" || pass.PassID != "LP-2" || record != nil {
		t.Fatalf("wrong pass selection: kind=%q pass=%#v record=%#v", kind, pass, record)
	}
	pass, record, kind = laboratoryOverallHistorySelection(2, passes, records)
	if kind != "legacy" || pass.PassID != "" || record == nil || record.ID != "OLD-1" {
		t.Fatalf("wrong legacy selection: kind=%q pass=%#v record=%#v", kind, pass, record)
	}
	_, _, kind = laboratoryOverallHistorySelection(4, passes, records)
	if kind != "" {
		t.Fatalf("out-of-range selection must be ignored, got %q", kind)
	}
}

func TestPassConclusionUsesSavedScaleModel(t *testing.T) {
	before := labScaleModelCode
	pass := LabPassRecord{ObjectKey: "test-object", ScaleModel: "ideal", Groups: map[string]map[string]string{"scale": {"ca": "40.078", "co3": "60.01"}}}
	text := buildLaboratoryPassConclusion(pass)
	if !strings.Contains(text, "Модель: Идеальный раствор") {
		t.Fatalf("saved model missing: %s", text)
	}
	if labScaleModelCode != before {
		t.Fatal("conclusion changed selected model")
	}
}
