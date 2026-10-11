//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC557ApprovedClassificationHistoryAndSource(t *testing.T) {
	b, err := os.ReadFile("assessment_classification.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, token := range []string{
		"Источник: %s",
		"assessmentProgramSource",
		"Утверждено",
		"historyVisible = true",
		"listH = max32(190, min32(listH, 250))",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing RC557 assessment contract %q", token)
		}
	}
	m, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	ms := string(m)
	for _, token := range []string{
		"РАНЕЕ СОХРАНЁННЫЕ КЛАССИФИКАЦИИ (УТВЕРЖДЁННЫЕ)",
		`{"Дата", 140}, {"Направление", 165}, {"Класс", 100}, {"Источник", 355}, {"Статус", 120}, {"Подтвердил", 160}`,
	} {
		if !strings.Contains(ms, token) {
			t.Fatalf("missing RC557 history UI contract %q", token)
		}
	}
}

func TestRC557ApprovedStartupPanel(t *testing.T) {
	b, err := os.ReadFile("startup_loading.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, token := range []string{
		"ROSAStartupLoadingV4",
		"startupDrawCenteredInitialWord",
		`"РАСЧЁТ"`, `"ОЦЕНКА"`, `"СОПРОВОЖДЕНИЕ"`, `"АВТОМАТИЗАЦИЯ"`,
		"startupDrawApprovedPumpjack",
		"buildStartupPumpjackBitmap(232, 138)",
	} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing RC557 startup contract %q", token)
		}
	}
	if _, err := os.Stat("startup_pumpjack_v1.png"); err != nil {
		t.Fatal("approved pumpjack asset missing")
	}
}

func TestRC557BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.557-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
