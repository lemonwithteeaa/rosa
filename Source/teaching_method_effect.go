//go:build windows

package main

import "strings"

// Fixed lesson inputs, not scientific efficacy claims. In particular these
// never depend on TargetDays: raising the target cannot raise the prediction.
func teachingMethodEffect(name string) (MethodEffectAssessment, bool) {
	n := normalizeMethodText(name)
	base := 0.0
	direction, class := "Эксплуатационный риск", "учебная оценка"
	applicability, reliability := 82, 78
	reasons := []string{"Метод сопоставлен с утверждённой учебной классификацией и историей отказов за 36 месяцев.", "Встроенный пример содержит лабораторные показатели и параметры реализации для проверки полного отчёта."}
	warnings := []string{"Синтетический учебный пример: проценты не являются производственной гарантией эффективности.", "Перед ОПИ требуется лабораторное подтверждение совместимости, дозировки/режима и фактической доступности ресурсов."}
	switch {
	case strings.Contains(n, "ингиб") && strings.Contains(n, "сол"):
		base, direction, class, applicability = 17, "Солеотложения", "С2", 94
	case strings.Contains(n, "промыв") && strings.Contains(n, "сол"):
		base, direction, class, applicability = 12, "Солеотложения", "С2", 88
	case strings.Contains(n, "ингиб") && strings.Contains(n, "корроз"):
		base, direction, class, applicability = 15, "Коррозия", "К2", 95
	case strings.Contains(n, "контроль") && strings.Contains(n, "кислород"):
		base, direction, class, applicability = 7, "Коррозия", "К2", 84
	case strings.Contains(n, "деэмуль"):
		base, direction, class, applicability = 11, "Высокая вязкость / эмульсия", "В2", 91
	case strings.Contains(n, "раствор") && (strings.Contains(n, "аспо") || strings.Contains(n, "парафин")):
		base, direction, class, applicability = 13, "АСПО", "А2", 93
	case strings.Contains(n, "аспо") || strings.Contains(n, "парафин"):
		base, direction, class, applicability = 13, "АСПО", "А2", 90
	case strings.Contains(n, "фильтр") || strings.Contains(n, "сепарац"):
		base, direction, class, applicability = 9, "Механические примеси", "М2", 86
	case strings.Contains(n, "нкт") && strings.Contains(n, "покрыт"):
		base, direction, class, applicability = 8, "Защита НКТ / комбинированный риск", "С2 / К2", 82
	case strings.Contains(n, "механичес") && strings.Contains(n, "очист"):
		base, direction, class, applicability = 10, "АСПО / механические отложения", "А2", 80
	case strings.Contains(n, "нагрев"):
		base, direction, class, applicability = 12, "АСПО / высокая вязкость", "А2 / В2", 76
	case strings.Contains(n, "оптимизац") && strings.Contains(n, "режим"):
		base, direction, class, applicability = 6, "Эксплуатационный режим", "сопутствующий фактор", 83
	default:
		return MethodEffectAssessment{}, false
	}
	return MethodEffectAssessment{
		Method: name, Direction: direction, Classification: class,
		Applicability: applicability, Reliability: reliability, ReliabilityLevel: "СРЕДНЯЯ / УЧЕБНАЯ", Confidence: "УЧЕБНЫЙ СНИМОК",
		HistoryMatches: 4, HistoryApprovedMatch: 4, CauseRiskShare: 0.75,
		ProbabilityAvailable: false,
		LowPct:               base * 0.7, BasePct: base, HighPct: base * 1.25,
		EffectQuantified: true,
		Evidence:         "УЧЕБНЫЙ ПРИМЕР: синтетический коэффициент для проверки расчёта и полной выгрузки; не фактическая эффективность",
		EffectBasisType:  "синтетические входные данные учебного сценария + лабораторный паспорт + история 4 циклов",
		Reasons:          reasons, Warnings: warnings,
	}, true
}
