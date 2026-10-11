//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC541StartupTeachingSetUsesOneDurableBatch(t *testing.T) {
	demo, err := os.ReadFile("demo_seed.go")
	if err != nil {
		t.Fatal(err)
	}
	pass, err := os.ReadFile("laboratory_pass.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(demo) + string(pass) + string(repository)
	for _, required := range []string{
		"beginLabPassBulkWrite()", "flushLabPassBulkWrite()",
		"durableIDs[receipt.PassID]", "bundledDemoPassSnapshot()",
		"SaveBatch(passes []LabPassRecord)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("RC541 batch/recovery contract missing %q", required)
		}
	}
}

func TestRC541AssessmentPrefersExactOpenTeachingPass(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(mainSource)
	for _, required := range []string{
		"labPassDrafts[normalizeLaboratoryObjectKey(activeObject)+\"|\"+labActivePassID]",
		"открытая лабораторная запись",
		"canonicalizeLaboratoryValues",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("RC541 open-pass classification contract missing %q", required)
		}
	}
}

func TestRC541BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.541-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
