//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

func kmObs(days float64, event bool) MRPSurvivalObservation {
	return MRPSurvivalObservation{ObjectKey: "OBJ", DurationDays: days, Event: event, EventType: func() string {
		if event {
			return "отказ"
		}
		return "в работе"
	}(), Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)}
}

func TestKaplanMeierHandlesRightCensoring(t *testing.T) {
	pts := kaplanMeier([]MRPSurvivalObservation{kmObs(100, true), kmObs(150, false), kmObs(200, true)})
	if len(pts) != 3 {
		t.Fatalf("points=%d", len(pts))
	}
	if math.Abs(pts[0].Survival-2.0/3.0) > 1e-9 {
		t.Fatalf("S100=%v", pts[0].Survival)
	}
	if pts[1].Censored != 1 || pts[1].Events != 0 {
		t.Fatalf("censor point=%+v", pts[1])
	}
	if pts[2].Survival != 0 {
		t.Fatalf("S200=%v", pts[2].Survival)
	}
}

func TestPlannedInterventionIsNotFailure(t *testing.T) {
	if mrpEventObserved("плановая смена ГНО", true, MRPEventFailure) {
		t.Fatal("planned intervention must not be failure")
	}
	if !mrpEventObserved("плановая смена ГНО", true, MRPEventAnyIntervention) {
		t.Fatal("planned intervention is still an intervention")
	}
}

func TestUnknownCompletedCycleCountsAsInterventionButNotFailure(t *testing.T) {
	if !mrpEventObserved("не классифицировано", false, MRPEventAnyIntervention) {
		t.Fatal("completed cycle must count for any intervention target")
	}
	if mrpEventObserved("не классифицировано", false, MRPEventFailure) {
		t.Fatal("unknown event must not be promoted to failure")
	}
}

func TestConditionalSurvivalUsesAlreadyAchievedAge(t *testing.T) {
	pts := kaplanMeier([]MRPSurvivalObservation{kmObs(100, true), kmObs(200, true), kmObs(300, true), kmObs(400, true)})
	p, ok := conditionalSurvival(pts, 150, 100)
	if !ok || p <= 0 || p >= 1 {
		t.Fatalf("conditional p=%v ok=%v", p, ok)
	}
}

func TestCurrentCycleIsAddedAsCensored(t *testing.T) {
	s := summarizeMRPSurvival([]MRPSurvivalObservation{kmObs(100, true), kmObs(200, true)}, MRPEventAnyIntervention, 250, "current")
	if s.CensoredCount != 1 || s.ObservationCount != 3 {
		t.Fatalf("summary=%+v", s)
	}
}

func TestGreenwoodIntervalIsBounded(t *testing.T) {
	pts := kaplanMeier([]MRPSurvivalObservation{kmObs(100, true), kmObs(150, false), kmObs(200, true), kmObs(250, false)})
	for _, p := range pts {
		if p.Lower95 < 0 || p.Upper95 > 1 || p.Lower95 > p.Upper95 {
			t.Fatalf("bad CI %+v", p)
		}
	}
}

func TestOutcomeClassificationSeparatesFailureAndPlanned(t *testing.T) {
	e, p, m := classifyMRPOutcomeEvent("Отказ УЭЦН", "пробой кабеля")
	if e != "отказ" || p || !strings.Contains(strings.ToLower(m), "кабел") {
		t.Fatalf("classification=%q %v %q", e, p, m)
	}
	e, p, _ = classifyMRPOutcomeEvent("Плановая смена типоразмера", "оптимизация")
	if e != "плановое вмешательство" || !p {
		t.Fatalf("planned=%q %v", e, p)
	}
}

func TestMRPCurrentCycleAgeFromLatestEvent(t *testing.T) {
	in := MRPForecastInput{CurrentDate: time.Date(2026, 1, 11, 0, 0, 0, 0, time.Local), Observations: []MRPObservation{{MRPDays: 100, Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)}}}
	age, src, w := mrpCurrentCycleAge(in)
	if age != 10 || src == "" || w != "" {
		t.Fatalf("age=%v src=%q warn=%q", age, src, w)
	}
}

func TestSurvivalEconomicTripletRequiresGate(t *testing.T) {
	a := MRPAdvancedSummary{ObjectAny: MRPSurvivalSummary{HasQuartiles: true, Q25Days: 100, MedianDays: 200, Q75Days: 300, EventCount: 6, Backtest: MRPSurvivalBacktest{Eligible: false}}}
	if _, _, _, ok, _ := mrpSurvivalEconomicTriplet(a); ok {
		t.Fatal("triplet must not pass without backtest")
	}
	a.ObjectAny.Backtest.Eligible = true
	lo, base, hi, ok, _ := mrpSurvivalEconomicTriplet(a)
	if !ok || lo != 100 || base != 200 || hi != 300 {
		t.Fatalf("triplet=%v %v %v %v", lo, base, hi, ok)
	}
}

func TestRecurrentTrendIsDescriptive(t *testing.T) {
	obs := mrpObs(500, 480, 300, 250)
	r := mrpRecurrentTrend(obs)
	if r.Count != 4 || r.Direction != "интервалы заметно сокращаются" {
		t.Fatalf("trend=%+v", r)
	}
}

func TestFailureTargetCensorsOtherCauses(t *testing.T) {
	obs := []MRPSurvivalObservation{
		{ObjectKey: "A", DurationDays: 100, EventType: "отказ", ClassificationConfirmed: true, ClassificationSource: "ручное подтверждение"},
		{ObjectKey: "B", DurationDays: 120, EventType: "ремонт"},
		{ObjectKey: "C", DurationDays: 140, EventType: "плановое вмешательство", Planned: true},
	}
	s := summarizeMRPSurvival(obs, MRPEventFailure, 0, "")
	if s.EventCount != 1 || s.CensoredCount != 2 {
		t.Fatalf("failure summary=%+v", s)
	}
}

func TestSurvivalBacktestUsesChronologicalPastOnly(t *testing.T) {
	obs := []MRPSurvivalObservation{}
	for i, d := range []float64{100, 120, 140, 160, 180, 200, 220, 240, 260} {
		obs = append(obs, MRPSurvivalObservation{ObjectKey: "O", DurationDays: d, EventType: "ремонт", Date: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.Local)})
	}
	r := mrpSurvivalBacktest(obs, MRPEventAnyIntervention)
	if !r.Eligible || r.TestCount != 4 {
		t.Fatalf("backtest=%+v", r)
	}
}

func TestRepeatedCyclesTriggerClusterWarning(t *testing.T) {
	obs := []MRPSurvivalObservation{{ObjectKey: "O", DurationDays: 100, EventType: "ремонт"}, {ObjectKey: "O", DurationDays: 200, EventType: "ремонт"}, {ObjectKey: "O", DurationDays: 300, EventType: "ремонт"}}
	s := summarizeMRPSurvival(obs, MRPEventAnyIntervention, 0, "")
	if !strings.Contains(strings.Join(s.Warnings, " "), "кластер") {
		t.Fatalf("warnings=%v", s.Warnings)
	}
}

func TestEconomicContractCarriesSurvivalProvenance(t *testing.T) {
	c := TechnicalScenarioContract{ContractVersion: technicalScenarioContractVersion, ObjectKey: "OBJ", ScenarioID: "S", BaseMethod: "M", SurvivalLowDays: 300, SurvivalBaseDays: 400, SurvivalHighDays: 550, HasSurvivalTriplet: true, SurvivalSource: "KM", SurvivalModelVersion: mrpSurvivalModelVersion, SurvivalEventCount: 8, SurvivalCensoredCount: 2, CurrentCycleAgeDays: 120, SurvivalBacktestStatus: "walk-forward PASS"}
	e := economicContractFromTechnical(c)
	if !e.HasSurvivalTriplet || e.SurvivalBaseDays != 400 || e.SurvivalSource != "KM" || e.SurvivalEventCount != 8 {
		t.Fatalf("contract=%+v", e)
	}
}

func TestLegacyBaselineRemainsSeparateFromSurvival(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ", CurrentMRPDays: 300, CurrentDate: time.Date(2026, 1, 10, 0, 0, 0, 0, time.Local), Observations: mrpObs(200, 300, 400)})
	if r.BaseDays != 300 {
		t.Fatalf("legacy baseline changed: %v", r.BaseDays)
	}
}
