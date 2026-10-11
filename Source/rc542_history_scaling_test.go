//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC542ComplexAnalysisUsesOneHistorySnapshot(t *testing.T) {
	source, err := os.ReadFile("laboratory_complex_analysis.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"prepareComplexAnalysisIndex()",
		"complexCaptionIndex = readableObjectKeyIndex()",
		"complexPassIndex = make(map[string][]LabPassRecord)",
		"complexPasses = append(complexPasses, complexPassIndex[normalizeLaboratoryObjectKey(key)]...)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("RC542 indexed complex-analysis contract missing %q", required)
		}
	}
	loadStart := strings.Index(text, "func complexLoadObjects(")
	findStart := strings.Index(text, "func complexFind(")
	if loadStart < 0 || findStart <= loadStart {
		t.Fatal("complex-analysis function boundaries not found")
	}
	if strings.Contains(text[loadStart:findStart], "repositoryLabPassesForObject") {
		t.Fatal("complexLoadObjects must not query the repository once per selected object")
	}
}

func TestRC542LaboratoryHistoryReusesCaptionIndex(t *testing.T) {
	source, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"captions := readableObjectKeyIndex()",
		"laboratoryHistoryEntryColumnsWithCaptions(labHistoryEntries[i], captions)",
		"readableObjectKeyFromIndex(pass.ObjectKey, captions)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("RC542 indexed history contract missing %q", required)
		}
	}
}

func TestRC542BuildVersionWasNotRolledBack(t *testing.T) {
	if appBuildVersion == "7.3.11.542-RC" || !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") {
		t.Fatalf("unexpected or rolled-back build version %q", appBuildVersion)
	}
}
