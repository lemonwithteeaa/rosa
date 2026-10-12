//go:build windows

package main

import (
	"fmt"
	"strings"
)

// Compact, actionable interpretation of the saved economic result. The full
// assumptions and figures remain accessible through the dedicated detail view.
func economicsTechnologistConclusion(objectKey string, scenario AlternativeMethodScenario, hasScenario bool, target float64) string {
	if strings.TrimSpace(objectKey) == "" {
		return "ВЫВОД ТЕХНОЛОГУ · Сначала закрепите объект на Главной. Архивные данные других объектов автоматически не подставляются."
	}
	if !hasScenario {
		return "ВЫВОД ТЕХНОЛОГУ · По выбранному объекту нет схем. Сформируйте альтернативы в разделе «Методы и мероприятия»."
	}
	if !economicsScenarioCalculated(scenario) {
		return "ВЫВОД ТЕХНОЛОГУ · " + dash(scenario.Name) + ": экономический расчёт не подтверждён или исходные данные изменились. Проверьте затраты и выполните пересчёт."
	}
	benefit := "расчётный положительный эффект"
	if scenario.NetBenefitK < 0 {
		benefit = "расчётные затраты превышают выгоду"
	} else if scenario.NetBenefitK == 0 {
		benefit = "расчётный эффект нулевой"
	}
	goal := ""
	if target > 0 && scenario.ForecastBaseDays > 0 {
		if scenario.ForecastBaseDays >= target {
			goal = fmt.Sprintf(" Ориентир %.0f сут. достигнут по прогнозу.", target)
		} else {
			goal = fmt.Sprintf(" До ориентира %.0f сут. по прогнозу не хватает %.0f сут.", target, target-scenario.ForecastBaseDays)
		}
	}
	return fmt.Sprintf("ВЫВОД ТЕХНОЛОГУ · %s: %s (%+.1f тыс. руб. за расчётный горизонт); прогноз МРП %.0f сут.%s Итоговое решение — после проверки достоверности исходных данных и доступности мероприятия.",
		dash(scenario.Name), benefit, scenario.NetBenefitK, scenario.ForecastBaseDays, goal)
}

func refreshEconomicsTechnologistBrief() {
	if econTechnologistBrief == 0 {
		return
	}
	key := strings.TrimSpace(selectedObjectKey)
	s, ok := activeAlternativeScenario(key)
	target := float64(0)
	if key != "" {
		target = targetRunlifeResolvedForSelectedObject().Days
	}
	setEconomicsCaption(econTechnologistBrief, economicsTechnologistConclusion(key, s, ok, target))
}
