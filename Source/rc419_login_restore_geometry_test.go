package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC419LoginRestoreGeometryGuards(t *testing.T) {
	raw, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	wants := []string{
		"case WM_GETMINMAXINFO:",
		"mmi.PtMinTrackSize.X = 1080",
		"mmi.PtMinTrackSize.Y = 930",
		"1360, 960, 0, 0, hInstRaw, 0)",
		"if wParam == SIZE_MINIMIZED_AUTH",
		"authApplyResizeLayout(clientW, clientH)",
	}
	for _, want := range wants {
		if !strings.Contains(src, want) {
			t.Fatalf("RC419 marker missing: %s", want)
		}
	}
}
