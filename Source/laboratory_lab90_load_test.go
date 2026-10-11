//go:build windows

package main

import (
	"fmt"
	"testing"
	"time"
)

func TestLAB90MassHistoryRegressionAtOneHundredThousandPasses(t *testing.T) {
	const objects = 2000
	const perObject = 50
	passes := make([]LabPassRecord, 0, objects*perObject)
	keys := make([]string, 0, objects)
	for i := 0; i < objects; i++ {
		key := fmt.Sprintf("OBJ-%05d", i)
		keys = append(keys, key)
		for j := 0; j < perObject; j++ {
			passes = append(passes, LabPassRecord{
				PassID: fmt.Sprintf("P-%05d-%02d", i, j), ObjectKey: key, Status: labPassSavedStatus,
				Date:      fmt.Sprintf("2026-%02d-%02d", 1+(j/28)%9, 1+j%28),
				UpdatedAt: fmt.Sprintf("2026-09-18T20:%02d:%02dZ", j%60, i%60),
				Groups:    map[string]map[string]string{"corrosion": {"ph": fmt.Sprintf("%.2f", 6.5+float64(j%10)/10)}},
				Units:     map[string]map[string]string{"corrosion": {}},
			})
		}
	}
	latest := laboratoryLatestSavedPassesForKeys(keys, passes)
	if len(latest) != objects {
		t.Fatalf("100k latest selection returned %d, want %d", len(latest), objects)
	}
	analytics := laboratoryMassAnalyticsForKeys(keys, latest, time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local))
	if analytics.ScopeObjects != objects {
		t.Fatalf("mass analytics lost objects: %+v", analytics)
	}
}
