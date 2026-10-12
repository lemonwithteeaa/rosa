//go:build windows

package main

import (
	"testing"
	"time"
)

func TestNNOTrainingPreviewRequiresOnlyTeachingRecordsInScope(t *testing.T) {
	from := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	demo := FailureEvent{Equipment: "УЭЦН", Source: "ДЕМО — учебный отказ", Date: "22.01.2026"}
	otherDemo := FailureEvent{Equipment: "УШГН", Source: "ДЕМО", Date: "18.09.2026"}
	production := FailureEvent{Equipment: "УЭЦН", Source: "Акт расследования", Date: "23.01.2026"}
	oldProduction := FailureEvent{Equipment: "УЭЦН", Source: "Акт расследования", Date: "22.01.2024"}

	if !nnoTrainingOnlyInScope([]FailureEvent{demo, otherDemo}, "Все способы", from, to) {
		t.Fatal("labelled demo rows must enable chart preview even when present in the journal")
	}
	if nnoTrainingOnlyInScope([]FailureEvent{demo, production}, "Все способы", from, to) {
		t.Fatal("mixed teaching and production records must not enable synthetic charts")
	}
	if nnoTrainingOnlyInScope([]FailureEvent{demo, production}, "УШГН", from, to) {
		t.Fatal("a scope with no demo examples must stay empty")
	}
	if !nnoTrainingOnlyInScope([]FailureEvent{demo, otherDemo, production}, "УШГН", from, to) {
		t.Fatal("production rows from another lift type must not suppress a demo-only scope")
	}
	if !nnoTrainingOnlyInScope([]FailureEvent{demo, oldProduction}, "УЭЦН", from, to) {
		t.Fatal("production failures outside the selected date window must not suppress preview")
	}
	if nnoTrainingOnlyInScope([]FailureEvent{production}, "Все способы", from, to) {
		t.Fatal("production-only data cannot enable a training preview")
	}
}
