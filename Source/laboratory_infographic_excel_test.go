//go:build windows

package main

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfographicExcelContainsEditableChartsAndAllMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "infographic.xlsx")
	passes := []LabPassRecord{
		{ObjectKey: "Авиловское / 114 / Бобриковский горизонт", Date: "12.01.2022", Groups: map[string]map[string]string{"scale": {"mineralization": "18"}, "corrosion": {"corrosion_rate": "0.05", "watercut": "70"}}, SharedValues: map[string]string{"watercut": "70"}},
		{ObjectKey: "Авиловское / 114 / Бобриковский горизонт", Date: "21.08.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "24"}, "corrosion": {"corrosion_rate": "0.08", "watercut": "88"}}, SharedValues: map[string]string{"watercut": "88"}},
	}
	if err := writeInfographicDependenciesXLSX(path, passes, true, 0); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	entries := map[string]bool{}
	for _, entry := range archive.File {
		entries[entry.Name] = true
	}
	if !entries["xl/worksheets/sheet1.xml"] || !entries["xl/charts/chart2.xml"] || !entries["xl/drawings/drawing2.xml"] {
		t.Fatalf("missing workbook parts: %#v", entries)
	}
	for _, entry := range archive.File {
		if entry.Name != "xl/charts/chart2.xml" {
			continue
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		body, readErr := io.ReadAll(reader)
		_ = reader.Close()
		xml := string(body)
		if readErr != nil || !strings.Contains(xml, `<c:scatterChart>`) || !strings.Contains(xml, `<c:xVal>`) || !strings.Contains(xml, `<c:trendlineType val="linear"/>`) {
			t.Fatalf("chart is not editable or trend is absent: %v", readErr)
		}
	}
}

func TestAllInfographicExcelMetricsIncludeBothWindows(t *testing.T) {
	metrics := allInfographicExcelMetrics()
	found := map[string]bool{}
	for _, metric := range metrics {
		found[metric.Group+"|"+metric.Key] = true
	}
	for _, key := range []string{"corrosion|watercut", "corrosion|ph", "scale|ca", "aspo|viscosity", "solids|suspended"} {
		if !found[key] {
			t.Fatalf("metric %s is missing from Excel export", key)
		}
	}
}
