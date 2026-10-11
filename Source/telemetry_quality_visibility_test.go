//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestTelemetryRC152MissingAndOutliersAreExplicit(t *testing.T) {
	c := TelemetryChannelSummary{Display: "Ток", Count: 60, Missing: 40, Outliers: 4}
	if got := telemetryMissingLabel(c); got != "40 (40%)" {
		t.Fatalf("unexpected missing label: %q", got)
	}
	events := buildTelemetryEvents([]TelemetryChannelSummary{c}, 60, 60, time.Now(), time.Now())
	var hasMissing, hasOutliers bool
	for _, e := range events {
		hasMissing = hasMissing || e.Kind == "missing"
		hasOutliers = hasOutliers || e.Kind == "outliers"
		if e.Kind == "missing" && !strings.Contains(e.Description, "Не интерполировать незаметно") {
			t.Fatalf("missing-data event lacks explicit rule: %#v", e)
		}
	}
	if !hasMissing || !hasOutliers {
		t.Fatalf("missing/outlier visibility incomplete: %#v", events)
	}
}

func TestTelemetryRC152SevereMissingGetsLowQualitySeverity(t *testing.T) {
	c := TelemetryChannelSummary{Display: "Давление", Count: 40, Missing: 60}
	events := buildTelemetryEvents([]TelemetryChannelSummary{c}, 60, 60, time.Now(), time.Now())
	for _, e := range events {
		if e.Kind == "missing" {
			if e.Severity != "Низкое качество" {
				t.Fatalf("severity=%q", e.Severity)
			}
			return
		}
	}
	t.Fatal("missing-data event not emitted")
}
