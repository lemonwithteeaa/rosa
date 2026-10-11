//go:build windows

package main

import "testing"

func TestLaboratoryCanonicalValue(t *testing.T) {
	cases := []struct {
		field LabFieldDef
		raw   string
		unit  string
		want  string
	}{
		{LabFieldDef{Unit: "мг/л"}, "1,5", "г/л", "1500"},
		{LabFieldDef{Unit: "МПа"}, "10", "бар", "1"},
		{LabFieldDef{Unit: "°C"}, "300", "K", "26.850000000000023"},
		{LabFieldDef{Unit: "мм"}, "2500", "мкм", "2.5"},
	}
	for _, tc := range cases {
		if got := laboratoryCanonicalValue(tc.field, tc.raw, tc.unit); got != tc.want {
			t.Fatalf("canonical value for %s: got %q, want %q", tc.unit, got, tc.want)
		}
	}
}

func TestLaboratoryInfluenceDataKeepsMissingSeparate(t *testing.T) {
	pass := LabPassRecord{
		Groups: map[string]map[string]string{
			"corrosion": {"ph": "5.8", "h2s": "2"},
		},
		Units: map[string]map[string]string{
			"corrosion": {"ph": "ед.", "h2s": "мг/л"},
		},
	}
	items := laboratoryInfluenceData(pass)
	if len(items) != 6 {
		t.Fatalf("got %d influence rows, want 6", len(items))
	}
	if items[0].Level != 3 || items[0].State != "ПОВЫШЕННОЕ" {
		t.Fatalf("unexpected corrosion influence: %#v", items[0])
	}
	if items[1].Level != 0 || items[1].State != "нет данных" {
		t.Fatalf("missing direction must stay explicit: %#v", items[1])
	}
}

func TestLaboratoryDisplayValueRoundTripRC151(t *testing.T) {
	field := LabFieldDef{Key: "chlorides", Label: "Хлориды", Unit: "мг/л"}
	canonical := laboratoryCanonicalValue(field, "1.5", "г/л")
	if canonical != "1500" {
		t.Fatalf("canonical=%q", canonical)
	}
	if got := laboratoryDisplayValue(field, canonical, "г/л"); got != "1.5" {
		t.Fatalf("display=%q", got)
	}
}
