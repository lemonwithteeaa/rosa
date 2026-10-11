//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC538TeachingObjectCanBecomeGlobalSelection(t *testing.T) {
	lab, _ := os.ReadFile("laboratory.go")
	mainSource, _ := os.ReadFile("main.go")
	text := string(lab) + string(mainSource)
	for _, want := range []string{
		"approveOpenLaboratoryObject", "ЗАКРЕПИТЬ ЭТОТ ОБЪЕКТ",
		"Перезакрепить объект вне Главной?", "проверить, дополнить или обновить",
		"Saved laboratory and teaching records remain selectable",
		"pass.Status != labPassSavedStatus", "max32(140, desiredObjectH)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("teaching-object contract missing %q", want)
		}
	}
}

func TestRC538AssessmentUsesVisibleRecordAndExplainsInsufficientClass(t *testing.T) {
	mainSource, _ := os.ReadFile("main.go")
	assessment, _ := os.ReadFile("assessment_classification.go")
	text := string(mainSource) + string(assessment)
	for _, want := range []string{
		"labDrafts[groupCode]", "выбранная лабораторная запись",
		"данных недостаточно для класса", "approveOpenLaboratoryObject()",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("selected-laboratory classification contract missing %q", want)
		}
	}
	if appBuildVersion != "7.3.11.538-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}

func TestRC538FilledTeachingDataAssignsMethodClasses(t *testing.T) {
	tests := []struct {
		direction assessmentClassDirection
		values    map[string]string
		want      string
	}{
		{assessmentClassDirections[1], map[string]string{"corrosion_rate": "1.2"}, "К3"},
		{assessmentClassDirections[2], map[string]string{"viscosity": "210"}, "В3"},
		{assessmentClassDirections[3], map[string]string{"external_saturation_index": "1.2"}, "С2"},
		{assessmentClassDirections[5], map[string]string{"suspended": "501"}, "М3"},
		{assessmentClassDirections[0], map[string]string{"paraffins": "4", "asphaltenes": "2", "pour_point": "20"}, "А2"},
		{assessmentClassDirections[6], map[string]string{"hydrate_temp": "12", "sample_temp": "8"}, "Гд1"},
		{assessmentClassDirections[7], map[string]string{"pump_temp": "130"}, "Т2"},
	}
	for _, tc := range tests {
		record := &LabRecord{ObjectKey: "Учебный объект / Штатные бланки", GroupCode: tc.direction.LabGroup, Values: tc.values}
		got, _, reason := assessmentProgramClassFromRecord(tc.direction, record)
		if got != tc.want {
			t.Fatalf("%s: got %s, want %s; %s", tc.direction.Name, got, tc.want, reason)
		}
	}
}

func TestLaboratoryOffMainRepinWarningCoversFullWorkflow(t *testing.T) {
	warning := laboratoryOffMainRepinWarning("Учебный объект / Штатные бланки")
	for _, required := range []string{
		"НЕ С ГЛАВНОЙ",
		"всех вкладок",
		"проверить, дополнить или обновить",
		"лабораторию",
		"классы осложнений",
		"методы и мероприятия",
		"МРП и отказы",
		"экономику",
		"итоговое заключение",
	} {
		if !strings.Contains(warning, required) {
			t.Fatalf("warning must contain %q: %s", required, warning)
		}
	}
}
