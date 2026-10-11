//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestRC564TeachingFailureProfiles(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local)
	cases := map[string]int{
		"Учебный пример 001 · Объект": 2,
		"Учебный пример 058 · Объект": 3,
		"Учебный пример 062 · Объект": 4,
		"Учебный пример 074 · Объект": 3,
		"Учебный пример 097 · Объект": 4,
	}
	for key, want := range cases {
		rows := teachingFailureHistoryForObject(key, now)
		if len(rows) != want {
			t.Fatalf("%s: got %d failures, want %d", key, len(rows), want)
		}
		for _, row := range rows {
			if !inFailureWindow(row.Date, now) {
				t.Fatalf("%s: teaching failure %s outside 36-month window", key, row.Date)
			}
			if !strings.Contains(strings.ToLower(row.Source), "учеб") || !row.ConclusionApproved {
				t.Fatalf("%s: teaching row must be explicitly synthetic and approved for workflow testing", key)
			}
		}
	}
}

func TestRC564ClassSeverity(t *testing.T) {
	cases := map[string]int{"С1": 1, "К2": 2, "А3": 3, "Гд1": 1, "НД": 0, "": 0}
	for value, want := range cases {
		if got := methodClassSeverity(value); got != want {
			t.Fatalf("severity %q = %d, want %d", value, got, want)
		}
	}
}

func TestRC564FiveTeachingProfilesOnly(t *testing.T) {
	if got := teachingFailureHistoryForObject("Учебный пример 064 · Объект", time.Now()); len(got) != 0 {
		t.Fatalf("example 064 must not receive synthetic section-1 failure history in RC564")
	}
}
