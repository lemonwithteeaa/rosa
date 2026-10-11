//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC398LoginBoldInputsAndRedInitials(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `^uintptr(18-1), 0, 0, 0, 700`) {
		t.Fatal("login input font must keep 18px size and use bold weight 700")
	}
	for _, want := range []string{
		`{"Р", true}, {"АСЧЁТ", false}`,
		`{"О", true}, {"ЦЕНКА", false}`,
		`{"С", true}, {"ОПРОВОЖДЕНИЕ", false}`,
		`{"А", true}, {"ВТОМАТИЗАЦИЯ", false}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing red initial segment: %s", want)
		}
	}
	if appBuildVersion != "7.3.11.398-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
