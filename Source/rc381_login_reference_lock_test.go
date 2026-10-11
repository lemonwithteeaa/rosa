//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC381ReferenceDidNotRegressOutsideApprovedRC382Replacement(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`authApplyCenteredLayout(clientW)`,
		`authTitleAccent = 0`,
		`buildLoginSideBitmap(false)`,
		`buildLoginSideBitmap(true)`,
		`buildFooterRitekBitmap()`,
		`{"Вход в ", false}, {"РОСА", true}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing retained login requirement: %s", want)
		}
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
