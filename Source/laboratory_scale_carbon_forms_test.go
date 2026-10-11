//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestCarbonFormsMatchPHREEQCMassAction(t *testing.T) {
	data, err := os.ReadFile("reference/temperature/carbon_forms_golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		fs := strings.Fields(line)
		if len(fs) != 9 {
			t.Fatalf("bad reference: %s", line)
		}
		x := make([]float64, 9)
		for i, v := range fs {
			x[i], err = strconv.ParseFloat(v, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		f, err := laboratoryCarbonFormsFromPH(x[0], x[1], math.Pow(10, x[3]))
		if err != nil {
			t.Fatal(err)
		}
		// Actual PHREEQC activities; correct for this route's explicit a_water=1.
		for name, check := range map[string][2]float64{"CO3": {math.Log10(f.CO3), x[4]}, "CO2": {math.Log10(f.CO2), x[6] + x[8]}, "OH": {math.Log10(f.Hydroxide), x[7] - x[8]}} {
			if math.Abs(check[0]-check[1]) > 1e-9 {
				t.Fatalf("T=%g %s %v", x[0], name, check)
			}
		}
		if math.Abs(f.TotalCarbon-f.CO2-f.HCO3-f.CO3) > 1e-15 {
			t.Fatal("carbon sum")
		}
	}
}
func TestCarbonFormsAlkalinityCheck(t *testing.T) {
	v := map[string]string{"sample_temp": "25", "ph": "8", "hco3": "61.016"}
	if len(laboratoryCarbonAlkalinityWarnings(v)) == 0 {
		t.Fatal("missing independent check")
	}
	f, err := laboratoryCarbonFormsFromPH(25, 8, 0.001)
	if err != nil {
		t.Fatal(err)
	}
	v["alkalinity"] = fmt.Sprint(f.Alkalinity * 50.04345 * 1000)
	if len(laboratoryCarbonAlkalinityWarnings(v)) != 0 {
		t.Fatal("matching alkalinity")
	}
	v["alkalinity"] = "200"
	if len(laboratoryCarbonAlkalinityWarnings(v)) == 0 {
		t.Fatal("missed discrepancy")
	}
	for _, x := range []float64{math.NaN(), math.Inf(1), -1, 15} {
		if _, e := laboratoryCarbonFormsFromPH(25, x, 0.001); e == nil {
			t.Fatal("accepted invalid pH")
		}
	}
}
