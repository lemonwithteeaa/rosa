//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC379VisualRulesRetainedInRC382Reference(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`drawAuthProductLabel(di)`,
		`rgb(203, 27, 43)`,
		`{"Вход в ", false}, {"РОСА", true}`,
		`{"РАСЧЁТ", false}, {"  •  ", true}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing retained visual rule: %s", want)
		}
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
