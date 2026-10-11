//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC375LoginRefreshSurvivesRC382Reference(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `authCreate("STATIC", "РОСА"`) {
		t.Fatal("duplicate ROSA title returned")
	}
	if strings.Contains(s, `РОСА  •  ООО "РИТЭК"`) {
		t.Fatal("duplicate footer branding returned")
	}
	if strings.Contains(s, "Локальная работа") || strings.Contains(s, "История пользователя") || strings.Contains(s, "Резервирование данных") {
		t.Fatal("obsolete feature text returned")
	}
	if !strings.Contains(s, `buildLoginReferenceCardBitmap(false)`) {
		t.Fatal("approved right reference card missing")
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
