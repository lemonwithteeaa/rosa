//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC545ComplexChartUsesOnlyTrueLineSeries(t *testing.T) {
	metric := complexMetric{group: "corrosion", key: "corrosion_rate"}
	groups := [][]LabPassRecord{
		{
			{ObjectKey: "Объект 1", Date: "01.01.2024", Groups: map[string]map[string]string{"corrosion": {"corrosion_rate": "0.1"}}},
			{ObjectKey: "Объект 1", Date: "01.02.2024", Groups: map[string]map[string]string{"corrosion": {"corrosion_rate": "0.2"}}},
		},
		{{ObjectKey: "Объект 2", Date: "01.01.2024", Groups: map[string]map[string]string{"corrosion": {"corrosion_rate": "0.3"}}}},
	}
	lineGroups := complexLineGroups(groups, metric)
	if len(lineGroups) != 1 || len(lineGroups[0]) != 2 {
		t.Fatalf("single-point series leaked into line chart: %#v", lineGroups)
	}
}

func TestRC545AxesAndExcelLinesAreProtected(t *testing.T) {
	multi, err := os.ReadFile("laboratory_complex_multi.go")
	if err != nil {
		t.Fatal(err)
	}
	excel, err := os.ReadFile("laboratory_infographic_excel.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"pCreatePenStiff.Call(0, 2, rgb(15, 48, 82))",
		"len(chartSeriesValues[i]) >= 2",
		"len(chartSeriesValues[i]) < 2",
	} {
		if !strings.Contains(string(multi)+string(excel), required) {
			t.Fatalf("line-chart regression guard missing %q", required)
		}
	}
}

func TestRC545BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.545-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}

func TestRC545LegendIsolationHistoryAndChartCacheAreProtected(t *testing.T) {
	source, err := os.ReadFile("laboratory_complex_analysis.go")
	if err != nil {
		t.Fatal(err)
	}
	multi, err := os.ReadFile("laboratory_complex_multi.go")
	if err != nil {
		t.Fatal(err)
	}
	joined := string(source) + string(multi)
	for _, required := range []string{
		"complexLegendGroupAtPoint(point)",
		"complexIsolateObjectAt(index)",
		"complexUndoSelection()",
		"complexRedoSelection()",
		"complexHistoryBackButton = modalCreate(\"BUTTON\", \"←\"",
		"complexHistoryNextButton = modalCreate(\"BUTTON\", \"→\"",
		"complexChartGroupsCacheValid",
		"invalidateComplexChartGroups()",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("legend/history/performance regression guard missing %q", required)
		}
	}
}
