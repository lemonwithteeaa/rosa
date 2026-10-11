//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC377BrandAndProfileLogicRetainedInRC382(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`ID_AUTH_NGDO`,
		`authenticateLocalProfile(cits, cdng, ngdo, position, fio, password)`,
		`createLocalAccountProfile(cits, cdng, ngdo, position, fio, password, first)`,
		`authApplyCenteredLayout(clientW)`,
		`buildLoginSideBitmap(false)`,
		`buildLoginSideBitmap(true)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing retained RC377 rule: %s", want)
		}
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
