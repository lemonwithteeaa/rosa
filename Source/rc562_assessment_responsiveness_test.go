//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC562AssessmentCustomDrawDoesNotRecalculate(t *testing.T) {
	b, err := os.ReadFile("assessment_classification.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "func assessmentCompareCustomDraw")
	end := strings.Index(s[start:], "func assessmentDirectionCustomDraw")
	if start < 0 || end < 0 {
		t.Fatal("custom draw block not found")
	}
	block := s[start : start+end]
	if strings.Contains(block, "assessmentCompareRows()") || strings.Contains(block, "assessmentProgramClass(") || strings.Contains(block, "repositoryLabPassesForObject(") {
		t.Fatal("custom draw still performs assessment/storage recalculation")
	}
	for _, token := range []string{"assessmentCompareRowsCache", "listBeginUpdate(assessDecisionHistoryList)", "assessmentLabPassesForObjectCached"} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing responsiveness contract %q", token)
		}
	}
}

func TestRC562BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.562-RC" {
		t.Fatalf("unexpected version %q", appBuildVersion)
	}
}
