//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC568MethodsPageUsesApprovedReferenceLayout(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	required := []string{
		"УТВЕРЖДЁННАЯ КЛАССИФИКАЦИЯ ОСЛОЖНЕНИЙ",
		"КЛЮЧЕВЫЕ ДАННЫЕ ДЛЯ ПОДБОРА МЕТОДОВ",
		"ПОДБОР МЕТОДОВ И МЕРОПРИЯТИЙ",
		"ПРОГНОЗ НАРАБОТКИ",
		"ПЛАН МЕРОПРИЯТИЙ (УТВЕРЖДАЕМЫЙ)",
		"+ ДОБАВИТЬ МЕРОПРИЯТИЕ",
		"УТВЕРДИТЬ ПЛАН",
		"ЭКСПОРТ В PDF",
		"ЭКСПОРТ В XLSX",
		"ЭКСПОРТ В DOCX",
		"ВНИМАНИЕ: без предыстории отказов прогноз значительно менее точен.",
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Fatalf("RC568 visual/reference requirement missing: %q", want)
		}
	}
}

func TestRC568MethodsCandidateTableAndPlanAreResponsive(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"methodsCandidateList", "methodsClassList", "methodsKeyData", "methodsPlanList", "layoutMethodsPlanColumns(contentW)", "rightW :=", "leftW :="} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC568 responsive methods layout missing: %q", want)
		}
	}
}
