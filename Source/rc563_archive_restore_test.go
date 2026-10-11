//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC563ArchiveClassificationRestoreContract(t *testing.T) {
	a, err := os.ReadFile("assessment_classification.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(a)
	for _, token := range []string{
		"restoreAssessmentDecisionSnapshot",
		"Текущая классификация по закреплённому объекту будет полностью очищена",
		"assessmentDecisionSnapshotAt",
		"RestoredFromArchive",
		"ProgramSourceAtDecision",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing archive restore token %q", token)
		}
	}
	m, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(m), "assessDecisionHistoryList") || !strings.Contains(string(m), "NM_DBLCLK") {
		t.Fatal("double-click history restore handler missing")
	}
}

func TestRC563StartupSmoothingContract(t *testing.T) {
	b, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "resizeContainOnBackgroundBilinear") {
		t.Fatal("bilinear startup pumpjack scaling missing")
	}
}
