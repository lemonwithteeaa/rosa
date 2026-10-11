//go:build windows

package main

import (
	"strings"
	"testing"
)

func referenceComparisonManifest() DynamogramReferenceManifest {
	item := validReferenceCase()
	item.ExpectedInterpretation = "возможное неполное заполнение насоса или газовое влияние"
	item.ExpectedHypotheses = []string{
		"возможное неполное заполнение насоса или газовое влияние",
		"неравномерное заполнение или нестабильность рабочего цикла",
	}
	return DynamogramReferenceManifest{SchemaVersion: 1, SetID: "REFERENCE-SET", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{item}}
}

func referenceComparisonRun() DynamogramReferenceRun {
	return DynamogramReferenceRun{
		SchemaVersion: 1, RunID: "RUN-1", SetID: "REFERENCE-SET",
		CreatedAt: "2026-09-15T12:00:00Z", ApplicationVersion: "checkpoint",
		ThresholdStatusAtRun: dynThresholdsScreening,
		Cases: []DynamogramReferenceRunCase{{
			ReferenceID: "REAL-001", AnalysisID: "DYN-1", SourceSHA256: strings.Repeat("a", 64), ProcessingStatus: "completed",
			ActualInterpretation: "Возможное неполное заполнение насоса или газовое влияние.",
			ActualHypotheses: []string{
				"Предварительная гипотеза: возможное неполное заполнение насоса или газовое влияние.\r\nОснование: тест",
				"Предварительная гипотеза: неравномерное заполнение или нестабильность рабочего цикла.\r\nОснование: тест",
			},
		}},
	}
}

func TestDynamogramReferenceComparisonExactMatch(t *testing.T) {
	report := compareDynamogramReferenceRun(referenceComparisonManifest(), referenceComparisonRun())
	if !report.ProtocolValid || !report.ComparisonComplete || !report.ReadyForExpertReview || report.ExactMatches != 1 || report.MatchedHypotheses != 2 || report.MissingHypotheses != 0 || report.UnexpectedHypotheses != 0 {
		t.Fatalf("unexpected exact comparison: %#v", report)
	}
}

func TestDynamogramReferenceComparisonListsMissingAndUnexpectedHypotheses(t *testing.T) {
	run := referenceComparisonRun()
	run.Cases[0].ActualInterpretation = "другая фактическая интерпретация"
	run.Cases[0].ActualHypotheses = []string{
		"возможное неполное заполнение насоса или газовое влияние",
		"неожиданная гипотеза",
	}
	report := compareDynamogramReferenceRun(referenceComparisonManifest(), run)
	if report.ExactMatches != 0 || report.InterpretationMatches != 0 || report.MatchedHypotheses != 1 || report.MissingHypotheses != 1 || report.UnexpectedHypotheses != 1 {
		t.Fatalf("differences must be counted separately: %#v", report)
	}
}

func TestDynamogramReferenceComparisonRejectsWrongSource(t *testing.T) {
	run := referenceComparisonRun()
	run.Cases[0].SourceSHA256 = strings.Repeat("b", 64)
	report := compareDynamogramReferenceRun(referenceComparisonManifest(), run)
	if report.ComparisonComplete || report.Compared != 0 || report.Cases[0].Status != "source_hash_mismatch" {
		t.Fatalf("wrong source must not be compared: %#v", report)
	}
}

func TestDynamogramReferenceComparisonShowsMissingAndUnknownRuns(t *testing.T) {
	run := referenceComparisonRun()
	run.Cases[0].ReferenceID = "UNKNOWN"
	report := compareDynamogramReferenceRun(referenceComparisonManifest(), run)
	if report.ComparisonComplete || len(report.UnexpectedRunIDs) != 1 || report.Cases[0].Status != "missing_run" {
		t.Fatalf("missing and unknown runs must be explicit: %#v", report)
	}
}

func TestEmptyDynamogramReferenceSetIsNotComparisonComplete(t *testing.T) {
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "EMPTY", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{}}
	run := DynamogramReferenceRun{SchemaVersion: 1, RunID: "EMPTY-RUN", SetID: "EMPTY", CreatedAt: "2026-09-15T12:00:00Z", ApplicationVersion: "checkpoint", ThresholdStatusAtRun: dynThresholdsScreening, Cases: []DynamogramReferenceRunCase{}}
	report := compareDynamogramReferenceRun(manifest, run)
	if !report.ProtocolValid || report.ComparisonComplete || report.ReadyForExpertReview {
		t.Fatalf("empty set must not be ready: %#v", report)
	}
}
