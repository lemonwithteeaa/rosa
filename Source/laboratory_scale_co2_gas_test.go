//go:build windows

package main

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func co2ReferenceFinal(t *testing.T, name string) map[string]float64 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("reference", "co2_plan", name, "golden.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	headers := strings.Split(strings.TrimSpace(lines[0]), "\t")
	result := map[string]float64{}
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) < len(headers) || strings.TrimSpace(fields[1]) != "react" {
			continue
		}
		for i, key := range headers {
			value, err := strconv.ParseFloat(strings.TrimSpace(fields[i]), 64)
			if err == nil {
				result[strings.TrimSpace(key)] = value
			}
		}
	}
	if len(result) == 0 {
		t.Fatal("missing final gas reference")
	}
	return result
}

func TestCO2InitialMolesMatchPHREEQCPengRobinson(t *testing.T) {
	for name, volume := range map[string]float64{"closed_small_volume": .1, "closed_volume": 1, "closed_large_volume": 10} {
		input := laboratoryCO2GasInput{Mode: "closed_volume", Pressure: .01, PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, GasVolumeL: volume, CO2Percent: 100}
		got, err := laboratoryCO2InitialGasMoles(input)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join("reference", "co2_plan", name, "initial_gas.dump"))
		if err != nil {
			t.Fatal(err)
		}
		match := regexp.MustCompile(`(?m)^\s+-moles\s+(\S+)`).FindStringSubmatch(string(raw))
		if len(match) != 2 {
			t.Fatal("no initial gas amount")
		}
		want, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(got/want-1) > 1e-8 {
			t.Fatalf("%s initial moles got %.15g want %.15g", name, got, want)
		}
	}
}

func TestCO2FugacityAndAqueousActivityMatchPHREEQC(t *testing.T) {
	for _, name := range []string{"external_reservoir", "closed_small_volume", "closed_volume", "closed_large_volume"} {
		row := co2ReferenceFinal(t, name)
		gas, err := laboratoryCO2PureGasState(row["co2_partial_atm"], 25)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(gas.Phi/row["co2_phi"]-1) > 1e-8 {
			t.Fatalf("%s phi got %.15g want %.15g", name, gas.Phi, row["co2_phi"])
		}
		got, err := laboratoryCO2ActivityFromFugacity(math.Pow(10, row["co2_phase_si"]), 25)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(got/row["dissolved_co2_activity"]-1) > 1e-8 {
			t.Fatalf("%s CO2 activity got %.15g want %.15g relative %.9g", name, got, row["dissolved_co2_activity"], got/row["dissolved_co2_activity"]-1)
		}
	}
}

func TestCO2GasLimitsAndIdealLimit(t *testing.T) {
	for _, pressure := range []float64{0, -1, 2, math.NaN(), math.Inf(1)} {
		if _, err := laboratoryCO2PureGasState(pressure, 25); err == nil {
			t.Fatal("invalid pressure", pressure)
		}
	}
	if _, err := laboratoryCO2PureGasState(.01, 40); err == nil {
		t.Fatal("temperature scope ignored")
	}
	gas, err := laboratoryCO2PureGasState(1e-8, 25)
	if err != nil || math.Abs(gas.Z-1) > 1e-9 || math.Abs(gas.Phi-1) > 1e-9 {
		t.Fatal("ideal limit", gas, err)
	}
	input := laboratoryCO2GasInput{Mode: "external_reservoir", Pressure: .01, PressureUnit: "атм", AbsolutePressure: true, TemperatureC: 25, WaterKG: 1, ReservoirMoles: 1}
	if _, err := laboratoryCO2InitialGasMoles(input); err == nil {
		t.Fatal("reservoir treated as closed volume")
	}
}
