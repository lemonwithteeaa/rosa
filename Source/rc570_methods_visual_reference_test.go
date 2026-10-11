//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC570MethodsPageKeepsReferenceComposition(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	required := []string{
		"ТЕХНИЧЕСКИЙ РЕЖИМ ОБЪЕКТА",
		"УТВЕРЖДЁННАЯ КЛАССИФИКАЦИЯ ИЗ РАЗДЕЛА 3",
		"ПРЕДЛОЖЕННЫЕ МЕТОДЫ — ПО УТВЕРЖДЁННЫМ КЛАССАМ И ТЕХНОЛОГИЧЕСКОМУ МОДУЛЮ",
		"ОСНОВАНИЕ ВЫБОРА",
		"ПАРАМЕТРЫ РЕАЛИЗАЦИИ",
		"ПРОГНОЗ НАРАБОТКИ ПОСЛЕ УТВЕРЖДЕНИЯ МЕТОДА",
		"АРХИВ РАНЕЕ УТВЕРЖДЁННЫХ ВАРИАНТОВ ПО ОБЪЕКТУ",
		"УТВЕРДИТЬ МЕТОД",
		"СФОРМИРОВАТЬ ПЛАН",
		"СПРАВОЧНИК",
		"ТИРАЖИРОВАНИЕ",
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Fatalf("RC570 reference visual element missing: %q", want)
		}
	}
}

func TestRC570MethodsPageUsesThreeZoneWorkingArea(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"leftW :=", "midW :=", "rightW :=", "methodsBasisTitle", "methodsImplementationTitle", "methodsCandidateList"} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC570 three-zone layout missing: %q", want)
		}
	}
}

func TestRC570ReferenceImageIsBundled(t *testing.T) {
	if _, err := os.Stat("Дашборд_методов_и_мероприятий_РЕФЕРЕНС.png"); err != nil {
		t.Fatalf("RC570 visual reference image must be bundled: %v", err)
	}
}
