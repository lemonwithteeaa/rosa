//go:build windows

package main

import (
	"math"
	"testing"
)

func TestGasPressureUnitsAgreeAtOneAtmosphere(t *testing.T) {
	for _, item := range []struct {
		unit  string
		value float64
	}{{"Па", 101325}, {"кПа", 101.325}, {"МПа", .101325}, {"бар", 1.01325}, {"атм", 1}} {
		got, err := laboratoryGasPressureATM(item.value, item.unit, true)
		if err != nil || math.Abs(got-1) > 1e-14 {
			t.Fatal(item, got, err)
		}
	}
}
func TestGasPressureRejectsAmbiguousAndInvalidValues(t *testing.T) {
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := laboratoryGasPressureATM(value, "Па", true); err == nil {
			t.Fatal(value)
		}
	}
	if _, err := laboratoryGasPressureATM(1, "кгс/см²", true); err == nil {
		t.Fatal("unknown pressure unit accepted")
	}
	if _, err := laboratoryGasPressureATM(1, "атм", false); err == nil {
		t.Fatal("gauge pressure accepted")
	}
}
func TestGasInputKeepsDistinctModesAndDoesNotAssumeMoles(t *testing.T) {
	input := laboratoryCO2GasInput{Mode: "closed_volume", Pressure: .01, PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, GasVolumeL: 1, CO2Percent: 100}
	got, err := laboratoryNormalizeCO2GasInput(input)
	if err != nil || got.PressureATM != .01 || got.GasVolumeL != 1 || got.CO2Fraction != 1 || got.ReservoirMoles != 0 {
		t.Fatal(got, err)
	}
	input.Mode = "external_reservoir"
	input.GasVolumeL = 0
	input.CO2Percent = 0
	input.ReservoirMoles = 1e-8
	got, err = laboratoryNormalizeCO2GasInput(input)
	if err != nil || got.ReservoirMoles != 1e-8 {
		t.Fatal("insufficient stock must reach solver rather than be replaced", got, err)
	}
}
func TestGasInputRejectsUnsupportedOrContradictoryConditions(t *testing.T) {
	valid := laboratoryCO2GasInput{Mode: "closed_volume", Pressure: .01, PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, GasVolumeL: 1, CO2Percent: 100}
	changes := []func(*laboratoryCO2GasInput){
		func(v *laboratoryCO2GasInput) { v.Mode = "" }, func(v *laboratoryCO2GasInput) { v.TemperatureC = 35 },
		func(v *laboratoryCO2GasInput) { v.WaterKG = 0 }, func(v *laboratoryCO2GasInput) { v.GasVolumeL = math.NaN() },
		func(v *laboratoryCO2GasInput) { v.GasVolumeL = 0 }, func(v *laboratoryCO2GasInput) { v.CO2Percent = 10 },
		func(v *laboratoryCO2GasInput) { v.Pressure = 2 }, func(v *laboratoryCO2GasInput) { v.ReservoirMoles = 1 },
		func(v *laboratoryCO2GasInput) { v.Mode = "external_reservoir" },
	}
	for i, change := range changes {
		v := valid
		change(&v)
		if _, err := laboratoryNormalizeCO2GasInput(v); err == nil {
			t.Fatal("accepted invalid case", i)
		}
	}
}
