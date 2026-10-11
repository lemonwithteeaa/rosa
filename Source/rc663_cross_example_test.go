//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC663CanonicalTeachingKeyVariantsStayOneObject(t *testing.T) {
	variants := []string{
		"Авиловское / скв. 6 / Бобриковский горизонт",
		"Авиловское / скв 6 / Бобриковский горизонт",
		"Авиловское / скважина №6 / Бобриковский горизонт",
		"Авиловское / 6 / Бобриковский горизонт",
		"Учебный пример 001 · Авиловское / 6 / Бобриковский горизонт",
	}
	for _, key := range variants {
		if n, ok := teachingWorkflowOrdinal(key); !ok || n != 1 {
			t.Fatalf("variant lost teaching identity: %q n=%d ok=%v", key, n, ok)
		}
	}
}

func TestRC663CombinedMethodsSurviveCaptionVariant(t *testing.T) {
	old := combinedScenarioMethods
	combinedScenarioMethods = map[string][]string{}
	defer func() { combinedScenarioMethods = old }()

	setCombinedMethods(canonicalCrossExampleObject, []string{"Метод А", "Метод Б"})
	got, ok := combinedMethodsForObject("Авиловское / 6 / Бобриковский горизонт")
	if !ok || strings.Join(normalizeMethodList(got), "|") != "Метод А|Метод Б" {
		t.Fatalf("combined methods not restored through equivalent key: %#v ok=%v", got, ok)
	}
}

func TestRC663TeachingDecisionCoversAllThreeApprovedComplications(t *testing.T) {
	d, ok := teachingActiveMethodDecision(canonicalCrossExampleObject, "УТВЕРЖДЕНО: Коррозия — К1; Солеотложения — С2; Механические примеси — М2")
	if !ok {
		t.Fatal("teaching method decision missing")
	}
	joined := strings.ToLower(strings.Join(d.Methods, " | "))
	for _, want := range []string{"корроз", "солеотлож", "мехприм"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("teaching complex does not cover %q: %v", want, d.Methods)
		}
	}
	if len(d.Methods) != 3 {
		t.Fatalf("expected complete three-method teaching complex, got %d: %v", len(d.Methods), d.Methods)
	}
}

func TestRC663TeachingMethodsHaveConcreteImplementationFields(t *testing.T) {
	cases := []struct {
		name      string
		direction string
		wantRate  string
	}{
		{"Ингибитор коррозии (дозирование / закачка)", "Коррозия", "35"},
		{"Ингибитор солеотложений (дозирование / закачка)", "Солеотложения", "30"},
	}
	for _, tc := range cases {
		s := methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: tc.name}, Direction: tc.direction}
		_, base := methodImplementationSuggestedFields(s)
		fields := teachingMethodImplementationFields(canonicalCrossExampleObject, s, base)
		values := map[string]string{}
		for _, f := range fields {
			values[f.Key] = f.Value
		}
		for _, key := range []string{"reagent", "basis", "unit", "rate", "point", "periodicity", "control"} {
			if strings.TrimSpace(values[key]) == "" || strings.Contains(strings.ToLower(values[key]), "уточнить") {
				t.Fatalf("%s field %s is not concretely filled: %q", tc.name, key, values[key])
			}
		}
		if values["rate"] != tc.wantRate || values["unit"] != "гр/м³" {
			t.Fatalf("%s bad dosage: rate=%q unit=%q", tc.name, values["rate"], values["unit"])
		}
	}
}

func TestRC663TeachingMechanicalMethodHasConcreteImplementationFields(t *testing.T) {
	s := methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: "Фильтр / сепаратор мехпримесей"}, Direction: "Механические примеси"}
	_, base := methodImplementationSuggestedFields(s)
	fields := teachingMethodImplementationFields(canonicalCrossExampleObject, s, base)
	values := map[string]string{}
	for _, f := range fields {
		values[f.Key] = f.Value
	}
	for _, key := range []string{"mode", "duration", "equipment", "stop", "control", "note"} {
		if strings.TrimSpace(values[key]) == "" || strings.Contains(strings.ToLower(values[key]), "уточнить") {
			t.Fatalf("mechanical field %s is not concretely filled: %q", key, values[key])
		}
	}
}

func TestRC663MethodsLayoutGivesFieldsMoreRoom(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"planReserve = 58", "archiveReserve = 76", "workspaceMaxH = 190",
		"leftW := contentW * 44 / 100", "midW := contentW * 25 / 100", "rightW < 270",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC663 layout marker missing: %q", want)
		}
	}
}
