//go:build windows

package main

import (
	"fmt"
	"testing"
	"time"
)

func TestRC618StaleWorkerCannotUpdateAnotherSessionOrObject(t *testing.T) {
	r := &methodsBackgroundResult{token: 7, key: "OBJ"}
	if !methodsBackgroundResultIsCurrent(r, 7, 7, " OBJ ") {
		t.Fatal("current result rejected")
	}
	for _, c := range []struct {
		token, generation uint64
		key               string
	}{{6, 7, "OBJ"}, {7, 8, "OBJ"}, {7, 7, "OTHER"}} {
		if methodsBackgroundResultIsCurrent(r, c.token, c.generation, c.key) {
			t.Fatalf("stale result accepted: %+v", c)
		}
	}
	if methodsBackgroundResultIsCurrent(nil, 7, 7, "OBJ") {
		t.Fatal("missing result accepted")
	}
}
func TestRC618ProbabilityUsesImmutableIndexedOutcomes(t *testing.T) {
	records := []VersionRecord{}
	facts := []Outcome{}
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("v%d", i)
		records = append(records, VersionRecord{VersionID: id, ObjectKey: fmt.Sprintf("obj%d", i%3), Method: "Ингибитор солеотложений", Complication: "Солеотложения", Equipment: "УЭЦН"})
		facts = append(facts, Outcome{VersionID: id, EffectAttribution: "Подтверждено специалистом", ObservedEffect: "Наблюдается улучшение"})
	}
	// Preserve outcomeFor's first matching record semantics.
	facts = append(facts, Outcome{VersionID: "v0", EffectAttribution: "Подтверждено специалистом", ObservedEffect: "Наблюдается ухудшение"})
	available, _, _, _, n, success, objects := empiricalMethodProbabilityFromSnapshot("Ингибитор солеотложений", "Солеотложения", "УЭЦН", records, indexMethodOutcomes(facts))
	if !available || n != 6 || success != 6 || objects != 3 {
		t.Fatalf("unexpected cohort %v %d %d %d", available, n, success, objects)
	}
	available, _, _, _, n, _, _ = empiricalMethodProbabilityFromSnapshot("Ингибитор солеотложений", "Солеотложения", "УШГН", records, indexMethodOutcomes(facts))
	if available || n != 0 {
		t.Fatal("pooled incompatible equipment")
	}
}
func TestRC618ObjectBaselineWindowAndFallback(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	observations := []MRPObservation{{MRPDays: 500, Date: now.AddDate(0, -10, 0)}, {MRPDays: 700, Date: now.AddDate(0, -20, 0)}, {MRPDays: 9000, Date: now.AddDate(-4, 0, 0)}, {MRPDays: 9000, Date: now.AddDate(0, 1, 0)}}
	h := MethodHistoryBasis{}
	s := methodsBaselineFromSnapshot(h, 800, observations, now)
	if s.Base != 600 || s.Low > s.Base || s.High < s.Base {
		t.Fatalf("invalid object baseline %+v", s)
	}
	h.BaseDays = 876
	if s = methodsBaselineFromSnapshot(h, 800, observations, now); s.Base != 876 {
		t.Fatal("history was replaced")
	}
	if s = methodsBaselineFromSnapshot(MethodHistoryBasis{}, 800, nil, now); s.Base != 800 {
		t.Fatal("current MRP fallback lost")
	}
}
func BenchmarkRC618ProbabilityIndexed50000(b *testing.B) {
	records := make([]VersionRecord, 50000)
	facts := make([]Outcome, 50000)
	for i := range records {
		id := fmt.Sprintf("v%d", i)
		records[i] = VersionRecord{VersionID: id, ObjectKey: fmt.Sprintf("o%d", i%100), Method: "Ингибитор солеотложений", Complication: "Солеотложения", Equipment: "УЭЦН"}
		facts[i] = Outcome{VersionID: id, EffectAttribution: "Подтверждено специалистом", ObservedEffect: "Наблюдается улучшение"}
	}
	index := indexMethodOutcomes(facts)
	cohort := prepareMethodProbabilityCohort(records, index)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		empiricalMethodProbabilityFromCohort("Ингибитор солеотложений", "Солеотложения", "УЭЦН", cohort)
	}
}

func TestRC618NumericLessonsRankBeforeQualitativeActions(t *testing.T) {
	out := []methodWorkflowSuggestion{{Method: assessmentMethodCandidate{Name: "Мониторинг"}}, {Method: assessmentMethodCandidate{Name: "Ингибитор солеотложений"}}}
	scores := map[string]MethodEffectAssessment{"Мониторинг": {Applicability: 95}, "Ингибитор солеотложений": {Applicability: 80, EffectQuantified: true, EffectBasisType: "синтетические входные данные учебного сценария"}}
	ranked := rankMethodSuggestions(out, scores)
	if ranked[0].Method.Name != "Ингибитор солеотложений" {
		t.Fatal("lesson starts with a qualitative action")
	}
	scores["Ингибитор солеотложений"] = MethodEffectAssessment{Applicability: 80, EffectQuantified: true, EffectBasisType: "полевые данные"}
	ranked = rankMethodSuggestions(out, scores)
	if ranked[0].Method.Name != "Мониторинг" {
		t.Fatal("production ranking changed by lesson priority")
	}
}
