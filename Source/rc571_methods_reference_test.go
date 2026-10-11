//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC571MethodsReferenceInteraction(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"Прим. / дост. / вероятность",
		"Приоритет / подход",
		"1. Класс и механизм:",
		"5. Прогноз и переносимость:",
		"selectedComboIndex = 1",
		"selectListRow(methodsCandidateList",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC571 methods workflow missing %q", want)
		}
	}
}

func TestRC571SelectedMethodHasFullReferenceCard(t *testing.T) {
	b, err := os.ReadFile("methods_reference.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"КАРТОЧКА МЕТОДА — ПОЛНАЯ ИНФОРМАЦИЯ",
		"Приоритет рассмотрения:",
		"ТИРАЖИРОВАНИЕ:",
		"ПОДХОД К РЕАЛИЗАЦИИ",
		"ОПИСАНИЕ / ОГРАНИЧЕНИЯ / ИСТОЧНИК",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC571 method card missing %q", want)
		}
	}
}
