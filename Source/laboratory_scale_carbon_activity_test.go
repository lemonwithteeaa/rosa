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

func carbonDaviesTestValues() map[string]string {
	return map[string]string{"sample_temp": "25", "ph": "8", "ca": "4.0078", "mg": "1.21525", "na": "22.99", "k": "0.390983", "cl": "35.45", "so4": "0.9606", "hco3": "61.016"}
}
func TestCarbonDaviesPHREEQCReducedReference(t *testing.T) {
	b, e := os.ReadFile("reference/carbon_activity/davies_golden.tsv")
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n")[1:] {
		fs := strings.Fields(line)
		if len(fs) != 16 {
			t.Fatal(line)
		}
		x := make([]float64, 16)
		for i, f := range fs {
			x[i], e = strconv.ParseFloat(f, 64)
			if e != nil {
				t.Fatal(e)
			}
		}
		background := 0.5 * (4*x[1] + 4*x[2] + x[3] + x[4] + x[5] + 4*x[6])
		s, e := laboratoryCarbonDaviesSolve(x[0], x[1], x[7], background)
		if e != nil {
			t.Fatal(e)
		}
		// PHREEQC evaluates its temperature-dependent Debye constant and water activity.
		// Go uses version-pinned A(25 C), a_water=1. Tolerances are explicit.
		if math.Abs(s.SI-x[13]) > 0.000001 {
			t.Fatalf("SI: %g vs %g", s.SI, x[13])
		}
		for name, pair := range map[string][2]float64{"CO3": {s.Forms.CO3, x[8]}, "CO2": {s.Forms.CO2, x[9] * math.Pow(10, x[14]+x[15])}, "H": {s.Forms.Hydrogen, x[10]}, "OH": {s.Forms.Hydroxide, x[11] / math.Pow(10, x[14])}} {
			if math.Abs(pair[0]/pair[1]-1) > 0.000001 {
				t.Fatalf("%s: %v", name, pair)
			}
		}
	}
}
func TestCarbonDaviesGuardsAndPreservesInputs(t *testing.T) {
	v := carbonDaviesTestValues()
	before := map[string]string{}
	for k, x := range v {
		before[k] = x
	}
	r, _, w := laboratoryCarbonDaviesResults(v)
	if len(r) != 1 || len(w) == 0 || !reflect.DeepEqual(v, before) {
		t.Fatal("result/provenance")
	}
	for k, bad := range map[string]string{"sample_temp": "60", "na": "", "k": "", "cl": "50000", "ph": "NaN", "ph_measurement_temp": "40"} {
		c := map[string]string{}
		for a, b := range before {
			c[a] = b
		}
		c[k] = bad
		if _, e := laboratoryCarbonDaviesFromValues(c); e == nil {
			t.Fatalf("accepted %s", k)
		}
	}
}
func TestCarbonDaviesSelfConsistentStrength(t *testing.T) {
	s, e := laboratoryCarbonDaviesSolve(9, 0.0001, 0.001, 0.01)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Forms
	target := 0.01 + 0.5*(f.HCO3+4*f.CO3+f.Hydrogen+f.Hydroxide)
	if math.Abs(target-s.IonicStrength) > 1e-12 {
		t.Fatal("strength inconsistent")
	}
	k, _ := laboratoryCarbonateAssociationLogK(25)
	if math.Abs(math.Log10(f.CO3*s.Gamma2)+(-9)+k-math.Log10(f.HCO3*s.Gamma1)) > 1e-12 {
		t.Fatal("mass action inconsistent")
	}
}
func TestCarbonIndependentMeasuredBalance(t *testing.T) {
	f, e := laboratoryCarbonFormsFromPH(25, 8, 0.001)
	if e != nil {
		t.Fatal(e)
	}
	v := map[string]string{"tic": fmt.Sprint(f.TotalCarbon * 12011.1), "alkalinity": fmt.Sprint(f.Alkalinity * 50043.45)}
	if len(laboratoryCarbonMeasuredChecks(v, f)) != 0 {
		t.Fatal("matching independent analysis")
	}
	v["tic"] = "100"
	w := laboratoryCarbonMeasuredChecks(v, f)
	if len(w) != 1 || !strings.Contains(w[0], "углерод") {
		t.Fatal(w)
	}
	delete(v, "tic")
	if !strings.Contains(laboratoryCarbonIndependentBalanceText(v, f), "не подтверждён") {
		t.Fatal("missing analysis claims balance")
	}
	v["tic"] = "0"
	if len(laboratoryCarbonMeasuredChecks(v, f)) != 1 {
		t.Fatal("zero analysis discrepancy")
	}
}
func TestCarbonDaviesExplicitModelInReport(t *testing.T) {
	v := carbonDaviesTestValues()
	labScaleModelCode = "ideal"
	defer func() { labScaleModelCode = "davies" }()
	text := laboratorySaturationTextForModel(v, "carbonate_davies")
	if !strings.Contains(text, "Равновесие Дэвиса") || !strings.Contains(text, "Баланс углерода") || labScaleModelCode != "ideal" {
		t.Fatal(text)
	}
}
func TestCarbonDaviesIgnoresLegacySodiumPotassiumSum(t *testing.T) {
	v := carbonDaviesTestValues()
	s, e := laboratoryCarbonDaviesFromValues(v)
	if e != nil {
		t.Fatal(e)
	}
	balance := laboratoryCarbonDaviesChargeBalance(v, s)
	v["na_k"] = "50000"
	s2, e := laboratoryCarbonDaviesFromValues(v)
	if e != nil || s != s2 || balance != laboratoryCarbonDaviesChargeBalance(v, s2) {
		t.Fatal("aggregate duplicated separate ions")
	}
	v["k"] = "0"
	if _, e := laboratoryCarbonDaviesFromValues(v); e != nil {
		t.Fatal("explicit zero rejected")
	}
}
