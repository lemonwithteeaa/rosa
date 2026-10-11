//go:build windows

package main

import "testing"

func TestLaboratoryChartObjectLabelKeepsShortAndTruncatesLong(t *testing.T) {
	if got := laboratoryChartObjectLabel("СКВ-23"); got != "СКВ-23" {
		t.Fatalf("short label changed: %q", got)
	}
	if got := laboratoryChartObjectLabel("Очень-длинный-ключ-объекта-123456"); len([]rune(got)) > 16 {
		t.Fatalf("long label not compact: %q", got)
	}
}

func TestLaboratoryRiskFilterIndex(t *testing.T) {
	if got := laboratoryRiskFilterCodeByIndex(1); got != "corrosion" {
		t.Fatalf("filter index mismatch: %q", got)
	}
	if got := laboratoryRiskFilterCodeByIndex(999); got != "all" {
		t.Fatalf("invalid filter must be all: %q", got)
	}
}

func TestLaboratoryMassKPIsExposeStaleCard(t *testing.T) {
	cards := laboratoryMassKPIs(laboratoryMassAnalytics{
		ScopeObjects: 10, ObjectsWithPass: 8, ObjectsWithoutPass: 2,
		Fresh30: 3, Stale180: 2, FutureDate: 1,
	})
	if len(cards) != 5 {
		t.Fatalf("mass KPI count = %d, want 5", len(cards))
	}
	if cards[4].Label != "УСТАРЕЛИ" || cards[4].Value != "3" {
		t.Fatalf("stale KPI = %+v", cards[4])
	}
}
