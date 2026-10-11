//go:build windows

package main

import (
	"strings"
	"testing"
)

func inhibitorPass(values map[string]string) LabPassRecord {
	return LabPassRecord{PassID: "P1", ObjectKey: "OBJ-1", Date: "01.09.2026", Source: "LAB-1", Status: labPassSavedStatus,
		Groups: map[string]map[string]string{"corrosion": values}, Units: map[string]map[string]string{"corrosion": {}}, ScaleModel: "davies"}
}

func TestLaboratoryInhibitorTrialRequiresActualEndpoint(t *testing.T) {
	pass := inhibitorPass(map[string]string{"inhibitor_trial_brand": "R1", "inhibitor_trial_target": "коррозия", "inhibitor_trial_dose": "25"})
	if _, ok := laboratoryInhibitorTrialFromPass(pass, "corrosion"); ok {
		t.Fatal("brand+dose without endpoint must not become trial")
	}
}

func TestLaboratoryInhibitorTrialDirectEfficiency(t *testing.T) {
	pass := inhibitorPass(map[string]string{"inhibitor_trial_brand": "R1", "inhibitor_trial_target": "коррозия", "inhibitor_trial_dose": "25", "inhibitor_trial_temp": "60", "inhibitor_trial_duration_h": "24", "inhibitor_trial_efficiency": "75", "inhibitor_trial_protocol": "P-77"})
	trial, ok := laboratoryInhibitorTrialFromPass(pass, "corrosion")
	if !ok || !trial.HasEfficiency || trial.Efficiency != 75 || trial.Dose != 25 {
		t.Fatalf("unexpected trial: %+v ok=%v", trial, ok)
	}
}

func TestLaboratoryInhibitorTrialCanDeriveCorrosionPairEfficiency(t *testing.T) {
	pass := inhibitorPass(map[string]string{"inhibitor_trial_brand": "R1", "inhibitor_trial_target": "коррозия", "inhibitor_trial_dose": "50", "corrosion_rate_without": "1", "corrosion_rate_with": "0.2"})
	trial, ok := laboratoryInhibitorTrialFromPass(pass, "corrosion")
	if !ok || !trial.EfficiencyDerived || trial.Efficiency < 79.9 || trial.Efficiency > 80.1 {
		t.Fatalf("derived efficiency lost: %+v ok=%v", trial, ok)
	}
}

func TestLaboratoryDoseResponseUsesOnlyMeasuredPointsAndNoOptimum(t *testing.T) {
	trials := []laboratoryInhibitorTrial{{GroupCode: "scale", Brand: "S1", Target: "барит", Dose: 10, HasDose: true, Efficiency: 40, HasEfficiency: true}, {GroupCode: "scale", Brand: "S1", Target: "барит", Dose: 20, HasDose: true, Efficiency: 70, HasEfficiency: true}}
	text := laboratoryInhibitorDoseResponseText(trials)
	for _, want := range []string{"10 мг/л → 40 %", "20 мг/л → 70 %", "оптимальная дозы не рассчитываются"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "рекомендуемая доза =") || strings.Contains(strings.ToLower(text), "оптимальная доза:") {
		t.Fatalf("invented recommendation: %s", text)
	}
}

func TestLaboratoryInhibitorDatabaseFiltersByObject(t *testing.T) {
	a := inhibitorPass(map[string]string{"inhibitor_trial_brand": "R1", "inhibitor_trial_target": "коррозия", "inhibitor_trial_dose": "25", "inhibitor_trial_efficiency": "75"})
	b := a
	b.PassID = "P2"
	b.ObjectKey = "OTHER"
	b.Groups = cloneLabGroups(a.Groups)
	got := laboratoryInhibitorTrialsForObject("OBJ-1", []LabPassRecord{a, b}, nil)
	if len(got) != 1 || got[0].ObjectKey != "OBJ-1" {
		t.Fatalf("unexpected filtered trials: %+v", got)
	}
}

func TestLaboratoryPassConclusionIncludesInhibitorTrialDatabase(t *testing.T) {
	pass := inhibitorPass(map[string]string{"ph": "7", "chlorides": "1000", "inhibitor_trial_brand": "R1", "inhibitor_trial_target": "коррозия", "inhibitor_trial_dose": "25", "inhibitor_trial_efficiency": "75", "inhibitor_trial_protocol": "P-77"})
	text := buildLaboratoryPassConclusion(pass)
	for _, want := range []string{"БАЗА ФАКТИЧЕСКИХ ИСПЫТАНИЙ ИНГИБИТОРОВ", "R1", "доза 25 мг/л"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestLAB90BestDoseResponseSeriesRequiresComparablePoints(t *testing.T) {
	trials := []laboratoryInhibitorTrial{
		{GroupCode: "corrosion", Brand: "R1", Target: "коррозия", Dose: 10, Efficiency: 50, HasDose: true, HasEfficiency: true, Date: "01.09.2026"},
		{GroupCode: "corrosion", Brand: "R1", Target: "коррозия", Dose: 20, Efficiency: 70, HasDose: true, HasEfficiency: true, Date: "02.09.2026"},
		{GroupCode: "corrosion", Brand: "R2", Target: "коррозия", Dose: 30, Efficiency: 90, HasDose: true, HasEfficiency: true, Date: "03.09.2026"},
	}
	series, ok := laboratoryBestInhibitorDoseSeries(trials)
	if !ok || series.Brand != "R1" || len(series.Points) != 2 {
		t.Fatalf("unexpected comparable series: %+v ok=%v", series, ok)
	}
	if series.Points[0].Dose != 10 || series.Points[1].Dose != 20 {
		t.Fatalf("dose points are not ordered: %+v", series.Points)
	}
}

func TestLAB90DoseResponsePanelNeverClaimsOptimum(t *testing.T) {
	series := laboratoryInhibitorDoseSeries{GroupCode: "scale", Brand: "R1", Target: "барит", Points: []laboratoryInhibitorTrial{{Dose: 10, Efficiency: 40}, {Dose: 20, Efficiency: 60}}}
	caption := laboratoryInhibitorDoseSeriesCaption(series)
	if strings.Contains(strings.ToLower(caption), "оптим") {
		t.Fatalf("visual caption must not claim optimum: %q", caption)
	}
	if got := laboratoryDoseResponsePanelHeight(series, 360); got < 100 {
		t.Fatalf("dose-response panel too short for normal layout: %d", got)
	}
}
