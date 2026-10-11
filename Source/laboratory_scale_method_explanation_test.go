//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLaboratoryScaleMethodsHavePlainExplanation(t *testing.T) {
	for _, model := range laboratoryScaleModels {
		got := strings.TrimSpace(laboratoryScaleMethodPlainExplanation(model.Code))
		if got == "" || strings.Contains(got, "пока не подготовлено") {
			t.Fatalf("model %s has no plain explanation: %q", model.Code, got)
		}
	}
}

func TestLaboratoryScalePanelAndConclusionExplainMethod(t *testing.T) {
	values := map[string]string{
		"total_mineralization": "15000", "ph": "7.2", "sample_temp": "25",
		"ca": "500", "mg": "100", "na_k": "4500", "cl": "7000", "hco3": "300", "so4": "100",
	}
	panel := laboratoryScalePanelTextForModel(values, "davies")
	if !strings.Contains(panel, "ЧТО ЭТО ЗА МЕТОД") || !strings.Contains(panel, "Метод Дэвиса") {
		t.Fatalf("panel lacks plain method explanation: %s", panel)
	}
	decision := laboratoryScaleDecisionText(values, "davies")
	if !strings.Contains(decision, "Что это за метод:") || !strings.Contains(decision, "Метод Дэвиса") {
		t.Fatalf("decision lacks plain method explanation: %s", decision)
	}
}
