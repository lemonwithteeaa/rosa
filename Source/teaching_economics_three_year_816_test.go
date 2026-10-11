//go:build windows

package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestTeachingEconomicsThreeYear816(t *testing.T) {
	corpus, err := loadTrainingCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Wells) != 20 {
		t.Fatalf("wells: %d", len(corpus.Wells))
	}
	total := 0
	for _, well := range corpus.Wells {
		if len(well.Failures) != 3 {
			t.Fatalf("%s: %d events", well.ID, len(well.Failures))
		}
		for _, letter := range []string{"A", "B", "C"} {
			id := fmt.Sprintf("TRN-%02d-%s", well.Ordinal, letter)
			entries := teachingEconomicsHistory816(well.ObjectKey, id)
			if len(entries) != 3 {
				t.Fatalf("%s: %d snapshots", id, len(entries))
			}
			for i, point := range entries {
				event := well.Failures[i]
				if point.Date != event.Date || point.FailureID != event.ID || point.LabPassID != event.LabPassID {
					t.Fatalf("%s event %d: broken cross-section link", id, i)
				}
				if point.Result.Status != "PASS" || point.Result.FormulaVersion != baseEconomicsFormulaVersion {
					t.Fatalf("%s / %s: economic model rejected data: %v", id, event.ID, point.Result.Errors)
				}
				if !strings.Contains(event.ID, well.ID) {
					t.Fatalf("%s: wrong event ID", id)
				}
				total++
			}
			current, ok := teachingLatestEconomics816(well.ObjectKey, id)
			if !ok || current.Status != "PASS" {
				t.Fatalf("%s: no 36-month current result", id)
			}
		}
	}
	if total != 180 {
		t.Fatalf("historical results: %d", total)
	}
}
