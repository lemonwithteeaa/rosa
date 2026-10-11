//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestRC565MethodsForecastUsesOnly36MonthFailureHistory(t *testing.T) {
	oldFailures := failureEvents
	oldOutcomes := outcomes
	oldVersions := versions
	defer func() { failureEvents = oldFailures; outcomes = oldOutcomes; versions = oldVersions }()

	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local)
	key := "RC565-OBJ"
	failureEvents = []FailureEvent{
		{ID: "a", ObjectKey: key, Date: "28.09.2026", CycleStartDate: "01.01.2026", EventType: "Отказ", Source: "test"},
		{ID: "b", ObjectKey: key, Date: "28.09.2025", CycleStartDate: "01.01.2025", EventType: "Отказ", Source: "test"},
		{ID: "old", ObjectKey: key, Date: "01.01.2022", CycleStartDate: "01.01.2021", EventType: "Отказ", Source: "test"},
	}
	outcomes = nil
	versions = nil

	h := methodHistoryBasisForObject(key, now)
	if h.FailureCount != 2 || h.CycleCount != 2 {
		t.Fatalf("36-month history mismatch: %+v", h)
	}
	if h.BaseDays <= 0 || !h.HasRange {
		t.Fatalf("expected empirical forecast from two cycles: %+v", h)
	}
}

func TestRC565NoHistoryShowsStrongAccuracyWarning(t *testing.T) {
	oldFailures := failureEvents
	oldOutcomes := outcomes
	oldVersions := versions
	defer func() { failureEvents = oldFailures; outcomes = oldOutcomes; versions = oldVersions }()
	failureEvents = nil
	outcomes = nil
	versions = nil

	h := methodHistoryBasisForObject("RC565-NO-HISTORY", time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local))
	if h.FailureCount != 0 || h.Precision != "НИЗКАЯ" {
		t.Fatalf("unexpected basis: %+v", h)
	}
	low := strings.ToLower(h.Warning)
	for _, want := range []string{"предыстория отказов", "36 месяцев", "точность значительно ниже"} {
		if !strings.Contains(low, strings.ToLower(want)) {
			t.Fatalf("warning must contain %q: %s", want, h.Warning)
		}
	}
}

func TestRC565CoatingAndScaleInhibitorAreMarkedSynergistic(t *testing.T) {
	got := strings.ToLower(methodInteractionSummary([]string{"Защитное покрытие рабочих органов", "Дозирование ингибитора солеотложений"}))
	if !strings.Contains(got, "синерги") || !strings.Contains(got, "прост") && !strings.Contains(got, "числен") {
		t.Fatalf("unexpected interaction summary: %s", got)
	}
}

func TestRC565CombinedMethodsNeverPromiseSimpleAddition(t *testing.T) {
	got := strings.ToLower(methodInteractionSummary([]string{"Ингибитор коррозии", "Защитное покрытие НКТ"}))
	if !strings.Contains(got, "синерги") || !strings.Contains(got, "слож") {
		t.Fatalf("combined effect must reject simple addition: %s", got)
	}
}
