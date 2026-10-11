//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC533CalculationWorkbookShowsFormulaAndResultSeparately(t *testing.T) {
	xml, _ := calculationWorkbookFormulaRows("gases", map[string]string{
		"sample_temp": "18", "hydrate_temp": "21",
	}, "", 12)
	for _, want := range []string{"Excel-формула", "Результат Excel", "=B13-C13", `<c r="F13"`, `<f>B13-C13</f>`} {
		if !strings.Contains(xml, want) {
			t.Fatalf("formula audit column missing %q", want)
		}
	}
}

func TestRC533ChartSeriesUsesExcelSchemaOrder(t *testing.T) {
	source, err := os.ReadFile("laboratory_infographic_excel.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(source)
	trend := strings.Index(s, `infographicTrendlineXML(trendEnabled, trendType), formulaSheet`)
	if trend < 0 || !strings.Contains(s, `trendline must precede xVal/yVal`) {
		t.Fatal("trendline ordering safeguard missing")
	}
	for _, want := range []string{`<c:style val="13"/>`, `ДЛЯ_ПРОВЕРКИ_И_ИЗМЕНЕНИЙ`, `Изменения в этом Excel-файле не загружаются обратно в РОСА`} {
		if !strings.Contains(s, want) {
			t.Fatalf("chart export safeguard missing %q", want)
		}
	}
}

func TestRC533AssessmentWorkflowAndLayout(t *testing.T) {
	source, err := os.ReadFile("assessment_classification.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(source)
	for _, want := range []string{
		"skipAssessmentLaboratoryReview", "ПРОПУСТИТЬ ЛАБОРАТОРИЮ",
		"ПРИНЯТЬ КЛАССЫ ТЕХНОЛОГА", "assessmentResizeColumns",
		"assessmentDirectionCustomDraw", "сохранённая лабораторная диагностика",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("assessment requirement missing %q", want)
		}
	}
	if appBuildVersion != "7.3.11.533-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
