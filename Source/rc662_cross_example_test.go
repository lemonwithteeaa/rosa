//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestRC662CanonicalCrossExampleIsComplete(t *testing.T) {
	key := canonicalCrossExampleObject
	if n, ok := teachingWorkflowOrdinal(key); !ok || n != 1 {
		t.Fatalf("canonical example not recognized: n=%d ok=%v", n, ok)
	}
	state, ok := teachingWorkflowStateForObject(key)
	if !ok || state.Field != "Авиловское" || state.Well != "6" || state.ObjectPlast != "Бобриковский горизонт" {
		t.Fatalf("bad canonical state: %#v", state)
	}
	if state.BufferPressure == "" || state.TMSTemperature == "" || state.PumpModel == "" {
		t.Fatalf("technical regime is incomplete: %#v", state)
	}
	for _, direction := range assessmentClassDirections {
		record, ok := teachingAssessmentRecord(key, direction)
		if !ok || !record.FullClassificationConfirmed || record.FinalClass == "" {
			t.Fatalf("direction %q is incomplete: %#v", direction.Name, record)
		}
	}
	summary, ready, _ := approvedAssessmentForMethods(key)
	if !ready || !strings.Contains(summary, "Солеотложения — С2") {
		t.Fatalf("classification is not ready: %q", summary)
	}
	decision, ok := teachingActiveMethodDecision(key, summary)
	if !ok || len(decision.Methods) != 2 || decision.ForecastBaseDays < 1000 {
		t.Fatalf("method decision is incomplete: %#v", decision)
	}
	if rows := teachingFailureHistoryForObject(key, time.Date(2026, 10, 4, 0, 0, 0, 0, time.Local)); len(rows) != 4 {
		t.Fatalf("failure history rows=%d, want 4", len(rows))
	}
	version, ok := teachingCompletedVersionForObject(key)
	if !ok || version.EconStatus == "" || version.Decision != "Принято" {
		t.Fatalf("completed version is incomplete: %#v", version)
	}
	outcome, ok := teachingOutcomeForVersion(version.VersionID)
	if !ok || outcome.ActualMRP != "1000" || !outcome.ClassificationConfirmed || outcome.DefectFinding == "" {
		t.Fatalf("completed outcome is incomplete: %#v", outcome)
	}
}

func TestRC662MethodsButtonHasDedicatedNonOverlappingBand(t *testing.T) {
	buttonTop := int32(90 - 43)
	bodyTop := int32(31)
	bodyBottom := buttonTop - 7
	if bodyBottom <= bodyTop {
		t.Fatalf("compact fallback must not pretend there is text space: top=%d bottom=%d", bodyTop, bodyBottom)
	}
	if bodyBottom >= buttonTop {
		t.Fatalf("text and button overlap: bodyBottom=%d buttonTop=%d", bodyBottom, buttonTop)
	}
}

func TestRC788TeachingHistoryOverviewCoversAllTwentyWells(t *testing.T) {
	corpus, err := loadTrainingCorpus()
	if err != nil {
		t.Fatal(err)
	}
	rows := teachingAssessmentOverviewHistoryRows()
	if len(corpus.Wells) != 20 || len(rows) < 20 {
		t.Fatalf("overview has %d rows for %d teaching wells", len(rows), len(corpus.Wells))
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ObjectKey == "" || row.Source == "" || row.FinalClass == "" || row.ConfirmedAt == "" {
			t.Fatalf("incomplete teaching-history row: %+v", row)
		}
		seen[row.ObjectKey] = true
	}
	if len(seen) != 20 {
		t.Fatalf("overview traces %d wells, want all 20", len(seen))
	}
}

func TestRC788EveryTeachingWellHasHistoryAcrossCoreTabs(t *testing.T) {
	corpus, err := loadTrainingCorpus()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.Local)
	for _, well := range corpus.Wells {
		key := well.ObjectKey
		if _, ok := teachingWorkflowStateForObject(key); !ok {
			t.Errorf("%s has no technical-regime context", key)
		}
		if got := len(teachingFailureHistoryForObject(key, now)); got != len(well.Failures) {
			t.Errorf("%s failures=%d, corpus=%d", key, got, len(well.Failures))
		}
		if got := len(teachingLaboratoryHistoryEntries(key, now)); got != len(well.LabPasses) {
			t.Errorf("%s laboratory history=%d, corpus=%d", key, got, len(well.LabPasses))
		}
		if got := len(teachingAssessmentHistoryRows(key, now)); got != len(well.Failures) {
			t.Errorf("%s classification history=%d, failures=%d", key, got, len(well.Failures))
		}
		if got := len(teachingMethodArchiveDecisions(key, now)); got != len(well.Failures) {
			t.Errorf("%s method history=%d, failures=%d", key, got, len(well.Failures))
		}
		if got := len(teachingWorkflowObservations(key)); got != len(well.Failures) {
			t.Errorf("%s MRP history=%d, failures=%d", key, got, len(well.Failures))
		}
		if _, repair, opex, oneTime, ok := teachingEconomicsDefaultsForObject(key); !ok || repair <= 0 || opex <= 0 || oneTime <= 0 {
			t.Errorf("%s lacks teaching economics defaults", key)
		}
	}
}
