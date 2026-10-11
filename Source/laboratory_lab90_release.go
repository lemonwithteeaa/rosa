//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

type laboratoryLAB90Scenario struct {
	Code   string
	Name   string
	Model  string
	Values map[string]string
}

// laboratoryLAB90Scenarios are deliberately small, deterministic QA fixtures.
// They are not reference chemistry and must never be presented as field data.
// Their purpose is to prove that every visible direction has a working path
// from valid inputs to validation, summary and technologist-facing conclusion.
func laboratoryLAB90Scenarios() []laboratoryLAB90Scenario {
	return []laboratoryLAB90Scenario{
		{
			Code: "corrosion", Name: "Коррозия — обнаруженный фактор H₂S",
			Values: map[string]string{"ph": "6.5", "chlorides": "1200", "h2s": "2", "sample_temp": "25"},
		},
		{
			Code: "scale", Name: "Солеотложения — идеальный скрининг насыщения", Model: "ideal",
			Values: map[string]string{"ca": "40.078", "co3": "60.01", "mg": "0", "na_k": "0", "so4": "0", "hco3": "0", "cl": "0", "mineralization": "1", "ph": "7", "sample_temp": "25"},
		},
		{
			Code: "aspo", Name: "АСПО — достаточный отрицательный скрининг",
			Values: map[string]string{"paraffins": "1", "asphaltenes": "0.1", "pour_point": "0", "viscosity": "10"},
		},
		{
			Code: "solids", Name: "Механические примеси — выраженный сигнал",
			Values: map[string]string{"suspended": "150", "sand": "0.2", "size_d50": "70", "turbidity": "60"},
		},
		{
			Code: "microbiology", Name: "Микробиология — согласующиеся признаки",
			Values: map[string]string{"srb": "обнаружено", "gmb": "1500", "sulfide": "1", "ph": "7", "sample_temp": "25"},
		},
		{
			Code: "gases", Name: "Газы и гидраты — газовый фактор риска",
			Values: map[string]string{"h2s": "2", "co2": "60", "water_content": "0.2", "sample_pressure": "10", "sample_temp": "20"},
		},
	}
}

func laboratoryLAB90ScenarioQAIssues() []string {
	var issues []string
	seen := map[string]bool{}
	for _, scenario := range laboratoryLAB90Scenarios() {
		group, ok := labGroupByCode(scenario.Code)
		if !ok {
			issues = append(issues, scenario.Code+": направление не найдено")
			continue
		}
		if seen[scenario.Code] {
			issues = append(issues, scenario.Code+": дублирован QA-сценарий")
			continue
		}
		seen[scenario.Code] = true
		_, warnings, errors := validateLaboratoryGroup(group, scenario.Values)
		_ = warnings // warnings are allowed: QA fixtures may intentionally be limited.
		if len(errors) > 0 {
			issues = append(issues, fmt.Sprintf("%s: валидатор отклонил QA-набор: %s", scenario.Code, strings.Join(errors, "; ")))
			continue
		}
		summary := laboratoryGroupSummary(group, scenario.Values, scenario.Model)
		if !summary.HasData {
			issues = append(issues, scenario.Code+": валидные данные не распознаны как лабораторные")
		}
		if strings.TrimSpace(summary.Message) == "" || strings.TrimSpace(summary.State) == "" {
			issues = append(issues, scenario.Code+": итоговая карточка не сформирована")
		}
		conclusion := buildLaboratoryConclusionWithScaleModel(group, scenario.Values, scenario.Model)
		for _, marker := range []string{"Итог для технолога", "ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА"} {
			if !strings.Contains(conclusion, marker) {
				issues = append(issues, fmt.Sprintf("%s: заключение не содержит %q", scenario.Code, marker))
			}
		}
	}
	for _, code := range []string{"corrosion", "scale", "aspo", "solids", "microbiology", "gases"} {
		if !seen[code] {
			issues = append(issues, code+": отсутствует финальный LAB-90 QA-сценарий")
		}
	}
	sort.Strings(issues)
	return issues
}

func laboratoryLAB90RuntimeDiagnosticsText() string {
	structural := laboratorySixDirectionQAIssues()
	scenarios := laboratoryLAB90ScenarioQAIssues()
	catalog := validateLaboratoryFieldCatalog()
	latest := repositoryLatestSavedLabPasses()
	objects := map[string]bool{}
	for _, pass := range latest {
		if key := normalizeLaboratoryObjectKey(pass.ObjectKey); key != "" {
			objects[key] = true
		}
	}
	status := "OK"
	if len(structural)+len(scenarios)+len(catalog) > 0 {
		status = "ТРЕБУЕТ ПРОВЕРКИ"
	}
	lines := []string{
		"LAB-90 · ДИАГНОСТИКА ЛАБОРАТОРНОГО МОДУЛЯ",
		"Статус статических инвариантов: " + status,
		fmt.Sprintf("Направления: 6/6; structural issues: %d; scenario issues: %d; catalog issues: %d", len(structural), len(scenarios), len(catalog)),
		fmt.Sprintf("Repository: последних сохранённых объектов доступно для массовой аналитики: %d", len(objects)),
		"Давление в солевом расчёте: численно НЕ применяется до принятого внешнего PHREEQC golden benchmark.",
		"Dose-response: только фактические сопоставимые точки; оптимальная доза автоматически не назначается.",
		"Нативные Windows DPI/regression/fault/PDF проверки: должны быть подтверждены на целевой Windows-машине; эта диагностика их не подменяет.",
		"Хранилище: " + persistentStorageStatusText(),
	}
	appendIssues := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		lines = append(lines, "", title+":")
		limit := len(items)
		if limit > 12 {
			limit = 12
		}
		for _, item := range items[:limit] {
			lines = append(lines, "• "+item)
		}
		if len(items) > limit {
			lines = append(lines, fmt.Sprintf("• … ещё %d", len(items)-limit))
		}
	}
	appendIssues("Structural QA", structural)
	appendIssues("Scenario QA", scenarios)
	appendIssues("Каталог полей", catalog)
	return strings.Join(lines, "\r\n")
}
