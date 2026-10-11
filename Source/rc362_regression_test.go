//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC362AssessmentPriorityAndCoverage(t *testing.T) {
	if len(assessmentDirectionPriority) != 6 || assessmentDirectionPriority[0] != "scale" {
		t.Fatalf("assessment priority = %#v", assessmentDirectionPriority)
	}
	got := assessmentCoverageText(laboratoryResultSummary{BaseFilled: 3, BaseTotal: 4})
	if got != "3/4 · 75%" {
		t.Fatalf("coverage = %q", got)
	}
}

func TestRC362AssessmentMethodsExposeSource(t *testing.T) {
	c := assessmentMethodCandidatesForGroup("scale")
	if len(c) == 0 {
		t.Fatal("scale methods empty")
	}
	hasModel := false
	for _, m := range c {
		if strings.Contains(strings.ToLower(m.Source), "технологичес") {
			hasModel = true
		}
	}
	if !hasModel {
		t.Fatal("technology module layer missing")
	}
}

func TestRC362MethodDetailHasNextAction(t *testing.T) {
	text := assessmentMethodDetailText(assessmentMethodCandidate{Name: "Ингибитор солеотложений", Status: "Требуется лабораторный подбор", Source: "Технологический модуль", SourceKind: "исходный справочник"})
	for _, want := range []string{"Следующее действие", "лаборатор", "автоматически"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(want)) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestRC362SourceAndFreshnessLabels(t *testing.T) {
	if got := assessmentFreshnessText(""); got != "дата не указана" {
		t.Fatalf("freshness=%q", got)
	}
}

func TestRC362UILayoutGuardsPresent(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"actionsTotal", "methodsLabelY", "detailY := comboY + 66", "Лабораторный признак не равен подтверждённой причине отказа"} {
		if !strings.Contains(s, want) {
			t.Fatalf("main.go lost %q", want)
		}
	}
	b, err = os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	s = string(b)
	for _, want := range []string{"chartCap := int32(170)", "minConclusion = 230", "historyMinimumH int32 = 90"} {
		if !strings.Contains(s, want) {
			t.Fatalf("laboratory.go lost %q", want)
		}
	}
}

func TestRC362ProbabilityButtonNoLongerStartsLegacySync(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `button(ID_ASSESS, "Вероятность осложнений", 902, 126, 170, 36, ID_SYNC)`) {
		t.Fatal("probability button still wired to legacy sync")
	}
	if !strings.Contains(s, `case ID_ASSESS_PROBABILITY:`) {
		t.Fatal("probability button handler missing")
	}
}
