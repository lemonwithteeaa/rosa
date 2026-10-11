//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestRC619ParameterRowsFitAllWrappedValuesOrFallback(t *testing.T) {
	labels := []int32{15, 15, 15, 15, 15, 15}
	values := []int32{30, 15, 30, 30, 30, 45}
	rows, ok := methodsParameterRowHeights(labels, values, 251)
	if !ok || len(rows) != 6 {
		t.Fatal("wrapped rows rejected")
	}
	total := int32(20)
	for i, row := range rows {
		if row < values[i]+8 {
			t.Fatalf("row %d cuts wrapped text", i)
		}
		total += row
	}
	if _, ok := methodsParameterRowHeights(labels, values, total-1); ok {
		t.Fatal("overflowing panel did not fall back to vertical summary")
	}
	if _, ok := methodsParameterRowHeights(labels, []int32{15, 0, 15, 15, 15, 15}, 251); ok {
		t.Fatal("failed measurement displayed cropped values")
	}
}
func TestRC619ReferenceUsesPreparedObjectEstimate(t *testing.T) {
	oldKey, oldPreparedKey, oldScope, oldEvaluation := selectedObjectKey, methodsPreparedKey, methodsPreparedScope, methodsEvaluation
	defer func() {
		selectedObjectKey, methodsPreparedKey, methodsPreparedScope, methodsEvaluation = oldKey, oldPreparedKey, oldScope, oldEvaluation
	}()
	selectedObjectKey = "OBJ-REFERENCE"
	methodsPreparedKey = selectedObjectKey
	methodsEvaluation = nil
	methodsPreparedScope = newMethodsEvaluationScope()
	s := methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: "Ингибитор солеотложений"}, Direction: "Солеотложения", Classification: "С3", Severity: 3}
	methodsPreparedScope.assessments[methodsEvaluationKey{selectedObjectKey, s}] = MethodEffectAssessment{Applicability: 87, Reliability: 76, ReliabilityLevel: "ХОРОШАЯ"}
	methodsPreparedScope.snapshots[selectedObjectKey] = methodForecastSnapshot{Base: 876}
	started := time.Now()
	text := selectedMethodReferenceText(s)
	if !strings.Contains(text, "Применимость 87%") || !strings.Contains(text, "«АНАЛОГИ»") {
		t.Fatal("reference lost the prepared object estimate or analog action")
	}
	if time.Since(started) > time.Second {
		t.Fatal("reference preparation blocked")
	}
	if methodsEvaluation != nil {
		t.Fatal("reference leaked UI evaluation scope")
	}
}
func TestRC619BasisNumberingIsSeparateFromFullText(t *testing.T) {
	if methodsBasisItemText("5. Вклад: +17%.") != "Вклад: +17%." {
		t.Fatal("numbered row text changed")
	}
	if methodsBasisItemText("Основание без номера") != "Основание без номера" {
		t.Fatal("text without a number changed")
	}
}
