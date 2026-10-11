//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC539TeachingExampleKeysAreIsolatedAndReadable(t *testing.T) {
	seen := map[string]bool{}
	for i := 1; i <= 100; i++ {
		name := strings.TrimSpace(strings.Repeat("0", 0))
		_ = name
		filename := ""
		if i < 10 {
			filename = "0" + string(rune('0'+i)) + "_demo.xlsx"
		} else if i < 100 {
			filename = string(rune('0'+i/10)) + string(rune('0'+i%10)) + "_demo.xlsx"
		} else {
			filename = "100_demo.xlsx"
		}
		key, ok := bundledDemoObjectKey(filename, "Авиловское / 114 / Бобриковский горизонт")
		if !ok {
			t.Fatalf("example %d key not built", i)
		}
		if seen[key] {
			t.Fatalf("duplicate teaching key %q", key)
		}
		seen[key] = true
		if !strings.Contains(key, "Учебный пример") || !strings.Contains(key, "Авиловское") {
			t.Fatalf("key lost readable source context: %q", key)
		}
		if n, ok := teachingWorkflowOrdinal(key); !ok || n != i {
			t.Fatalf("key %q parsed as %d/%v, want %d", key, n, ok, i)
		}
	}
}

func TestRC539TeachingWorkflowDataNeverPretendsExpertApproval(t *testing.T) {
	key, _ := bundledDemoObjectKey("58_demo.xlsx", "Учебный объект / Штатные бланки")
	st, ok := teachingWorkflowStateForObject(key)
	if !ok || st.CurrentMRP == "" || !strings.Contains(strings.ToLower(st.SourceFile), "синтет") {
		t.Fatalf("teaching tech context missing or unlabelled: %+v", st)
	}
	obs := teachingWorkflowObservations(key)
	if len(obs) < 3 {
		t.Fatalf("teaching MRP history too short: %d", len(obs))
	}
	for _, o := range obs {
		if o.ClassificationConfirmed {
			t.Fatalf("teaching history must never auto-confirm classification")
		}
		if !strings.Contains(strings.ToLower(o.ClassificationSource), "учеб") {
			t.Fatalf("teaching history source not explicit: %q", o.ClassificationSource)
		}
	}
	_, cost, opex, one, ok := teachingEconomicsDefaultsForObject(key)
	if !ok || cost <= 0 || opex <= 0 || one <= 0 {
		t.Fatalf("teaching economics defaults unavailable")
	}
}

func TestRC539ObjectRepinUsesSingleGlobalTransition(t *testing.T) {
	if appBuildVersion != "7.3.11.539-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
	warning := laboratoryOffMainRepinWarning("Учебный пример 001 · Авиловское / 114 / Бобриковский горизонт")
	for _, text := range []string{"всех вкладок", "проверить, дополнить или обновить", "МРП и отказы", "экономику", "итоговое заключение"} {
		if !strings.Contains(warning, text) {
			t.Fatalf("repin warning missing %q", text)
		}
	}
}
