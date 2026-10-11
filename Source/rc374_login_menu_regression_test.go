//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC374MenuContentRetainedInStaticRC382Reference(t *testing.T) {
	b, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `login_right_reference_v2.png`) {
		t.Fatal("approved right-side analytical reference is missing")
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
