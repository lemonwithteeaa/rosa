//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestRC567CombinedEffectIsNotArithmeticAddition(t *testing.T) {
	methods := []MethodEffectAssessment{
		{Method: "Дозирование ингибитора солеотложений", BasePct: 30, EffectQuantified: true},
		{Method: "Защитное покрытие рабочих органов", BasePct: 25, EffectQuantified: true},
	}
	got := combineMethodEffectPercents(methods, func(m MethodEffectAssessment) float64 { return m.BasePct })
	if got <= 0 || got >= 55 {
		t.Fatalf("combined effect %.1f must be positive and below simple sum 55", got)
	}
}

func TestRC567ExplicitComplexDoesNotInjectAnchor(t *testing.T) {
	old := combinedScenarioMethods
	combinedScenarioMethods = map[string][]string{}
	defer func() { combinedScenarioMethods = old }()
	c := TechnicalScenarioContract{ObjectKey: "OBJ", BaseMethod: "Старый базовый метод"}
	setCombinedMethods("OBJ", []string{"Метод 1", "Метод 2"})
	got := selectedCombinedMethods(c)
	if len(got) != 2 || got[0] != "Метод 1" || got[1] != "Метод 2" {
		t.Fatalf("explicit complex was changed: %#v", got)
	}
}

func TestRC567NoHistoryCapsApplicabilityAndWarns(t *testing.T) {
	oldFailures, oldOutcomes, oldVersions := failureEvents, outcomes, versions
	defer func() { failureEvents, outcomes, versions = oldFailures, oldOutcomes, oldVersions }()
	failureEvents, outcomes, versions = nil, nil, nil
	s := methodWorkflowSuggestion{
		Method:    assessmentMethodCandidate{Name: "Дозирование ингибитора солеотложений", Source: "Технологический модуль", Status: "Требуется лабораторный подбор"},
		Direction: "Солеотложения", Classification: "С3", Severity: 3,
	}
	a := methodApplicabilityForSuggestion(s, "NO-HISTORY", time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local))
	if a.Applicability > 80 {
		t.Fatalf("applicability without history is too confident: %+v", a)
	}
	text := strings.ToLower(strings.Join(a.Warnings, " "))
	if !strings.Contains(text, "предыстор") {
		t.Fatalf("missing no-history warning: %+v", a)
	}
}

func TestRC567CombinedForecastKeepsScenarioOrder(t *testing.T) {
	h := MethodHistoryBasis{FailureCount: 3, CycleCount: 3, Precision: "ПОВЫШЕННАЯ"}
	snap := methodForecastSnapshot{Low: 600, Base: 800, High: 1000, Source: "test", History: h}
	f := combinedMethodForecastForNames("OBJ-NO-CATALOG", []string{"Дозирование ингибитора солеотложений", "Защитное покрытие рабочих органов"}, snap, time.Now())
	if f.ForecastLow <= 0 || f.ForecastLow > f.ForecastBase || f.ForecastBase > f.ForecastHigh {
		t.Fatalf("scenario order broken: %+v", f)
	}
	if !strings.Contains(strings.ToLower(f.Interaction), "синер") {
		t.Fatalf("expected synergy label: %s", f.Interaction)
	}
}

func TestRC567FormattingExplainsThreeScenarios(t *testing.T) {
	f := CombinedMethodForecast{BaselineLow: 600, BaselineBase: 800, BaselineHigh: 1000, ForecastLow: 650, ForecastBase: 900, ForecastHigh: 1200, EffectLowPct: 8, EffectBasePct: 12, EffectHighPct: 20, Interaction: "тест", History: MethodHistoryBasis{}}
	text := strings.ToLower(formatCombinedMethodForecast(f))
	for _, want := range []string{"осторожный", "ожидаемый", "благоприятный", "обоснование сценариев", "синергию"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}
