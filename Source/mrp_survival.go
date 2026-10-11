package main

import (
	"math"
	"sort"
	"strings"
	"time"
)

// MRP-survival-2.1 separates reporting MRP, completed intervention intervals and
// time-to-event reliability. It supports right-censoring, competing events and
// does not treat a planned intervention as a failure unless explicitly classified.
const mrpSurvivalModelVersion = "MRP-reliability-2.3-ru"

type MRPEventTarget string

const (
	MRPEventAnyIntervention MRPEventTarget = "any_intervention"
	MRPEventFailure         MRPEventTarget = "failure"
)

type MRPSurvivalObservation struct {
	ObjectKey               string
	DurationDays            float64
	Event                   bool
	EventType               string
	Planned                 bool
	FailureMode             string
	ClassificationSource    string
	ClassificationConfirmed bool
	CycleStartDate          time.Time
	CycleEndDate            time.Time
	DurationSource          string
	DurationWarning         string
	Date                    time.Time
	Source                  string
}

type MRPKMPoint struct {
	TimeDays float64
	AtRisk   int
	Events   int
	Censored int
	Survival float64
	StdError float64
	Lower95  float64
	Upper95  float64
}

type MRPSurvivalBacktest struct {
	Eligible        bool
	Target          MRPEventTarget
	TrainingCount   int
	TestCount       int
	MedianAbsError  float64
	MedianTests     int
	Brier90         float64
	Brier90Tests    int
	Brier180        float64
	Brier180Tests   int
	Brier365        float64
	Brier365Tests   int
	TripletCoverage float64
	CoverageTests   int
	Note            string
}

type MRPSurvivalSummary struct {
	ModelVersion              string
	Target                    MRPEventTarget
	ObservationCount          int
	EventCount                int
	CensoredCount             int
	DistinctObjectCount       int
	CurrentAgeDays            float64
	CurrentAgeSource          string
	MedianTotalLifeDays       float64
	HasMedianTotalLife        bool
	ConditionalMedianMoreDays float64
	HasConditionalMedianMore  bool
	Survival30More            float64
	HasSurvival30More         bool
	Survival90More            float64
	HasSurvival90More         bool
	Survival180More           float64
	HasSurvival180More        bool
	Survival365More           float64
	HasSurvival365More        bool
	Q25Days                   float64
	MedianDays                float64
	Q75Days                   float64
	HasQuartiles              bool
	Points                    []MRPKMPoint
	Backtest                  MRPSurvivalBacktest
	Warnings                  []string
}

func normalizeMRPEventType(eventType string, planned bool) string {
	n := strings.ToLower(strings.TrimSpace(eventType))
	if planned {
		return "плановое вмешательство"
	}
	switch {
	case strings.Contains(n, "отказ") || strings.Contains(n, "авар"):
		return "отказ"
	case strings.Contains(n, "план") || strings.Contains(n, "гтм") || strings.Contains(n, "оптимиз"):
		return "плановое вмешательство"
	case strings.Contains(n, "ремонт") || strings.Contains(n, "спо") || strings.Contains(n, "подъем") || strings.Contains(n, "подъём"):
		return "ремонт"
	case strings.Contains(n, "ценз") || strings.Contains(n, "в работе"):
		return "цензурировано"
	case n == "":
		return "не классифицировано"
	default:
		return strings.TrimSpace(eventType)
	}
}

func mrpEventObserved(eventType string, planned bool, target MRPEventTarget) bool {
	class := normalizeMRPEventType(eventType, planned)
	if target == MRPEventFailure {
		// Cause-specific reliability: planned/other/unknown completed interventions
		// terminate the observed interval but are censored for the failure target.
		return class == "отказ"
	}
	// For time to any intervention every completed factual cycle is an event;
	// only a current still-operating cycle is right-censored.
	return class != "цензурировано" && class != "в работе"
}

func greenwoodLogLogCI(s, greenwoodSum float64) (float64, float64) {
	if s <= 0 {
		return 0, 0
	}
	if s >= 1 || greenwoodSum <= 0 {
		return s, s
	}
	logS := math.Log(s)
	if logS == 0 {
		return s, s
	}
	seLogLog := math.Sqrt(greenwoodSum) / math.Abs(logS)
	eta := math.Log(-logS)
	z := 1.959963984540054
	lower := math.Exp(-math.Exp(eta + z*seLogLog))
	upper := math.Exp(-math.Exp(eta - z*seLogLog))
	if lower < 0 {
		lower = 0
	}
	if upper > 1 {
		upper = 1
	}
	return lower, upper
}

func kaplanMeier(obs []MRPSurvivalObservation) []MRPKMPoint {
	valid := make([]MRPSurvivalObservation, 0, len(obs))
	for _, o := range obs {
		if o.DurationDays <= 0 || math.IsNaN(o.DurationDays) || math.IsInf(o.DurationDays, 0) {
			continue
		}
		valid = append(valid, o)
	}
	sort.SliceStable(valid, func(i, j int) bool {
		if math.Abs(valid[i].DurationDays-valid[j].DurationDays) < 1e-9 {
			return valid[i].Event && !valid[j].Event
		}
		return valid[i].DurationDays < valid[j].DurationDays
	})
	if len(valid) == 0 {
		return nil
	}

	survival := 1.0
	greenwoodSum := 0.0
	points := make([]MRPKMPoint, 0)
	atRisk := len(valid)
	for i := 0; i < len(valid); {
		t := valid[i].DurationDays
		events, censored, j := 0, 0, i
		for j < len(valid) && math.Abs(valid[j].DurationDays-t) < 1e-9 {
			if valid[j].Event {
				events++
			} else {
				censored++
			}
			j++
		}
		if events > 0 && atRisk > 0 {
			survival *= 1.0 - float64(events)/float64(atRisk)
			if atRisk-events > 0 {
				greenwoodSum += float64(events) / (float64(atRisk) * float64(atRisk-events))
			}
		}
		se := 0.0
		if greenwoodSum > 0 {
			se = survival * math.Sqrt(greenwoodSum)
		}
		lo, hi := greenwoodLogLogCI(survival, greenwoodSum)
		points = append(points, MRPKMPoint{TimeDays: t, AtRisk: atRisk, Events: events, Censored: censored, Survival: survival, StdError: se, Lower95: lo, Upper95: hi})
		atRisk -= events + censored
		i = j
	}
	return points
}

func kmSurvivalAt(points []MRPKMPoint, t float64) (float64, bool) {
	if len(points) == 0 || t < 0 {
		return 0, false
	}
	s := 1.0
	for _, p := range points {
		if p.TimeDays > t {
			break
		}
		s = p.Survival
	}
	return s, true
}

// kmTimeAtSurvival returns the first observed event time where S(t) <= level.
func kmTimeAtSurvival(points []MRPKMPoint, level float64) (float64, bool) {
	for _, p := range points {
		if p.Events > 0 && p.Survival <= level {
			return p.TimeDays, true
		}
	}
	return 0, false
}

func conditionalSurvival(points []MRPKMPoint, age, additional float64) (float64, bool) {
	sAge, ok1 := kmSurvivalAt(points, age)
	sFuture, ok2 := kmSurvivalAt(points, age+additional)
	if !ok1 || !ok2 || sAge <= 0 {
		return 0, false
	}
	p := sFuture / sAge
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	return p, true
}

func conditionalMedianAdditional(points []MRPKMPoint, age float64) (float64, bool) {
	sAge, ok := kmSurvivalAt(points, age)
	if !ok || sAge <= 0 {
		return 0, false
	}
	threshold := 0.5 * sAge
	for _, p := range points {
		if p.TimeDays < age || p.Events == 0 {
			continue
		}
		if p.Survival <= threshold {
			return math.Max(0, p.TimeDays-age), true
		}
	}
	return 0, false
}

func countDistinctMRPObjects(obs []MRPSurvivalObservation) int {
	m := map[string]bool{}
	for _, o := range obs {
		k := strings.TrimSpace(o.ObjectKey)
		if k != "" {
			m[k] = true
		}
	}
	return len(m)
}

func mrpSurvivalBacktest(obs []MRPSurvivalObservation, target MRPEventTarget) MRPSurvivalBacktest {
	valid := make([]MRPSurvivalObservation, 0, len(obs))
	for _, o := range obs {
		if o.DurationDays <= 0 || o.Date.IsZero() {
			continue
		}
		x := o
		x.Event = mrpObservationEventObserved(o, target)
		valid = append(valid, x)
	}
	sort.SliceStable(valid, func(i, j int) bool { return valid[i].Date.Before(valid[j].Date) })
	r := MRPSurvivalBacktest{Target: target, Note: "Последовательная проверка анализа безотказной работы: каждая тестовая точка оценивается только по более ранней истории."}
	if len(valid) < 8 {
		r.Note = "Для последовательной проверки анализа безотказной работы требуется минимум 8 датированных циклов; модель остаётся описательной."
		return r
	}
	absErr := []float64{}
	brier90, brier180, brier365 := []float64{}, []float64{}, []float64{}
	coverage := []float64{}
	for i := 5; i < len(valid); i++ {
		train := valid[:i]
		pts := kaplanMeier(train)
		if med, ok := kmTimeAtSurvival(pts, 0.5); ok && valid[i].Event {
			absErr = append(absErr, math.Abs(med-valid[i].DurationDays))
		}
		q25, q25ok := kmTimeAtSurvival(pts, 0.75)
		q75, q75ok := kmTimeAtSurvival(pts, 0.25)
		if q25ok && q75ok && valid[i].Event {
			v := 0.0
			if valid[i].DurationDays >= q25 && valid[i].DurationDays <= q75 {
				v = 1
			}
			coverage = append(coverage, v)
		}
		for _, h := range []struct {
			days float64
			dst  *[]float64
		}{{90, &brier90}, {180, &brier180}, {365, &brier365}} {
			if sh, ok := kmSurvivalAt(pts, h.days); ok {
				o := valid[i]
				// A cycle censored before the horizon has unknown status and is excluded.
				if o.Event || o.DurationDays >= h.days {
					y := 0.0
					if o.DurationDays >= h.days {
						y = 1
					}
					d := y - sh
					*h.dst = append(*h.dst, d*d)
				}
			}
		}
	}
	r.TrainingCount = 5
	r.TestCount = len(valid) - 5
	r.MedianTests = len(absErr)
	if len(absErr) > 0 {
		r.MedianAbsError = medianFloat64(absErr)
	}
	mean := func(v []float64) float64 {
		if len(v) == 0 {
			return 0
		}
		s := 0.0
		for _, x := range v {
			s += x
		}
		return s / float64(len(v))
	}
	r.Brier90Tests, r.Brier90 = len(brier90), mean(brier90)
	r.Brier180Tests, r.Brier180 = len(brier180), mean(brier180)
	r.Brier365Tests, r.Brier365 = len(brier365), mean(brier365)
	r.CoverageTests, r.TripletCoverage = len(coverage), mean(coverage)
	r.Eligible = r.TestCount >= 3 && (r.MedianTests >= 2 || r.Brier180Tests >= 3 || r.Brier365Tests >= 3)
	if target == MRPEventFailure {
		confirmed := 0
		for _, o := range valid {
			if mrpConfirmedClassification(o) && normalizeMRPEventType(o.EventType, o.Planned) == "отказ" {
				confirmed++
			}
		}
		if confirmed < 5 {
			r.Eligible = false
			r.Note = "Проверка прогноза по конкретной причине не активирована: требуется минимум 5 подтверждённых отказов."
		}
	}
	return r
}

func summarizeMRPSurvival(obs []MRPSurvivalObservation, target MRPEventTarget, currentAge float64, currentAgeSource string) MRPSurvivalSummary {
	r := MRPSurvivalSummary{ModelVersion: mrpSurvivalModelVersion, Target: target, CurrentAgeDays: currentAge, CurrentAgeSource: currentAgeSource}
	cleaned := make([]MRPSurvivalObservation, 0, len(obs)+1)
	for _, o := range obs {
		if o.DurationDays <= 0 {
			continue
		}
		o.Event = mrpObservationEventObserved(o, target)
		cleaned = append(cleaned, o)
		if o.Event {
			r.EventCount++
		} else {
			r.CensoredCount++
		}
	}
	r.DistinctObjectCount = countDistinctMRPObjects(cleaned)
	if currentAge > 0 {
		objectKey := ""
		if len(cleaned) > 0 {
			objectKey = cleaned[len(cleaned)-1].ObjectKey
		}
		cleaned = append(cleaned, MRPSurvivalObservation{ObjectKey: objectKey, DurationDays: currentAge, Event: false, EventType: "в работе", Source: currentAgeSource})
		r.CensoredCount++
	}
	r.ObservationCount = len(cleaned)
	r.Points = kaplanMeier(cleaned)
	r.Backtest = mrpSurvivalBacktest(obs, target)
	if len(r.Points) == 0 {
		r.Warnings = append(r.Warnings, "Недостаточно данных для оценки Каплана–Майера (Kaplan–Meier).")
		return r
	}
	if x, ok := kmTimeAtSurvival(r.Points, 0.5); ok {
		r.MedianTotalLifeDays, r.HasMedianTotalLife = x, true
	}
	q25, ok25 := kmTimeAtSurvival(r.Points, 0.75)
	med, ok50 := kmTimeAtSurvival(r.Points, 0.5)
	q75, ok75 := kmTimeAtSurvival(r.Points, 0.25)
	if ok25 && ok50 && ok75 {
		r.Q25Days, r.MedianDays, r.Q75Days, r.HasQuartiles = q25, med, q75, true
	}
	if currentAge > 0 {
		if x, ok := conditionalMedianAdditional(r.Points, currentAge); ok {
			r.ConditionalMedianMoreDays, r.HasConditionalMedianMore = x, true
		}
		if p, ok := conditionalSurvival(r.Points, currentAge, 30); ok {
			r.Survival30More, r.HasSurvival30More = p, true
		}
		if p, ok := conditionalSurvival(r.Points, currentAge, 90); ok {
			r.Survival90More, r.HasSurvival90More = p, true
		}
		if p, ok := conditionalSurvival(r.Points, currentAge, 180); ok {
			r.Survival180More, r.HasSurvival180More = p, true
		}
		if p, ok := conditionalSurvival(r.Points, currentAge, 365); ok {
			r.Survival365More, r.HasSurvival365More = p, true
		}
	}
	if r.EventCount < 2 {
		r.Warnings = append(r.Warnings, "Событий недостаточно для устойчивой оценки медианы ресурса; результат считать ориентиром.")
	}
	if r.CensoredCount == 0 {
		r.Warnings = append(r.Warnings, "В выборке нет цензурированных интервалов; проверьте, что действующие циклы не были потеряны.")
	}
	if r.ObservationCount > r.DistinctObjectCount && r.DistinctObjectCount > 0 {
		r.Warnings = append(r.Warnings, "Есть повторные циклы одних и тех же объектов: интервалы Гринвуда (Greenwood) для оценки Каплана–Майера (Kaplan–Meier) не являются кластер-робастными; использовать как описательную неопределённость, а не как окончательную статистическую гарантию.")
	}
	return r
}

// Descriptive recurrent-event trend. It deliberately does not infer causality.
type MRPRecurrentTrend struct {
	Count           int
	EarlyMedianDays float64
	LateMedianDays  float64
	ChangePct       float64
	Direction       string
}

func mrpRecurrentTrend(obs []MRPObservation) MRPRecurrentTrend {
	valid := validMRPObservations(obs)
	r := MRPRecurrentTrend{Count: len(valid), Direction: "недостаточно данных"}
	if len(valid) < 4 {
		return r
	}
	mid := len(valid) / 2
	a, b := []float64{}, []float64{}
	for _, o := range valid[:mid] {
		a = append(a, o.MRPDays)
	}
	for _, o := range valid[mid:] {
		b = append(b, o.MRPDays)
	}
	r.EarlyMedianDays, r.LateMedianDays = medianFloat64(a), medianFloat64(b)
	if r.EarlyMedianDays > 0 {
		r.ChangePct = (r.LateMedianDays/r.EarlyMedianDays - 1) * 100
	}
	switch {
	case r.ChangePct <= -20:
		r.Direction = "интервалы заметно сокращаются"
	case r.ChangePct >= 20:
		r.Direction = "интервалы заметно увеличиваются"
	default:
		r.Direction = "выраженного изменения медианы интервалов не видно"
	}
	return r
}
