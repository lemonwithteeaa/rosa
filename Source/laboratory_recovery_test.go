//go:build windows

package main

import "testing"

func TestRecoveredSharedLaboratoryLinks(t *testing.T) {
	want := map[string]string{
		"corrosion/chlorides": "chlorides",
		"scale/cl":            "chlorides",
		"corrosion/co2":       "dissolved_co2",
		"gases/co2":           "dissolved_co2",
		"scale/pump_temp":     "pump_temp",
		"gases/pump_temp":     "pump_temp",
	}
	for path, expected := range want {
		var group, key string
		for i, r := range path {
			if r == '/' {
				group, key = path[:i], path[i+1:]
				break
			}
		}
		if got := laboratorySharedKey(group, key); got != expected {
			t.Fatalf("%s: got %q, want %q", path, got, expected)
		}
	}
}

func TestRecoveredExtendedLaboratoryInventory(t *testing.T) {
	minimum := map[string]int{"corrosion": 20, "scale": 20, "aspo": 12, "solids": 15, "microbiology": 14, "gases": 25}
	for group, count := range minimum {
		definition, ok := labGroupByCode(group)
		if !ok || len(definition.Fields) < count {
			t.Fatalf("group %s has %d fields; expected at least %d", group, len(definition.Fields), count)
		}
	}
}

func TestSharedCollectionDoesNotMergeNonEquivalentWater(t *testing.T) {
	groups := map[string]map[string]string{
		"aspo":      {"water": "7", "watercut": "48"},
		"corrosion": {"watercut": "48"},
	}
	shared := collectLaboratorySharedValues(groups)
	if shared["watercut"] != "48" {
		t.Fatalf("watercut was not shared: %#v", shared)
	}
	if _, exists := shared["water"]; exists {
		t.Fatalf("ASPO sample water must remain independent: %#v", shared)
	}
}
