//go:build windows

package main

import "testing"

func TestLaboratorySelectedObjectKeysText(t *testing.T) {
	got := laboratorySelectedObjectKeysText("OBJ-2; OBJ-1\nOBJ-2,OBJ-3")
	if len(got) != 3 || got[0] != "obj-1" || got[1] != "obj-2" || got[2] != "obj-3" {
		t.Fatalf("%#v", got)
	}
}

func TestLaboratoryRC181GroupTrendSortingByValue(t *testing.T) {
	metric := laboratoryTrendMetric{Code: "ph", Label: "pH", Group: "corrosion", Key: "ph", Unit: "ед."}
	passes := []LabPassRecord{
		{PassID: "A", ObjectKey: "OBJ-A", Date: "01.09.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "7.0"}}, Units: map[string]map[string]string{}},
		{PassID: "B", ObjectKey: "OBJ-B", Date: "01.09.2026", Status: labPassSavedStatus, Groups: map[string]map[string]string{"corrosion": {"ph": "5.0"}}, Units: map[string]map[string]string{}},
	}
	asc := laboratoryGroupTrendPointsSorted([]string{"OBJ-A", "OBJ-B"}, metric, passes, 1)
	if len(asc) != 2 || asc[0].ObjectKey != "OBJ-B" || asc[1].ObjectKey != "OBJ-A" {
		t.Fatalf("ascending sort mismatch: %+v", asc)
	}
	desc := laboratoryGroupTrendPointsSorted([]string{"OBJ-A", "OBJ-B"}, metric, passes, 2)
	if len(desc) != 2 || desc[0].ObjectKey != "OBJ-A" || desc[1].ObjectKey != "OBJ-B" {
		t.Fatalf("descending sort mismatch: %+v", desc)
	}
}
