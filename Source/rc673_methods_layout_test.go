//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC673CandidateCardHeightKeepsMeasuredTitle(t *testing.T) {
	title, detail := methodsCandidateCardHeights(52, 18, 124)
	if title < 64 {
		t.Fatalf("measured method title lost required vertical room: title=%d", title)
	}
	if detail < 32 {
		t.Fatalf("detail row became unreadably small: detail=%d", detail)
	}
	if title+detail > 124 {
		t.Fatalf("card geometry exceeds its allocated height: %d+%d", title, detail)
	}
}

func TestRC673LayoutUsesActualCandidateFontsAndLargerCompactWorkbench(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"workspaceMaxH = 190",
		"workspaceMaxH = 270",
		"if workspaceH < 230",
		"methodsTypeBody, true",
		"methodsTypeCompact, false",
		"planReserve = 58",
		"archiveReserve = 76",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC673 layout marker missing: %q", want)
		}
	}
}
