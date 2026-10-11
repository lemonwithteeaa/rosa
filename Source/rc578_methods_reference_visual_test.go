//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC578MethodsReferenceVisualContract(t *testing.T) {
	b, err := os.ReadFile("methods_reference_dialog.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	checks := map[string]string{
		"reference dialog class":      "ROSAMethodsReferenceDialogV",
		"corporate header":            "methodsReferenceHeaderBand",
		"red accent":                  "methodsReferenceAccent",
		"single concise window title": "РОСА — Справочник",
		"owner-drawn tabs":            "BS_OWNERDRAW",
		"active tab state":            "methodsReferenceActiveSection",
		"graphite body typography":    "rgb(43, 46, 50)",
		"blue section typography":     "rgb(15, 48, 82)",
		"red key heading":             "rgb(190, 24, 49)",
		"compact footer":              "methodsReferenceFootnoteHandle",
		"adaptive tab geometry":       "tabW := available / 4",
		"bounded max window width":    "min32(1500",
	}
	for name, needle := range checks {
		if !strings.Contains(s, needle) {
			t.Errorf("%s missing: %q", name, needle)
		}
	}
	if strings.Contains(s, `modalCreate("BUTTON", "МЕТОДЫ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON`) {
		t.Error("legacy push-button style returned for reference tabs")
	}
}
