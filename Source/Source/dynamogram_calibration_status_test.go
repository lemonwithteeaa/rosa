//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestDynamogramRC152CalibrationStatusIsExplicit(t *testing.T) {
	if got := dynamogramCalibrationSummary(&DynamogramAnalysis{}); !strings.Contains(got, "нормированные координаты") {
		t.Fatalf("uncalibrated state unclear: %q", got)
	}
	a := &DynamogramAnalysis{Calibrated: true, XMin: 0, XMax: 3.2, YMin: 5, YMax: 80, XUnit: "м", YUnit: "кН"}
	got := dynamogramCalibrationSummary(a)
	for _, want := range []string{"шкалы подтверждены", "3.2", "м", "80", "кН"} {
		if !strings.Contains(got, want) {
			t.Fatalf("calibration summary missing %q: %s", want, got)
		}
	}
}
