//go:build windows

package main

import (
	"testing"
	"time"
)

func TestNNORollingHitDetectsSegmentsNotOnlyDots(t *testing.T) {
	originalMonths := nnoFleetMonthData
	originalSeries := nnoFleetSeries
	originalShowBoth, originalMedian := nnoFleetShowBoth, nnoFleetShowMedian
	originalTarget := nnoTarget
	defer func() {
		nnoFleetMonthData, nnoFleetSeries = originalMonths, originalSeries
		nnoFleetShowBoth, nnoFleetShowMedian, nnoTarget = originalShowBoth, originalMedian, originalTarget
	}()
	nnoFleetMonthData = []nnoMonth{
		{Month: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Month: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	for g := range nnoFleetSeries {
		nnoFleetSeries[g] = nil
	}
	nnoFleetSeries[0] = []nnoCycleStats{{Defined: true, Mean: 640, Median: 640}, {Defined: true, Mean: 760, Median: 760}}
	nnoFleetShowBoth, nnoFleetShowMedian, nnoTarget.Visible = false, false, false
	rc := RECT{Left: 0, Top: 0, Right: 810, Bottom: 230}
	plot := nnoRollingPlot(rc)
	scale := nnoRollingScale()
	// Middle of the line: far from both dot hit boxes.
	x := (plot.Left + plot.Right) / 2
	y := plot.Bottom - int32(700/scale*float64(plot.Bottom-plot.Top))
	picked := nnoPickRolling(rc, x, y)
	if !picked.Valid || picked.Group != 0 {
		t.Fatalf("line segment is not clickable: picked=%+v", picked)
	}
	if selected := nnoPickRolling(rc, rc.Right-15, rc.Top+40); selected.Valid {
		t.Fatalf("legend must not capture chart selection: %+v", selected)
	}
}

func TestNNOLegendUsesReadableCompactLabels(t *testing.T) {
	compact := RECT{Left: 0, Top: 0, Right: 225, Bottom: 173}
	labels := nnoDistributionLegendLabels(compact)
	if labels[0] != "Типичный диапазон" || labels[2] != "Медиана" {
		t.Fatalf("compact legend was not selected: %+v", labels)
	}
	large := RECT{Left: 0, Top: 0, Right: 300, Bottom: 320}
	if nnoDistributionLegendLabels(large)[0] != "Диапазон типичных значений" {
		t.Fatal("full legend labels unavailable on large chart")
	}
}

func TestEconomicsConclusionDoesNotImplyApproval(t *testing.T) {
	text := economicsTechnologistConclusion("", AlternativeMethodScenario{}, false, 1000)
	if text == "" {
		t.Fatal("missing instruction when object not pinned")
	}
	text = economicsTechnologistConclusion("object", AlternativeMethodScenario{Name: "Схема А"}, true, 1000)
	if text == "" || text == "Расчёт актуален" {
		t.Fatal("an uncalculated scheme must require verification")
	}
}
