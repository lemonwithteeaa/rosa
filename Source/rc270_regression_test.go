package main

import (
	"os"
	"strings"
	"testing"
)

func targetSettingsForTest() TargetRunlifeSettings {
	return TargetRunlifeSettings{
		SchemaVersion: 2,
		DefaultDays:   1000,
		NearPct:       5,
		LiftDays:      map[string]float64{},
		ObjectDays:    map[string]float64{},
	}
}

func TestRC270TargetHierarchyObjectLiftGlobal(t *testing.T) {
	old := targetRunlifeSettings
	defer func() { targetRunlifeSettings = old }()
	targetRunlifeSettings = targetSettingsForTest()
	targetRunlifeSettings.LiftDays["УЭЦН"] = 1050
	targetRunlifeSettings.ObjectDays[normalizeLaboratoryObjectKey("OBJ-1")] = 1030

	if got := targetRunlifeResolved("OBJ-1", "ЭЦН-80"); got.Days != 1030 || got.Scope != targetScopeObject {
		t.Fatalf("object precedence failed: %+v", got)
	}
	if got := targetRunlifeResolved("OBJ-2", "ЭЦН-80"); got.Days != 1050 || got.Scope != targetScopeLift {
		t.Fatalf("lift precedence failed: %+v", got)
	}
	if got := targetRunlifeResolved("OBJ-3", "Фонтанный"); got.Days != 1000 || got.Scope != targetScopeGlobal {
		t.Fatalf("global fallback failed: %+v", got)
	}
}

func TestRC270LiftTypeNormalization(t *testing.T) {
	cases := map[string]string{
		"ЭЦН":     "УЭЦН",
		"УЭЦН-80": "УЭЦН",
		"ШГН":     "УШГН",
		"УШГН":    "УШГН",
		"PCP":     "ВШН",
		"ШВН":     "ВШН",
		"УЭОВН":   "ВШН",
		"ФОН":     "Фонтанный",
	}
	for in, want := range cases {
		if got := normalizeTargetLiftType(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestRC270TargetToleranceIsConfigurable(t *testing.T) {
	_, _, near := targetRunlifeComparisonWithTolerance(930, 1000, 10)
	if near != "вблизи целевого ориентира" {
		t.Fatalf("expected near, got %q", near)
	}
	_, _, low := targetRunlifeComparisonWithTolerance(930, 1000, 5)
	if low != "ниже целевого ориентира" {
		t.Fatalf("expected below, got %q", low)
	}
}

func TestRC270EconomicsActuallyCalculatesTargetStatus(t *testing.T) {
	r := CalculateBaseEconomics(BaseEconomicsInput{
		CurrentMRPDays: 700, MRPLowDays: 850, MRPBaseDays: 950, MRPHighDays: 1100,
		TargetMRPDays: 1000, TargetSource: "общий ориентир 1000 сут.", TargetNearPct: 5,
		SelectedScenario: "Центральный", HorizonMonths: 12, RepairCostK: 1000,
	})
	if r.Status != "PASS" {
		t.Fatalf("economics failed: %+v", r.Errors)
	}
	if r.TargetStatus != "вблизи целевого ориентира" || r.TargetRatioPct != 95 || r.TargetGapDays != -50 {
		t.Fatalf("target economics not calculated: %+v", r)
	}
	if r.TargetSource == "" {
		t.Fatal("target source missing")
	}
}

func TestRC270MRPUISeparatesForecastAndTargetCards(t *testing.T) {
	b, err := os.ReadFile("mrp_forecast_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"Основной прогноз", "Цель / достижение", "mrpTargetStatusLabel", "Уровень настройки", "Для типа ГНО", "Общий ориентир"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(s, "setText(mrpBaseLabel, fmt.Sprintf(\"%.0f сут. | цель") {
		t.Fatal("forecast card is still overwritten by target text")
	}
}

func TestRC270TargetUIUsesRussianTerms(t *testing.T) {
	b, _ := os.ReadFile("mrp_forecast_ui.go")
	s := strings.ToLower(string(b))
	for _, forbidden := range []string{"target scope", "target runlife", "baseline", "survival probability"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("user-facing/near-UI english residue %q", forbidden)
		}
	}
}

func TestRC270BuildMarkerExists(t *testing.T) {
	b, _ := os.ReadFile("main.go")
	if !strings.Contains(string(b), "const appBuildVersion = \"7.3.11.") {
		t.Fatal("build marker missing")
	}
}
