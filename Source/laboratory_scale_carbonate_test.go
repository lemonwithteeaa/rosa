//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCarbonatePHReference(t *testing.T) {
	data, err := os.ReadFile("reference/temperature/carbonate_golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		fields := strings.Fields(line)
		x := make([]float64, len(fields))
		for i, f := range fields {
			x[i], err = strconv.ParseFloat(f, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		// Feed PHREEQC free-ion activities as ideal equivalent concentrations.
		// This verifies mass action and SI, not equality of concentration screening to full speciation.
		v := map[string]string{"sample_temp": fmt.Sprint(x[0]), "ph": fmt.Sprint(x[1]), "ca": fmt.Sprint(math.Pow(10, x[2]) * 40.078 * 1000), "hco3": fmt.Sprint(math.Pow(10, x[3]) * 61.016 * 1000)}
		r, _, _ := laboratoryCarbonatePHResults(v)
		if len(r) != 1 || math.Abs(r[0].SI-x[5]) > 1e-9 {
			t.Fatalf("T=%v: %v, expected %v", x[0], r, x[5])
		}
		k, _ := laboratoryCarbonateAssociationLogK(x[0])
		if math.Abs(x[3]+x[1]-k-x[4]) > 1e-9 {
			t.Fatal("carbonate mass action mismatch")
		}
	}
}
func TestCarbonatePHInputsAndSensitivity(t *testing.T) {
	v := map[string]string{"sample_temp": "25", "ph": "8", "ca": "40.078", "hco3": "61.016", "co3": "123"}
	before := map[string]string{}
	for k, x := range v {
		before[k] = x
	}
	r, _, w := laboratoryCarbonatePHResults(v)
	if len(r) != 1 || len(w) == 0 || !reflect.DeepEqual(v, before) {
		t.Fatal("result or provenance")
	}
	v["ph"] = "9"
	r2, _, _ := laboratoryCarbonatePHResults(v)
	if math.Abs(r2[0].SI-r[0].SI-1) > 1e-12 {
		t.Fatal("pH sensitivity")
	}
	for k, bad := range map[string]string{"sample_temp": "", "ph": "15", "ca": "0", "hco3": "NaN", "ph_measurement_temp": "60"} {
		copy := map[string]string{}
		for a, b := range before {
			copy[a] = b
		}
		copy[k] = bad
		rr, _, ww := laboratoryCarbonatePHResults(copy)
		if len(rr) != 0 || len(ww) == 0 {
			t.Fatalf("accepted %s=%s", k, bad)
		}
	}
}
