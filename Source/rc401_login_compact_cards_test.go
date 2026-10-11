//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC401LoginCompactCardGeometry(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	wants := []string{
		`authBaseWidth              int32 = 1060`,
		`const leftCardW int32 = 500`,
		`const rightCardX int32 = 544`,
		`const rightCardW int32 = 500`,
		`const leftInnerW int32 = 432`,
		`const rightInnerW int32 = 432`,
		`navW := (rightInnerW - 3*navGap) / 4`,
	}
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Fatalf("missing RC401 geometry: %s", want)
		}
	}
}

func TestRC401Version(t *testing.T) {
	if appBuildVersion != "7.3.11.401-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
