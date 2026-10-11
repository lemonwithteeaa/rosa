//go:build windows

package main

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestTemperatureLogKMatchesPHREEQCRelease(t *testing.T) {
	data, err := os.ReadFile("reference/temperature/phreeqc_temperature_golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	minerals := []string{"Кальцит", "Гипс", "Барит", "Целестин"}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		cells := strings.Fields(line)
		temperature, err := strconv.ParseFloat(cells[0], 64)
		if err != nil {
			t.Fatal(err)
		}
		for i, mineral := range minerals {
			want, err := strconv.ParseFloat(cells[i+1], 64)
			if err != nil {
				t.Fatal(err)
			}
			got, err := laboratoryTemperatureLogK(mineral, temperature)
			if err != nil || math.Abs(got-want) > 1e-9 {
				t.Fatalf("%s at %g C: got %.12g, PHREEQC %.12g, err=%v", mineral, temperature, got, want, err)
			}
		}
	}
}

func TestTemperatureModelRequiresExplicitValidTemperature(t *testing.T) {
	for _, value := range []string{"", "bad", "NaN", "+Inf", "-1", "101"} {
		results, _, warnings := laboratorySaturationResults(map[string]string{"ca": "40.078", "co3": "60.01", "sample_temp": value}, "temperature_ideal")
		if len(results) != 0 || len(warnings) == 0 {
			t.Fatalf("invalid temperature %q accepted", value)
		}
	}
	for _, value := range []string{"0", "25", "100"} {
		results, _, _ := laboratorySaturationResults(map[string]string{"ca": "40.078", "co3": "60.01", "sample_temp": value}, "temperature_ideal")
		if len(results) != 1 || results[0].Partial {
			t.Fatalf("valid temperature %q failed: %#v", value, results)
		}
	}
}

func TestTemperatureChangesConstantWithoutChangingLaboratoryInputs(t *testing.T) {
	values := map[string]string{"ca": "40.078", "co3": "60.01", "sample_temp": "25"}
	at25, _, _ := laboratorySaturationResults(values, "temperature_ideal")
	values["sample_temp"] = "60"
	at60, _, _ := laboratorySaturationResults(values, "temperature_ideal")
	if at60[0].SI <= at25[0].SI || values["ca"] != "40.078" || values["co3"] != "60.01" {
		t.Fatal("temperature route did not preserve inputs or change log K")
	}
	values["ca"] = "80.156"
	doubled, _, _ := laboratorySaturationResults(values, "temperature_ideal")
	if math.Abs(doubled[0].SI-at60[0].SI-math.Log10(2)) > 1e-10 {
		t.Fatal("concentration SI invariant failed")
	}
}

func TestTemperatureDatabaseIsPinned(t *testing.T) {
	data, err := os.ReadFile("reference/temperature/phreeqc.dat")
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != "59373961d648dfbf68a40744060c1d64f57ecbec98f4f5fb89f3a1b4213ccd10" {
		t.Fatal("temperature database changed:", got)
	}
}

func TestLaboratoryNumbersRejectNonFiniteValues(t *testing.T) {
	for _, value := range []string{"NaN", "Inf", "+Inf", "-Inf", "1e999"} {
		if _, ok := parseLabNumber(value); ok {
			t.Fatalf("non-finite number %q accepted", value)
		}
	}
}
