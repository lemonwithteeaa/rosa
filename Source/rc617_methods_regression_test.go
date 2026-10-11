//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRC617TeachingForecastsDerivedFromEffects(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 100; i++ {
		key := fmt.Sprintf("Учебный пример %03d · Объект", i)
		points := teachingCrossHistoryPoints(key, now)
		if len(points) != 4 {
			t.Fatalf("%s: %d lessons", key, len(points))
		}
		for _, point := range points {
			if point.ForecastBase < 950 || point.ForecastBase > 1150 {
				t.Fatalf("%s: forecast away from target: %+v", key, point)
			}
			if point.ForecastBase <= float64(point.MRPDays) || point.ForecastLow > point.ForecastBase || point.ForecastBase > point.ForecastHigh {
				t.Fatalf("inconsistent scenario: %+v", point)
			}
			effects := []MethodEffectAssessment{}
			for _, name := range strings.Split(point.Method, " + ") {
				a, ok := teachingMethodEffect(name)
				if !ok {
					t.Fatalf("unmodelled lesson method: %s", name)
				}
				effects = append(effects, a)
			}
			want := float64(point.MRPDays) * (1 + combineMethodEffectPercents(effects, func(a MethodEffectAssessment) float64 { return a.BasePct })/100)
			if math.Abs(want-point.ForecastBase) > 1e-6 {
				t.Fatalf("manual forecast bypass: got %f want %f", point.ForecastBase, want)
			}
		}
	}
	if got := teachingCrossHistoryPoints("Производственный объект", now); len(got) != 0 {
		t.Fatal("lessons leaked to production")
	}
}

func TestRC617RefreshCacheEndsWithAction(t *testing.T) {
	if methodsEvaluation != nil {
		t.Fatal("unexpected retained cache")
	}
	finish := beginMethodsEvaluation()
	nested := beginMethodsEvaluation()
	original := methodsEvaluation
	methodsEvaluation.suggestions["old-object"] = []methodWorkflowSuggestion{{Direction: "test"}}
	nested()
	if methodsEvaluation != original {
		t.Fatal("nested scope removed outer cache")
	}
	finish()
	if methodsEvaluation != nil {
		t.Fatal("cache survives refresh")
	}
	finish = beginMethodsEvaluation()
	defer finish()
	if _, stale := methodsEvaluation.suggestions["old-object"]; stale {
		t.Fatal("data leaked across actions")
	}
}

func TestRC617ProductionDoesNotReceiveTeachingEffects(t *testing.T) {
	old := failureEvents
	failureEvents = nil
	defer func() { failureEvents = old }()
	a := methodApplicabilityForSuggestion(methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: "Ингибитор солеотложений"}, Direction: "Солеотложения", Classification: "С2", Severity: 2}, "REAL-WELL", time.Now())
	if a.EffectQuantified || a.BasePct != 0 {
		t.Fatalf("synthetic effect on real well: %+v", a)
	}
}

func TestRC617SharedUserSwitchCompletesLifecycle(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	main := string(b)
	for _, marker := range []string{"if id == ID_HEADER_EXIT {", "case ID_HEADER_APPROVE:"} {
		part := main[strings.Index(main, marker):]
		if !strings.Contains(part[:120], "switchCurrentUser()") {
			t.Fatal("split user switch lifecycle", marker)
		}
	}
	b, err = os.ReadFile("user_switch.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, marker := range []string{"ensureDataWithVisibleTransition()", "activatePage(ID_DASH, true)", "SW_MAXIMIZE", "WM_SETICON", "assessmentClassStore = nil", "pageBackStack, pageForwardStack = nil, nil"} {
		if !strings.Contains(s, marker) {
			t.Fatal("missing user switch restoration", marker)
		}
	}
}

func TestRC617NativeSelectionAndLazyTables(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "func refreshMethodsVisualTables")
	end := strings.Index(s[start:], "func methodsKeyDataText") + start
	if strings.Contains(s[start:end], "listAdd(") {
		t.Fatal("hidden tables rebuilt during page entry")
	}
	for _, marker := range []string{"BM_SETCHECK", "BS_CHECKBOX", "methodsCandidateDetails", "scores[out[i].Method.Name]", "setCombinedMethods(key, next)"} {
		if !strings.Contains(s, marker) {
			t.Fatal("missing native workflow", marker)
		}
	}
}

func TestRC617UncheckedDraftDoesNotApproveSelectedFallback(t *testing.T) {
	old := combinedScenarioMethods
	combinedScenarioMethods = map[string][]string{"OBJ": {}}
	defer func() { combinedScenarioMethods = old }()
	if _, err := methodsForWorkflowApproval("OBJ", methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: "Метод"}}, true); err == nil {
		t.Fatal("unchecked empty draft was approved using selected fallback")
	}
}
