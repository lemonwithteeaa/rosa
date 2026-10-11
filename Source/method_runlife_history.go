//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const methodForecastHistoryMonths = 36

type MethodHistoryBasis struct {
	FailureCount int
	CycleCount   int
	LowDays      float64
	BaseDays     float64
	HighDays     float64
	HasRange     bool
	Precision    string
	Warning      string
	Basis        string
}

// methodHistoryBasisForObject deliberately uses only the rolling 36-month
// failure history from section 1. Older failures do not influence the primary
// forecast shown on the Methods page. This keeps the forecast aligned with the
// user's requested three-year operating context and avoids mixing obsolete
// equipment/regime eras into the current decision.
func calculateMethodHistoryBasis(objectKey string, now time.Time) MethodHistoryBasis {
	rows := historyForWell(objectKey, now)
	out := MethodHistoryBasis{FailureCount: len(rows)}
	durations := make([]float64, 0, len(rows))
	for _, e := range rows {
		end, errEnd := failureDate(e.Date)
		start, errStart := failureDate(e.CycleStartDate)
		if errEnd != nil || errStart != nil || !end.After(start) {
			continue
		}
		days := end.Sub(start).Hours() / 24
		if days <= 0 || days > 3650 {
			continue
		}
		durations = append(durations, days)
	}
	sort.Float64s(durations)
	out.CycleCount = len(durations)

	switch {
	case len(rows) == 0:
		out.Precision = "НИЗКАЯ"
		out.Warning = "ВНИМАНИЕ: предыстория отказов за последние 36 месяцев отсутствует. Прогноз строится по текущему МРП, лабораторным/режимным данным и доступным аналогам; его точность значительно ниже, чем при наличии трёхлетней предыстории отказов."
		out.Basis = "Предыстория отказов за 36 месяцев отсутствует."
		return out
	case len(durations) == 0:
		out.Precision = "НИЗКАЯ"
		out.Warning = "ВНИМАНИЕ: отказы за последние 36 месяцев зарегистрированы, но по ним недостаточно дат запуска циклов для расчёта фактической наработки. Точность прогноза значительно ниже; заполните даты запуска циклов/ремонтов."
		out.Basis = fmt.Sprintf("За 36 месяцев зарегистрировано отказов: %d; завершённых циклов с корректными датами: 0.", len(rows))
		return out
	case len(durations) == 1:
		out.Precision = "ОГРАНИЧЕННАЯ"
		out.BaseDays = durations[0]
		out.Basis = fmt.Sprintf("За 36 месяцев: %d отказ(а/ов), один завершённый цикл %.0f сут.", len(rows), out.BaseDays)
		out.Warning = "Предыстория есть, но для устойчивого диапазона прогноза нужен минимум ещё один завершённый цикл."
	case len(durations) == 2:
		out.Precision = "УМЕРЕННАЯ"
		out.LowDays, out.HighDays = durations[0], durations[1]
		out.BaseDays = medianFloat64(durations)
		out.HasRange = true
		out.Basis = fmt.Sprintf("За 36 месяцев: %d отказ(а/ов), завершённых циклов: 2; основной ориентир — медиана, границы — фактический минимум/максимум.", len(rows))
		out.Warning = "Два завершённых цикла дают только ограниченный эмпирический диапазон; не трактовать его как доверительный интервал."
	default:
		out.Precision = "ПОВЫШЕННАЯ"
		out.LowDays = empiricalQuantile(durations, 0.25)
		out.BaseDays = medianFloat64(durations)
		out.HighDays = empiricalQuantile(durations, 0.75)
		out.HasRange = true
		out.Basis = fmt.Sprintf("За 36 месяцев: %d отказ(а/ов), завершённых циклов с датами запуска: %d; основной ориентир — медиана, осторожный/благоприятный — P25/P75 трёхлетней фактической истории.", len(rows), len(durations))
	}
	return out
}

func methodHistoryWarningForObject(objectKey string, now time.Time) string {
	b := methodHistoryBasisForObject(objectKey, now)
	if strings.TrimSpace(b.Warning) != "" {
		return b.Warning
	}
	return fmt.Sprintf("Трёхлетняя предыстория учтена: отказов %d; завершённых циклов %d; достаточность прогноза: %s.", b.FailureCount, b.CycleCount, strings.ToLower(b.Precision))
}

func methodInteractionSummary(methods []string) string {
	normalized := make([]string, 0, len(methods))
	original := normalizeMethodList(methods)
	for _, m := range original {
		n := normalizeMethodText(m)
		if n != "" {
			normalized = append(normalized, n)
		}
	}
	joined := strings.Join(normalized, " | ")
	has := func(needles ...string) bool {
		for _, needle := range needles {
			if strings.Contains(joined, normalizeMethodText(needle)) {
				return true
			}
		}
		return false
	}

	switch {
	case has("ингибитор солеотложений", "ингибитор солей", "комплексон") && has("покрыт", "эпокс", "гидрофоб"):
		return "УСИЛЕНИЕ / СИНЕРГИЯ: покрытие снижает закрепление кристаллов, абразивный износ и доступ среды к поверхности, а ингибитор подавляет образование и рост отложений в потоке. Механизмы дополняют друг друга. Численный синергетический бонус автоматически не начисляется до калибровки по факту объекта или сопоставимых объектов."
	case has("ингибитор коррозии") && has("покрыт", "эпокс", "защитн"):
		return "УСИЛЕНИЕ / СИНЕРГИЯ: барьерное покрытие и ингибитор коррозии действуют разными механизмами и могут взаимно усиливать защиту. Простое сложение процентов запрещено; отдельный численный бонус за синергию требует подтверждения фактом."
	case has("ингибитор") && has("растворител", "кислот", "промыв"):
		return "ПОСЛЕДОВАТЕЛЬНЫЙ ЭФФЕКТ: восстановительное мероприятие удаляет уже сформированное ограничение, а последующее ингибирование снижает скорость его повторного образования. Требуется проверить последовательность, совместимость реагентов и лабораторный подбор."
	}

	if len(original) > 1 {
		dupPairs := 0
		for i := 0; i < len(original); i++ {
			for j := i + 1; j < len(original); j++ {
				if sameMethodFamily(original[i], original[j]) {
					dupPairs++
				}
			}
		}
		if dupPairs > 0 {
			return "ЧАСТИЧНОЕ ДУБЛИРОВАНИЕ: несколько методов воздействуют на одно направление осложнения. Для совместного прогноза вклад второго и последующих методов дисконтируется; арифметическое сложение эффектов запрещено. Проверьте, действительно ли методы работают разными механизмами и нужны одновременно."
		}
		return "НЕЗАВИСИМЫЕ / КОМПЛЕКСНЫЕ НАПРАВЛЕНИЯ: методы относятся к разным механизмам осложнений. Их вклад объединяется через остаточный риск, а не суммированием процентов. Совместимость по реагентам, материалам и режиму эксплуатации должна быть подтверждена технологом."
	}
	return "ОДИНОЧНЫЙ МЕТОД: взаимодействие с другими мероприятиями не оценивается."
}
