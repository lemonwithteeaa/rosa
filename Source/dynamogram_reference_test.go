//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validReferenceCase() DynamogramReferenceCase {
	return DynamogramReferenceCase{
		ID:                     "REAL-001",
		SourceFile:             "samples/REAL-001.png",
		SourceSHA256:           strings.Repeat("a", 64),
		SourceKind:             "real_image",
		LiftType:               "УШГН",
		ExpectedClasses:        []string{"контрольный диагностический класс"},
		ExpectedInterpretation: "подтверждённая экспертом интерпретация",
		ExpectedHypotheses:     []string{"пример проверяемой гипотезы"},
		ExpertStatus:           "confirmed",
		ExpertRole:             "эксперт по эксплуатации УШГН",
		ReviewedAt:             "2026-09-15",
	}
}

func TestEmbeddedDynamogramReferenceManifestIsSafeDraft(t *testing.T) {
	manifest, err := loadEmbeddedDynamogramReferenceManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SetStatus != dynReferenceSetDraft || manifest.ThresholdStatus != dynThresholdsScreening || len(manifest.Cases) != 0 {
		t.Fatalf("unexpected initial reference manifest: %#v", manifest)
	}
}

func TestDynamogramThresholdsCannotBeApprovedBeforeQualification(t *testing.T) {
	manifest := DynamogramReferenceManifest{
		SchemaVersion:   1,
		SetID:           "DRAFT",
		SetStatus:       dynReferenceSetDraft,
		ThresholdStatus: dynThresholdsApproved,
		Cases:           []DynamogramReferenceCase{validReferenceCase()},
	}
	if err := validateDynamogramReferenceManifest(manifest); err == nil || !strings.Contains(err.Error(), "нельзя утвердить") {
		t.Fatalf("approval without qualification must be rejected, got %v", err)
	}
}

func TestDynamogramReferenceRejectsSyntheticAndUntraceableCases(t *testing.T) {
	for name, mutate := range map[string]func(*DynamogramReferenceCase){
		"synthetic": func(item *DynamogramReferenceCase) { item.SourceKind = "synthetic" },
		"no hash":   func(item *DynamogramReferenceCase) { item.SourceSHA256 = "" },
		"absolute":  func(item *DynamogramReferenceCase) { item.SourceFile = `C:\\temp\\case.png` },
		"no class":  func(item *DynamogramReferenceCase) { item.ExpectedClasses = nil },
	} {
		t.Run(name, func(t *testing.T) {
			item := validReferenceCase()
			mutate(&item)
			manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "TEST", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{item}}
			if err := validateDynamogramReferenceManifest(manifest); err == nil {
				t.Fatal("invalid reference case was accepted")
			}
		})
	}
}

func TestQualifiedDynamogramReferenceRequiresExpertConfirmation(t *testing.T) {
	item := validReferenceCase()
	item.ExpertStatus = "pending"
	item.ExpertRole = ""
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "QUALIFIED", SetStatus: dynReferenceSetQualified, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{item}}
	if err := validateDynamogramReferenceManifest(manifest); err == nil || !strings.Contains(err.Error(), "только подтверждённые") {
		t.Fatalf("pending case in qualified set must be rejected, got %v", err)
	}
}

func TestQualifiedDynamogramReferenceCanBeValidated(t *testing.T) {
	manifest := DynamogramReferenceManifest{
		SchemaVersion:       1,
		SetID:               "QUALIFIED",
		SetStatus:           dynReferenceSetQualified,
		ThresholdStatus:     dynThresholdsApproved,
		QualificationReport: "reports/qualification-1.txt",
		Cases:               []DynamogramReferenceCase{validReferenceCase()},
	}
	if err := validateDynamogramReferenceManifest(manifest); err != nil {
		t.Fatal(err)
	}
}

func TestDynamogramReferenceFilesVerifySHA256(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "samples"), 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte("real test export")
	sum := sha256.Sum256(data)
	if err := os.WriteFile(filepath.Join(root, "samples", "REAL-001.csv"), data, 0600); err != nil {
		t.Fatal(err)
	}
	item := validReferenceCase()
	item.SourceFile = "samples/REAL-001.csv"
	item.SourceKind = "digital_export"
	item.SourceSHA256 = hex.EncodeToString(sum[:])
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "FILES", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{item}}
	report := validateDynamogramReferenceFiles(manifest, root)
	if !report.FilesReady || !report.ReadyForQualification || report.Passed != 1 || report.HashMismatch != 0 {
		t.Fatalf("unexpected verified report: %#v", report)
	}
}

func TestDynamogramReferenceFilesReportMissingAndHashMismatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "samples"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "samples", "REAL-001.png"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	first := validReferenceCase()
	second := validReferenceCase()
	second.ID = "REAL-002"
	second.SourceFile = "samples/REAL-002.png"
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "BROKEN", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{first, second}}
	report := validateDynamogramReferenceFiles(manifest, root)
	if report.FilesReady || report.HashMismatch != 1 || report.Missing != 1 {
		t.Fatalf("missing and mismatch must be reported separately: %#v", report)
	}
	formatted := formatDynamogramReferenceValidationReport(report)
	for _, want := range []string{"SHA-256 не совпал: 1", "отсутствует: 1", "Готово к квалификации: нет"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted report does not contain %q: %s", want, formatted)
		}
	}
}

func TestDynamogramReferenceFilesReportUnregisteredFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "samples"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "samples", "UNKNOWN.png"), []byte("unregistered"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := DynamogramReferenceManifest{SchemaVersion: 1, SetID: "EMPTY", SetStatus: dynReferenceSetDraft, ThresholdStatus: dynThresholdsScreening, Cases: []DynamogramReferenceCase{}}
	report := validateDynamogramReferenceFiles(manifest, root)
	if report.FilesReady || len(report.UnexpectedFiles) != 1 || report.UnexpectedFiles[0] != "samples/UNKNOWN.png" {
		t.Fatalf("unregistered file must be explicit: %#v", report)
	}
}
