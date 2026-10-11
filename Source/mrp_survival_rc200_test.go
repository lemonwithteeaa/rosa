//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestRC200AutoFailureIsNotConfirmedFailure(t *testing.T) {
	o := MRPSurvivalObservation{ObjectKey: "A", DurationDays: 120, EventType: "отказ", ClassificationSource: "автоклассификация старой записи"}
	if mrpObservationEventObserved(o, MRPEventFailure) {
		t.Fatal("auto-classified legacy failure must not become confirmed failure")
	}
	if !mrpObservationEventObserved(o, MRPEventAnyIntervention) {
		t.Fatal("completed intervention must remain event for any-intervention target")
	}
}

func TestRC200ManualFailureCountsAsFailure(t *testing.T) {
	o := MRPSurvivalObservation{ObjectKey: "A", DurationDays: 120, EventType: "отказ", ClassificationConfirmed: true, ClassificationSource: "ручное подтверждение"}
	if !mrpObservationEventObserved(o, MRPEventFailure) {
		t.Fatal("confirmed failure must count")
	}
}

func TestRC200DurationPrefersExplicitCycleDates(t *testing.T) {
	o := Outcome{ActualMRP: "80", CycleStartDate: "01.01.2026", CycleEndDate: "11.04.2026"}
	d, src, warn, start, end := mrpDurationFromOutcome(o)
	if start.IsZero() || end.IsZero() || d != 100 {
		t.Fatalf("duration=%v start=%v end=%v", d, start, end)
	}
	if !strings.Contains(src, "дат") || warn == "" {
		t.Fatalf("src=%q warn=%q", src, warn)
	}
}

func TestRC200CompetingRisksSeparatePlannedAndFailure(t *testing.T) {
	obs := []MRPSurvivalObservation{
		{ObjectKey: "A", DurationDays: 100, EventType: "отказ", ClassificationConfirmed: true},
		{ObjectKey: "B", DurationDays: 120, EventType: "плановое вмешательство", Planned: true, ClassificationConfirmed: true},
		{ObjectKey: "C", DurationDays: 150, EventType: "ремонт", ClassificationConfirmed: true},
		{ObjectKey: "D", DurationDays: 180, EventType: "отказ", ClassificationSource: "автоклассификация"},
		{ObjectKey: "E", DurationDays: 220, EventType: "в работе"},
	}
	r := aalenJohansenMRP(obs, 0)
	if r.FailureCount != 1 || r.PlannedCount != 1 || r.RepairCount != 1 || r.UnknownCount != 1 || r.CensoredCount != 1 {
		t.Fatalf("competing=%+v", r)
	}
	if len(r.Points) == 0 {
		t.Fatal("missing AJ points")
	}
	last := r.Points[len(r.Points)-1]
	sum := last.FailureCIF + last.PlannedCIF + last.RepairCIF + last.UnknownCIF + last.Survival
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("probability mass=%v", sum)
	}
}

func TestRC200ClassificationCoverage(t *testing.T) {
	obs := []MRPSurvivalObservation{
		{DurationDays: 100, EventType: "отказ", ClassificationConfirmed: true},
		{DurationDays: 120, EventType: "ремонт", ClassificationConfirmed: false},
		{DurationDays: 140, EventType: "плановое вмешательство", Planned: true, ClassificationConfirmed: true},
		{DurationDays: 160, EventType: "в работе"},
	}
	q := mrpClassificationQuality(obs)
	if q.CompletedCycles != 3 || q.ConfirmedCycles != 2 || q.ConfirmedFailures != 1 {
		t.Fatalf("quality=%+v", q)
	}
	if math.Abs(q.Coverage-2.0/3.0) > 1e-9 {
		t.Fatalf("coverage=%v", q.Coverage)
	}
}

func TestRC200BacktestCarriesMultipleHorizons(t *testing.T) {
	obs := []MRPSurvivalObservation{}
	for i, d := range []float64{100, 140, 180, 220, 260, 300, 340, 380, 420, 460} {
		obs = append(obs, MRPSurvivalObservation{ObjectKey: "O", DurationDays: d, EventType: "ремонт", Date: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.Local)})
	}
	r := mrpSurvivalBacktest(obs, MRPEventAnyIntervention)
	if r.TestCount < 3 || r.Brier90Tests == 0 || r.Brier180Tests == 0 || r.Brier365Tests == 0 {
		t.Fatalf("backtest=%+v", r)
	}
}

func TestRC200CauseSpecificBacktestRequiresConfirmedFailures(t *testing.T) {
	obs := []MRPSurvivalObservation{}
	for i := 0; i < 10; i++ {
		obs = append(obs, MRPSurvivalObservation{ObjectKey: "O", DurationDays: float64(100 + i*20), EventType: "отказ", ClassificationSource: "автоклассификация", Date: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.Local)})
	}
	r := mrpSurvivalBacktest(obs, MRPEventFailure)
	if r.Eligible || !strings.Contains(r.Note, "подтверждённых отказов") {
		t.Fatalf("cause-specific gate=%+v", r)
	}
}

func TestRC200CovariateReadinessDoesNotFitWeights(t *testing.T) {
	rows := make([]MRPObservation, 0, 30)
	for i := 0; i < 30; i++ {
		rows = append(rows, MRPObservation{ObjectKey: string(rune('A' + i%6)), MRPDays: 200, Equipment: "УЭЦН", Complication: "С3", EventType: "отказ", ClassificationConfirmed: i < 12, Snapshot: MRPRegimeSnapshot{Completeness: "полный снимок"}})
	}
	r := mrpCovariateReadiness(rows)
	if !r.ResearchBacktestReady {
		t.Fatalf("readiness=%+v", r)
	}
	if !strings.Contains(r.Note, "коэффициенты не оцениваются") {
		t.Fatalf("note=%q", r.Note)
	}
}

func TestRC200EconomicContractCarriesQualityProvenance(t *testing.T) {
	c := TechnicalScenarioContract{
		ContractVersion: technicalScenarioContractVersion, ObjectKey: "OBJ", ScenarioID: "S", BaseMethod: "M",
		HasSurvivalTriplet: true, SurvivalLowDays: 300, SurvivalBaseDays: 400, SurvivalHighDays: 520,
		SurvivalTarget: "до любого следующего вмешательства", SurvivalDistinctObjects: 4,
		SurvivalClassificationCoverage: 0.75, SurvivalConfirmedFailures: 7,
		SurvivalBrier90: 0.11, SurvivalBrier180: 0.18, SurvivalBrier365: 0.22,
		SurvivalCompetingUnknown: 2,
	}
	e := economicContractFromTechnical(c)
	if e.SurvivalTarget != c.SurvivalTarget || e.SurvivalDistinctObjects != 4 || e.SurvivalConfirmedFailures != 7 || e.SurvivalCompetingUnknown != 2 {
		t.Fatalf("provenance lost: %+v", e)
	}
	if math.Abs(e.SurvivalClassificationCoverage-0.75) > 1e-9 || math.Abs(e.SurvivalBrier180-0.18) > 1e-9 {
		t.Fatalf("quality metrics lost: %+v", e)
	}
}
