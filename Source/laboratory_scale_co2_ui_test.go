//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
)

func TestCO2EquilibriumInputFromValuesReservoir(t *testing.T) {
	v := map[string]string{
		"co2_eq_mode":          "резервуар",
		"co2_eq_pressure_atm":  "0.000316227766",
		"co2_eq_water_kg":      "1",
		"co2_eq_na_molal":      "0.01",
		"co2_eq_cl_molal":      "0.01",
		"co2_eq_reservoir_mol": "10",
		"co2_eq_temp":          "25",
	}
	in, err := laboratoryCO2EquilibriumInputFromValues(v)
	if err != nil {
		t.Fatal(err)
	}
	if in.Gas.Mode != "external_reservoir" || in.Gas.PressureUnit != "атм" || !in.Gas.AbsolutePressure {
		t.Fatalf("unexpected gas input %+v", in.Gas)
	}
	got, err := laboratoryCO2EquilibriumFromValues(v)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "equilibrium" || math.Abs(got.PH-5.61) > .15 {
		t.Fatalf("unexpected result %+v", got)
	}
}

func TestCO2EquilibriumInputFromValuesClosed(t *testing.T) {
	v := map[string]string{
		"co2_eq_mode":         "замкнутый",
		"co2_eq_pressure_atm": "0.01",
		"co2_eq_water_kg":     "1",
		"co2_eq_na_molal":     "0.01",
		"co2_eq_cl_molal":     "0.01",
		"co2_eq_gas_volume_l": "1",
		"co2_eq_gas_percent":  "100",
		"co2_eq_temp":         "25",
	}
	got, err := laboratoryCO2EquilibriumFromValues(v)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "equilibrium" || got.PressureATM <= 0 || got.PressureATM >= .01 {
		t.Fatalf("unexpected closed result %+v", got)
	}
}

func TestCO2EquilibriumInsufficientReservoirText(t *testing.T) {
	v := map[string]string{
		"co2_eq_mode":          "резервуар",
		"co2_eq_pressure_atm":  "0.000316227766",
		"co2_eq_water_kg":      "1",
		"co2_eq_na_molal":      "0.01",
		"co2_eq_cl_molal":      "0.01",
		"co2_eq_reservoir_mol": "1e-8",
		"co2_eq_temp":          "25",
	}
	got, err := laboratoryCO2EquilibriumFromValues(v)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "stock_exhausted" {
		t.Fatal(got.Status)
	}
	text := laboratoryCO2EquilibriumDecisionText(v)
	if !strings.Contains(text, "исчерпан") || !strings.Contains(text, "не выдаётся за достигнутое") {
		t.Fatal(text)
	}
}

func TestCO2EquilibriumModelHasOnlyDedicatedFields(t *testing.T) {
	group, ok := labGroupByCode("scale")
	if !ok {
		t.Fatal("scale group missing")
	}
	group = laboratoryScaleInputGroup(group, "co2_gas_water")
	if len(group.Fields) != len(laboratoryCO2EquilibriumKeys) {
		t.Fatalf("fields %d want %d", len(group.Fields), len(laboratoryCO2EquilibriumKeys))
	}
	for _, field := range group.Fields {
		if !laboratoryIsCO2EquilibriumField(field.Key) {
			t.Fatalf("unexpected field %s", field.Key)
		}
	}
}
