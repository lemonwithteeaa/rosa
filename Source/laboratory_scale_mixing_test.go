//go:build windows

package main

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// External reference agreement is checked to one part per billion for a(H2O).
// The unchanged reduced core differs from this golden by about 5.06e-10.
const laboratoryMixingWaterActivityReferenceTolerance = 1e-9

func TestConservativeWaterMixingAndEquilibrium(t *testing.T) {
	a := laboratoryMixTotals{Ca: .001, Mg: .002, Ba: 1e-5, Sr: 2e-5, Carbon: .003, Sulfate: .001, Background: .001}
	b := laboratoryMixTotals{Ca: .003, Mg: .001, Ba: 3e-5, Sr: 1e-5, Carbon: .009, Sulfate: .005, Background: .002}
	m, err := laboratoryMixConservativeTotals(a, b, .25)
	if err != nil {
		t.Fatal(err)
	}
	for name, pair := range map[string][2]float64{"Ca": {m.Ca, .0015}, "Mg": {m.Mg, .00175}, "Ba": {m.Ba, 1.5e-5}, "Sr": {m.Sr, 1.75e-5}, "C": {m.Carbon, .0045}, "S": {m.Sulfate, .002}, "background": {m.Background, .00125}} {
		if math.Abs(pair[0]-pair[1]) > 1e-14 {
			t.Fatal(name, pair)
		}
	}
	s, err := laboratoryMultiComplexSolveMixed(8.25, a, b, .25)
	if err != nil {
		t.Fatal(err)
	}
	balances := map[string][2]float64{
		"Ca": {s.Ca + s.CaCO3 + s.CaHCO3 + s.CaSO4 + s.CaOH, m.Ca},
		"Mg": {s.Mg + s.MgCO3 + s.MgHCO3 + s.MgSO4 + s.MgSO42 + s.MgOH, m.Mg},
		"Ba": {s.Ba + s.BaSO4 + s.BaCO3 + s.BaHCO3 + s.BaOH, m.Ba},
		"Sr": {s.Sr + s.SrSO4 + s.SrCO3 + s.SrHCO3 + s.SrOH, m.Sr},
		"C":  {s.Forms.TotalCarbon + s.CaCO3 + s.CaHCO3 + s.MgCO3 + s.MgHCO3 + s.BaCO3 + s.BaHCO3 + s.SrCO3 + s.SrHCO3, m.Carbon},
		"S":  {s.SO4 + s.CaSO4 + s.MgSO4 + 2*s.MgSO42 + s.BaSO4 + s.SrSO4, m.Sulfate},
	}
	for name, pair := range balances {
		if math.Abs(pair[0]/pair[1]-1) > 1e-10 {
			t.Fatal(name, pair)
		}
	}
}

func TestMolalMixingPHREEQCReference(t *testing.T) {
	b, err := os.ReadFile("reference/mixing/golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) != 34 {
		t.Fatalf("expected 34 PHREEQC columns, got %d", len(fields))
	}
	x := make([]float64, len(fields))
	for i, field := range fields {
		x[i], err = strconv.ParseFloat(field, 64)
		if err != nil {
			t.Fatal(err)
		}
	}
	first := laboratoryMixTotals{Ca: .001, Mg: .002, Ba: 1e-5, Sr: 2e-5, Carbon: .003, Sulfate: .001, Background: .001, BackgroundSpecies: .002}
	second := laboratoryMixTotals{Ca: .003, Mg: .001, Ba: 3e-5, Sr: 1e-5, Carbon: .009, Sulfate: .005, Background: .002, BackgroundSpecies: .004}
	s, err := laboratoryMultiComplexSolveMixedMolal(x[0], first, second, .25)
	if err != nil {
		t.Fatal(err)
	}
	aw := math.Pow(10, x[33])
	for name, pair := range map[string][2]float64{
		"Ca": {s.Ca, x[1]}, "Mg": {s.Mg, x[2]}, "SO4": {s.SO4, x[3]}, "Ba": {s.Ba, x[4]}, "Sr": {s.Sr, x[5]},
		"HCO3": {s.Forms.HCO3, x[6]}, "CaSO4": {s.CaSO4, x[11]}, "MgSO4": {s.MgSO4, x[15]}, "BaSO4": {s.BaSO4, x[18]}, "SrSO4": {s.SrSO4, x[19]},
	} {
		if math.Abs(pair[0]/pair[1]-1) > 1e-5 {
			t.Fatalf("%s mismatch %v", name, pair)
		}
	}
	if math.Abs(s.WaterActivity-aw) > laboratoryMixingWaterActivityReferenceTolerance || math.Abs(s.CalciteSI-x[29]) > 1e-5 || math.Abs(s.GypsumSI-x[30]) > 1e-5 || math.Abs(s.BariteSI-x[31]) > 1e-5 || math.Abs(s.CelestiteSI-x[32]) > 1e-5 {
		t.Fatal("mixed-water activity or SI mismatch", s, aw)
	}
}

func TestWaterMixingRejectsFalsePrecision(t *testing.T) {
	a := laboratoryMixTotals{Ca: .001, Carbon: .001}
	for _, fraction := range []float64{-0.01, 1.01, math.NaN()} {
		if _, err := laboratoryMixConservativeTotals(a, a, fraction); err == nil {
			t.Fatal("accepted fraction", fraction)
		}
	}
	if _, err := laboratoryMultiComplexSolveMixed(math.NaN(), a, a, .5); err == nil {
		t.Fatal("accepted absent measured mixed-water pH")
	}
}
