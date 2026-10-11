//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC528LegendViewportScrollsWithoutClipping(t *testing.T) {
	columns, totalRows, visibleRows := complexLegendViewport(18, 350)
	if columns != 2 || totalRows != 9 {
		t.Fatalf("layout columns=%d rows=%d", columns, totalRows)
	}
	if visibleRows <= 0 || visibleRows >= totalRows {
		t.Fatalf("expected scrollable viewport, visible=%d total=%d", visibleRows, totalRows)
	}
}

func TestRC528InteractionContracts(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	complexSource, err := os.ReadFile("laboratory_complex_analysis.go")
	if err != nil {
		t.Fatal(err)
	}
	excelSource, err := os.ReadFile("laboratory_infographic_excel.go")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]bool{
		"history loads only on double click": strings.Contains(string(mainSource), "currentPage == ID_LAB && hdr.Code == NM_DBLCLK"),
		"visible startup transition":         strings.Contains(string(mainSource), "ensureDataWithVisibleTransition()"),
		"line double click":                  strings.Contains(string(complexSource), "hiWord(wParam) == STN_DBLCLK"),
		"right double click":                 strings.Contains(string(complexSource), "complexHandleRightDoubleClick"),
		"legend scroll":                      strings.Contains(string(complexSource), "case WM_VSCROLL:"),
		"automatic Excel path":               strings.Contains(string(excelSource), "infographicExcelAutoPath()"),
		"no Excel save dialog":               !strings.Contains(string(excelSource), "pGetSaveFileNameW.Call"),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("missing contract: %s", name)
		}
	}
}
