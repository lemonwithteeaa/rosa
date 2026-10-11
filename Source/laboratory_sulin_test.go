//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLaboratorySulinWaterTypeFourClasses(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name:   "hydrocarbonate sodium",
			values: map[string]string{"na_k": "1000", "cl": "1000", "so4": "50", "mg": "20"},
			want:   "Гидрокарбонатно-натриевый",
		},
		{
			name:   "sulfate sodium",
			values: map[string]string{"na_k": "1000", "cl": "1000", "so4": "1200", "mg": "20"},
			want:   "Сульфатно-натриевый",
		},
		{
			name:   "chloride calcium",
			values: map[string]string{"na_k": "300", "cl": "1000", "so4": "50", "mg": "20"},
			want:   "Хлоридно-кальциевый",
		},
		{
			name:   "chloride magnesium",
			values: map[string]string{"na_k": "600", "cl": "1000", "so4": "50", "mg": "300"},
			want:   "Хлоридно-магниевый",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := laboratorySulinWaterType(tt.values)
			if !strings.HasPrefix(got, tt.want) {
				t.Fatalf("got %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestLaboratorySulinUsesSeparateNaAndKWhenAvailable(t *testing.T) {
	got := laboratorySulinWaterType(map[string]string{
		"na": "700", "k": "100", "na_k": "1", "cl": "700", "so4": "100", "mg": "20",
	})
	if strings.Contains(got, "оценочно") {
		t.Fatalf("separate Na and K must not be marked approximate: %q", got)
	}
}

func TestLaboratorySulinIsCalculatedNotManualCoverage(t *testing.T) {
	group, ok := labGroupByCode("scale")
	if !ok {
		t.Fatal("scale group not found")
	}
	values := map[string]string{"water_type_sulin": "Хлоридно-кальциевый"}
	bFilled, _, oFilled, _ := laboratoryDataCoverage(group, values)
	if bFilled != 0 || oFilled != 0 {
		t.Fatalf("calculated Sulin field must not count as entered data: base=%d optional=%d", bFilled, oFilled)
	}
}

func TestLaboratoryExpandedLabels(t *testing.T) {
	if got := laboratoryFieldDisplayLabel(LabFieldDef{Key: "orp", Label: "Окислительно-восстановительный потенциал (ОВП)", Unit: "мВ"}); !strings.Contains(got, "Окислительно-восстановительный") {
		t.Fatalf("ORP label is not expanded: %q", got)
	}
	if got := laboratoryFieldDisplayLabel(LabFieldDef{Key: "qpcr_apb", Label: "x", Unit: "копий/мл"}); strings.Contains(got, "бакт.") || !strings.Contains(got, "количественная ПЦР") {
		t.Fatalf("qPCR label is not expanded: %q", got)
	}
}
