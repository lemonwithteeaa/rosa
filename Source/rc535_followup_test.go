//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC535ExcelInfographicUsesEmbeddedPNG(t *testing.T) {
	source, err := os.ReadFile("laboratory_infographic_excel.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(source)
	for _, want := range []string{
		`ContentType="image/png"`, `renderInfographicExcelChartPNG`,
		`spreadsheetDrawing`, `relationships/image`, `xl/media/image`,
		`<xdr:pic>`, `r:embed="rId1"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("embedded Excel infographic safeguard missing %q", want)
		}
	}
	if strings.Contains(s, `drawingml/chart`) || strings.Contains(s, `xl/charts/chart`) {
		t.Fatal("fragile native chart part must not be emitted")
	}
}

func TestRC535AssessmentFormattingAndObjectWarnings(t *testing.T) {
	assessment, err := os.ReadFile("assessment_classification.go")
	if err != nil {
		t.Fatal(err)
	}
	lab, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"setAssessmentTabDetail", "EM_SETCHARFORMAT", "ПРЕДПОЛОЖЕНИЕ ПРОГРАММЫ", "ЧТО НУЖНО ДЛЯ ПОДТВЕРЖДЕНИЯ"} {
		if !strings.Contains(string(assessment), want) {
			t.Fatalf("assessment rich formatting missing %q", want)
		}
	}
	for _, want := range []string{"⚠ ДРУГОЙ ОБЪЕКТ", "ВНИМАНИЕ: в лаборатории будет открыт другой объект", "labConclusionOpen = true"} {
		if !strings.Contains(string(lab), want) {
			t.Fatalf("laboratory context safeguard missing %q", want)
		}
	}
	if !strings.Contains(string(mainSource), "В ЛАБОРАТОРНОМ РАЗДЕЛЕ ОТКРЫТ ДРУГОЙ ОБЪЕКТ") {
		t.Fatal("assessment page does not warn about another laboratory object")
	}
	if appBuildVersion != "7.3.11.535-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}

func TestRC535ThresholdCaptionStaysAwayFromYAxis(t *testing.T) {
	for _, name := range []string{"laboratory_history_chart.go", "laboratory_complex_infographic.go"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if !strings.Contains(s, "plot.Right-360") || !strings.Contains(s, "y - 2") {
			t.Fatalf("%s does not anchor threshold caption beside the line", name)
		}
	}
}
