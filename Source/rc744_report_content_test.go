//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC744SharedIndicatorsAreExplainedAndNotRepeated(t *testing.T) {
	pass := LabPassRecord{
		ScaleModel: "davies",
		Groups:     map[string]map[string]string{"scale": {"ca": "520", "bottomhole_pressure": "9.7798"}},
		SharedValues: map[string]string{
			"bottomhole_pressure": "9.7798", "oil_rate_t_day": "30.4",
			"profile_pressure_min": "8.693199999999999", "mystery_internal_key": "1",
		},
	}
	text := reportLaboratoryRawData(pass)
	if strings.Count(text, "Забойное давление") != 1 {
		t.Fatalf("repeated measured pressure: %s", text)
	}
	for _, want := range []string{"Дебит нефти: 30.4\u00a0т/сут", "Давление (Р) при минимальной температуре", "8.6932"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing explained value %q", want)
		}
	}
	if strings.Contains(text, "mystery_internal_key") || strings.Contains(text, "profile_pressure_min") {
		t.Fatal("internal keys leaked to the approved report")
	}
}

func TestRC744MissingFieldsAppearOnlyInFinalAppendix(t *testing.T) {
	section := "РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ\r\nМодель солеотложений: davies\r\nНАПРАВЛЕНИЕ: СОЛЕОТЛОЖЕНИЯ\r\n• Ca²⁺: 520\u00a0мг/л\r\n"
	r := FullReportRecord{ID: "TEST", Sections: map[string]string{"section_2": section, "section_7": "РАЗДЕЛ 7. ИТОГ"}}
	report := storedSectionDetailedReport(r, true)
	appendix := strings.Index(report, "ПРИЛОЖЕНИЕ 1.")
	if appendix < strings.Index(report, "РАЗДЕЛ 7. ИТОГ") {
		t.Fatal("missing fields were not moved after the final section")
	}
	if !strings.Contains(report[appendix:], "Магний") && !strings.Contains(report[appendix:], "Mg²⁺") {
		t.Fatal("missing measured ion absent from appendix")
	}
	if strings.Contains(report[appendix:], "• Ca²⁺") {
		t.Fatal("a filled field was listed as missing")
	}
}
