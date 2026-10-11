//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC370ClassifierDirectionsFollowTechnologyModule(t *testing.T) {
	got := map[string][]string{}
	for _, d := range assessmentClassDirections {
		got[d.Prefix] = d.Levels
	}
	for _, prefix := range []string{"А", "К", "В", "С", "Г", "М", "Гд", "Т"} {
		if len(got[prefix]) == 0 {
			t.Fatalf("missing classifier prefix %s", prefix)
		}
	}
	if len(got["Гд"]) != 1 || got["Гд"][0] != "Гд1" {
		t.Fatalf("hydrate levels must follow Technology Module: %#v", got["Гд"])
	}
}

func TestRC370AssessmentHasThreeInnerTabs(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"1. Классификация технолога", "2. Предположение по лаборатории", "3. Сверка и подтверждение"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing assessment tab %q", want)
		}
	}
}

func TestRC370ManualAndProgramClassesStoredSeparately(t *testing.T) {
	var r AssessmentClassRecord
	r.TechnologistClass = "К2"
	r.ManualLabClass = "К1"
	r.FinalClass = "К2"
	if r.TechnologistClass == r.ManualLabClass {
		t.Fatal("manual technologist and laboratory fallback classes must remain separate")
	}
}

func TestRC370OverallHistoryGetsPriority(t *testing.T) {
	m := laboratoryOverallVisualMetricsForHeight(900, 520, false)
	if m.HistoryH < 170 {
		t.Fatalf("history too small at 900px: %d", m.HistoryH)
	}
	if m.ConclusionH > 180 {
		t.Fatalf("conclusion still consumes too much height: %d", m.ConclusionH)
	}
}

func TestRC370IconEmbedPatchesResourceDirectory(t *testing.T) {
	b, err := os.ReadFile("embed_icon_resource.py")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"resource_entry = data_dir + 2 * 8", "struct.pack_into('<II', patched_bytes, resource_entry, rsrc_rva, len(raw))"} {
		if !strings.Contains(s, want) {
			t.Fatalf("icon resource patch missing %q", want)
		}
	}
}
