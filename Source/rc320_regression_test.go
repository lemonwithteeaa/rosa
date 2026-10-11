//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func readRC320Source(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRC320ArchivesReturnedToMainNavigation(t *testing.T) {
	s := readRC320Source(t, "main.go")
	if !strings.Contains(s, `{ID_ARCHIVE, "8. Утверждённый реестр"}`) {
		t.Fatal("8-я кнопка «Архивы» не возвращена в главное меню")
	}
}

func TestRC320EconomicsHasTwoVisibleSubsections(t *testing.T) {
	s := readRC320Source(t, "workflow_pages.go")
	for _, want := range []string{`"Базовая"`, `"Расширенная"`, "ID_ECON_BASIC", "ID_ECON_EXTENDED"} {
		if !strings.Contains(s, want) {
			t.Fatalf("экономика не содержит %q", want)
		}
	}
}

func TestRC320AssessmentIsCurrentObjectOriented(t *testing.T) {
	s := readRC320Source(t, "main.go") + readRC320Source(t, "laboratory.go")
	for _, want := range []string{
		"assessmentDirectionViews", "Коррозия", "Солеотложения", "АСПО", "Механические примеси", "Микробиология", "Газы и гидраты",
		"Базовый набор", "Ключевое основание", "showAssessmentDirectionDetail",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("третья вкладка не содержит %q", want)
		}
	}
}

func TestRC320AssessmentDoesNotTreatMissingDataAsNoRisk(t *testing.T) {
	s := readRC320Source(t, "main.go")
	if !strings.Contains(s, "Лабораторные данные по направлению не внесены") || !strings.Contains(s, "НЕТ ДАННЫХ") {
		t.Fatal("нет явного состояния недостатка данных")
	}
}

func TestRC320OnlyRequestedLongButtonUsesForcedCompactFont(t *testing.T) {
	s := readRC320Source(t, "main.go")
	if !strings.Contains(s, `caption == "Вероятность осложнений"`) || !strings.Contains(s, "hFontButtonCompact") {
		t.Fatal("точечное уменьшение шрифта кнопки «Вероятность осложнений» не закреплено")
	}
}

func TestRC320PromptHasPriorityCorrections(t *testing.T) {
	b, err := os.ReadFile("../ПРОМТ_АКТУАЛЬНЫЙ_РОСА_7.3.11.320-RC.txt")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"ПРИОРИТЕТ №1", "поехала", "8. Утверждённый реестр", "Базовая", "Расширенная", "Вероятность осложнений", "ТРЕТЬЯ ВКЛАДКА"} {
		if !strings.Contains(s, want) {
			t.Fatalf("в приоритетном промте отсутствует %q", want)
		}
	}
}
