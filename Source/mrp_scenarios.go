//go:build windows

package main

import (
	"fmt"
	"strings"
)

type MRPScenarioPresentation struct {
	Name       string
	Days       float64
	Available  bool
	DeltaDays  float64
	DeltaPct   float64
	Confidence string
	Basis      string
	Detail     string
}

func normalizeScenarioConfidence(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch {
	case strings.Contains(s, "повыш") || strings.Contains(s, "достаточ"):
		return "повышенная"
	case strings.Contains(s, "умерен") || strings.Contains(s, "ограниченная история"):
		return "умеренная"
	case strings.Contains(s, "огранич") || strings.Contains(s, "низкая достаточ"):
		return "ограниченная"
	case strings.Contains(s, "низк") || strings.Contains(s, "отсутств"):
		return "низкая"
	default:
		if s == "" {
			return "не определена"
		}
		return s
	}
}

func scenarioDelta(days, baseline float64) (float64, float64) {
	if days <= 0 || baseline <= 0 {
		return 0, 0
	}
	delta := days - baseline
	return delta, delta / baseline * 100
}

func mrpBaselineScenarioDays(r MRPForecastResult) float64 {
	if r.TargetForecastDays > 0 {
		return r.TargetForecastDays
	}
	return r.BaseDays
}

func mrpScenarioBand(r MRPForecastResult) []MRPScenarioPresentation {
	methodState := currentMethodsForecastDetailState()
	baseline := mrpBaselineScenarioDays(r)
	baseConfidence := normalizeScenarioConfidence(r.EvidenceLevel)
	baseBasis := "без нового мероприятия; фактическая история объекта"
	baseSource := "расчёт раздела «Наработка и отказы»"

	// Для сравнения мероприятия с базой используем ту же центральную базу,
	// на которой рассчитан текущий комплекс. Архивный расчёт не смешивается
	// со свежей базой объекта: его базу восстанавливаем из сохранённого эффекта.
	if methodState.HasCombo && methodState.Combo.BaselineBase > 0 {
		baseline = methodState.Combo.BaselineBase
		baseConfidence = normalizeScenarioConfidence(methodState.Combo.History.Precision)
		baseBasis = "без нового мероприятия; 36-месячная база текущего комплекса"
		baseSource = "та же база, на которой рассчитан текущий комплекс методов"
	} else if methodState.HasArchive && methodState.Archive.ForecastBaseDays > 0 {
		factor := 1 + methodState.Archive.EffectBasePct/100
		if factor > 0.01 {
			derived := methodState.Archive.ForecastBaseDays / factor
			if finiteNumber(derived) && derived > 0 {
				baseline = derived
				baseConfidence = normalizeScenarioConfidence(methodState.Archive.ForecastPrecision)
				baseBasis = "без нового мероприятия; база сохранённого утверждения"
				baseSource = "база восстановлена из сохранённого утверждённого расчёта без подмешивания текущих условий"
			}
		}
	} else if r.HasSurvivalTriplet && strings.TrimSpace(r.SurvivalEconSource) != "" {
		baseBasis = "без нового мероприятия; анализ фактической истории"
		baseSource = strings.TrimSpace(r.SurvivalEconSource)
	} else if r.ObservationCount == 0 {
		baseBasis = "без нового мероприятия; фактическая история пока недостаточна"
	}

	baseDetail := fmt.Sprintf("Базовая оценка отражает ожидаемую наработку при сохранении текущего подхода. Источник базы: %s. Фактических завершённых циклов в разделе наработки: %d.", baseSource, r.ObservationCount)
	if len(r.Warnings) > 0 {
		baseDetail += " Ключевое ограничение: " + strings.TrimSpace(r.Warnings[0])
	}
	out := []MRPScenarioPresentation{{
		Name:       "Базовая оценка",
		Days:       baseline,
		Available:  baseline > 0,
		Confidence: baseConfidence,
		Basis:      baseBasis,
		Detail:     baseDetail,
	}}

	methodConfidence := baseConfidence
	methodStatus := strings.TrimSpace(methodState.Status)
	if methodState.HasCombo {
		methodConfidence = normalizeScenarioConfidence(methodState.Combo.History.Precision)
	} else if methodState.HasArchive {
		methodConfidence = normalizeScenarioConfidence(methodState.Archive.ForecastPrecision)
	}
	if methodStatus == "" {
		methodStatus = "технологический комплекс"
	}

	add := func(name string, days float64, basis, detail string) {
		delta, pct := scenarioDelta(days, baseline)
		available := days > 0 && baseline > 0
		if !available {
			basis = "сценарий появится после выбора или утверждения комплекса мероприятий"
			detail = "Численный сценарий не формируется без выбранного технологического решения. Базовая оценка при этом остаётся доступной отдельно."
		}
		out = append(out, MRPScenarioPresentation{
			Name:       name,
			Days:       days,
			Available:  available,
			DeltaDays:  delta,
			DeltaPct:   pct,
			Confidence: methodConfidence,
			Basis:      basis,
			Detail:     detail,
		})
	}
	add("Сдержанный", methodState.Low,
		"эффект ниже расчётного; риск неполной реализации · "+methodStatus,
		"Сдержанный сценарий показывает результат при неблагоприятных условиях, частичном эффекте или неполном соблюдении предпосылок комплекса.")
	add("Расчётный", methodState.Calc,
		"центральная оценка при ожидаемой реализации · "+methodStatus,
		"Расчётный сценарий является центральным рабочим вариантом при ожидаемом выполнении выбранного комплекса и контрольных условий.")
	add("Благоприятный", methodState.High,
		"устойчивый эффект при выполнении условий · "+methodStatus,
		"Благоприятный сценарий показывает верхний инженерный вариант при устойчивом эффекте, соблюдении параметров реализации и отсутствии новых ограничивающих факторов.")
	return out
}

func mrpScenarioCardText(s MRPScenarioPresentation) string {
	if !s.Available {
		return "—\r\nк базе: —\r\nкомплекс не выбран"
	}
	if s.Name == "Базовая оценка" {
		return fmt.Sprintf("%.0f сут.\r\nбез нового мероприятия\r\nуверенность: %s · история объекта", s.Days, s.Confidence)
	}
	basis := "ожидаемая реализация"
	switch s.Name {
	case "Сдержанный":
		basis = "риск неполного эффекта"
	case "Благоприятный":
		basis = "устойчивый эффект"
	}
	return fmt.Sprintf("%.0f сут.\r\nк базе: %+.0f сут. (%+.0f%%)\r\nуверенность: %s · %s", s.Days, s.DeltaDays, s.DeltaPct, s.Confidence, basis)
}

func formatMRPScenarioDetail(scenarios []MRPScenarioPresentation) string {
	if len(scenarios) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("СЦЕНАРНАЯ МОДЕЛЬ\r\n")
	for _, s := range scenarios {
		if !s.Available {
			fmt.Fprintf(&b, "• %s: не сформирован. %s\r\n", s.Name, s.Detail)
			continue
		}
		if s.Name == "Базовая оценка" {
			fmt.Fprintf(&b, "• %s: %.0f сут.; уверенность %s. %s\r\n", s.Name, s.Days, s.Confidence, s.Detail)
			continue
		}
		fmt.Fprintf(&b, "• %s: %.0f сут.; к базе %+.0f сут. (%+.0f%%); уверенность %s. %s\r\n", s.Name, s.Days, s.DeltaDays, s.DeltaPct, s.Confidence, s.Detail)
	}
	b.WriteString("Сценарии являются инженерными оценками и не трактуются как гарантия фактической наработки.")
	return b.String()
}
