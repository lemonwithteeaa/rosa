//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC421CurrentLoginContract(t *testing.T) {
	raw, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	wants := []string{
		`{"Р", "АСЧЁТ"}`,
		`{"О", "ЦЕНКА"}`,
		`{"С", "ОПРОВОЖДЕНИЕ"}`,
		`{"А", "ВТОМАТИЗАЦИЯ"}`,
		`if gap > 58`,
		`gap = 58`,
		`authFeatureIconControls[i]`,
		`fmt.Sprintf("%02d", i+1)`,
		`rightInnerX, 688, rightInnerW, 1`,
		`navX, 700, navW, 112`,
		`case WM_GETMINMAXINFO:`,
		`mmi.PtMinTrackSize.X = 1080`,
		`mmi.PtMinTrackSize.Y = 930`,
		`if wParam == SIZE_MINIMIZED_AUTH`,
		`authApplyResizeLayout(clientW, clientH)`,
		`authScheduleFullRedraw(hwnd)`,
	}
	for _, want := range wants {
		if !strings.Contains(src, want) {
			t.Fatalf("current login contract marker missing: %s", want)
		}
	}
	if appBuildVersion != "7.3.11.421-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
