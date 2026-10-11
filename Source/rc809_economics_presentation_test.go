//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC809EconomicsFundFilteringKeepsRowMapping(t *testing.T) {
	groups := []economicsFundObjectRow{
		{ObjectKey: "A", Schemes: make([]AlternativeMethodScenario, 3), Calculated: 0},
		{ObjectKey: "B", Schemes: make([]AlternativeMethodScenario, 2), Calculated: 1, HasBenefit: true, BestBenefit: 20},
		{ObjectKey: "C", Schemes: make([]AlternativeMethodScenario, 1), Calculated: 1, HasBenefit: true, BestBenefit: -5},
	}
	for _, tc := range []struct {
		mode int
		keys string
	}{
		{0, "ABC"}, {1, "AB"}, {2, "BC"}, {3, "B"},
	} {
		got := economicsFundFilterGroups(groups, tc.mode)
		keys := ""
		for _, g := range got {
			keys += g.ObjectKey
		}
		if keys != tc.keys {
			t.Fatalf("mode %d: want %s, got %s", tc.mode, tc.keys, keys)
		}
	}
	// Filtering must not mutate source order or source row identity.
	if groups[1].ObjectKey != "B" || len(groups) != 3 {
		t.Fatal("source rows mutated")
	}
	summary := economicsFundSummaryText(groups)
	for _, fragment := range []string{"3 объектов", "6 вариантов", "2 расчётов", "эффектом: 1"} {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("summary missing %q: %s", fragment, summary)
		}
	}
	if len(economicsFundFilterGroups(groups, 999)) != len(groups) {
		t.Fatal("unknown filter must show all")
	}
}
