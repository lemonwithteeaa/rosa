//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC397LoginVisibleLabelsCentered(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	wants := []string{
		`"Используйте корпоративные данные\r\nдля входа в систему", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX`,
		`caption, WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE`,
		`"ФИО", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE`,
		`"ПАРОЛЬ", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE`,
		`"Забыли пароль?", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX`,
		`"РОСА — ваш инструмент\r\nдля принятия технологических решений", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX`,
		`featureTitles[i], WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE`,
		`featureDetails[i], WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX`,
	}
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Fatalf("missing centered login label rule: %s", want)
		}
	}
	if appBuildVersion != "7.3.11.397-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
