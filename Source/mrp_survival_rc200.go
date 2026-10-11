package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type MRPCompetingCause string

const (
	MRPCompetingFailure MRPCompetingCause = "failure"
	MRPCompetingPlanned MRPCompetingCause = "planned"
	MRPCompetingRepair  MRPCompetingCause = "repair_other"
	MRPCompetingUnknown MRPCompetingCause = "unknown"
)

type MRPClassificationQuality struct {
	CompletedCycles   int
	ConfirmedCycles   int
	AutoCycles        int
	UnknownCycles     int
	ConfirmedFailures int
	Coverage          float64
	FailureCoverage   float64
	Note              string
}

type MRPCycleDateQuality struct {
	CompletedCycles       int
	ExplicitDateCycles    int
	DurationMismatchCount int
	MissingStartCount     int
	InvalidDateCount      int
	Note                  string
}

type MRPCIFPoint struct {
	TimeDays   float64
	AtRisk     int
	Survival   float64
	FailureCIF float64
	PlannedCIF float64
	RepairCIF  float64
	UnknownCIF float64
}

type MRPCompetingRiskSummary struct {
	ObservationCount int
	EventCount       int
	CensoredCount    int
	FailureCount     int
	PlannedCount     int
	RepairCount      int
	UnknownCount     int
	FailureCIF365    float64
	PlannedCIF365    float64
	RepairCIF365     float64
	UnknownCIF365    float64
	Has365           bool
	FailureCIF730    float64
	PlannedCIF730    float64
	RepairCIF730     float64
	UnknownCIF730    float64
	Has730           bool
	Points           []MRPCIFPoint
	Warnings         []string
}

type MRPFailureModeCount struct {
	Mode  string
	Count int
}

type MRPRecurrentExposureSummary struct {
	CompletedEvents int
	ExposureDays    float64
	EventsPerYear   float64
	MedianGapDays   float64
	FirstHalfMedian float64
	LastHalfMedian  float64
	Trend           string
	Note            string
}

type MRPCovariateReadiness struct {
	Rows                    int
	EventRows               int
	ConfirmedFailureRows    int
	DistinctObjects         int
	CompleteEquipment       int
	CompleteComplication    int
	CompleteRegimeSnapshots int
	ResearchBacktestReady   bool
	Note                    string
}

func mrpConfirmedClassification(o MRPSurvivalObservation) bool {
	if o.ClassificationConfirmed {
		return true
	}
	s := strings.ToLower(strings.TrimSpace(o.ClassificationSource))
	if s == "" || strings.Contains(s, "авто") || strings.Contains(s, "требуется") {
		return false
	}
	return strings.Contains(s, "руч") || strings.Contains(s, "подтверж") || strings.Contains(s, "verified") || strings.Contains(s, "confirmed")
}

func mrpObservationEventObserved(o MRPSurvivalObservation, target MRPEventTarget) bool {
	class := normalizeMRPEventType(o.EventType, o.Planned)
	if target == MRPEventFailure {
		// RC200: an inferred legacy label must never silently become a confirmed failure.
		return class == "отказ" && mrpConfirmedClassification(o)
	}
	return class != "цензурировано" && class != "в работе"
}

func mrpCompetingCause(o MRPSurvivalObservation) (MRPCompetingCause, bool) {
	class := normalizeMRPEventType(o.EventType, o.Planned)
	if class == "цензурировано" || class == "в работе" {
		return "", false
	}
	if class == "отказ" {
		if mrpConfirmedClassification(o) {
			return MRPCompetingFailure, true
		}
		return MRPCompetingUnknown, true
	}
	if class == "плановое вмешательство" {
		return MRPCompetingPlanned, true
	}
	if class == "ремонт" || class == "прочее вмешательство" {
		return MRPCompetingRepair, true
	}
	return MRPCompetingUnknown, true
}

func mrpClassificationQuality(obs []MRPSurvivalObservation) MRPClassificationQuality {
	r := MRPClassificationQuality{Note: "Для прогноза отказов по конкретной причине учитываются только подтверждённые классификации; автоклассификация остаётся подсказкой."}
	completedFailures := 0
	for _, o := range obs {
		if o.DurationDays <= 0 {
			continue
		}
		class := normalizeMRPEventType(o.EventType, o.Planned)
		if class == "цензурировано" || class == "в работе" {
			continue
		}
		r.CompletedCycles++
		if mrpConfirmedClassification(o) {
			r.ConfirmedCycles++
		} else {
			r.AutoCycles++
		}
		if class == "не классифицировано" || class == "" {
			r.UnknownCycles++
		}
		if class == "отказ" {
			completedFailures++
			if mrpConfirmedClassification(o) {
				r.ConfirmedFailures++
			}
		}
	}
	if r.CompletedCycles > 0 {
		r.Coverage = float64(r.ConfirmedCycles) / float64(r.CompletedCycles)
	}
	if completedFailures > 0 {
		r.FailureCoverage = float64(r.ConfirmedFailures) / float64(completedFailures)
	}
	return r
}

func mrpDateQuality(obs []MRPObservation) MRPCycleDateQuality {
	r := MRPCycleDateQuality{Note: "Явные даты начала/окончания предпочтительны для новых циклов; старые записи сохраняются без ретроспективного домысливания дат."}
	for _, o := range validMRPObservations(obs) {
		r.CompletedCycles++
		if !o.CycleStartDate.IsZero() && !o.CycleEndDate.IsZero() {
			if o.CycleEndDate.Before(o.CycleStartDate) {
				r.InvalidDateCount++
				continue
			}
			r.ExplicitDateCycles++
			d := o.CycleEndDate.Sub(o.CycleStartDate).Hours() / 24
			tolerance := math.Max(2, 0.05*math.Max(d, 1))
			if o.MRPDays > 0 && math.Abs(o.MRPDays-d) > tolerance {
				r.DurationMismatchCount++
			}
		} else if o.CycleStartDate.IsZero() {
			r.MissingStartCount++
		}
	}
	return r
}

func causeValue(point MRPCIFPoint, cause MRPCompetingCause) float64 {
	switch cause {
	case MRPCompetingFailure:
		return point.FailureCIF
	case MRPCompetingPlanned:
		return point.PlannedCIF
	case MRPCompetingRepair:
		return point.RepairCIF
	default:
		return point.UnknownCIF
	}
}

func aalenJohansenMRP(obs []MRPSurvivalObservation, currentAge float64) MRPCompetingRiskSummary {
	valid := make([]MRPSurvivalObservation, 0, len(obs)+1)
	for _, o := range obs {
		if o.DurationDays > 0 && !math.IsNaN(o.DurationDays) && !math.IsInf(o.DurationDays, 0) {
			valid = append(valid, o)
		}
	}
	if currentAge > 0 {
		valid = append(valid, MRPSurvivalObservation{DurationDays: currentAge, EventType: "в работе", Source: "текущий незавершённый цикл"})
	}
	sort.SliceStable(valid, func(i, j int) bool { return valid[i].DurationDays < valid[j].DurationDays })
	r := MRPCompetingRiskSummary{ObservationCount: len(valid)}
	if len(valid) == 0 {
		r.Warnings = append(r.Warnings, "Нет наблюдений для оценки конкурирующих рисков (competing risks).")
		return r
	}
	atRisk := len(valid)
	s := 1.0
	cif := map[MRPCompetingCause]float64{MRPCompetingFailure: 0, MRPCompetingPlanned: 0, MRPCompetingRepair: 0, MRPCompetingUnknown: 0}
	for i := 0; i < len(valid); {
		t := valid[i].DurationDays
		counts := map[MRPCompetingCause]int{}
		censored := 0
		j := i
		for j < len(valid) && math.Abs(valid[j].DurationDays-t) < 1e-9 {
			c, event := mrpCompetingCause(valid[j])
			if event {
				counts[c]++
			} else {
				censored++
			}
			j++
		}
		totalEvents := 0
		for _, n := range counts {
			totalEvents += n
		}
		if atRisk > 0 && totalEvents > 0 {
			sPrev := s
			for _, c := range []MRPCompetingCause{MRPCompetingFailure, MRPCompetingPlanned, MRPCompetingRepair, MRPCompetingUnknown} {
				if counts[c] > 0 {
					cif[c] += sPrev * float64(counts[c]) / float64(atRisk)
				}
			}
			s *= 1 - float64(totalEvents)/float64(atRisk)
		}
		r.EventCount += totalEvents
		r.CensoredCount += censored
		r.FailureCount += counts[MRPCompetingFailure]
		r.PlannedCount += counts[MRPCompetingPlanned]
		r.RepairCount += counts[MRPCompetingRepair]
		r.UnknownCount += counts[MRPCompetingUnknown]
		r.Points = append(r.Points, MRPCIFPoint{TimeDays: t, AtRisk: atRisk, Survival: s, FailureCIF: cif[MRPCompetingFailure], PlannedCIF: cif[MRPCompetingPlanned], RepairCIF: cif[MRPCompetingRepair], UnknownCIF: cif[MRPCompetingUnknown]})
		atRisk -= totalEvents + censored
		i = j
	}
	if p, ok := mrpCIFAt(r.Points, 365); ok {
		r.FailureCIF365, r.PlannedCIF365, r.RepairCIF365, r.UnknownCIF365, r.Has365 = p.FailureCIF, p.PlannedCIF, p.RepairCIF, p.UnknownCIF, true
	}
	if p, ok := mrpCIFAt(r.Points, 730); ok {
		r.FailureCIF730, r.PlannedCIF730, r.RepairCIF730, r.UnknownCIF730, r.Has730 = p.FailureCIF, p.PlannedCIF, p.RepairCIF, p.UnknownCIF, true
	}
	if r.UnknownCount > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d завершённых циклов имеют неподтверждённую/неизвестную причину и вынесены в отдельный отдельный класс конкурирующей причины.", r.UnknownCount))
	}
	return r
}

func mrpCIFAt(points []MRPCIFPoint, t float64) (MRPCIFPoint, bool) {
	if len(points) == 0 || t < 0 {
		return MRPCIFPoint{}, false
	}
	last := MRPCIFPoint{Survival: 1}
	for _, p := range points {
		if p.TimeDays > t {
			break
		}
		last = p
	}
	return last, true
}

func mrpFailureModes(obs []MRPSurvivalObservation) []MRPFailureModeCount {
	counts := map[string]int{}
	for _, o := range obs {
		if normalizeMRPEventType(o.EventType, o.Planned) != "отказ" || !mrpConfirmedClassification(o) {
			continue
		}
		m := strings.TrimSpace(o.FailureMode)
		if m == "" {
			m = "не классифицировано"
		}
		counts[m]++
	}
	out := make([]MRPFailureModeCount, 0, len(counts))
	for m, n := range counts {
		out = append(out, MRPFailureModeCount{Mode: m, Count: n})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Mode < out[j].Mode
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func mrpRecurrentExposure(obs []MRPObservation) MRPRecurrentExposureSummary {
	valid := validMRPObservations(obs)
	r := MRPRecurrentExposureSummary{CompletedEvents: len(valid), Note: "Описательная частота повторных вмешательств по суммарной экспозиции; не является подгонкой модели неоднородного пуассоновского процесса / модели Кроу–AMSAA (Crow–AMSAA) и не доказывает причинность."}
	gaps := make([]float64, 0, len(valid))
	for _, o := range valid {
		if o.MRPDays > 0 {
			r.ExposureDays += o.MRPDays
			gaps = append(gaps, o.MRPDays)
		}
	}
	if r.ExposureDays > 0 {
		r.EventsPerYear = float64(r.CompletedEvents) / (r.ExposureDays / 365.25)
	}
	if len(gaps) > 0 {
		r.MedianGapDays = medianFloat64(gaps)
	}
	if len(gaps) >= 4 {
		mid := len(gaps) / 2
		r.FirstHalfMedian = medianFloat64(gaps[:mid])
		r.LastHalfMedian = medianFloat64(gaps[mid:])
		pct := 0.0
		if r.FirstHalfMedian > 0 {
			pct = (r.LastHalfMedian/r.FirstHalfMedian - 1) * 100
		}
		switch {
		case pct <= -20:
			r.Trend = "интервалы сокращаются"
		case pct >= 20:
			r.Trend = "интервалы увеличиваются"
		default:
			r.Trend = "выраженного изменения не видно"
		}
	} else {
		r.Trend = "недостаточно циклов для тренда"
	}
	return r
}

func mrpCovariateReadiness(obs []MRPObservation) MRPCovariateReadiness {
	r := MRPCovariateReadiness{Rows: len(obs), Note: "Эта версия только проверяет полноту набора для будущей регрессии Кокса / модели ускоренного времени отказа; коэффициенты не оцениваются и веса факторов не назначаются."}
	objects := map[string]bool{}
	for _, o := range obs {
		if strings.TrimSpace(o.ObjectKey) != "" {
			objects[o.ObjectKey] = true
		}
		if normalizeMRPEventType(o.EventType, o.Planned) != "цензурировано" {
			r.EventRows++
		}
		if normalizeMRPEventType(o.EventType, o.Planned) == "отказ" && o.ClassificationConfirmed {
			r.ConfirmedFailureRows++
		}
		if strings.TrimSpace(o.Equipment) != "" {
			r.CompleteEquipment++
		}
		if strings.TrimSpace(o.Complication) != "" {
			r.CompleteComplication++
		}
		if strings.Contains(strings.ToLower(o.Snapshot.Completeness), "полный") {
			r.CompleteRegimeSnapshots++
		}
	}
	r.DistinctObjects = len(objects)
	// Conservative engineering gate for starting a research backtest only; not a universal statistical rule.
	r.ResearchBacktestReady = r.Rows >= 30 && r.DistinctObjects >= 5 && r.ConfirmedFailureRows >= 10 && r.CompleteEquipment*100 >= 80*maxInt(1, r.Rows) && r.CompleteComplication*100 >= 80*maxInt(1, r.Rows)
	return r
}

func mrpDurationFromOutcome(o Outcome) (days float64, source, warning string, start, end time.Time) {
	stated := toFloat(o.ActualMRP)
	start = parseDate(strings.TrimSpace(o.CycleStartDate))
	end = parseDate(strings.TrimSpace(o.CycleEndDate))
	if !start.IsZero() && !end.IsZero() {
		if end.Before(start) {
			return stated, "введённый фактический МРП", "дата окончания цикла раньше даты начала; длительность по датам не используется", start, end
		}
		d := end.Sub(start).Hours() / 24
		tolerance := math.Max(2, 0.05*math.Max(d, 1))
		if stated > 0 && math.Abs(stated-d) > tolerance {
			warning = fmt.Sprintf("введённый МРП %.0f сут. расходится с длительностью по датам %.0f сут.; в анализе безотказной работы используется длительность по датам", stated, d)
		}
		return d, "явные даты начала/окончания цикла", warning, start, end
	}
	if stated > 0 {
		return stated, "введённый фактический МРП", warning, start, end
	}
	return 0, "", "нет ни корректных дат цикла, ни фактического МРП", start, end
}
