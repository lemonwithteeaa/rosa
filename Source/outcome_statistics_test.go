package main

import (
	"strings"
	"testing"
)

func TestOutcomeCycleStatisticsSeparatesEventTypesAndConfirmedCauses(t *testing.T) {
	vs := []VersionRecord{
		{VersionID: "v1", ObjectKey: "OBJ"},
		{VersionID: "v2", ObjectKey: "OBJ"},
		{VersionID: "v3", ObjectKey: "OBJ"},
		{VersionID: "v4", ObjectKey: "OBJ"},
		{VersionID: "v5", ObjectKey: "OBJ"},
		{VersionID: "other", ObjectKey: "OTHER"},
	}
	os := []Outcome{
		{VersionID: "v1", EventType: "Отказ", ClassificationConfirmed: true, FailureCause: "Коррозия", DefectFinding: "язва"},
		{VersionID: "v2", EventType: "отказ", ClassificationConfirmed: true, FailureCause: "  коррозия  ", ObservedEffect: "Наблюдается улучшение"},
		{VersionID: "v3", EventType: "Ремонт", ClassificationConfirmed: true, FailureCause: "Коррозия"},
		{VersionID: "v4", EventType: "Плановое вмешательство", PlannedIntervention: true, ClassificationConfirmed: true},
		{VersionID: "v5", EventType: "Отказ", ClassificationConfirmed: false, FailureCause: "АСПО"},
		{VersionID: "other", EventType: "Отказ", ClassificationConfirmed: true, FailureCause: "Соли"},
	}
	s := outcomeCycleStatisticsForObject("obj", vs, os)
	if s.TotalFacts != 5 || s.Confirmed != 4 || s.Unconfirmed != 1 {
		t.Fatalf("fact/confirmation counts = %+v", s)
	}
	if s.Failures != 2 || s.Repairs != 1 || s.PlannedInterventions != 1 || s.OtherConfirmed != 0 {
		t.Fatalf("event types were mixed: %+v", s)
	}
	if s.FailureCauseKnown != 2 || s.FailureCauseMissing != 0 || len(s.FailureCauses) != 1 || s.FailureCauses[0].Count != 2 {
		t.Fatalf("failure causes = %+v", s)
	}
	if s.DefectDocumented != 1 || s.ObservedEffectRecords != 1 {
		t.Fatalf("structured fact coverage = %+v", s)
	}
}

func TestOutcomeCycleStatisticsDoesNotTreatUnconfirmedCauseAsConfirmed(t *testing.T) {
	vs := []VersionRecord{{VersionID: "v1", ObjectKey: "OBJ"}}
	os := []Outcome{{VersionID: "v1", EventType: "Отказ", ClassificationConfirmed: false, FailureCause: "Коррозия"}}
	s := outcomeCycleStatisticsForObject("OBJ", vs, os)
	if s.Failures != 0 || s.FailureCauseKnown != 0 || len(s.FailureCauses) != 0 || s.Unconfirmed != 1 {
		t.Fatalf("unconfirmed event leaked into confirmed stats: %+v", s)
	}
}

func TestOutcomeCycleStatisticsSummaryStatesNoAutomaticMerging(t *testing.T) {
	text := outcomeCycleStatisticsSummary(OutcomeCycleStatistics{TotalFacts: 1, Confirmed: 1, Failures: 1, FailureCauseKnown: 1, FailureCauses: []OutcomeCauseCount{{Label: "Коррозия", Count: 1}}})
	for _, want := range []string{"отказы — 1", "ремонты — 0", "плановые вмешательства — 0", "автоматически не объединяются", "не получают вес в прогнозе МРП"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q: %s", want, text)
		}
	}
}

func TestOutcomeForecastFactComparisonIsDescriptive(t *testing.T) {
	v := VersionRecord{CurrentMRP: "300", ForecastMRP: "500"}
	o := Outcome{ActualMRP: "600", ActualCost: "120", NextCycleStartDate: "21.09.2026", NextCycleStatus: "Работает"}
	s := outcomeForecastFactComparison(v, o)
	for _, want := range []string{"500 → 600", "+100 сут.", "+20.0%", "300 → 600", "Фактические затраты: 120", "не доказывает эффект мероприятия", "не используется для автоматического рейтинга"} {
		if !strings.Contains(s, want) {
			t.Fatalf("comparison missing %q: %s", want, s)
		}
	}
}
