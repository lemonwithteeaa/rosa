//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC590MethodsExportCoversPromptSections(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(b)
	for _, want := range []string{
		"Объект / шифр:",
		"Классификация:",
		"ТЕКУЩЕЕ / УТВЕРЖДЁННОЕ РЕШЕНИЕ",
		"Вклад отдельных методов:",
		"ПАРАМЕТРЫ РЕАЛИЗАЦИИ",
		"Прогноз наработки ПОСЛЕ мероприятия:",
		"ПЛАН ИСПОЛНЕНИЯ",
		"ТИРАЖИРОВАНИЕ / АНАЛОГИ",
		"АРХИВ РЕШЕНИЙ ПО ОБЪЕКТУ",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("выгрузка методов не содержит обязательный блок %q", want)
		}
	}
}

func TestRC590MethodsExportUsesOneReportForAllFormats(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(b)
	for _, want := range []string{
		"report := methodsWorkflowReportText()",
		"writeLaboratoryXLSXReport(path, report)",
		"writeLaboratoryDOCXReport(path, report)",
		"writeLaboratoryPDFReport(path, report)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("нарушен паритет форматов выгрузки методов: нет %q", want)
		}
	}
}

func TestRC590MethodsXLSXDoesNotCreateEmptyColumns(t *testing.T) {
	b, err := os.ReadFile("laboratory_report_export.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(b)
	for _, want := range []string{
		`<col min="1" max="1"`,
		`<c r="A%d"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("одноколоночная структура XLSX не закреплена: нет %q", want)
		}
	}
	for _, forbidden := range []string{`<c r="B`, `<c r="C`, `max="2"`, `max="3"`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("XLSX-отчёт создаёт лишний или пустой столбец: %q", forbidden)
		}
	}
}
