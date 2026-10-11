package main

import (
	"math"
	"testing"
	"time"
)

func nnoChartDate(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }
func TestNNOChartQuartilesAndNoFabricatedPoints(t *testing.T) {
	xs := []nnoCycleSample{
		{"УЭЦН", nnoChartDate(2026, 1), 100}, {"УЭЦН", nnoChartDate(2026, 6), 200}, {"УЭЦН", nnoChartDate(2026, 9), 300},
		{"УЭЦН", nnoChartDate(2024, 5), 9999}, {"УЭЦН", nnoChartDate(2026, 9), -4}, {"УШГН", nnoChartDate(2026, 9), 800},
	}
	got := nnoRollingCycleStats(xs, "УЭЦН", nnoChartDate(2026, 10))
	if !got.Defined || got.Count != 3 || got.Mean != 200 || got.Median != 200 || got.Min != 100 || got.Max != 300 || got.Q1 != 150 || got.Q3 != 250 {
		t.Fatalf("stats: %+v", got)
	}
	missing := nnoRollingCycleStats(xs, "УЭЦН", nnoChartDate(2024, 1))
	if missing.Defined || missing.Count != 0 {
		t.Fatalf("must be missing, not zero: %+v", missing)
	}
	if nnoSampleInWindow(nnoCycleSample{Group: "УЭЦН", End: nnoChartDate(2026, 9), Days: math.NaN()}, nnoChartDate(2026, 1), nnoChartDate(2026, 10)) {
		t.Fatal("NaN duration accepted")
	}
}
func TestNNOChartRollingWindowBoundary(t *testing.T) {
	xs := []nnoCycleSample{{"ОРД", nnoChartDate(2025, 10), 900}, {"ОРД", nnoChartDate(2025, 11), 200}, {"ОРД", nnoChartDate(2026, 10), 100}}
	got := nnoRollingCycleStats(xs, "ОРД", nnoChartDate(2026, 10))
	if got.Count != 2 || got.Median != 150 {
		t.Fatalf("boundary: %+v", got)
	}
}

func TestNNOBoxWhiskersExcludeExtremeObservation(t *testing.T) {
	date := nnoChartDate(2026, 9)
	xs := []nnoCycleSample{{"ОРД", date, 100}, {"ОРД", date, 110}, {"ОРД", date, 120}, {"ОРД", date, 130}, {"ОРД", date, 1000}}
	got := nnoCycleStatistics(xs, "ОРД", nnoChartDate(2026, 1), nnoChartDate(2026, 10))
	if got.Count != 5 || got.Outliers != 1 || got.WhiskerMin != 100 || got.WhiskerMax != 130 || got.Max != 1000 {
		t.Fatalf("box whiskers and source maximum must differ: %+v", got)
	}
}

func TestNNODistributionMarkerSelectionMapsLowerCenterAndUpper(t *testing.T) {
	rc := RECT{Left: 0, Top: 0, Right: 700, Bottom: 260}
	groups := []int{0, 1, 2}
	var summaries [nnoMethodCount]nnoCycleStats
	summaries[1] = nnoCycleStats{Count: 4, Mean: 450, Median: 400, Min: 200, Q1: 300, Q3: 500, Max: 700, Defined: true}
	plot := nnoDistributionPlot(rc)
	centerX := plot.Left + (plot.Right-plot.Left)/2
	scale := 1000.0
	for _, tc := range []struct {
		kind  int
		value float64
	}{{nnoDistributionLower, 200}, {nnoDistributionCentral, 450}, {nnoDistributionUpper, 700}} {
		y := plot.Bottom - int32(tc.value/scale*float64(plot.Bottom-plot.Top))
		got, ok := nnoPickDistributionMarker(rc, groups, scale, centerX, y, false, summaries)
		if !ok || got.Group != 1 || got.Kind != tc.kind || got.Value != tc.value {
			t.Fatalf("pick kind=%d value=%v: got %+v, ok=%t", tc.kind, tc.value, got, ok)
		}
	}
	y := plot.Bottom - int32(400/scale*float64(plot.Bottom-plot.Top))
	got, ok := nnoPickDistributionMarker(rc, groups, scale, centerX, y, true, summaries)
	if !ok || got.Kind != nnoDistributionCentral || got.Value != 400 {
		t.Fatalf("median marker: got %+v, ok=%t", got, ok)
	}
	if _, ok := nnoPickDistributionMarker(rc, groups, scale, centerX, y-20, false, summaries); ok {
		t.Fatal("click away from marker should not select a value")
	}
}
