//go:build windows

package main

import (
	"fmt"
	"testing"
)

func TestRC620AllMethodsIncludesWholeCatalogWithoutDuplicates(t *testing.T) {
	recommended := []methodWorkflowSuggestion{{Method: assessmentMethodCandidate{Name: "Рекомендованный"}, Direction: "Соли", Classification: "С3", Severity: 3}}
	catalog := []MethodCatalogEntry{{Name: "Рекомендованный"}}
	for i := 0; i < 42; i++ {
		catalog = append(catalog, MethodCatalogEntry{ID: fmt.Sprint(i), Name: fmt.Sprintf("Справочный %02d", i)})
	}
	all := mergeAllMethodSuggestions(recommended, catalog)
	if len(all) != 43 {
		t.Fatalf("full catalog truncated or duplicated: %d", len(all))
	}
	if all[0].Method.Name != "Рекомендованный" || methodSuggestionOutsideCurrentProfile(all[0]) {
		t.Fatal("recommended method lost first position")
	}
	scores := map[string]MethodEffectAssessment{}
	for _, s := range all {
		if methodSuggestionOutsideCurrentProfile(s) {
			scores[s.Method.Name] = methodOutsideProfileAssessment(s)
		} else {
			scores[s.Method.Name] = MethodEffectAssessment{Applicability: 80}
		}
	}
	ranked := rankMethodSuggestionsWithLimit(all, scores, 0)
	if len(ranked) != 43 {
		t.Fatalf("all-method mode capped at %d", len(ranked))
	}
	for _, s := range ranked[1:] {
		a := scores[s.Method.Name]
		if a.Applicability != 0 || a.EffectQuantified {
			t.Fatal("unrelated catalog method received artificial applicability/effect")
		}
	}
}
func TestRC620RecommendedModeStillCapsCandidateWork(t *testing.T) {
	rows := make([]methodWorkflowSuggestion, 50)
	scores := map[string]MethodEffectAssessment{}
	for i := range rows {
		rows[i].Method.Name = fmt.Sprintf("M%02d", i)
		scores[rows[i].Method.Name] = MethodEffectAssessment{Applicability: i}
	}
	if got := len(rankMethodSuggestions(rows, scores)); got != 30 {
		t.Fatalf("recommended screen cap=%d", got)
	}
}
