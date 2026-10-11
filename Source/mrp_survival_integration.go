package main

import (
	"fmt"
	"sort"
	"strings"
)

const mrpCohortMinObservations = 8
const mrpCohortMinEvents = 5

func classifyMRPOutcomeEvent(result, comment string) (eventType string, planned bool, failureMode string) {
	text := strings.ToLower(strings.TrimSpace(result + " " + comment))
	plannedWords := []string{"планов", "гтм", "оптимизац", "смена типоразмера", "перевод на", "профилактик"}
	for _, w := range plannedWords {
		if strings.Contains(text, w) {
			planned = true
			eventType = "плановое вмешательство"
			break
		}
	}
	if !planned {
		failureWords := []string{"отказ", "авари", "неисправ", "выход из строя", "клин", "обрыв", "пробой", "заклини"}
		for _, w := range failureWords {
			if strings.Contains(text, w) {
				eventType = "отказ"
				break
			}
		}
	}
	if eventType == "" {
		repairWords := []string{"ремонт", "спо", "подъем", "подъём", "замена гно", "трс", "крс"}
		for _, w := range repairWords {
			if strings.Contains(text, w) {
				eventType = "ремонт"
				break
			}
		}
	}
	if eventType == "" {
		eventType = "не классифицировано"
	}

	modes := []struct{ key, name string }{
		{"сол", "солеотложения"}, {"корроз", "коррозия"}, {"аспо", "АСПО"}, {"парафин", "АСПО"},
		{"мехприм", "механические примеси"}, {"песок", "механические примеси"}, {"газ", "газовое влияние"},
		{"электр", "электрический отказ"}, {"кабел", "кабель/электрика"}, {"двигат", "ПЭД/двигатель"},
		{"износ", "износ"}, {"подшип", "механический износ"}, {"клин", "заклинивание"}, {"обрыв", "обрыв"},
	}
	for _, m := range modes {
		if strings.Contains(text, m.key) {
			failureMode = m.name
			break
		}
	}
	if failureMode == "" {
		failureMode = "не классифицировано"
	}
	return
}

func mrpOutcomeClassification(o Outcome) (eventType string, planned bool, failureMode, source string, confirmed bool) {
	eventType = strings.TrimSpace(o.EventType)
	planned = o.PlannedIntervention
	failureMode = strings.TrimSpace(o.FailureMode)
	source = strings.TrimSpace(o.ClassificationSource)
	confirmed = o.ClassificationConfirmed
	if eventType == "" || failureMode == "" {
		autoType, autoPlanned, autoMode := classifyMRPOutcomeEvent(o.Result, o.Comment)
		if eventType == "" {
			eventType = autoType
		}
		if !planned {
			planned = autoPlanned
		}
		if failureMode == "" {
			failureMode = autoMode
		}
		if source == "" {
			source = "автоклассификация старой записи по результату/комментарию"
		}
	}
	return
}

func mrpSurvivalObservationsFromHistory(obs []MRPObservation) []MRPSurvivalObservation {
	out := make([]MRPSurvivalObservation, 0, len(obs))
	for _, o := range validMRPObservations(obs) {
		out = append(out, MRPSurvivalObservation{
			ObjectKey: o.ObjectKey, DurationDays: o.MRPDays, EventType: o.EventType, Planned: o.Planned,
			FailureMode: o.FailureMode, ClassificationSource: o.ClassificationSource, ClassificationConfirmed: o.ClassificationConfirmed,
			CycleStartDate: o.CycleStartDate, CycleEndDate: o.CycleEndDate, DurationSource: o.DurationSource, DurationWarning: o.DurationWarning,
			Date: o.Date, Source: o.Source,
		})
	}
	return out
}

func mrpCurrentCycleAge(in MRPForecastInput) (float64, string, string) {
	if in.CurrentDate.IsZero() {
		return 0, "", "Текущая дата не задана; действующий цикл нельзя добавить как цензурированный."
	}
	obs := validMRPObservations(in.Observations)
	if len(obs) == 0 {
		return 0, "", "Нет даты последнего зарегистрированного вмешательства; возраст текущего цикла не восстановлен."
	}
	last := obs[len(obs)-1]
	if last.Date.IsZero() {
		return 0, "", "У последнего фактического цикла нет даты события; возраст текущего цикла не восстановлен."
	}
	d := in.CurrentDate.Sub(last.Date).Hours() / 24
	if d < 0 {
		return 0, "", "Дата последнего факта находится в будущем относительно даты расчёта; цензурирование отключено."
	}
	source := "дата последнего зарегистрированного фактического вмешательства → дата расчёта"
	warning := ""
	if d > 3650 {
		warning = "Возраст текущего цикла превышает 10 лет; проверьте дату последнего вмешательства."
	}
	return d, source, warning
}

func mrpCohortHistory(in MRPForecastInput) []MRPObservation {
	equipment := normalizedText(in.Equipment)
	family := normalizeComplicationFamily(in.Complication)
	if equipment == "" || family == "" {
		return nil
	}
	versionByID := map[string]VersionRecord{}
	for _, v := range versions {
		versionByID[v.VersionID] = v
	}
	out := []MRPObservation{}
	seen := map[string]bool{}
	for _, o := range outcomes {
		v, ok := versionByID[o.VersionID]
		if !ok {
			continue
		}
		if normalizedText(v.Equipment) != equipment {
			continue
		}
		if normalizeComplicationFamily(v.Complication) != family {
			continue
		}
		d, durationSource, durationWarning, cycleStart, cycleEnd := mrpDurationFromOutcome(o)
		if d <= 0 {
			continue
		}
		eventType, planned, mode, source, confirmed := mrpOutcomeClassification(o)
		date := parseDate(o.Date)
		if !cycleEnd.IsZero() {
			date = cycleEnd
		}
		fp := fmt.Sprintf("%s|%s|%.6f|%s", normalizeLaboratoryObjectKey(v.ObjectKey), date.Format("2006-01-02"), d, o.VersionID)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		out = append(out, MRPObservation{ObjectKey: v.ObjectKey, Field: v.Field, Well: v.Well, ObjectPlast: v.ObjectPlast, Date: date, CycleStartDate: cycleStart, CycleEndDate: cycleEnd, MRPDays: d, DurationSource: durationSource, DurationWarning: durationWarning, Source: "сопоставимая группа: " + v.Equipment + " / " + family, VersionID: v.VersionID, Equipment: v.Equipment, Method: v.Method, Complication: v.Complication, EventType: eventType, Planned: planned, FailureMode: mode, ClassificationSource: source, ClassificationConfirmed: confirmed})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

type MRPAdvancedSummary struct {
	CurrentAgeDays         float64
	CurrentAgeSource       string
	ObjectAny              MRPSurvivalSummary
	ObjectFailure          MRPSurvivalSummary
	CohortAny              MRPSurvivalSummary
	CohortFailure          MRPSurvivalSummary
	CohortObservationCount int
	CohortDistinctObjects  int
	CohortEligible         bool
	Recurrent              MRPRecurrentTrend
	RecurrentExposure      MRPRecurrentExposureSummary
	Classification         MRPClassificationQuality
	CohortClassification   MRPClassificationQuality
	CycleDates             MRPCycleDateQuality
	Competing              MRPCompetingRiskSummary
	CohortCompeting        MRPCompetingRiskSummary
	FailureModes           []MRPFailureModeCount
	CovariateReadiness     MRPCovariateReadiness
	Warnings               []string
}

func calculateMRPAdvanced(in MRPForecastInput) MRPAdvancedSummary {
	age, ageSource, ageWarn := mrpCurrentCycleAge(in)
	objectObs := mrpSurvivalObservationsFromHistory(in.Observations)
	cohortHist := mrpCohortHistory(in)
	cohortObs := mrpSurvivalObservationsFromHistory(cohortHist)
	r := MRPAdvancedSummary{CurrentAgeDays: age, CurrentAgeSource: ageSource, Recurrent: mrpRecurrentTrend(in.Observations), RecurrentExposure: mrpRecurrentExposure(in.Observations)}
	r.Classification = mrpClassificationQuality(objectObs)
	r.CohortClassification = mrpClassificationQuality(cohortObs)
	r.CycleDates = mrpDateQuality(in.Observations)
	r.Competing = aalenJohansenMRP(objectObs, age)
	r.CohortCompeting = aalenJohansenMRP(cohortObs, 0)
	r.FailureModes = mrpFailureModes(objectObs)
	if len(cohortHist) > 0 {
		r.CovariateReadiness = mrpCovariateReadiness(cohortHist)
	} else {
		r.CovariateReadiness = mrpCovariateReadiness(in.Observations)
	}
	r.ObjectAny = summarizeMRPSurvival(objectObs, MRPEventAnyIntervention, age, ageSource)
	r.ObjectFailure = summarizeMRPSurvival(objectObs, MRPEventFailure, age, ageSource)
	// Cohort is used as population context. Do not append the selected object's current
	// cycle to a multi-object cohort, otherwise one active well is given special weight.
	r.CohortAny = summarizeMRPSurvival(cohortObs, MRPEventAnyIntervention, 0, "")
	r.CohortFailure = summarizeMRPSurvival(cohortObs, MRPEventFailure, 0, "")
	r.CohortObservationCount = len(cohortObs)
	r.CohortDistinctObjects = countDistinctMRPObjects(cohortObs)
	r.CohortEligible = r.CohortObservationCount >= mrpCohortMinObservations && r.CohortAny.EventCount >= mrpCohortMinEvents && r.CohortDistinctObjects >= 3
	if ageWarn != "" {
		r.Warnings = append(r.Warnings, ageWarn)
	}
	if !r.CohortEligible && len(cohortObs) > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("Сопоставимая группа пока только справочная: циклов %d, событий %d, уникальных объектов %d; для автоматического использования требуется минимум %d циклов, %d событий и 3 объекта.", r.CohortObservationCount, r.CohortAny.EventCount, r.CohortDistinctObjects, mrpCohortMinObservations, mrpCohortMinEvents))
	}
	if r.Classification.CompletedCycles > 0 && r.Classification.Coverage < 0.8 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("Ручно подтверждено %.0f%% классификаций завершённых циклов; прогноз отказов по конкретной причине (cause-specific) считать предварительным.", r.Classification.Coverage*100))
	}
	if r.CycleDates.DurationMismatchCount > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("У %d циклов есть заметное расхождение между введённым МРП и длительностью по датам; анализ безотказной работы использует даты, если они заданы корректно.", r.CycleDates.DurationMismatchCount))
	}
	return r
}

func mrpSurvivalEconomicTriplet(a MRPAdvancedSummary) (low, base, high float64, ok bool, source string) {
	// First preference: object-level KM distribution. It remains separate from the
	// legacy median baseline and is not silently substituted elsewhere.
	if a.ObjectAny.HasQuartiles && a.ObjectAny.EventCount >= 5 && a.ObjectAny.Backtest.Eligible {
		return a.ObjectAny.Q25Days, a.ObjectAny.MedianDays, a.ObjectAny.Q75Days, true, "Каплан–Майер по истории объекта; последовательная проверка по времени подтверждена"
	}
	if a.CohortEligible && a.CohortAny.HasQuartiles && a.CohortAny.Backtest.Eligible {
		return a.CohortAny.Q25Days, a.CohortAny.MedianDays, a.CohortAny.Q75Days, true, "Каплан–Майер по сопоставимой группе; последовательная проверка по времени подтверждена"
	}
	return 0, 0, 0, false, ""
}

func formatProbability(p float64, ok bool) string {
	if !ok {
		return "не рассчитано"
	}
	return fmt.Sprintf("%.0f%%", p*100)
}

func formatMRPAdvanced(a MRPAdvancedSummary) string {
	var b strings.Builder
	b.WriteString("\r\n\r\nБЕЗОТКАЗНАЯ РАБОТА И НАДЁЖНОСТЬ\r\n")
	if a.CurrentAgeDays > 0 {
		fmt.Fprintf(&b, "Возраст текущего незавершённого цикла: %.0f сут. (%s).\r\n", a.CurrentAgeDays, a.CurrentAgeSource)
	}
	s := a.ObjectAny
	fmt.Fprintf(&b, "До любого следующего вмешательства: циклов %d; событий %d; цензурировано %d.\r\n", s.ObservationCount, s.EventCount, s.CensoredCount)
	if s.HasMedianTotalLife {
		fmt.Fprintf(&b, "P50 общей длительности цикла по Каплану–Майеру: %.0f сут.\r\n", s.MedianTotalLifeDays)
	}
	if s.HasQuartiles {
		fmt.Fprintf(&b, "Распределение времени до события P25/P50/P75: %.0f / %.0f / %.0f сут. (это квантили распределения, не доверительный интервал).\r\n", s.Q25Days, s.MedianDays, s.Q75Days)
	}
	if a.CurrentAgeDays > 0 {
		if s.HasConditionalMedianMore {
			fmt.Fprintf(&b, "Условная медиана дополнительной работы при уже достигнутых %.0f сут.: ещё %.0f сут.\r\n", a.CurrentAgeDays, s.ConditionalMedianMoreDays)
		}
		ready := s.EventCount >= 5 && s.Backtest.Eligible
		fmt.Fprintf(&b, "Вероятность проработать ещё 30/90/180/365 сут.: %s / %s / %s / %s.\r\n", formatProbability(s.Survival30More, ready && s.HasSurvival30More), formatProbability(s.Survival90More, ready && s.HasSurvival90More), formatProbability(s.Survival180More, ready && s.HasSurvival180More), formatProbability(s.Survival365More, ready && s.HasSurvival365More))
		if !ready {
			b.WriteString("Проценты скрыты: нужно не менее 5 событий и успешная последовательная проверка на последующих циклах.\r\n")
		}
	}
	f := a.ObjectFailure
	fmt.Fprintf(&b, "До классифицированного отказа: событий-отказов %d; прочие/неопределённые завершения рассматриваются как цензурирование для этой цели.\r\n", f.EventCount)
	if f.HasMedianTotalLife {
		fmt.Fprintf(&b, "P50 времени до отказа: %.0f сут.\r\n", f.MedianTotalLifeDays)
	} else {
		b.WriteString("P50 времени до отказа: пока не определяется из-за недостатка классифицированных отказов.\r\n")
	}
	if a.CohortObservationCount > 0 {
		fmt.Fprintf(&b, "Сопоставимая группа по ГНО + семейству осложнения: %d циклов / %d объектов; автоматическое использование: %s.\r\n", a.CohortObservationCount, a.CohortDistinctObjects, ruYesNo(a.CohortEligible))
		if a.CohortAny.HasMedianTotalLife {
			fmt.Fprintf(&b, "P50 группы до вмешательства: %.0f сут.\r\n", a.CohortAny.MedianTotalLifeDays)
		}
	}
	if a.Recurrent.Count >= 4 {
		fmt.Fprintf(&b, "Повторные события объекта: ранняя медиана %.0f сут., поздняя %.0f сут., изменение %+.0f%% — %s. Это описательный тренд, не причинная модель.\r\n", a.Recurrent.EarlyMedianDays, a.Recurrent.LateMedianDays, a.Recurrent.ChangePct, a.Recurrent.Direction)
	}
	if s.Backtest.Eligible {
		fmt.Fprintf(&b, "Последовательная проверка прогноза по времени: тестовых циклов %d; медианная абсолютная ошибка %.0f сут. (%d); оценка Брайера на 90/180/365 сут. %.3f / %.3f / %.3f; покрытие P25–P75 %.0f%% (%d).\r\n", s.Backtest.TestCount, s.Backtest.MedianAbsError, s.Backtest.MedianTests, s.Backtest.Brier90, s.Backtest.Brier180, s.Backtest.Brier365, s.Backtest.TripletCoverage*100, s.Backtest.CoverageTests)
	} else {
		fmt.Fprintf(&b, "Последовательная проверка прогноза по времени: %s\r\n", s.Backtest.Note)
	}
	low, base, high, ok, src := mrpSurvivalEconomicTriplet(a)
	if ok {
		fmt.Fprintf(&b, "Набор неопределённости для экономики доступен отдельно: %.0f / %.0f / %.0f сут., источник: %s. Он не подменяет старый базовый расчёт автоматически.\r\n", low, base, high, src)
	}
	if a.Classification.CompletedCycles > 0 {
		fmt.Fprintf(&b, "Классификация событий: подтверждено %d/%d (%.0f%%); подтверждённых отказов %d; автоклассификаций %d; неизвестных %d.\r\n", a.Classification.ConfirmedCycles, a.Classification.CompletedCycles, a.Classification.Coverage*100, a.Classification.ConfirmedFailures, a.Classification.AutoCycles, a.Classification.UnknownCycles)
	}
	if a.CycleDates.CompletedCycles > 0 {
		fmt.Fprintf(&b, "Даты циклов: явные начало+конец %d/%d; расхождений длительности %d; без явной даты начала %d.\r\n", a.CycleDates.ExplicitDateCycles, a.CycleDates.CompletedCycles, a.CycleDates.DurationMismatchCount, a.CycleDates.MissingStartCount)
	}
	if a.Competing.ObservationCount > 0 {
		fmt.Fprintf(&b, "Конкурирующие причины завершения цикла: подтверждённые отказы %d; плановые вмешательства %d; прочие ремонты %d; неизвестные/неподтверждённые %d.\r\n", a.Competing.FailureCount, a.Competing.PlannedCount, a.Competing.RepairCount, a.Competing.UnknownCount)
		if a.Competing.Has365 && s.EventCount >= 5 && s.Backtest.Eligible {
			fmt.Fprintf(&b, "Накопленная вероятность к 365 сут.: отказ %.0f%%; плановое вмешательство %.0f%%; прочий ремонт %.0f%%; неизвестное событие %.0f%%.\r\n", a.Competing.FailureCIF365*100, a.Competing.PlannedCIF365*100, a.Competing.RepairCIF365*100, a.Competing.UnknownCIF365*100)
		}
	}
	if len(a.FailureModes) > 0 {
		parts := []string{}
		for i, m := range a.FailureModes {
			if i >= 5 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s — %d", m.Mode, m.Count))
		}
		b.WriteString("Подтверждённые механизмы отказов: " + strings.Join(parts, "; ") + ".\r\n")
	}
	if a.RecurrentExposure.CompletedEvents > 0 {
		fmt.Fprintf(&b, "Повторные события: экспозиция %.0f сут.; %.2f вмешательства/год экспозиции; медианный интервал %.0f сут.; %s.\r\n", a.RecurrentExposure.ExposureDays, a.RecurrentExposure.EventsPerYear, a.RecurrentExposure.MedianGapDays, a.RecurrentExposure.Trend)
	}
	fmt.Fprintf(&b, "Готовность данных для будущей регрессии Кокса и модели ускоренного времени отказа (AFT): строк %d; объектов %d; подтверждённых отказов %d; полный режимный снимок %d; достаточно данных для исследовательской проверки: %s. Коэффициенты в этой версии не оцениваются.\r\n", a.CovariateReadiness.Rows, a.CovariateReadiness.DistinctObjects, a.CovariateReadiness.ConfirmedFailureRows, a.CovariateReadiness.CompleteRegimeSnapshots, ruYesNo(a.CovariateReadiness.ResearchBacktestReady))
	warnings := append([]string{}, a.Warnings...)
	warnings = append(warnings, s.Warnings...)
	if len(warnings) > 0 {
		b.WriteString("Ограничения анализа безотказной работы:\r\n")
		for _, w := range warnings {
			b.WriteString("• " + w + "\r\n")
		}
	}
	return strings.TrimSpace(b.String())
}
