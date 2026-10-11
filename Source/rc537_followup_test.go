//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC537ExcelAxesThresholdAndLegendContracts(t *testing.T) {
	source, err := os.ReadFile("laboratory_infographic_excel.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"infographicExcelAxisBounds", `<c:min val=`, `<c:max val=`,
		`<c:smooth val="0"/>`, `<c:legendPos val="b"/>`, `<c:overlay val="0"/>`,
		`strings.ToUpper(threshold.caption(metric.Unit))`, `xlsxInlineCell("B1"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Excel contract missing %q", want)
		}
	}
	if strings.Contains(text, `xlsxInlineCell(lastColumnName+"4", "Порог внимания"`) {
		t.Fatal("threshold must not be exported as a plotted data series")
	}
}

func TestRC537ObjectApprovalAndSelectedLaboratoryContracts(t *testing.T) {
	mainSource, _ := os.ReadFile("main.go")
	objectSource, _ := os.ReadFile("object_selection.go")
	assessmentSource, _ := os.ReadFile("assessment_classification.go")
	all := string(mainSource) + string(objectSource) + string(assessmentSource)
	for _, want := range []string{
		"confirmSelectedObjectAfterQuestion", "Утвердить объект?", "restoreSelectedObjectWorkspace",
		"latestSavedLaboratoryPassForObject", "выбранная лабораторная запись", "labDrafts[groupCode]",
		"⚠ ОБЪЕКТ ИЗМЕНЁН", "historyVisible", "desiredObjectH",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("object/laboratory contract missing %q", want)
		}
	}
	if appBuildVersion != "7.3.11.538-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
