//go:build windows

package main

import (
	"testing"
	"time"
)

func TestLaboratoryRC173PersonalScopeDoesNotMixObjects(t *testing.T) {
	passes := []LabPassRecord{
		{PassID: "a", Status: labPassSavedStatus, ObjectKey: "OBJ-A", Date: "01.01.2026", Groups: map[string]map[string]string{"corrosion": {"ph": "5"}}, Units: map[string]map[string]string{}},
		{PassID: "b", Status: labPassSavedStatus, ObjectKey: "OBJ-B", Date: "01.01.2026", Groups: map[string]map[string]string{"corrosion": {"ph": "9"}}, Units: map[string]map[string]string{}},
	}
	latest := laboratoryLatestSavedPassesForKeys([]string{"OBJ-A"}, passes)
	if len(latest) != 1 || latest[0].ObjectKey != "OBJ-A" {
		t.Fatalf("personal infographic mixed objects: %#v", latest)
	}
}

func TestLaboratoryRC173LatestPassPerObjectForMassInfographic(t *testing.T) {
	passes := []LabPassRecord{
		{PassID: "a1", Status: labPassSavedStatus, ObjectKey: "OBJ-A", Date: "01.01.2026"},
		{PassID: "a2", Status: labPassSavedStatus, ObjectKey: "OBJ-A", Date: "02.02.2026"},
		{PassID: "b1", Status: labPassSavedStatus, ObjectKey: "OBJ-B", Date: "03.02.2026"},
	}
	latest := laboratoryLatestSavedPassesForKeys([]string{"OBJ-A", "OBJ-B"}, passes)
	if len(latest) != 2 {
		t.Fatalf("latest mass set len=%d", len(latest))
	}
	foundA := false
	for _, p := range latest {
		if p.ObjectKey == "OBJ-A" {
			foundA = true
			if p.PassID != "a2" {
				t.Fatalf("wrong latest OBJ-A: %s", p.PassID)
			}
		}
	}
	if !foundA {
		t.Fatal("OBJ-A missing")
	}
}

func TestLaboratoryRC173ScopeIndexFallback(t *testing.T) {
	if got := laboratoryScopeCodeByIndex(-1); got != laboratoryScopeObject {
		t.Fatalf("got %q", got)
	}
	if got := laboratoryScopeCodeByIndex(99); got != laboratoryScopeObject {
		t.Fatalf("got %q", got)
	}
}

func TestLaboratoryRC175MassAnalyticsCountsCoverageAndFreshness(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	passes := []LabPassRecord{
		{PassID: "A", ObjectKey: "OBJ-1", Date: "10.09.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "5.5"}}, Units: map[string]map[string]string{}},
		{PassID: "B", ObjectKey: "OBJ-2", Date: "01.05.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "7.0"}}, Units: map[string]map[string]string{}},
	}
	a := laboratoryMassAnalyticsForKeys([]string{"OBJ-1", "OBJ-2", "OBJ-3"}, passes, now)
	if a.ScopeObjects != 3 || a.ObjectsWithPass != 2 || a.ObjectsWithoutPass != 1 {
		t.Fatalf("coverage mismatch: %+v", a)
	}
	if a.Fresh30 != 1 || a.Recent180 != 1 || a.Stale180 != 0 {
		t.Fatalf("freshness mismatch: %+v", a)
	}
}

func TestLaboratoryRC175MassAnalyticsDirectionRiskUsesLatestPassOnly(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	passes := []LabPassRecord{
		{PassID: "OLD", ObjectKey: "OBJ-1", Date: "01.01.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "5.0"}}, Units: map[string]map[string]string{}},
		{PassID: "NEW", ObjectKey: "OBJ-1", Date: "10.09.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "7.0"}}, Units: map[string]map[string]string{}},
	}
	a := laboratoryMassAnalyticsForKeys([]string{"OBJ-1"}, passes, now)
	if a.DirectionKnown["corrosion"] != 1 {
		t.Fatalf("latest object must count once: %+v", a.DirectionKnown)
	}
}

func TestLaboratoryRC176MassKPIsDoNotTreatMissingAsLowRisk(t *testing.T) {
	a := laboratoryMassAnalytics{ScopeObjects: 10, ObjectsWithPass: 6, ObjectsWithoutPass: 4, Fresh30: 3}
	cards := laboratoryMassKPIs(a)
	if len(cards) != 4 {
		t.Fatalf("KPI count=%d", len(cards))
	}
	if cards[2].Value != "4" || cards[2].Note != "не считать низким риском" {
		t.Fatalf("missing-analysis KPI lost safety meaning: %+v", cards[2])
	}
}

func TestLaboratoryRC176PersonalKPIsStayObjectScoped(t *testing.T) {
	passes := []LabPassRecord{
		{PassID: "a1", Status: labPassSavedStatus, ObjectKey: "OBJ-A", Date: "01.01.2026", Groups: map[string]map[string]string{"corrosion": {"ph": "6"}}, Units: map[string]map[string]string{}},
		{PassID: "a2", Status: labPassSavedStatus, ObjectKey: "OBJ-A", Date: "02.02.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "20"}}, Units: map[string]map[string]string{}},
		{PassID: "b1", Status: labPassSavedStatus, ObjectKey: "OBJ-B", Date: "03.03.2026", Groups: map[string]map[string]string{"corrosion": {"ph": "5"}}, Units: map[string]map[string]string{}},
	}
	cards := laboratoryPersonalKPIs("OBJ-A", passes)
	if cards[0].Value != "2" {
		t.Fatalf("personal KPI mixed objects: %+v", cards)
	}
	if cards[1].Value != "02.02.2026" {
		t.Fatalf("wrong latest date: %+v", cards)
	}
}

func TestLaboratoryRC181AttentionSeparatesMissingStaleAndRisk(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	passes := []LabPassRecord{
		{PassID: "RISK", ObjectKey: "OBJ-RISK", Date: "10.09.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "4.5"}}, Units: map[string]map[string]string{}},
		{PassID: "OLD", ObjectKey: "OBJ-OLD", Date: "01.01.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "7.0"}}, Units: map[string]map[string]string{}},
	}
	a := laboratoryAttentionForKeys([]string{"OBJ-RISK", "OBJ-OLD", "OBJ-MISSING"}, passes, now)
	if len(a.WithoutAnalysis) != 1 || a.WithoutAnalysis[0] != "obj-missing" {
		t.Fatalf("missing objects mismatch: %+v", a)
	}
	if len(a.Stale) != 1 || a.Stale[0] != "obj-old" {
		t.Fatalf("stale objects mismatch: %+v", a)
	}
	if len(a.Risk) == 0 || a.Risk[0] != "obj-risk" {
		t.Fatalf("risk objects mismatch: %+v", a)
	}
}
