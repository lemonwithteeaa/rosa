//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLaboratoryMixingRiskRangesContiguous(t *testing.T) {
	points := []laboratoryMixingCurvePoint{
		{Fraction: 0.00, SI: [4]float64{0.0, 0, 0, 0}, Present: [4]bool{true, false, false, false}},
		{Fraction: 0.05, SI: [4]float64{0.12, 0, 0, 0}, Present: [4]bool{true, false, false, false}},
		{Fraction: 0.10, SI: [4]float64{0.20, 0, 0, 0}, Present: [4]bool{true, false, false, false}},
		{Fraction: 0.15, SI: [4]float64{0.08, 0, 0, 0}, Present: [4]bool{true, false, false, false}},
		{Fraction: 0.20, SI: [4]float64{0.14, 0, 0, 0}, Present: [4]bool{true, false, false, false}},
	}
	got := laboratoryMixingRiskRanges(points, 0.10)
	if len(got) != 2 {
		t.Fatalf("ranges=%v", got)
	}
	if got[0].StartPct != 5 || got[0].EndPct != 10 || got[1].StartPct != 20 || got[1].EndPct != 20 {
		t.Fatalf("unexpected ranges: %#v", got)
	}
}

func TestLaboratoryMixingRiskTextStatesScreeningLimits(t *testing.T) {
	text := laboratoryMixingRiskText(mixingInputs())
	if !strings.Contains(text, "термодинамической") || !strings.Contains(text, "5 %") {
		t.Fatalf("screening limits missing: %s", text)
	}
	if strings.Contains(strings.ToLower(text), "осадок точно") {
		t.Fatalf("text overclaims precipitation: %s", text)
	}
}
