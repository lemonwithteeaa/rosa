//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC569ReplicationWeightsAndHardGatesArePresent(t *testing.T) {
	b, err := os.ReadFile("methods_replication.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	required := []string{
		"0.30*float64(out.MechanismFit)",
		"0.20*float64(out.OperatingFit)",
		"0.15*float64(out.EquipmentFit)",
		"0.20*float64(out.Repeatability)",
		"0.10*float64(out.Durability)",
		"0.05*float64(out.OperationalReproducibility)",
		"out.QualifiedAnalogCount < 3",
		"a.ProbabilitySample >= 10",
		"a.ProbabilityObjects >= 5",
		"a.ProbabilityLow >= 55",
		"out.QualifiedAnalogCount >= 10",
		"Высокий потенциал переноса после контрольного пилота",
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Fatalf("RC569 replication requirement missing: %q", want)
		}
	}
}

func TestRC569ReferenceSeparatesApplicabilityProbabilityReliabilityAndReplication(t *testing.T) {
	b, err := os.ReadFile("methods_reference.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"ПРИМЕНИМОСТЬ МЕТОДА, %",
		"ДОСТОВЕРНОСТЬ ОЦЕНКИ, %",
		"ВЕРОЯТНОСТЬ ПОДТВЕРЖДЕНИЯ ЭФФЕКТА, %",
		"АНАЛОГИ И ПЕРЕНОС РЕШЕНИЯ — ГДЕ ЕЩЁ МЕТОД МОЖЕТ БЫТЬ ПОЛЕЗЕН",
		"НЕ равен применимости",
		"НЕ является вероятностью успеха",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("RC569 reference requirement missing: %q", want)
		}
	}
}

func TestRC569MonitoringDoesNotAddRunLifeEffect(t *testing.T) {
	b, err := os.ReadFile("methods_effect_model.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "мониторинг повышает управляемость и достоверность программы, но сам по себе не является самостоятельным источником прироста МРП") {
		t.Fatal("RC569 monitoring must not receive standalone MRP gain")
	}
}
