//go:build windows

package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRC529CalculationBasisHasEveryComplication(t *testing.T) {
	want := map[string]bool{"scale": false, "corrosion": false, "aspo": false, "solids": false, "microbiology": false, "gases": false}
	for _, section := range calculationBasisSections {
		if _, ok := want[section.Code]; ok {
			want[section.Code] = true
		}
		if strings.TrimSpace(section.Method) == "" || strings.TrimSpace(section.Formula) == "" || strings.TrimSpace(section.Limits) == "" {
			t.Fatalf("incomplete calculation basis: %+v", section)
		}
	}
	for code, found := range want {
		if !found {
			t.Fatalf("missing calculation basis for %s", code)
		}
	}
}

func TestRC529ExcelOmitsSingletonsFromDependencyChart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dependencies.xlsx")
	passes := []LabPassRecord{
		{ObjectKey: "Объект A", Date: "01.01.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "10"}}},
		{ObjectKey: "Объект A", Date: "02.01.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "12"}}},
		{ObjectKey: "Объект B", Date: "03.01.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "999"}}},
	}
	if err := writeInfographicDependenciesXLSX(path, passes, false, 0); err != nil {
		t.Fatal(err)
	}
	book, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for _, entry := range book.File {
		if entry.Name != "xl/charts/chart2.xml" {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		xml := string(body)
		if !strings.Contains(xml, "Объект A") || strings.Contains(xml, "Объект B") {
			t.Fatalf("chart must contain only real time dependencies: %s", xml)
		}
		return
	}
	t.Fatal("mineralization chart not found")
}

func TestRC529LayoutAndSessionContracts(t *testing.T) {
	lab, _ := os.ReadFile("laboratory.go")
	auth, _ := os.ReadFile("auth.go")
	trash, _ := os.ReadFile("session_trash.go")
	loading, _ := os.ReadFile("startup_loading.go")
	checks := map[string]bool{
		"calculation button stays in output column": strings.Contains(string(lab), "buttonAreaW := min32(outputW"),
		"FIO vertically centred":                    strings.Contains(string(auth), "EM_SETRECT"),
		"session-only trash":                        strings.Contains(string(trash), "finishSessionTrash"),
		"oilfield loading screen":                   strings.Contains(string(loading), "startupDrawPumpjack"),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("missing contract: %s", name)
		}
	}
}
