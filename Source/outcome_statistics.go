package main

import (
	"fmt"
	"sort"
	"strings"
)

type OutcomeCauseCount struct {
	Label string
	Count int
}

type OutcomeCycleStatistics struct {
	TotalFacts            int
	Confirmed             int
	Unconfirmed           int
	Failures              int
	Repairs               int
	PlannedInterventions  int
	OtherConfirmed        int
	FailureCauseKnown     int
	FailureCauseMissing   int
	DefectDocumented      int
	ObservedEffectRecords int
	FailureCauses         []OutcomeCauseCount
}

func normalizedOutcomeCauseKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

// outcomeCycleStatisticsForObject is deliberately descriptive. It keeps
// failures, repairs and planned interventions separate and only counts an event
// in those groups after manual classification confirmation. Free-text failure
// causes are grouped only by exact case-insensitive spelling after whitespace
// normalization; no semantic synonym merging is performed.
func outcomeCycleStatisticsForObject(objectKey string, versionRows []VersionRecord, outcomeRows []Outcome) OutcomeCycleStatistics {
	key := normalizeLaboratoryObjectKey(objectKey)
	stats := OutcomeCycleStatistics{}
	if key == "" {
		return stats
	}
	versionIDs := map[string]bool{}
	for _, v := range versionRows {
		if normalizeLaboratoryObjectKey(v.ObjectKey) == key {
			versionIDs[strings.TrimSpace(v.VersionID)] = true
		}
	}
	type causeAggregate struct {
		label string
		count int
	}
	causes := map[string]causeAggregate{}
	for _, o := range outcomeRows {
		if !versionIDs[strings.TrimSpace(o.VersionID)] {
			continue
		}
		stats.TotalFacts++
		if strings.TrimSpace(o.DefectFinding) != "" {
			stats.DefectDocumented++
		}
		if strings.TrimSpace(o.ObservedEffect) != "" && !strings.EqualFold(strings.TrimSpace(o.ObservedEffect), "Недостаточно данных") {
			stats.ObservedEffectRecords++
		}
		if !o.ClassificationConfirmed {
			stats.Unconfirmed++
			continue
		}
		stats.Confirmed++
		switch normalizeMRPEventType(o.EventType, o.PlannedIntervention) {
		case "отказ":
			stats.Failures++
			cause := strings.TrimSpace(o.FailureCause)
			causeKey := normalizedOutcomeCauseKey(cause)
			if causeKey == "" {
				stats.FailureCauseMissing++
				continue
			}
			stats.FailureCauseKnown++
			agg := causes[causeKey]
			if agg.label == "" {
				agg.label = strings.Join(strings.Fields(cause), " ")
			}
			agg.count++
			causes[causeKey] = agg
		case "ремонт":
			stats.Repairs++
		case "плановое вмешательство":
			stats.PlannedInterventions++
		default:
			stats.OtherConfirmed++
		}
	}
	for _, agg := range causes {
		stats.FailureCauses = append(stats.FailureCauses, OutcomeCauseCount{Label: agg.label, Count: agg.count})
	}
	sort.SliceStable(stats.FailureCauses, func(i, j int) bool {
		if stats.FailureCauses[i].Count == stats.FailureCauses[j].Count {
			return strings.Compare(strings.ToLower(stats.FailureCauses[i].Label), strings.ToLower(stats.FailureCauses[j].Label)) < 0
		}
		return stats.FailureCauses[i].Count > stats.FailureCauses[j].Count
	})
	return stats
}

func outcomeCycleStatisticsCompact(stats OutcomeCycleStatistics) string {
	if stats.TotalFacts == 0 {
		return "Статистика завершений: фактов пока нет."
	}
	return fmt.Sprintf("Статистика завершений: фактов %d • подтверждено %d • отказы %d • ремонты %d • плановые %d • причины отказов %d/%d",
		stats.TotalFacts, stats.Confirmed, stats.Failures, stats.Repairs, stats.PlannedInterventions, stats.FailureCauseKnown, stats.Failures)
}

func outcomeCycleStatisticsSummary(stats OutcomeCycleStatistics) string {
	if stats.TotalFacts == 0 {
		return "Фактические завершения циклов по выбранному объекту ещё не зарегистрированы."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Фактов: %d; классификация подтверждена: %d; требует подтверждения: %d.\r\n", stats.TotalFacts, stats.Confirmed, stats.Unconfirmed)
	fmt.Fprintf(&b, "Подтверждённые типы: отказы — %d; ремонты — %d; плановые вмешательства — %d; прочие — %d.\r\n", stats.Failures, stats.Repairs, stats.PlannedInterventions, stats.OtherConfirmed)
	fmt.Fprintf(&b, "По подтверждённым отказам причина заполнена: %d; не заполнена: %d.", stats.FailureCauseKnown, stats.FailureCauseMissing)
	if len(stats.FailureCauses) > 0 {
		b.WriteString("\r\nПричины отказов по точным пользовательским формулировкам: ")
		limit := len(stats.FailureCauses)
		if limit > 6 {
			limit = 6
		}
		parts := make([]string, 0, limit)
		for _, c := range stats.FailureCauses[:limit] {
			parts = append(parts, fmt.Sprintf("%s — %d", c.Label, c.Count))
		}
		b.WriteString(strings.Join(parts, "; "))
		if len(stats.FailureCauses) > limit {
			fmt.Fprintf(&b, "; ещё %d формулировок", len(stats.FailureCauses)-limit)
		}
		b.WriteString(".")
	}
	fmt.Fprintf(&b, "\r\nДефектовка заполнена для %d фактов; наблюдение после мероприятия — для %d.", stats.DefectDocumented, stats.ObservedEffectRecords)
	b.WriteString("\r\nСтатистика описательная: ремонты, плановые вмешательства и подтверждённые отказы не смешиваются; формулировки причин автоматически не объединяются и не получают вес в прогнозе МРП.")
	return b.String()
}

func outcomeCycleStatisticsTextForObject(objectKey string) string {
	versionRows := versions
	outcomeRows := outcomes
	if version, ok := teachingCompletedVersionForObject(objectKey); ok {
		versionRows = append(append([]VersionRecord(nil), versions...), version)
		if outcome, found := teachingOutcomeForVersion(version.VersionID); found {
			outcomeRows = append(append([]Outcome(nil), outcomes...), *outcome)
		}
	}
	return outcomeCycleStatisticsSummary(outcomeCycleStatisticsForObject(objectKey, versionRows, outcomeRows))
}

func outcomePercentDelta(actual, reference float64) (float64, bool) {
	if reference == 0 {
		return 0, false
	}
	return (actual - reference) / reference * 100.0, true
}

// outcomeForecastFactComparison compares values stored in the same historical
// calculation version with its factual outcome. It is descriptive only and
// never ranks methods or changes the MRP model.
func outcomeForecastFactComparison(v VersionRecord, o Outcome) string {
	actual := toFloat(o.ActualMRP)
	baseline := toFloat(v.CurrentMRP)
	forecast := toFloat(v.ForecastMRP)
	if forecast <= 0 {
		forecast = toFloat(v.MRPBase)
	}
	parts := []string{}
	if actual > 0 && forecast > 0 {
		delta := actual - forecast
		pct, ok := outcomePercentDelta(actual, forecast)
		if ok {
			parts = append(parts, fmt.Sprintf("Прогноз → факт МРП: %.0f → %.0f сут. (%+.0f сут.; %+.1f%%)", forecast, actual, delta, pct))
		} else {
			parts = append(parts, fmt.Sprintf("Прогноз → факт МРП: %.0f → %.0f сут. (%+.0f сут.)", forecast, actual, delta))
		}
	} else if actual > 0 {
		parts = append(parts, fmt.Sprintf("Факт МРП: %.0f сут.; сопоставимый числовой прогноз в версии не сохранён", actual))
	}
	if actual > 0 && baseline > 0 {
		delta := actual - baseline
		if pct, ok := outcomePercentDelta(actual, baseline); ok {
			parts = append(parts, fmt.Sprintf("Исходный МРП → факт: %.0f → %.0f сут. (%+.0f сут.; %+.1f%%)", baseline, actual, delta, pct))
		}
	}
	if strings.TrimSpace(o.ActualCost) != "" {
		parts = append(parts, "Фактические затраты: "+strings.TrimSpace(o.ActualCost)+" тыс. руб.")
	}
	cycle := []string{}
	if strings.TrimSpace(o.NextCycleStartDate) != "" {
		cycle = append(cycle, "старт "+strings.TrimSpace(o.NextCycleStartDate))
	}
	if strings.TrimSpace(o.NextCycleStatus) != "" && !strings.EqualFold(strings.TrimSpace(o.NextCycleStatus), "Нет данных") {
		cycle = append(cycle, "статус "+strings.TrimSpace(o.NextCycleStatus))
	}
	if len(cycle) > 0 {
		parts = append(parts, "Следующий цикл: "+strings.Join(cycle, "; "))
	}
	if len(parts) == 0 {
		return "Для этой версии ещё недостаточно числовых фактов для сопоставления «прогноз → факт»."
	}
	parts = append(parts, "Сопоставление описательное: отклонение факта от прогноза не доказывает эффект мероприятия и не используется для автоматического рейтинга методов.")
	return strings.Join(parts, "\r\n")
}
