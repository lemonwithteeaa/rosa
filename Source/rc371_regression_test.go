//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC371InfographicIsOpenedFromAccentButton(t *testing.T) {
	b, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"ID_LAB_OPEN_INFOGRAPHIC",
		"ОТКРЫТЬ ИНФОГРАФИКУ ОСЛОЖНЕНИЙ",
		"show(labInfluenceChart, false)",
		"show(labTrendChart, false)",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC371 infographic regression: missing %q", want)
		}
	}
}

func TestRC371AssessmentReadsUnifiedLabPass(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"repositoryLabPassesForObject(objectKey)",
		"pass.Status != labPassSavedStatus",
		"единая диагностика • ",
		"canonicalizeLaboratoryValues(group, values, units)",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC371 lab pass regression: missing %q", want)
		}
	}
}

func TestRC371BuildVersionIsNotRolledBack(t *testing.T) {
	if strings.TrimSpace(appBuildVersion) == "" || strings.Contains(appBuildVersion, "7.3.11.370-RC") {
		t.Fatalf("build version rolled back: %s", appBuildVersion)
	}
}
