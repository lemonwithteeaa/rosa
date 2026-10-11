//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLAB90ScenarioMatrixCoversAllSixDirections(t *testing.T) {
	scenarios := laboratoryLAB90Scenarios()
	if len(scenarios) != 6 {
		t.Fatalf("LAB-90 scenario matrix has %d scenarios, want 6", len(scenarios))
	}
	if issues := laboratoryLAB90ScenarioQAIssues(); len(issues) != 0 {
		t.Fatalf("LAB-90 scenario QA failed: %v", issues)
	}
}

func TestLAB90RuntimeDiagnosticsPreserveCriticalLimitations(t *testing.T) {
	text := laboratoryLAB90RuntimeDiagnosticsText()
	for _, want := range []string{
		"6/6",
		"PHREEQC golden benchmark",
		"оптимальная доза автоматически не назначается",
		"Нативные Windows DPI/regression/fault/PDF проверки",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("LAB-90 diagnostics lost %q: %s", want, text)
		}
	}
}
