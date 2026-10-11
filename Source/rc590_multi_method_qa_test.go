//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestRC590CombinedScenarioSupportsThreeMethodsTogether(t *testing.T) {
	old := combinedScenarioMethods
	combinedScenarioMethods = map[string][]string{}
	defer func() { combinedScenarioMethods = old }()

	c := TechnicalScenarioContract{ObjectKey: "RC590-LIMIT", BaseMethod: "Метод 1"}
	setCombinedMethods(c.ObjectKey, []string{"Метод 1", "Метод 2", "Метод 3"})
	got := selectedCombinedMethods(c)
	if len(got) != 3 {
		t.Fatalf("complex contains %d methods, want the three selected methods: %#v", len(got), got)
	}
	if got[2] != "Метод 3" {
		t.Fatalf("method order/truncation changed: %#v", got)
	}
}

func TestRC590DedicatedComplicationFieldsDriveSeveralDirections(t *testing.T) {
	e := FailureEvent{
		PrimaryComplication:     "Солеотложения",
		AssociatedComplication1: "Коррозия CO2",
		AssociatedComplication2: "АСПО / парафин",
	}
	for _, direction := range []string{"Солеотложения", "Коррозия", "АСПО"} {
		if !failureMatchesMethodDirection(e, direction) {
			t.Errorf("direction %q was not found in the dedicated complication fields", direction)
		}
	}
}

func TestRC590UnapprovedCauseCannotCreateNumericMRPGain(t *testing.T) {
	oldFailures, oldOutcomes, oldVersions := failureEvents, outcomes, versions
	defer func() { failureEvents, outcomes, versions = oldFailures, oldOutcomes, oldVersions }()
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.Local)
	failureEvents = []FailureEvent{{
		ID: "rc590-preliminary", ObjectKey: "RC590-EVIDENCE", Date: "01.09.2026",
		CycleStartDate: "01.01.2026", EventType: "Отказ", PrimaryComplication: "Солеотложения",
		Draft: false, ConclusionApproved: false,
	}}
	outcomes, versions = nil, nil
	s := methodWorkflowSuggestion{
		Method:    assessmentMethodCandidate{Name: "Дозирование ингибитора солеотложений", Source: "Технологический модуль"},
		Direction: "Солеотложения", Classification: "С2", Severity: 2,
	}
	a := methodApplicabilityForSuggestion(s, "RC590-EVIDENCE", now)
	if a.HistoryMatches != 1 || a.HistoryApprovedMatch != 0 {
		t.Fatalf("unexpected evidence counters: %+v", a)
	}
	if a.EffectQuantified || a.LowPct != 0 || a.BasePct != 0 || a.HighPct != 0 || a.CauseRiskShare != 0 {
		t.Fatalf("unapproved cause created a numerical MRP contribution: %+v", a)
	}
	if !strings.Contains(strings.ToLower(strings.Join(a.Warnings, " ")), "утвержд") {
		t.Fatalf("missing evidence-limit warning: %+v", a.Warnings)
	}
}

func TestRC590SeveralApprovedComplicationsFeedThreeMethodResidualRisk(t *testing.T) {
	oldFailures, oldOutcomes, oldVersions := failureEvents, outcomes, versions
	defer func() { failureEvents, outcomes, versions = oldFailures, oldOutcomes, oldVersions }()
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.Local)
	failureEvents = []FailureEvent{
		{ID: "rc590-salt", ObjectKey: "RC590-MULTI", Date: "01.09.2026", CycleStartDate: "01.01.2026", EventType: "Отказ", PrimaryComplication: "Солеотложения", ConclusionApproved: true},
		{ID: "rc590-corrosion", ObjectKey: "RC590-MULTI", Date: "01.06.2026", CycleStartDate: "01.09.2025", EventType: "Отказ", AssociatedComplication1: "Коррозия CO2", ConclusionApproved: true},
		{ID: "rc590-aspo", ObjectKey: "RC590-MULTI", Date: "01.03.2026", CycleStartDate: "01.05.2025", EventType: "Отказ", RefinedComplication: "АСПО / парафин", ConclusionApproved: true},
	}
	outcomes, versions = nil, nil
	suggestions := []methodWorkflowSuggestion{
		{Method: assessmentMethodCandidate{Name: "Ингибитор солеотложений", Source: "Технологический модуль"}, Direction: "Солеотложения", Classification: "С2", Severity: 2},
		{Method: assessmentMethodCandidate{Name: "Ингибитор коррозии", Source: "Технологический модуль"}, Direction: "Коррозия", Classification: "К2", Severity: 2},
		{Method: assessmentMethodCandidate{Name: "Ингибитор АСПО", Source: "Технологический модуль"}, Direction: "АСПО", Classification: "А2", Severity: 2},
	}
	assessments := make([]MethodEffectAssessment, 0, len(suggestions))
	sum := 0.0
	for _, s := range suggestions {
		a := methodApplicabilityForSuggestion(s, "RC590-MULTI", now)
		if !a.EffectQuantified || a.HistoryApprovedMatch != 1 || math.Abs(a.CauseRiskShare-1.0/3.0) > 1e-9 {
			t.Fatalf("approved direction %q did not produce its evidence-limited share: %+v", s.Direction, a)
		}
		assessments = append(assessments, a)
		sum += a.BasePct
	}
	combined := combineMethodEffectPercents(assessments, func(m MethodEffectAssessment) float64 { return m.BasePct })
	if combined <= 0 || combined >= sum {
		t.Fatalf("three-method effect must use residual risk: combined %.4f, arithmetic sum %.4f", combined, sum)
	}
}

func TestRC590ResidualRiskCombinationIsNotArithmeticSum(t *testing.T) {
	methods := []MethodEffectAssessment{
		{Method: "Ингибитор солеотложений", BasePct: 30, EffectQuantified: true},
		{Method: "Ингибитор коррозии", BasePct: 20, EffectQuantified: true},
		{Method: "Фильтр механических примесей", BasePct: 10, EffectQuantified: true},
	}
	got := combineMethodEffectPercents(methods, func(m MethodEffectAssessment) float64 { return m.BasePct })
	want := (1 - (1-0.30)*(1-0.20)*(1-0.10)) * 100
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("residual-risk result %.12f, want %.12f", got, want)
	}
	if got >= 60 {
		t.Fatalf("combined effect %.1f must be below the arithmetic sum 60", got)
	}
}

func TestRC590SameMechanismReceivesDuplicationDiscount(t *testing.T) {
	methods := []MethodEffectAssessment{
		{Method: "Ингибитор солеотложений", BasePct: 30, EffectQuantified: true},
		{Method: "Комплексон против солей", BasePct: 20, EffectQuantified: true},
	}
	got := combineMethodEffectPercents(methods, func(m MethodEffectAssessment) float64 { return m.BasePct })
	withoutDiscount := (1 - (1-0.30)*(1-0.20)) * 100
	if got >= withoutDiscount {
		t.Fatalf("same-family methods were not discounted: got %.1f, undiscounted %.1f", got, withoutDiscount)
	}
}

func TestRC590ImplementationParametersDependOnEventType(t *testing.T) {
	cases := []struct {
		name      string
		direction string
		kind      string
		keys      []string
	}{
		{"Дозирование ингибитора коррозии", "Коррозия", "Дозирование", []string{"reagent", "basis", "rate", "point", "periodicity", "control"}},
		{"Промывка растворителем АСПО", "АСПО", "Промывка / обработка", []string{"volume", "strength", "reagent", "sequence", "contact", "control"}},
		{"Фильтр механических примесей", "Механические примеси", "Механическое / технологическое", []string{"mode", "duration", "equipment", "stop", "control", "note"}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			kind, fields := methodImplementationSuggestedFields(methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: tc.name}, Direction: tc.direction})
			if kind != tc.kind || len(fields) != len(tc.keys) {
				t.Fatalf("kind=%q fields=%+v", kind, fields)
			}
			for i, key := range tc.keys {
				if fields[i].Key != key || strings.TrimSpace(fields[i].Suggested) == "" {
					t.Fatalf("field %d=%+v, want key %q and non-empty suggestion", i, fields[i], key)
				}
			}
		})
	}
}
