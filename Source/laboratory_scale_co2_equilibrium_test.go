//go:build windows

package main

import (
	"math"
	"testing"
)

func TestCO2ReducedExternalReservoirMatchesReference(t *testing.T) {
	in := laboratoryCO2AqueousInput{Gas: laboratoryCO2GasInput{Mode: "external_reservoir", Pressure: math.Pow(10, -3.5), PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, ReservoirMoles: 10}, SodiumMolal: .01, ChlorideMolal: .01}
	got, err := laboratoryCO2SolveReduced(in)
	if err != nil {
		t.Fatal(err)
	}
	want := co2ReferenceFinal(t, "external_reservoir")
	if math.Abs(got.PH-want["pH"]) > .003 {
		t.Fatalf("pH %.12g want %.12g", got.PH, want["pH"])
	}
	if math.Abs(got.CarbonWaterMol/want["carbon_water_mol"]-1) > .001 {
		t.Fatalf("carbon %.12g want %.12g", got.CarbonWaterMol, want["carbon_water_mol"])
	}
	if got.Status != "equilibrium" {
		t.Fatal(got.Status)
	}
}

func TestCO2ReducedClosedVolumesMatchReference(t *testing.T) {
	for name, vol := range map[string]float64{"closed_small_volume": .1, "closed_volume": 1, "closed_large_volume": 10} {
		in := laboratoryCO2AqueousInput{Gas: laboratoryCO2GasInput{Mode: "closed_volume", Pressure: .01, PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, GasVolumeL: vol, CO2Percent: 100}, SodiumMolal: .01, ChlorideMolal: .01}
		got, err := laboratoryCO2SolveReduced(in)
		if err != nil {
			t.Fatal(name, err)
		}
		want := co2ReferenceFinal(t, name)
		if math.Abs(got.PH-want["pH"]) > .003 {
			t.Fatalf("%s pH %.12g want %.12g", name, got.PH, want["pH"])
		}
		pressureWant := want["co2_partial_atm"]
		if name == "closed_small_volume" {
			// PHREEQC 3.8.9 reports a diagnostic PR_P affected by its 1e4 L/mol
			// molar-volume cap in this tiny-residual-gas case. The aqueous SI is
			// internally consistent with pH/C and is the accepted comparison here.
			pressureWant = math.Pow(10, want["co2_phase_si"])
		}
		if math.Abs(got.PressureATM/pressureWant-1) > .003 {
			t.Fatalf("%s pressure %.12g want %.12g", name, got.PressureATM, pressureWant)
		}
		if math.Abs(got.CarbonWaterMol/want["carbon_water_mol"]-1) > .002 {
			t.Fatalf("%s carbon %.12g want %.12g", name, got.CarbonWaterMol, want["carbon_water_mol"])
		}
		if math.Abs(got.TotalCarbonMol/want["total_carbon_mol"]-1) > .2e-6 {
			t.Fatalf("%s balance %.12g want %.12g", name, got.TotalCarbonMol, want["total_carbon_mol"])
		}
	}
}

func TestCO2ReducedInsufficientReservoirStatus(t *testing.T) {
	in := laboratoryCO2AqueousInput{Gas: laboratoryCO2GasInput{Mode: "external_reservoir", Pressure: math.Pow(10, -3.5), PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, ReservoirMoles: 1e-8}, SodiumMolal: .01, ChlorideMolal: .01}
	got, err := laboratoryCO2SolveReduced(in)
	if err != nil {
		t.Fatal(err)
	}
	want := co2ReferenceFinal(t, "insufficient_reservoir")
	if got.Status != "stock_exhausted" || got.PressureATM != 0 || got.GasMoles != 0 {
		t.Fatalf("bad exhausted state %+v", got)
	}
	if math.Abs(got.PH-want["pH"]) > .005 {
		t.Fatalf("pH %.12g want %.12g", got.PH, want["pH"])
	}
	if math.Abs(got.CarbonWaterMol/want["carbon_water_mol"]-1) > .002 {
		t.Fatalf("carbon %.12g want %.12g", got.CarbonWaterMol, want["carbon_water_mol"])
	}
}

func TestCO2ReducedRejectsUnsupportedWater(t *testing.T) {
	in := laboratoryCO2AqueousInput{Gas: laboratoryCO2GasInput{Mode: "external_reservoir", Pressure: .001, PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, ReservoirMoles: 1}, SodiumMolal: .01, ChlorideMolal: .02}
	if _, err := laboratoryCO2SolveReduced(in); err == nil {
		t.Fatal("unequal Na/Cl accepted")
	}
}
