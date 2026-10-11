//go:build windows

package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestLaboratoryRC147CoverageCountersUseOneInputSet(t *testing.T) {
	group, ok := labGroupByCode("scale")
	if !ok {
		t.Fatal("scale group missing")
	}
	values := map[string]string{
		"ca":               "100",
		"cl":               "500",
		"water_type_sulin": "Хлоркальциевый", // calculated: must never count
		"watercut":         "87",             // Tech Regime: must never count
		"mix2_ca_total":    "999",            // inactive outside mixing model
	}
	for _, model := range []string{"davies", "ideal", "multicomplex"} {
		active := laboratoryScaleInputGroup(group, model)
		bf, bt, of, ot := laboratoryDataCoverageForModel(group, values, model)
		hf, ht := laboratoryHistoryFieldCoverage(group, values, model)
		if bf+of != hf || bt+ot != ht {
			t.Fatalf("model=%s readiness=%d/%d history=%d/%d", model, bf+of, bt+ot, hf, ht)
		}
		if laboratoryLabValueCount(active, values) != hf {
			t.Fatalf("model=%s short-summary data count differs: got %d want %d", model, laboratoryLabValueCount(active, values), hf)
		}
		if strings.TrimSpace(values["water_type_sulin"]) != "" && laboratoryInputFieldEligible("scale", "water_type_sulin") {
			t.Fatal("calculated Sulin result became eligible input")
		}
		if laboratoryInputFieldEligible("scale", "watercut") {
			t.Fatal("Tech Regime watercut became eligible laboratory input")
		}
	}
}

func TestLaboratoryRC147GeneralAndExportCoverageMatchesHistory(t *testing.T) {
	group, ok := labGroupByCode("scale")
	if !ok {
		t.Fatal("scale group missing")
	}
	values := map[string]string{"ca": "100", "cl": "500", "water_type_sulin": "calculated", "watercut": "90", "mix2_ca_total": "999"}
	model := "davies"
	filled, total := laboratoryHistoryFieldCoverage(group, values, model)
	pass := LabPassRecord{
		PassID: "LP-RC147", ObjectKey: "OBJ", Date: "18.09.2026", Source: "TEST", ScaleModel: model,
		Groups: map[string]map[string]string{"scale": values}, Units: map[string]map[string]string{"scale": {}},
	}
	report := buildLaboratoryPassConclusion(pass)
	want := fmt.Sprintf("Заполнено показателей: %d из %d.", filled, total)
	if !strings.Contains(report, want) {
		t.Fatalf("general/export report coverage mismatch; want %q in:\n%s", want, report)
	}
}
