//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC543ReleaseBuildUsesCanonicalApplicationIcon(t *testing.T) {
	source, err := os.ReadFile("build_windows_gui.ps1")
	if err != nil {
		t.Fatal(err)
	}
	line := `embed_icon_resource.py") $rawExe (Join-Path $PSScriptRoot "OF_Model_icon_round.ico") $finalExe`
	if !strings.Contains(string(source), line) {
		t.Fatal("release build no longer uses the canonical OF_Model icon")
	}
	if appBuildVersion == "7.3.11.543-RC" || !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") {
		t.Fatalf("unexpected or rolled-back build version %q", appBuildVersion)
	}
}

func TestRC543SingleInfographicHasNoTrendGraph(t *testing.T) {
	source, err := os.ReadFile("laboratory_infographic_dialog.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "chartH := int32(310)") || !strings.Contains(text, "drawLaboratoryTrendChart(di)") {
		t.Fatal("single infographic must preserve the approved RC502 card layout")
	}
	if strings.Contains(text, `modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER|SS_OWNERDRAW, 24, 326, 1110, 344, parent, idLabInfographicTrend)`) {
		t.Fatal("single-object trend graph was restored")
	}
}

func TestRC543ComplexAnalysisDoesNotBlockWindowThread(t *testing.T) {
	source, err := os.ReadFile("laboratory_complex_analysis.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"wmComplexAnalysisDone", "complexAnalysisBusy", "go func()",
		"Выполняется комплексный анализ выбранных диагностик. Окно остаётся доступным.",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("non-blocking complex analysis contract missing %q", required)
		}
	}
}
