//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC585ImplementationSchemasChangeByMethodType(t *testing.T) {
	dosage := methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: "Ингибитор солеотложений (дозирование / закачка)"}, Direction: "Солеотложения"}
	kind, fields := methodImplementationSuggestedFields(dosage)
	if kind != "Дозирование" || len(fields) != 6 {
		t.Fatalf("dosage schema=%q fields=%d", kind, len(fields))
	}
	if fields[1].Label != "База дозирования" || fields[1].Suggested != "По воде" {
		t.Fatalf("unexpected dosage basis: %+v", fields[1])
	}

	wash := methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: "Промывка растворителем"}, Direction: "АСПО"}
	kind, fields = methodImplementationSuggestedFields(wash)
	if kind != "Промывка / обработка" || len(fields) != 6 || fields[0].Label != "Объём" {
		t.Fatalf("wash schema=%q fields=%+v", kind, fields)
	}

	mechanical := methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: "Механическая очистка НКТ / скребки"}, Direction: "АСПО"}
	kind, fields = methodImplementationSuggestedFields(mechanical)
	if kind != "Механическое / технологическое" || len(fields) != 6 {
		t.Fatalf("mechanical schema=%q fields=%d", kind, len(fields))
	}
}

func TestRC585SourceNavigatorUsesCompactLabels(t *testing.T) {
	src := RuleSource{Title: "Справочник методов Технологического модуля", Organization: "Рабочая модель РОСА", Year: 2026}
	got := methodsReferenceSourceLabel(src)
	if got != "ФАЙЛ · Справочник методов Технологического модуля" {
		t.Fatalf("label=%q", got)
	}
	if strings.Contains(got, src.Organization) {
		t.Fatalf("source navigator must not append organisation into clipped list row: %q", got)
	}
}

func TestRC585Version(t *testing.T) {
	if appBuildVersion != "7.3.11.585-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
