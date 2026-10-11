//go:build windows

package main

import (
	"strings"
	"testing"
)

func readyDynamogramQualificationFixture() (DynamogramReferenceManifest, DynamogramReferenceQualificationReport) {
	item := validReferenceCase()
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "QUAL", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{item}}
	files := DynamogramReferenceValidationReport{SetID: "QUAL", ManifestValid: true, FilesReady: true, ReadyForQualification: true, Total: 1, Passed: 1}
	comparison := DynamogramReferenceComparisonReport{SetID: "QUAL", RunID: "RUN", ProtocolValid: true, ComparisonComplete: true, ReadyForExpertReview: true, TotalReferences: 1, Compared: 1}
	aggregate := DynamogramReferenceAggregateReport{SetID: "QUAL", RunID: "RUN", ComparisonComplete: true, AllClassesCovered: true, ReadyForExpertAssessment: true}
	return manifest, buildDynamogramReferenceQualificationReport(manifest, files, comparison, aggregate)
}

func TestDynamogramQualificationReportRequiresAllThreeLayers(t *testing.T) {
	manifest, ready := readyDynamogramQualificationFixture()
	if !ready.ReadyForExpertDecision || ready.Status != "ready_for_expert_decision" {
		t.Fatalf("complete package must be ready for a manual decision: %#v", ready)
	}
	files := ready.FileValidation
	files.ReadyForQualification = false
	notReady := buildDynamogramReferenceQualificationReport(manifest, files, ready.Comparison, ready.Aggregate)
	if notReady.ReadyForExpertDecision || notReady.Status != "not_ready" {
		t.Fatalf("incomplete file validation must block decision: %#v", notReady)
	}
}

func TestDynamogramExpertDecisionCannotApproveThresholds(t *testing.T) {
	_, report := readyDynamogramQualificationFixture()
	hash, err := hashDynamogramReferenceQualificationReport(report)
	if err != nil {
		t.Fatal(err)
	}
	decision := DynamogramReferenceExpertDecision{DecisionID: "D-1", SetID: "QUAL", RunID: "RUN", CreatedAt: "2026-09-15", Author: "Эксперт", AuthorRole: "профильный специалист", Status: "reference_set_qualified", Rationale: "рассмотрены материалы", QualificationReportSHA256: hash, ThresholdDecision: "approved"}
	if err = validateDynamogramReferenceExpertDecision(decision, report); err == nil || !strings.Contains(err.Error(), "не может утверждать пороги") {
		t.Fatalf("threshold approval must be rejected, got %v", err)
	}
}

func TestDynamogramExpertDecisionRequiresReadyReportAndMatchingHash(t *testing.T) {
	manifest, report := readyDynamogramQualificationFixture()
	hash, _ := hashDynamogramReferenceQualificationReport(report)
	decision := DynamogramReferenceExpertDecision{DecisionID: "D-1", SetID: "QUAL", RunID: "RUN", CreatedAt: "2026-09-15", Author: "Эксперт", AuthorRole: "профильный специалист", Status: "reference_set_qualified", Rationale: "рассмотрены материалы", QualificationReportSHA256: hash, ThresholdDecision: dynThresholdDecisionNotMade}
	if err := validateDynamogramReferenceExpertDecision(decision, report); err != nil {
		t.Fatal(err)
	}
	report.ReadyForExpertDecision = false
	if err := validateDynamogramReferenceExpertDecision(decision, report); err == nil {
		t.Fatal("qualification on incomplete report must be rejected")
	}
	report = buildDynamogramReferenceQualificationReport(manifest, DynamogramReferenceValidationReport{SetID: "QUAL"}, DynamogramReferenceComparisonReport{SetID: "QUAL", RunID: "RUN", ProtocolValid: true}, DynamogramReferenceAggregateReport{SetID: "QUAL", RunID: "RUN"})
	decision.Status = "needs_more_data"
	decision.QualificationReportSHA256 = strings.Repeat("0", 64)
	if err := validateDynamogramReferenceExpertDecision(decision, report); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("wrong report hash must be rejected, got %v", err)
	}
}

func TestDynamogramExpertDecisionJournalRejectsDuplicateIDs(t *testing.T) {
	_, report := readyDynamogramQualificationFixture()
	hash, _ := hashDynamogramReferenceQualificationReport(report)
	decision := DynamogramReferenceExpertDecision{DecisionID: "D-1", SetID: "QUAL", RunID: "RUN", CreatedAt: "2026-09-15", Author: "Эксперт", AuthorRole: "профильный специалист", Status: "needs_more_data", Rationale: "нужны дополнительные случаи", QualificationReportSHA256: hash, ThresholdDecision: dynThresholdDecisionNotMade}
	journal := DynamogramReferenceExpertDecisionJournal{SchemaVersion: 1, SetID: "QUAL", Decisions: []DynamogramReferenceExpertDecision{decision, decision}}
	if err := validateDynamogramReferenceExpertDecisionJournal(journal, report); err == nil || !strings.Contains(err.Error(), "повторяется") {
		t.Fatalf("duplicate decision IDs must be rejected, got %v", err)
	}
}
