//go:build windows

package main

import "testing"

func TestComplexWatercutUsesSavedContextWhenTechKeyDoesNotMatch(t *testing.T) {
	previousTech, previousSelected := techCurrent, selectedObjectKey
	defer func() { techCurrent, selectedObjectKey = previousTech, previousSelected }()
	techCurrent = nil
	selectedObjectKey = ""
	passes := []LabPassRecord{
		{ObjectKey: "Авиловское / 114 / бобриковский горизонт", Date: "01.08.2026", SharedValues: map[string]string{"watercut": "61,5"}},
		{ObjectKey: "Авиловское / 114 / бобриковский горизонт", Date: "01.09.2026", Groups: map[string]map[string]string{"corrosion": {"watercut": "64"}}},
	}
	values := complexMetricValues(passes, complexMetric{group: "corrosion", key: "watercut", unit: "%"})
	if len(values) != 2 || values[0].value != 61.5 || values[1].value != 64 {
		t.Fatalf("saved watercut context not shown: %#v", values)
	}
}

func TestComplexWatercutPrefersCurrentTechRegime(t *testing.T) {
	previousTech, previousSelected := techCurrent, selectedObjectKey
	defer func() { techCurrent, selectedObjectKey = previousTech, previousSelected }()
	techCurrent = []TechObjectState{{ObjectKey: "OBJ-1", WaterCut: "72.3", ImportedAt: "2026-09-20T12:00:00Z"}}
	selectedObjectKey = "OBJ-1"
	values := complexMetricValues([]LabPassRecord{{ObjectKey: "OBJ-1", Date: "01.09.2026", SharedValues: map[string]string{"watercut": "50"}}}, complexMetric{group: "corrosion", key: "watercut", unit: "%"})
	if len(values) != 1 || values[0].value != 72.3 || values[0].date != "20.09.2026" {
		t.Fatalf("current tech-regime watercut not preferred: %#v", values)
	}
}
