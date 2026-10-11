//go:build windows

package main

import (
	"math"
	"testing"
	"time"
)

func TestMRPRollingSeparatesMethodsAndPlannedRepairs(t *testing.T) {
	day := func(n int) time.Time { return time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC) }
	ob := func(method string, n int, duration float64, event string) MRPObservation {
		return MRPObservation{Method: method, Date: day(n), CycleStartDate: day(1), CycleEndDate: day(n),
			MRPDays: duration, EventType: event, ClassificationConfirmed: true}
	}
	rows, excluded := mrpRollingRows(MRPForecastInput{ObjectKey: "PROD", Observations: []MRPObservation{
		ob("A", 8, 100, "Отказ"), ob("A", 10, 200, "Отказ"),
		ob("B", 12, 999, "Отказ"), ob("A", 15, 300, "Отказ"),
		ob("A", 19, 400, "Отказ"), ob("A", 21, 888, "Плановое вмешательство"),
	}})
	if len(rows) != 2 || rows[0].Method != "A" || len(rows[0].Cycles) != 4 {
		t.Fatalf("methods and cycles: %#v", rows)
	}
	if math.Abs(rows[0].Mean3-300) > 0.001 || math.Abs(rows[0].Mean4-250) > 0.001 {
		t.Fatalf("rolling 3/4 = %.1f / %.1f", rows[0].Mean3, rows[0].Mean4)
	}
	if rows[1].Mean3 != 0 || len(excluded) != 1 {
		t.Fatalf("insufficient sample and planned exclusion: %#v, %#v", rows[1], excluded)
	}
}

func TestMRPRollingUsesEqualWeightPerObject(t *testing.T) {
	day := func(n int) time.Time { return time.Date(2026, 2, n, 0, 0, 0, 0, time.UTC) }
	obs := []MRPObservation{}
	for i, d := range []float64{100, 200, 300, 400} {
		obs = append(obs, MRPObservation{ObjectKey: "A", Method: "M", Date: day(i + 5), CycleStartDate: day(1), CycleEndDate: day(i + 5), MRPDays: d, EventType: "отказ", ClassificationConfirmed: true})
	}
	obs = append(obs, MRPObservation{ObjectKey: "B", Method: "M", Date: day(15), CycleStartDate: day(1), CycleEndDate: day(15), MRPDays: 900, EventType: "отказ", ClassificationConfirmed: true})
	rows, _ := mrpRollingRows(MRPForecastInput{ObjectKey: "A", Observations: obs})
	if len(rows) != 1 || len(rows[0].ObjectRows) != 2 {
		t.Fatalf("objects: %#v", rows)
	}
	// Object A averages 250 over four failures; B averages 900 over one.
	// The method average must be (250+900)/2, not 1900/5.
	if rows[0].MeanAll != 575 || rows[0].Mean3 != 300 || rows[0].Ready3 != 1 {
		t.Fatalf("two-stage result: %#v", rows[0])
	}
}
