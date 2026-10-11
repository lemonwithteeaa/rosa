//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC536ExcelUsesNativeCellLinkedLineCharts(t *testing.T) {
	b, err := os.ReadFile("laboratory_infographic_excel.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`drawingml/2006/chart`, `xl/charts/chart`, `<c:lineChart>`, `<c:dateAx>`,
		`<c:cat><c:numRef><c:f>%s!$A$%d:$A$%d`,
		`<c:val><c:numRef><c:f>%s!$%s$%d:$%s$%d`,
		`<c:strRef><c:f>%s!$%s$4`, `Target="/xl/charts/chart%d.xml"`,
		`линейные зависимости по исходным ячейкам`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("native Excel chart requirement missing %q", want)
		}
	}
	for _, forbidden := range []string{`relationships/image`, `xl/media/image`, `<xdr:pic>`, `renderInfographicExcelChartPNG`} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("Excel chart must not be replaced by image: %q", forbidden)
		}
	}
}

func TestRC536UIFollowupGuards(t *testing.T) {
	assessment, _ := os.ReadFile("assessment_classification.go")
	lab, _ := os.ReadFile("laboratory.go")
	mainSource, _ := os.ReadFile("main.go")
	for _, want := range []string{"setAssessmentTabDetail", "EM_SETCHARFORMAT", "CFE_BOLD"} {
		if !strings.Contains(string(assessment), want) {
			t.Fatalf("assessment formatting missing %q", want)
		}
	}
	for _, want := range []string{"⚠ ДРУГОЙ ОБЪЕКТ", "ВНИМАНИЕ: в лаборатории будет открыт другой объект", "A selected saved or teaching example"} {
		if !strings.Contains(string(lab), want) {
			t.Fatalf("laboratory safeguard missing %q", want)
		}
	}
	if !strings.Contains(string(mainSource), "В ЛАБОРАТОРНОМ РАЗДЕЛЕ ОТКРЫТ ДРУГОЙ ОБЪЕКТ") {
		t.Fatal("cross-page object warning missing")
	}
	if appBuildVersion != "7.3.11.538-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
