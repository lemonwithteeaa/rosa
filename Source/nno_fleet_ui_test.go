//go:build windows

package main

import (
	"testing"
	"time"
)

func TestNNOGroupNormalization(t *testing.T) {
	cases := []struct{ equipment, mode, want string }{
		{"УЭЦН", "", "УЭЦН"},
		{"ШГН", "", "УШГН"},
		{"УЭВН", "", "УЭВН"},
		{"ЭЦН + ШГН", "ОРД", "ОРД"},
		{"", "Фонтанная добыча", "Фонтанный"},
		{"", "", "Прочие / не определено"},
	}
	for _, v := range cases {
		got := nnoMethodOfFailure(FailureEvent{Equipment: v.equipment, OperationMode: v.mode})
		if got != v.want {
			t.Fatalf("%q %q -> %q, want %q", v.equipment, v.mode, got, v.want)
		}
	}
}
func TestNNO12CalendarMonthWindow(t *testing.T) {
	ref := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		month      time.Month
		year, want int
	}{{time.October, 2026, 11}, {time.November, 2025, 0}, {time.October, 2025, -1}, {time.November, 2026, 12}}
	for _, v := range cases {
		got := nnoMonthIndex(time.Date(v.year, v.month, 1, 0, 0, 0, 0, time.UTC), ref)
		if got != v.want {
			t.Fatalf("%d-%02d -> %d, want %d", v.year, v.month, got, v.want)
		}
	}
}
func TestNNOClassificationNeverConfusesDraftWithApproved(t *testing.T) {
	for _, e := range []FailureEvent{
		{EventType: "отказ", ConclusionApproved: false},
		{EventType: "отказ", ConclusionApproved: true, Draft: true},
		{EventType: "плановый ремонт", ConclusionApproved: true},
	} {
		if nnoEventIsConfirmed(e) {
			t.Fatalf("non-approved event counted: %+v", e)
		}
	}
	if !nnoEventIsConfirmed(FailureEvent{EventType: "отказ", ConclusionApproved: true}) {
		t.Fatal("confirmed failure not recognized")
	}
}
func TestNNORuleKeyRespectsClassifierBranch(t *testing.T) {
	a := OfficialFailureCause2024{Version: "2024 V10", Branch: "УЭЦН", Code: "3.6"}
	b := OfficialFailureCause2024{Version: "2024 V10", Branch: "УШГН", Code: "3.6"}
	if nnoRuleKey(a) == nnoRuleKey(b) {
		t.Fatal("different classifier branches collapsed")
	}
}
