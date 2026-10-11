//go:build windows

package main

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestRC748MRPScenarioBandUsesFourPromptScenarios(t *testing.T) {
	oldKey := selectedObjectKey
	defer func() { selectedObjectKey = oldKey }()
	selectedObjectKey = ""
	r := MRPForecastResult{TargetForecastDays: 720, EvidenceLevel: "ограниченная история", ObservationCount: 3}
	got := mrpScenarioBand(r)
	want := []string{"Базовая оценка", "Сдержанный", "Расчётный", "Благоприятный"}
	if len(got) != len(want) {
		t.Fatalf("scenario count=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("scenario %d=%q, want %q", i, got[i].Name, want[i])
		}
	}
	if !got[0].Available || got[0].Days != 720 {
		t.Fatalf("baseline=%+v", got[0])
	}
	for _, item := range got[1:] {
		if item.Available || item.Days != 0 {
			t.Fatalf("method scenario must not be invented without a selected/approved complex: %+v", item)
		}
	}
}

func TestRC748ScenarioDeltaIsRelativeToBaseline(t *testing.T) {
	days, pct := scenarioDelta(900, 720)
	if days != 180 || math.Abs(pct-25) > 1e-9 {
		t.Fatalf("delta=%.1f pct=%.2f", days, pct)
	}
}

func TestRC748MRPUIContainsScenarioStrip(t *testing.T) {
	b, err := os.ReadFile("mrp_forecast_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"СЦЕНАРНАЯ ПОЛОСА", "Базовая оценка", "Сдержанный", "Расчётный", "Благоприятный"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q", want)
		}
	}
}
