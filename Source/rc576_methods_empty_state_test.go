//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC576MethodsUnreadyStateUsesReferenceGeometry(t *testing.T) {
	b, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"even before full classification the page keeps the same three-zone",
		"show(methodsBasisTitle, true)",
		"show(methodsImplementationTitle, true)",
		"y += workspaceH + 10",
		"ПЕРЕЙТИ К КЛАССИФИКАЦИИ",
		"ОСНОВАНИЕ БУДУЩЕГО ВЫБОРА",
		"ПОСЛЕ УТВЕРЖДЕНИЯ",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC576 unready methods layout missing %q", want)
		}
	}
}
