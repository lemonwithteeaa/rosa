//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC572MethodsAreClickableWithoutImplicitAdd(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"openMethodsCandidateReference(i)",
		"subItem == 1 || subItem == 4",
		"case methodsPlanList:",
		"openMethodsReferenceByName(names[i])",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC572 missing %q", want)
		}
	}
	if strings.Contains(s, "selectMethodsCandidateRow(i, hdr.Code == NM_DBLCLK)") {
		t.Fatal("double click must not add method implicitly")
	}
}

func TestRC572ReferenceDialogIsLargeAndResizable(t *testing.T) {
	b, err := os.ReadFile("methods_reference_dialog.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"WS_OVERLAPPEDWINDOW|WS_VISIBLE", "case WM_SIZE:", "layoutMethodsReferenceDialog", "PtMinTrackSize.X = 980"} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC572 reference dialog missing %q", want)
		}
	}
}

func TestRC572MainMethodsScreenFollowsReference(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"show(methodsPlanTitle, false)", "show(methodsPlanList, false)", "АРХИВ РАНЕЕ УТВЕРЖДЁННЫХ ВАРИАНТОВ", "В плане", "Утверждён"} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC572 reference layout missing %q", want)
		}
	}
}

func TestRC572LaboratoryHistoryCannotBePushedByHiddenControls(t *testing.T) {
	b, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "copyY+52+36+12") {
		t.Fatal("hidden clear/copy controls still push history beyond client")
	}
	for _, want := range []string{"inputPanelBottom+8", "historyMinimumH", "Never leave an empty unlabelled rectangle where the table should be"} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC572 laboratory history guard missing %q", want)
		}
	}
}
