//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC560ClassificationHistoryIsPermanentAndCompact(t *testing.T) {
	b, err := os.ReadFile("assessment_classification.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, token := range []string{
		"show(assessDecisionHistoryLabel, true)",
		"show(assessDecisionHistoryList, true)",
		"historyListH := max32(86",
		"listH := int32(156)",
		"commentH := int32(46)",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing compact history contract %q", token)
		}
	}
}

func TestRC560StartupUsesOnlyEmbeddedRealPumpjack(t *testing.T) {
	b, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "pCreateDIBSection.Call") || !strings.Contains(s, "embeddedStartupPumpjackPNG") {
		t.Fatal("startup pumpjack is not decoded directly from embedded PNG")
	}
	b, err = os.ReadFile("startup_loading.go")
	if err != nil {
		t.Fatal(err)
	}
	s = string(b)
	if strings.Contains(s, "if !startupDrawApprovedPumpjack") {
		t.Fatal("vector fallback is still active")
	}
}

func TestRC560BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.560-RC" {
		t.Fatalf("unexpected build %q", appBuildVersion)
	}
}
