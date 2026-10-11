//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC365Stage1OverallLaboratoryPrioritizesText(t *testing.T) {
	low := laboratoryOverallVisualMetricsForHeight(768, 470, false)
	if low.ShowTrend {
		t.Fatal("768p: trend must yield space to conclusion/history")
	}
	if low.ConclusionH < 84 {
		t.Fatalf("768p: conclusion collapsed: %d", low.ConclusionH)
	}
	if low.HistoryH < 90 {
		t.Fatalf("768p: history collapsed: %d", low.HistoryH)
	}
	high := laboratoryOverallVisualMetricsForHeight(1080, 500, false)
	if high.ConclusionH < 180 || high.HistoryH < 120 {
		t.Fatalf("1080p: conclusion/history too small: %+v", high)
	}
}

func TestRC365Stage2AssessmentMethodSelectionIsPerObjectAndDirection(t *testing.T) {
	if assessmentMethodSelectionKey("A", "scale") == assessmentMethodSelectionKey("A", "corrosion") {
		t.Fatal("method selection key must separate complication directions")
	}
	if assessmentMethodSelectionKey("A", "scale") == assessmentMethodSelectionKey("B", "scale") {
		t.Fatal("method selection key must separate objects")
	}
}

func TestRC365Stage2DraftDoesNotClaimAutomaticEffect(t *testing.T) {
	plans = []PlanItem{{ObjectKey: "OBJ", Action: "Проверить применимость метода: Ингибитор солеотложений", Status: "Черновик из оценки осложнений — требует подтверждения"}}
	text := assessmentDraftSummaryForObject("OBJ")
	for _, want := range []string{"не передаются в экономику", "не изменяют прогноз МРП", "подтвердит применимость"} {
		if !strings.Contains(text, want) {
			t.Fatalf("draft guard lost %q: %s", want, text)
		}
	}
}

func TestRC365Stage3PrimaryWorkflowKeepsArchiveAndTwoEconomies(t *testing.T) {
	mainBytes, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	mainText := string(mainBytes)
	if !strings.Contains(mainText, `ID_ARCHIVE, "8. Утверждённый реестр"`) {
		t.Fatal("archive button lost")
	}
	if !strings.Contains(mainText, `ID_RESULT, ID_ARCHIVE, ID_OPI`) {
		t.Fatal("archive button is visible but not wired to page switching")
	}
	if !strings.Contains(mainText, `case ID_ARCHIVE:`) || !strings.Contains(mainText, `setPos(archiveList`) {
		t.Fatal("archive page lost adaptive layout")
	}
	wf, err := os.ReadFile("workflow_pages.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(wf)
	for _, want := range []string{`"Базовая"`, `"Расширенная"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("economics subsection lost %s", want)
		}
	}
}

func TestRC365BuildVersion(t *testing.T) {
	if !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
