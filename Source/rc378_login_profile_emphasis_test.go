//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC378ProfilesRemainFunctionalButHiddenInRC382Reference(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `authUsersList = authCreate("LISTBOX", "", WS_CHILD|LBS_NOTIFY|LBS_HASSTRINGS`) {
		t.Fatal("profile selection engine must remain available")
	}
	if strings.Contains(s, `authCreateCentered("STATIC", "СОХРАНЁННЫЙ ПРОФИЛЬ"`) {
		t.Fatal("visible saved-profile block must stay removed in RC382")
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
