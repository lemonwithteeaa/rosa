package main

import (
	"math"
	"testing"
)

func TestTargetRunlifeDefaultIs1000(t *testing.T) {
	old := targetRunlifeSettings
	defer func() { targetRunlifeSettings = old }()
	targetRunlifeSettings = TargetRunlifeSettings{SchemaVersion: 1, DefaultDays: 1000, ObjectDays: map[string]float64{}}
	if got := targetRunlifeDaysForObject("X"); got != 1000 {
		t.Fatalf("got %.0f", got)
	}
}

func TestTargetRunlifeObjectOverride(t *testing.T) {
	old := targetRunlifeSettings
	defer func() { targetRunlifeSettings = old }()
	targetRunlifeSettings = TargetRunlifeSettings{SchemaVersion: 1, DefaultDays: 1000, ObjectDays: map[string]float64{normalizeLaboratoryObjectKey("A"): 1050}}
	if got := targetRunlifeDaysForObject("A"); got != 1050 {
		t.Fatalf("got %.0f", got)
	}
}

func TestTargetRunlifeInvalidFallsBack(t *testing.T) {
	if got := normalizedTargetRunlifeDays(0); got != 1000 {
		t.Fatalf("got %.0f", got)
	}
	if got := normalizedTargetRunlifeDays(math.Inf(1)); got != 1000 {
		t.Fatalf("got %.0f", got)
	}
}

func TestTargetRunlifeComparisonNear(t *testing.T) {
	gap, ratio, status := targetRunlifeComparison(970, 1000)
	if gap != -30 || math.Abs(ratio-97) > 1e-9 || status != "вблизи целевого ориентира" {
		t.Fatalf("%v %.1f %s", gap, ratio, status)
	}
}

func TestTargetRunlifeDoesNotReplaceForecast(t *testing.T) {
	in := MRPForecastInput{ObjectKey: "A", TargetMRPDays: 1100, Observations: []MRPObservation{{MRPDays: 500}, {MRPDays: 700}, {MRPDays: 900}}}
	r := calculateMRPForecast(in)
	if r.BaseDays != 700 {
		t.Fatalf("target must not replace forecast; got %.0f", r.BaseDays)
	}
	if r.TargetMRPDays != 1100 || r.TargetForecastDays != 700 || r.TargetStatus != "ниже целевого ориентира" {
		t.Fatalf("target comparison missing: %+v", r)
	}
}
