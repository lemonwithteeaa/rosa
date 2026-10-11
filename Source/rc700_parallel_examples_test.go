//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC700ThreeTeachingObjectsHaveParallelSchemes(t *testing.T) {
	want := map[int]int{1: 5, 58: 4, 90: 3}
	for ordinal, count := range want {
		key := teachingObjectKeyForOrdinal(ordinal)
		rows, ok := teachingAlternativeTemplates(key)
		if !ok {
			t.Fatalf("teaching object %03d has no alternatives", ordinal)
		}
		if len(rows) != count {
			t.Fatalf("teaching object %03d: got %d alternatives, want %d", ordinal, len(rows), count)
		}
		selected := 0
		for _, row := range rows {
			if len(row.Methods) == 0 || row.ForecastBase < 1000 || row.SelectionBasis == "" {
				t.Fatalf("incomplete scheme %s for teaching object %03d: %+v", row.Name, ordinal, row)
			}
			if row.Selected {
				selected++
			}
		}
		if selected != 1 {
			t.Fatalf("teaching object %03d: selected schemes=%d, want 1", ordinal, selected)
		}
	}
}

func TestRC700TeachingChoiceLogicCoversEffectCostAndAvailability(t *testing.T) {
	cases := []struct {
		ordinal int
		want    string
	}{
		{1, "максим"},
		{58, "миним"},
		{90, "доступ"},
	}
	for _, tc := range cases {
		rows, _ := teachingAlternativeTemplates(teachingObjectKeyForOrdinal(tc.ordinal))
		found := ""
		for _, row := range rows {
			if row.Selected {
				found = strings.ToLower(row.SelectionKind + " " + row.SelectionBasis)
				if row.Availability != scenarioAvailabilityReady {
					t.Fatalf("selected teaching scheme %03d is not available: %s", tc.ordinal, row.Availability)
				}
			}
		}
		if !strings.Contains(found, tc.want) {
			t.Fatalf("teaching object %03d choice logic %q does not contain %q", tc.ordinal, found, tc.want)
		}
	}
}

func TestRC700UnavailableAlternativesRemainVisible(t *testing.T) {
	for _, ordinal := range []int{1, 58, 90} {
		rows, _ := teachingAlternativeTemplates(teachingObjectKeyForOrdinal(ordinal))
		hasLimited := false
		for _, row := range rows {
			if row.Availability == scenarioAvailabilityMissing || row.Availability == scenarioAvailabilityPartial {
				hasLimited = true
				if row.AvailabilityNote == "" {
					t.Fatalf("limited scheme %03d/%s has no resource explanation", ordinal, row.Name)
				}
			}
		}
		if !hasLimited {
			t.Fatalf("teaching object %03d has no limited alternative for comparison", ordinal)
		}
	}
}
