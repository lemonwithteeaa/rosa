//go:build windows

package main

import "testing"

func TestRC758FundArchiveGroupsAllObjectsWithoutChangingInputs(t *testing.T) {
	items := []AlternativeMethodScenario{
		{ObjectKey: "FIELD-11", Name: "Схема А", UpdatedAt: "2026-10-04T10:00:00", ForecastBaseDays: 1000,
			EconomicsCalculated: "2026-10-04", EconomicsStatus: "Расчёт актуален", NetBenefitK: 400},
		{ObjectKey: "FIELD-22", Name: "Схема А", UpdatedAt: "2026-10-05T10:00:00", ForecastBaseDays: 800},
		{ObjectKey: "FIELD-11", Name: "Схема Б", UpdatedAt: "2026-10-06T10:00:00", ForecastBaseDays: 1100,
			EconomicsCalculated: "2026-10-06", EconomicsStatus: "Расчёт актуален", NetBenefitK: 200},
		{ObjectKey: "FIELD-11", Name: "Схема В", UpdatedAt: "2026-10-06T11:00:00", ForecastBaseDays: 900,
			EconomicsCalculated: "2026-10-06", EconomicsStatus: "Требует пересчёта", NetBenefitK: 10000},
	}
	got := economicsFundGrouping(items)
	if len(got) != 2 {
		t.Fatalf("expected two objects, got %d", len(got))
	}
	var first, second economicsFundObjectRow
	for _, g := range got {
		switch g.ObjectKey {
		case "FIELD-11":
			first = g
		case "FIELD-22":
			second = g
		default:
			t.Fatalf("unexpected object: %s", g.ObjectKey)
		}
	}
	if len(first.Schemes) != 3 || len(second.Schemes) != 1 {
		t.Fatalf("object grouping lost saved alternatives: %d / %d", len(first.Schemes), len(second.Schemes))
	}
	if first.Calculated != 2 || !first.HasBenefit || first.BestBenefit != 400 || first.MaxMRP != 1100 {
		t.Fatalf("wrong group totals: %+v", first)
	}
	if second.Calculated != 0 || second.HasBenefit {
		t.Fatalf("uncomputed object falsely appears calculated: %+v", second)
	}
	if items[3].EconomicsStatus != "Требует пересчёта" {
		t.Fatal("archive read path mutated stored status")
	}
}
