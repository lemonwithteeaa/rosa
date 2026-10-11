//go:build windows

package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRC342OverallLaboratoryConclusionKeepsUsefulHeight(t *testing.T) {
	for _, tc := range []struct {
		h, trendY, minConclusion int32
	}{{960, 520, 150}, {1080, 540, 190}, {1200, 560, 200}} {
		g := laboratoryOverallVisualMetricsForHeight(tc.h, tc.trendY, false)
		if g.ConclusionH < tc.minConclusion {
			t.Fatalf("h=%d conclusion too small: %d < %d", tc.h, g.ConclusionH, tc.minConclusion)
		}
		if g.TrendH > 240 {
			t.Fatalf("h=%d chart still dominates lower conclusion: trend=%d", tc.h, g.TrendH)
		}
		if g.HistoryH < 80 {
			t.Fatalf("h=%d previous-analysis area disappeared: %d", tc.h, g.HistoryH)
		}
	}
}

func TestRC342OverallTabUsesUserFacingName(t *testing.T) {
	found := false
	for _, g := range labGroups {
		if g.Code == "overall" {
			found = true
			if g.Name != "Общая оценка" {
				t.Fatalf("unexpected overall caption: %q", g.Name)
			}
		}
	}
	if !found {
		t.Fatal("overall laboratory tab missing")
	}
}

func useEmbeddedRC342Rules(t *testing.T) {
	t.Helper()
	var c RuleCatalog
	if err := json.Unmarshal(embeddedRuleSeed, &c); err != nil {
		t.Fatal(err)
	}
	ruleCatalog = c
}

func TestRC342AssessmentMethodsKeepExcelFirst(t *testing.T) {
	useEmbeddedRC342Rules(t)
	got := assessmentMethodCandidatesForGroup("scale")
	if len(got) < 3 {
		t.Fatalf("scale method list unexpectedly short: %d", len(got))
	}
	if !strings.Contains(strings.ToLower(got[0].Source), "технологичес") {
		t.Fatalf("Excel/Technology Module is not first source: %+v", got[0])
	}
}

func TestRC342AssessmentMethodsHaveNoDuplicates(t *testing.T) {
	useEmbeddedRC342Rules(t)
	for _, group := range []string{"corrosion", "scale", "aspo", "solids", "microbiology", "gases"} {
		seen := map[string]bool{}
		for _, m := range assessmentMethodCandidatesForGroup(group) {
			key := normalizeMethodText(m.Name)
			if seen[key] {
				t.Fatalf("duplicate method in %s: %s", group, m.Name)
			}
			seen[key] = true
		}
	}
}

func TestRC342OpenSourceAdditionsAreVisibleButNotAutomatic(t *testing.T) {
	useEmbeddedRC342Rules(t)
	checks := map[string]string{
		"scale":        "Медленно высвобождаемый ингибитор солеотложений",
		"microbiology": "Биоцидная обработка",
		"gases":        "Кинетический ингибитор гидратообразования",
		"aspo":         "снижения вязкости",
	}
	for group, want := range checks {
		found := false
		for _, m := range assessmentMethodCandidatesForGroup(group) {
			if strings.Contains(strings.ToLower(m.Name), strings.ToLower(want)) {
				found = true
				if m.Source == "" || m.Status == "" {
					t.Fatalf("%s addition lost provenance/status: %+v", group, m)
				}
			}
		}
		if !found {
			t.Fatalf("%s missing open-source addition %q", group, want)
		}
	}
}

func TestRC342EquipmentMethodsRedirectToAutotechnologist(t *testing.T) {
	for _, name := range []string{"Газосепаратор", "Коррозионностойкое ГНО", "УЭВН / винтовая насосная установка"} {
		if got := assessmentMethodStatus(name); !strings.Contains(got, "Автотехнолог") {
			t.Fatalf("%q must be routed to Autotechnologist, got %q", name, got)
		}
	}
}

func TestRC342ChemicalMethodsRequireSelection(t *testing.T) {
	for _, name := range []string{"Ингибитор солеотложений", "Деэмульгатор", "Биоцидная обработка"} {
		if got := assessmentMethodStatus(name); !strings.Contains(strings.ToLower(got), "лаборатор") {
			t.Fatalf("%q must require laboratory selection, got %q", name, got)
		}
	}
}

func TestRC342MainAssessmentUIContainsMethodDropdown(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"Методы борьбы", "ID_ASSESS_METHOD_COMBO", "populateAssessmentMethodCombo", "Нет данных ≠ нет осложнения"} {
		if !strings.Contains(s, want) {
			t.Fatalf("assessment UI lost %q", want)
		}
	}
}

func TestRC342VersionBumped(t *testing.T) {
	if !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
