//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC295ScopeIsReliabilityNotGNOSelection(t *testing.T) {
	if !strings.Contains(rosaPurposeText, "диагностики осложнённого фонда") ||
		!strings.Contains(rosaPurposeText, "анализа надёжности") ||
		!strings.Contains(rosaPurposeText, "риска ремонта") ||
		!strings.Contains(rosaPurposeText, "эффективности технологических мероприятий") {
		t.Fatalf("назначение РОСА сформулировано неполно: %s", rosaPurposeText)
	}
	if !strings.Contains(rosaAutotechnologBoundaryText, "не выполняет подбор или проектирование ГНО") ||
		!strings.Contains(rosaAutotechnologBoundaryText, "Автотехнолог") {
		t.Fatalf("не зафиксирована граница с Автотехнологом: %s", rosaAutotechnologBoundaryText)
	}
}

func TestRC295RunlifeTargetRemainsManagementReference(t *testing.T) {
	if !strings.Contains(rosaRunlifeTargetText, "настраиваемым управленческим ориентиром") ||
		!strings.Contains(rosaRunlifeTargetText, "не подменяет фактический прогноз МРП") {
		t.Fatalf("цель наработки не отделена от прогноза: %s", rosaRunlifeTargetText)
	}
}

func TestRC295NavigationUsesFocusedProductTerms(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"1. Фонд и карточка скважины",
		"2. Лабораторные данные",
		"3. Оценка осложнений",
		"4. Методы и мероприятия",
		"5. Наработка и отказы",
		"6. Экономическая оценка",
		"7. Итоговый отчёт",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("в навигации отсутствует %q", want)
		}
	}
	from := strings.Index(s, "nav := []struct")
	to := strings.Index(s[from:], "y := int(NAV_TOP)")
	if from < 0 || to < 0 {
		t.Fatal("не найден блок главной навигации")
	}
	navBlock := s[from : from+to]
	if strings.Contains(navBlock, "ID_TECHPARAM") || strings.Contains(navBlock, "ID_HISTORY") {
		t.Fatalf("служебная диагностика оборудования или история всё ещё дублируются в главном меню: %s", navBlock)
	}
	if !strings.Contains(s, "objectEquipmentButton = button") || !strings.Contains(s, "ID_TECHPARAM") {
		t.Fatal("страница состояния оборудования не доступна из карточки скважины")
	}
}

func TestRC295UserFacingScenarioIsInterventionScenario(t *testing.T) {
	files := []string{"workflow_pages.go", "technical_scenario.go", "multi_method_scenario.go", "main.go"}
	var all strings.Builder
	for _, name := range files {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(b)
	}
	s := all.String()
	for _, want := range []string{"Сценарий мероприятия", "Сравнение сценариев мероприятий", "КОМБИНИРОВАННЫЙ СЦЕНАРИЙ МЕРОПРИЯТИЙ"} {
		if !strings.Contains(s, want) {
			t.Fatalf("не найдено новое пользовательское понятие %q", want)
		}
	}
	for _, old := range []string{"Технический сценарий", "КОМБИНИРОВАННЫЙ ТЕХНИЧЕСКИЙ СЦЕНАРИЙ", "Подбор мероприятий"} {
		if strings.Contains(s, old) {
			t.Fatalf("возвращён устаревший пользовательский термин %q", old)
		}
	}
}

func TestRC295GNOIsClearlyFactualContext(t *testing.T) {
	files := []string{"workflow_pages.go", "technical_scenario.go", "multi_method_scenario.go", "main.go"}
	var all strings.Builder
	for _, name := range files {
		b, _ := os.ReadFile(name)
		all.Write(b)
	}
	s := all.String()
	if !strings.Contains(s, "Фактическое ГНО") && !strings.Contains(s, "фактическое ГНО") {
		t.Fatal("фактическое ГНО не выделено как контекст")
	}
	if !strings.Contains(s, "Подбор ГНО выполняется в «Автотехнологе»") {
		t.Fatal("на странице состояния оборудования нет границы с Автотехнологом")
	}
}

func TestRC295NoUserFacingIPRVLPNodalSelectionPromise(t *testing.T) {
	files := []string{"main.go", "workflow_pages.go", "telemetry.go", "dynamogram_ui.go", "technical_scenario.go", "multi_method_scenario.go", "rosa_scope.go"}
	var all strings.Builder
	for _, name := range files {
		b, _ := os.ReadFile(name)
		all.Write(b)
	}
	s := strings.ToLower(all.String())
	for _, forbidden := range []string{"ipr", "vlp", "nodal analysis", "подобрать гно", "рекомендуется заменить на уэцн"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("в пользовательском контуре осталось обещание проектирования/подбора: %q", forbidden)
		}
	}
}

func TestRC295RussianBoundaryTerminology(t *testing.T) {
	for _, v := range []string{rosaPurposeText, rosaAutotechnologBoundaryText, rosaRunlifeTargetText} {
		for _, forbidden := range []string{"reliability", "failure risk", "run life", "baseline", "survival"} {
			if strings.Contains(strings.ToLower(v), forbidden) {
				t.Fatalf("нерусифицированный термин %q в %q", forbidden, v)
			}
		}
	}
}

func TestRC295BuildMarkerExists(t *testing.T) {
	b, _ := os.ReadFile("main.go")
	if !strings.Contains(string(b), "const appBuildVersion = \"7.3.11.") {
		t.Fatal("build marker missing")
	}
}
