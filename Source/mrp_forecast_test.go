//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

func mrpObs(days ...float64) []MRPObservation {
	out := make([]MRPObservation, len(days))
	for i, d := range days {
		out[i] = MRPObservation{ObjectKey: "OBJ-1", Date: time.Date(2025, time.January, 1+i, 0, 0, 0, 0, time.Local), MRPDays: d, Source: "fact"}
	}
	return out
}

func TestMRPForecastUsesMedianAndEmpiricalQuartiles(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 390, CurrentDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), Observations: mrpObs(300, 360, 400, 420, 500)})
	if math.Abs(r.BaseDays-400) > 0.001 {
		t.Fatalf("base = %.3f, want 400", r.BaseDays)
	}
	if !r.HasLowHigh || math.Abs(r.LowDays-360) > 0.001 || math.Abs(r.HighDays-420) > 0.001 {
		t.Fatalf("range = %.3f..%.3f has=%v, want 360..420", r.LowDays, r.HighDays, r.HasLowHigh)
	}
	if r.EvidenceLevel != "достаточная история" {
		t.Fatalf("evidence = %q", r.EvidenceLevel)
	}
}

func TestMRPForecastSmallHistoryDoesNotPretendConfidenceInterval(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 300, Observations: mrpObs(240, 360)})
	if r.BaseDays != 300 || r.LowDays != 240 || r.HighDays != 360 || !r.HasLowHigh {
		t.Fatalf("unexpected small-history scenarios: %+v", r)
	}
	joined := strings.Join(r.Basis, " ")
	if !strings.Contains(joined, "не доверительный интервал") {
		t.Fatalf("small history must explicitly reject confidence-interval wording: %q", joined)
	}
}

func TestMRPForecastOneFactDoesNotInventLowHigh(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 300, Observations: mrpObs(275)})
	if r.BaseDays != 275 || r.HasLowHigh || r.LowDays != 0 || r.HighDays != 0 {
		t.Fatalf("one fact must not invent bounds: %+v", r)
	}
}

func TestMRPForecastFallsBackToCurrentMRPWithoutHistory(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 412})
	if r.BaseDays != 412 || r.HasLowHigh {
		t.Fatalf("fallback = %+v", r)
	}
	if len(r.Warnings) == 0 || !strings.Contains(strings.Join(r.Warnings, " "), "недостаточно") {
		t.Fatalf("fallback must explain missing history: %+v", r.Warnings)
	}
}

func TestMRPForecastWithoutCurrentAndHistoryRefusesForecast(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1"})
	if r.BaseDays != 0 || r.Status != "прогноз не сформирован" {
		t.Fatalf("must refuse unsupported forecast: %+v", r)
	}
}

func TestMRPForecastDoesNotAssignNumericWeightsToContext(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 300, WaterCut: "75", LiquidRate: "120", Equipment: "УЭЦН"})
	for _, f := range r.Factors {
		if f.Name == "Обводнённость" || f.Name == "Дебит жидкости" || f.Name == "ГНО" {
			if !strings.Contains(f.Note, "не приписывает") {
				t.Fatalf("context factor received unexplained numerical influence: %+v", f)
			}
		}
	}
}

func TestMRPObservationHistoryUsesOnlyMatchingObjectAndActualMRP(t *testing.T) {
	oldVersions, oldOutcomes := versions, outcomes
	defer func() { versions, outcomes = oldVersions, oldOutcomes }()
	versions = []VersionRecord{
		{VersionID: "V1", ObjectKey: "FIELD/W1/A", Equipment: "УЭЦН", Complication: "С2"},
		{VersionID: "V2", ObjectKey: "FIELD/W2/A", Equipment: "УЭЦН", Complication: "К1"},
	}
	outcomes = []Outcome{
		{VersionID: "V1", Date: "01.02.2026", ActualMRP: "410"},
		{VersionID: "V2", Date: "02.02.2026", ActualMRP: "999"},
		{VersionID: "V1", Date: "03.02.2026", ActualMRP: "0"},
	}
	got := mrpObservationHistoryForObject("field/w1/a")
	if len(got) != 1 || got[0].MRPDays != 410 {
		t.Fatalf("matching factual history = %+v", got)
	}
}

func TestMRPComparabilityUsesStoredSnapshotWithoutInventingMissingRegime(t *testing.T) {
	in := MRPForecastInput{ObjectKey: "OBJ-1", Equipment: "УЭЦН", Complication: "С2", CurrentMRPDays: 300}
	o := MRPObservation{ObjectKey: "OBJ-1", MRPDays: 280, Snapshot: MRPRegimeSnapshot{Equipment: "УЭЦН", Complication: "С2", Completeness: "partial"}}
	if got := mrpObservationComparability(in, o); got != "сопоставимо" {
		t.Fatalf("comparability=%q", got)
	}
	o.Snapshot.Equipment = "УШГН"
	if got := mrpObservationComparability(in, o); got != "несопоставимо" {
		t.Fatalf("equipment mismatch=%q", got)
	}
	o.Snapshot = MRPRegimeSnapshot{}
	o.Equipment = ""
	o.Complication = ""
	if got := mrpObservationComparability(in, o); got != "недостаточно данных" {
		t.Fatalf("missing snapshot=%q", got)
	}
}

func TestMRPTimeSplitBacktestRequiresHistoryAndUsesPastOnly(t *testing.T) {
	short := mrpTimeSplitBacktest(mrpObs(100, 110, 120, 130, 140))
	if short.Eligible {
		t.Fatalf("short history must not activate backtest: %+v", short)
	}
	full := mrpTimeSplitBacktest(mrpObs(100, 110, 120, 130, 140, 150, 160))
	if !full.Eligible || full.TestCount != 2 {
		t.Fatalf("unexpected backtest: %+v", full)
	}
	if full.MedianAbsError <= 0 {
		t.Fatalf("backtest error must be factual: %+v", full)
	}
}

func TestMRPForecastLabClassesAreContextOnly(t *testing.T) {
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 300, LabClasses: []string{"С2", "К1"}})
	found := false
	for _, f := range r.Factors {
		if f.Name == "Лабораторные классы/признаки" {
			found = true
			if !strings.Contains(f.Note, "веса запрещены") {
				t.Fatalf("must not assign weights: %+v", f)
			}
		}
	}
	if !found {
		t.Fatal("lab class context missing")
	}
}

func TestMRPForecastAuditComparesSavedForecastWithActual(t *testing.T) {
	oldVersions, oldOutcomes := versions, outcomes
	defer func() { versions, outcomes = oldVersions, oldOutcomes }()
	versions = []VersionRecord{{VersionID: "V1", ObjectKey: "OBJ-1", Created: "01.01.2026", MRPBase: "400"}, {VersionID: "V2", ObjectKey: "OBJ-2", Created: "01.01.2026", MRPBase: "999"}}
	outcomes = []Outcome{{VersionID: "V1", Date: "01.02.2026", ActualMRP: "350"}, {VersionID: "V2", Date: "01.02.2026", ActualMRP: "1"}}
	got := mrpForecastAuditsForObject("obj-1")
	if len(got) != 1 || got[0].ActualMRPDays != 350 || got[0].AbsoluteError != 50 {
		t.Fatalf("audit=%+v", got)
	}
}

func TestMRPComparableBaselineIsSeparateDescriptiveReference(t *testing.T) {
	obs := []MRPObservation{
		{ObjectKey: "OBJ-1", Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local), MRPDays: 300, Snapshot: MRPRegimeSnapshot{Equipment: "УЭЦН", Complication: "С2"}},
		{ObjectKey: "OBJ-1", Date: time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local), MRPDays: 400, Snapshot: MRPRegimeSnapshot{Equipment: "УЭЦН", Complication: "С2"}},
		{ObjectKey: "OBJ-1", Date: time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local), MRPDays: 900, Snapshot: MRPRegimeSnapshot{Equipment: "УШГН", Complication: "К2"}},
	}
	r := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 350, Equipment: "УЭЦН", Complication: "С2", Observations: obs})
	if !r.HasComparableBase || math.Abs(r.ComparableBaseDays-350) > 0.001 {
		t.Fatalf("comparable baseline=%+v", r)
	}
	if math.Abs(r.BaseDays-400) > 0.001 {
		t.Fatalf("main factual baseline must stay unfiltered and transparent, got %.1f", r.BaseDays)
	}
}

func TestMRPMethodContextDoesNotChangeBaseline(t *testing.T) {
	base := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 300, Observations: mrpObs(200, 300, 400)})
	withMethod := calculateMRPForecast(MRPForecastInput{ObjectKey: "OBJ-1", CurrentMRPDays: 300, Observations: mrpObs(200, 300, 400), Method: "Ингибитор солеотложений"})
	if base.BaseDays != withMethod.BaseDays {
		t.Fatalf("method must not alter unvalidated baseline: %.1f vs %.1f", base.BaseDays, withMethod.BaseDays)
	}
	if withMethod.MethodContext == "" {
		t.Fatal("method context missing")
	}
}
