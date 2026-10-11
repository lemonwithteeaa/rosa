//go:build windows

package main

import (
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCarbonComplexPHREEQCReducedReference(t *testing.T) {
	b, e := os.ReadFile("reference/carbon_complex/complex_golden.tsv")
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n")[1:] {
		fs := strings.Fields(line)
		if len(fs) != 14 {
			t.Fatal(line)
		}
		x := make([]float64, 14)
		for i, f := range fs {
			x[i], e = strconv.ParseFloat(f, 64)
			if e != nil {
				t.Fatal(e)
			}
		}
		// Normalize actual PHREEQC CO2 to this route's explicit a_water=1.
		// Reference neutral coefficients are explicitly zero in the reduced database.
		totalC := x[4] + x[5] + x[6]*math.Pow(10, x[13]) + x[7] + x[8]
		s, e := laboratoryCarbonComplexSolve(x[0], x[1], totalC, 0.001)
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(s.SI-x[12]) > 1e-6 {
			t.Fatalf("pH %g SI %g vs %g", x[0], s.SI, x[12])
		}
		for name, p := range map[string][2]float64{"Ca": {s.FreeCalcium, x[3]}, "HCO3": {s.Forms.HCO3, x[4]}, "CO3": {s.Forms.CO3, x[5]}, "CaCO3": {s.CaCO3, x[7]}, "CaHCO3": {s.CaHCO3, x[8]}} {
			if math.Abs(p[0]/p[1]-1) > 1e-6 {
				t.Fatalf("pH %g %s %v", x[0], name, p)
			}
		}
	}
}
func TestCarbonComplexMassClosureAndStrongComplexation(t *testing.T) {
	for _, ph := range []float64{7, 9, 10, 12} {
		s, e := laboratoryCarbonComplexSolve(ph, 0.001, 0.01, 0.001)
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(s.TotalCalcium/0.001-1) > 1e-10 || math.Abs(s.TotalCarbon/0.01-1) > 1e-10 || s.FreeCalcium <= 0 || s.FreeCalcium > 0.001 {
			t.Fatal(s)
		}
	}
	for _, ph := range []float64{0, 14} {
		if _, e := laboratoryCarbonComplexSolve(ph, 0.001, 0.01, 0.001); e == nil {
			t.Fatal("out-of-domain strength accepted")
		}
	}
	s, e := laboratoryCarbonComplexSolve(10, .01, .001, .001)
	if e != nil || s.CaCO3 <= 0 || s.FreeCalcium >= .01 {
		t.Fatal("complexes have no effect", s, e)
	}
}
func TestCarbonComplexInputsAndExplicitReport(t *testing.T) {
	v := carbonDaviesTestValues()
	v["ca_total"] = "40.078"
	v["tic"] = "120.111"
	before := map[string]string{}
	for k, x := range v {
		before[k] = x
	}
	r, _, w := laboratoryCarbonComplexResults(v)
	if len(r) != 1 || len(w) == 0 || !reflect.DeepEqual(v, before) {
		t.Fatal("result or input preservation")
	}
	text := laboratorySaturationTextForModel(v, "carbonate_complex")
	if !strings.Contains(text, "CaCO₃(aq)") || !strings.Contains(text, "это проверка замыкания") || strings.Contains(text, "Баланс углерода: независимый") {
		t.Fatal(text)
	}
	for k, bad := range map[string]string{"tic": "", "ca_total": "", "sample_temp": "60", "na": "", "ph": "NaN", "cl": "50000"} {
		c := map[string]string{}
		for a, b := range before {
			c[a] = b
		}
		c[k] = bad
		if _, e := laboratoryCarbonComplexFromValues(c); e == nil {
			t.Fatalf("accepted %s", k)
		}
	}
}
