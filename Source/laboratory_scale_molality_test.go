//go:build windows

package main

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestMolalityConversionMatchesOfficialPHREEQC(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "molality", "phreeqc_molality_golden.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("unexpected PHREEQC rows: %d", len(lines))
	}
	type input struct {
		density, solutes, na, cl string
	}
	inputs := map[int]input{
		1: {"1058.44277", "58.44277", "22989.77", "35453.00"},
		2: {"979.221385", "29.221385", "11494.885", "17726.500"},
	}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) != 5 {
			t.Fatalf("unexpected PHREEQC row: %q", line)
		}
		solution, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		in, ok := inputs[solution]
		if !ok {
			t.Fatalf("unexpected PHREEQC solution %d", solution)
		}
		v := map[string]string{
			"brine_density": in.density, "solute_mass": in.solutes,
			"ca_total": "0", "mg_total": "0", "tic": "0", "so4_total": "0",
			"na": in.na, "k": "0", "cl": in.cl,
		}
		normalized, water, err := laboratoryMolalityValues(v)
		if err != nil {
			t.Fatal(err)
		}
		phreeqcWater, _ := strconv.ParseFloat(fields[2], 64)
		phreeqcNa, _ := strconv.ParseFloat(fields[3], 64)
		phreeqcCl, _ := strconv.ParseFloat(fields[4], 64)
		if math.Abs(phreeqcWater-1) > 1e-12 {
			t.Fatalf("PHREEQC did not normalize solution %d to one kg water", solution)
		}
		naMgKg, _ := strconv.ParseFloat(normalized["na"], 64)
		clMgKg, _ := strconv.ParseFloat(normalized["cl"], 64)
		// Official phreeqc.dat gram formula weights: Na 22.9898, Cl 35.453 g/mol.
		// The saved PHREEQC output has 12 digits after the decimal exponent; 1e-4 mg/kgw
		// is below 5e-9 relative for these cases and covers only that text rounding.
		if math.Abs(naMgKg-phreeqcNa*22.9898*1000) > 1e-4 || math.Abs(clMgKg-phreeqcCl*35.453*1000) > 1e-4 {
			t.Fatalf("solution %d: ROSA mg/kgw does not match PHREEQC: Na %.12g/%.12g, Cl %.12g/%.12g", solution, naMgKg, phreeqcNa*22.9898*1000, clMgKg, phreeqcCl*35.453*1000)
		}
		naMgL, _ := strconv.ParseFloat(in.na, 64)
		clMgL, _ := strconv.ParseFloat(in.cl, 64)
		if math.Abs(naMgKg*water-naMgL) > 1e-9 || math.Abs(clMgKg*water-clMgL) > 1e-9 {
			t.Fatalf("solution %d: reverse conversion did not restore mg/L", solution)
		}
	}
}

func molalityTestValues() map[string]string {
	v := carbonDaviesTestValues()
	v["ca_total"] = "40.078"
	v["mg_total"] = "24.305"
	v["so4_total"] = "96.06"
	v["tic"] = "12.0111"
	v["ba"] = "1.37327"
	v["sr"] = "0.8762"
	v["brine_density"] = "1001"
	v["solute_mass"] = "1"
	return v
}
func TestWaterMassDensityAndSolutes(t *testing.T) {
	v := molalityTestValues()
	m, e := laboratoryWaterMassPerLiter(v)
	if e != nil || m != 1 {
		t.Fatal(m, e)
	}
	v["brine_density"] = "1050"
	v["solute_mass"] = "100"
	m, e = laboratoryWaterMassPerLiter(v)
	if e != nil || math.Abs(m-.95) > 1e-14 {
		t.Fatal(m, e)
	}
	for k, bad := range map[string]string{"brine_density": "", "solute_mass": ""} {
		copy := molalityTestValues()
		copy[k] = bad
		if _, e := laboratoryWaterMassPerLiter(copy); e == nil {
			t.Fatal("accepted missing", k)
		}
	}
	for _, bad := range []string{"0", "NaN", "-1", "1001"} {
		v := molalityTestValues()
		v["solute_mass"] = bad
		if _, e := laboratoryWaterMassPerLiter(v); e == nil {
			t.Fatal("accepted solute mass", bad)
		}
	}
}
func TestMolalityConversionInvarianceAndPreservation(t *testing.T) {
	v := molalityTestValues()
	expected, e := laboratoryMultiComplexFromValuesMolal(v)
	if e != nil {
		t.Fatal(e)
	}
	// Same molal composition in 0.95 kg water/L: all analytical mg/L scale by water mass.
	for _, k := range []string{"ca_total", "mg_total", "tic", "so4_total", "na", "k", "cl", "ba", "sr"} {
		x, _ := strconv.ParseFloat(v[k], 64)
		v[k] = strconv.FormatFloat(x*.95, 'g', 17, 64)
	}
	v["brine_density"] = "1050"
	v["solute_mass"] = "100"
	before := map[string]string{}
	for k, x := range v {
		before[k] = x
	}
	normalized, m, e := laboratoryMolalityValues(v)
	if e != nil || math.Abs(m-.95) > 1e-14 || !reflect.DeepEqual(v, before) {
		t.Fatal(m, e)
	}
	actual, e := laboratoryMultiComplexFromValuesMolal(normalized)
	if e != nil || math.Abs(actual.CalciteSI-expected.CalciteSI) > 1e-12 || math.Abs(actual.GypsumSI-expected.GypsumSI) > 1e-12 || math.Abs(actual.BariteSI-expected.BariteSI) > 1e-12 || math.Abs(actual.CelestiteSI-expected.CelestiteSI) > 1e-12 {
		t.Fatal("composition changed", actual, e)
	}
	if math.Abs(actual.Ca*m/expected.Ca-.95) > 1e-12 {
		t.Fatal("reverse concentration conversion")
	}
}
func TestMolalityModelParityAndCorrectReportUnits(t *testing.T) {
	v := molalityTestValues()
	r, i, _ := laboratoryMolalityResults(v)
	s, err := laboratoryMultiComplexFromValuesMolal(v)
	if err != nil || len(r) == 0 || i != s.I || s.WaterActivity <= 0 || s.WaterActivity >= 1 {
		t.Fatal("molal water-activity route", s, err)
	}
	text := laboratorySaturationTextForModel(v, "multicomplex_molal")
	if !strings.Contains(text, "моль/кг воды") || !strings.Contains(text, "масса воды") || strings.Contains(text, "Оценённая ионная сила: "+strconv.FormatFloat(i, 'g', 3, 64)+" моль/л") {
		t.Fatal(text)
	}
	v["brine_density"] = ""
	r, _, w := laboratoryMolalityResults(v)
	if len(r) != 0 || len(w) == 0 {
		t.Fatal("missing density calculated")
	}
}
func TestMolalityAlkalinityComparisonUsesOriginalUnits(t *testing.T) {
	v := molalityTestValues()
	v["brine_density"] = "1050"
	v["solute_mass"] = "100"
	n, m, e := laboratoryMolalityValues(v)
	if e != nil {
		t.Fatal(e)
	}
	s, e := laboratoryMultiComplexFromValuesMolal(n)
	if e != nil {
		t.Fatal(e)
	}
	v["alkalinity"] = strconv.FormatFloat(s.Forms.Alkalinity*50043.45*m, 'g', 17, 64)
	_, _, w := laboratoryMolalityResults(v)
	for _, x := range w {
		if strings.HasPrefix(x, "Щёлочность:") {
			t.Fatal("matching laboratory alkalinity warned", w)
		}
	}
	v["alkalinity"] = "2000"
	_, _, w = laboratoryMolalityResults(v)
	found := false
	for _, x := range w {
		if strings.HasPrefix(x, "Щёлочность:") {
			found = true
		}
	}
	if !found {
		t.Fatal("discrepancy not checked")
	}
}
