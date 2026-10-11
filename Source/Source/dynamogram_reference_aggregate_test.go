//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
)

func TestDynamogramReferenceAggregateCalculatesOverallAndClasses(t *testing.T) {
	first := validReferenceCase()
	first.ID = "R-1"
	first.ExpectedClasses = []string{"Класс А", "Общий класс"}
	first.ExpectedHypotheses = []string{"A1", "A2"}
	second := validReferenceCase()
	second.ID = "R-2"
	second.SourceFile = "samples/R-2.png"
	second.SourceSHA256 = strings.Repeat("b", 64)
	second.ExpectedClasses = []string{"Класс Б", "Общий класс"}
	second.ExpectedHypotheses = []string{"B1", "B2"}
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "AGG", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{first, second}}
	comparison := DynamogramReferenceComparisonReport{
		SetID: "AGG", RunID: "RUN", ProtocolValid: true, ComparisonComplete: true,
		Cases: []DynamogramReferenceComparisonCase{
			{ReferenceID: "R-1", Status: "compared", InterpretationMatch: true, MatchedHypotheses: []string{"A1", "A2"}},
			{ReferenceID: "R-2", Status: "compared", InterpretationMatch: false, MatchedHypotheses: []string{"B1"}, MissingHypotheses: []string{"B2"}, UnexpectedHypotheses: []string{"BX"}},
		},
	}
	report := aggregateDynamogramReferenceComparison(manifest, comparison)
	if !report.ComparisonComplete || !report.AllClassesCovered || !report.ReadyForExpertAssessment || report.Overall.ExactMatches != 1 || report.Overall.ExpectedHypotheses != 4 || report.Overall.MatchedHypotheses != 3 || report.Overall.MissingHypotheses != 1 || report.Overall.UnexpectedHypotheses != 1 {
		t.Fatalf("unexpected aggregate: %#v", report)
	}
	if math.Abs(report.Overall.HypothesisRecall-0.75) > 1e-9 || math.Abs(report.Overall.UnexpectedHypothesisShare-0.25) > 1e-9 || math.Abs(report.Overall.ExactMatchRate-0.5) > 1e-9 {
		t.Fatalf("unexpected rates: %#v", report.Overall)
	}
	if len(report.ByClass) != 3 {
		t.Fatalf("multi-class cases must create three class rows: %#v", report.ByClass)
	}
}

func TestDynamogramReferenceAggregateDoesNotInventUndefinedRates(t *testing.T) {
	metrics := DynamogramReferenceAggregateMetrics{TotalCases: 1}
	finalizeDynamogramReferenceMetrics(&metrics)
	if metrics.HypothesisRecallDefined || metrics.UnexpectedShareDefined || metrics.ExactMatchRateDefined || metrics.InterpretationRateDefined {
		t.Fatalf("zero denominators must remain undefined: %#v", metrics)
	}
}

func TestDynamogramReferenceAggregateIncompleteComparisonIsNotReady(t *testing.T) {
	item := validReferenceCase()
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "AGG", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{item}}
	comparison := DynamogramReferenceComparisonReport{SetID: "AGG", RunID: "RUN", ProtocolValid: true, ComparisonComplete: false, Cases: []DynamogramReferenceComparisonCase{{ReferenceID: item.ID, Status: "missing_run"}}}
	report := aggregateDynamogramReferenceComparison(manifest, comparison)
	if report.ComparisonComplete || report.ReadyForExpertAssessment || report.Overall.UnprocessedCases != 1 {
		t.Fatalf("incomplete comparison must not be ready: %#v", report)
	}
	formatted := formatDynamogramReferenceAggregateReport(report)
	if !strings.Contains(formatted, "Автоматического решения о порогах нет") || !strings.Contains(formatted, "не рассчитывается") {
		t.Fatalf("formatted report must show safeguards: %s", formatted)
	}
}
