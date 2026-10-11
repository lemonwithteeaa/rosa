//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestMixingExplanationDistinguishesUnknownAndMeasuredZero(t *testing.T) {
	v := mixingInputs()
	for _, key := range []string{"ba", "mix2_ba", "sr", "mix2_sr"} {
		delete(v, key)
	}
	text := laboratoryWaterMixingText(v)
	for _, want := range []string{"Барит (сульфат бария): Не хватает анализа", "Целестин (сульфат стронция): Не хватает анализа"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing unknown-element status", want)
		}
	}
	for _, key := range []string{"ba", "mix2_ba", "sr", "mix2_sr"} {
		v[key] = "0"
	}
	text = laboratoryWaterMixingText(v)
	if strings.Contains(text, "Не хватает анализа") {
		t.Fatal("measured zero treated as missing")
	}
	if strings.Count(text, "численный SI этой соли не выдан") != 2 {
		t.Fatal("zero-component minerals disappeared", text)
	}
	v["ba"] = mixingInputs()["ba"]
	delete(v, "mix2_ba")
	text = laboratoryWaterMixingText(v)
	if !strings.Contains(text, "Расчёт частичный: барий") {
		t.Fatal("partial barite SI not qualified")
	}
}

func TestOverallSummaryIncludesMixtureAndCanonicalUnits(t *testing.T) {
	v := mixingInputs()
	v["mix2_ca_total"] = "0.120234"
	pass := LabPassRecord{ScaleModel: "water_mixing", Groups: map[string]map[string]string{"scale": v}, Units: map[string]map[string]string{"scale": {"mix2_ca_total": "г/л"}}}
	text := laboratoryPassTechnologistSummary(pass)
	group, _ := labGroupByCode("scale")
	expected := laboratoryWaterMixingText(canonicalizeLaboratoryValues(laboratoryScaleInputGroup(group, pass.ScaleModel), v, pass.Units["scale"]))
	if !strings.Contains(text, expected) {
		t.Fatal("overall summary differs from canonical mixing result")
	}
	if !strings.Contains(text, "СОВМЕСТИМОСТЬ ДВУХ ВОД В ОБЩЕМ ВЫВОДЕ") {
		t.Fatal("mixture absent from overall explanation")
	}
	if v["mix2_ca_total"] != "0.120234" {
		t.Fatal("stored input changed")
	}
	v["mix_ph"] = ""
	text = laboratoryPassTechnologistSummary(pass)
	if !strings.Contains(text, "Расчёт не выполнен") || !strings.Contains(text, "совместимость вод оценить нельзя") {
		t.Fatal("missing pH not explained in overall summary")
	}
	pass.ScaleModel = "multicomplex_molal"
	text = laboratoryPassTechnologistSummary(pass)
	if strings.Contains(text, "СОВМЕСТИМОСТЬ ДВУХ ВОД В ОБЩЕМ ВЫВОДЕ") {
		t.Fatal("mixing explanation leaked into single-water model")
	}
}

func TestSavedMixingRecordUsesOwnModelAndUnits(t *testing.T) {
	v := mixingInputs()
	v["ph"] = "7" // Retained input of another model must not count.
	v["mix2_ca_total"] = "0.120234"
	record := LabRecord{GroupCode: "scale", ScaleModel: "water_mixing", Values: v, Units: map[string]string{"mix2_ca_total": "г/л"}}
	group, _ := labGroupByCode("scale")
	text := laboratoryRecordPracticalExplanation(group, record)
	canonical := canonicalizeLaboratoryValues(laboratoryScaleInputGroup(group, record.ScaleModel), v, record.Units)
	if !strings.Contains(text, laboratoryWaterMixingText(canonical)) {
		t.Fatal("saved mixing report lost canonical calculation")
	}
	if labRecordFilled(record) != 28 {
		t.Fatal("inactive input counted", labRecordFilled(record))
	}
	record.ScaleModel = "multicomplex_molal"
	if strings.Contains(laboratoryRecordPracticalExplanation(group, record), "СОВМЕСТИМОСТЬ ДВУХ ВОД ИЗ СОХРАНЁННОЙ ЗАПИСИ") {
		t.Fatal("single-water record uses mixing block")
	}
}

func TestOpeningSavedPassCreatesIndependentDraft(t *testing.T) {
	source := LabPassRecord{PassID: "SAVED-1", ObjectKey: "TEST", Date: "17.09.2026", Status: labPassSavedStatus, ScaleModel: "water_mixing", Groups: map[string]map[string]string{"scale": mixingInputs()}, Units: map[string]map[string]string{"scale": {"mix2_ca_total": "г/л"}}, SharedValues: map[string]string{"watercut": "70"}, Completed: []string{"scale"}, Conclusion: "Original conclusion", CreatedAt: "old", UpdatedAt: "old"}
	draft, err := laboratoryDraftFromSavedPass(source, time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if draft.PassID == source.PassID || draft.BasedOn != source.PassID || draft.Status != labPassDraftStatus || draft.Conclusion != "" || draft.ScaleModel != source.ScaleModel {
		t.Fatal("incorrect draft identity")
	}
	draft.Groups["scale"]["mix_ph"] = "9"
	draft.Units["scale"]["mix2_ca_total"] = "мг/л"
	draft.SharedValues["watercut"] = "80"
	draft.Completed[0] = "changed"
	if source.Groups["scale"]["mix_ph"] != mixingInputs()["mix_ph"] || source.Units["scale"]["mix2_ca_total"] != "г/л" || source.SharedValues["watercut"] != "70" || source.Completed[0] != "scale" || source.Conclusion != "Original conclusion" {
		t.Fatal("saved pass mutated")
	}
	source.ObjectKey = ""
	if _, err := laboratoryDraftFromSavedPass(source, time.Now()); err == nil {
		t.Fatal("invalid source accepted")
	}
}
