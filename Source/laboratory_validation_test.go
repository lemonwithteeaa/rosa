//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLaboratoryValidationAcceptsCommaAndRejectsPhysicalError(t *testing.T) {
	ph := LabFieldDef{Key: "ph", Label: "pH", Unit: "ед."}
	if got := validateLaboratoryValue("corrosion", ph, "6,4"); got.State != labValueValid {
		t.Fatalf("comma decimal must be valid: %#v", got)
	}
	chlorides := LabFieldDef{Key: "chlorides", Label: "Хлориды", Unit: "мг/л"}
	if got := validateLaboratoryValue("corrosion", chlorides, "-1"); got.State != labValueError || !strings.Contains(got.Message, "неадекватным") {
		t.Fatalf("negative concentration must be rejected: %#v", got)
	}
}

func TestLaboratoryValidationSeparatesWarningFromError(t *testing.T) {
	ph := LabFieldDef{Key: "ph", Label: "pH", Unit: "ед."}
	if got := validateLaboratoryValue("corrosion", ph, "12"); got.State != labValueWarning {
		t.Fatalf("physical but implausible pH must warn: %#v", got)
	}
	if got := validateLaboratoryValue("corrosion", ph, "15"); got.State != labValueError {
		t.Fatalf("pH outside physical range must error: %#v", got)
	}
}

func TestLaboratoryValidationExcludesInvalidValues(t *testing.T) {
	group, _ := labGroupByCode("corrosion")
	clean, warnings, errors := validateLaboratoryGroup(group, map[string]string{"ph": "15", "chlorides": "12000"})
	if clean["ph"] != "" || clean["chlorides"] != "12000" || len(warnings) != 0 || len(errors) != 1 {
		t.Fatalf("unexpected validation result: clean=%#v warnings=%#v errors=%#v", clean, warnings, errors)
	}
}

func TestLaboratoryValidationChecksRelationshipsAndSums(t *testing.T) {
	solids, _ := labGroupByCode("solids")
	clean, _, errors := validateLaboratoryGroup(solids, map[string]string{"size_d50": "80", "size_max": "40", "carbonates": "70", "sulfates": "40"})
	if len(errors) != 2 || clean["size_d50"] != "" || clean["carbonates"] != "" {
		t.Fatalf("cross-field errors were not excluded: clean=%#v errors=%#v", clean, errors)
	}
}

func TestConclusionReportsRejectedValueWithoutUsingIt(t *testing.T) {
	group, _ := labGroupByCode("corrosion")
	text := buildLaboratoryConclusion(group, map[string]string{"ph": "15", "chlorides": "12000"})
	if !strings.Contains(text, "ОШИБКИ — ЗНАЧЕНИЯ НЕ ИСПОЛЬЗОВАНЫ") || strings.Contains(text, "pH — фактическое значение: 15") {
		t.Fatalf("invalid value leaked into calculation:\n%s", text)
	}
}

func TestHardnessWithoutScaleIsNotConfirmingEvidence(t *testing.T) {
	field := LabFieldDef{Key: "material_hardness_weld", Label: "Твёрдость сварного шва / ЗТВ", Unit: "HV/HRC"}
	got := validateLaboratoryValue("corrosion", field, "88.963")
	if got.State != labValueWarning || !strings.Contains(got.Message, "неоднозначно без шкалы") {
		t.Fatalf("ambiguous hardness must warn: %#v", got)
	}
	text := formatH2SCrackingAssessment(map[string]string{"material_hardness_weld": "88.963"})
	if !strings.Contains(text, "число введено без шкалы") || strings.Contains(text, "твёрдость сварного шва / ЗТВ = 88.963") {
		t.Fatalf("ambiguous hardness was used as evidence:\n%s", text)
	}
}

func TestPressureLabelsContainPressureSymbol(t *testing.T) {
	for _, group := range labGroups {
		for _, field := range group.Fields {
			if strings.Contains(field.Key, "pressure") || field.Key == "pco2" || field.Key == "ph2s" {
				if !strings.Contains(field.Label, "давление") || !strings.Contains(field.Label, "(Р") {
					t.Fatalf("pressure label is incomplete: %s = %q", field.Key, field.Label)
				}
			}
		}
	}
}

func TestLaboratoryRC151RejectsInvertedPressureProfile(t *testing.T) {
	g := LabGroupDef{Code: "scale", Fields: []LabFieldDef{{Key: "profile_pressure_min", Label: "P min", Unit: "МПа"}, {Key: "profile_pressure_max", Label: "P max", Unit: "МПа"}}}
	clean, _, errs := validateLaboratoryGroup(g, map[string]string{"profile_pressure_min": "20", "profile_pressure_max": "10"})
	if len(errs) == 0 || clean["profile_pressure_min"] != "" || clean["profile_pressure_max"] != "" {
		t.Fatalf("expected rejected inverted pressure profile: clean=%v errors=%v", clean, errs)
	}
}

func TestLaboratoryRC151RejectsPartialPressureAboveTotal(t *testing.T) {
	g := LabGroupDef{Code: "corrosion", Fields: []LabFieldDef{{Key: "pco2", Label: "pCO2", Unit: "МПа"}, {Key: "ph2s", Label: "pH2S", Unit: "МПа"}, {Key: "gas_absolute_pressure", Label: "P abs", Unit: "МПа abs"}}}
	clean, _, errs := validateLaboratoryGroup(g, map[string]string{"pco2": "2", "ph2s": "1", "gas_absolute_pressure": "2.5"})
	if len(errs) == 0 || clean["pco2"] != "" || clean["ph2s"] != "" {
		t.Fatalf("expected impossible partial pressures to be rejected: clean=%v errors=%v", clean, errs)
	}
}
