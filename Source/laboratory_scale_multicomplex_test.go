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

func TestMultiComplexPHREEQCReference(t *testing.T) {
	b, e := os.ReadFile("reference/multicomplex/golden.tsv")
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n")[1:] {
		fs := strings.Fields(line)
		if len(fs) != 34 {
			t.Fatal(line)
		}
		x := make([]float64, 34)
		for i, f := range fs {
			x[i], e = strconv.ParseFloat(f, 64)
			if e != nil {
				t.Fatal(e)
			}
		}
		aw := math.Pow(10, x[33])
		cat := x[1] + x[9] + x[10] + x[11] + x[12]/aw
		mgt := x[2] + x[13] + x[14] + x[15] + x[16] + x[17]/aw
		bat := x[4] + x[18] + x[20]/aw + x[21] + x[22]
		srt := x[5] + x[19] + x[23]/aw + x[24] + x[25]
		ct := x[6] + x[7] + x[8]*aw + x[9] + x[10] + x[13] + x[14] + x[21] + x[22] + x[24] + x[25]
		st := x[3] + x[11] + x[15] + 2*x[16] + x[18] + x[19]
		s, e := laboratoryMultiComplexSolveExtended(x[0], cat, mgt, bat, srt, ct, st, .001)
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(s.CalciteSI-x[29]) > 1e-5 || math.Abs(s.GypsumSI-(x[30]-2*x[33])) > 1e-5 || math.Abs(s.BariteSI-x[31]) > 1e-5 || math.Abs(s.CelestiteSI-x[32]) > 1e-5 {
			t.Fatal("SI mismatch", s)
		}
		for name, p := range map[string][2]float64{"Ca": {s.Ca, x[1]}, "Mg": {s.Mg, x[2]}, "SO4": {s.SO4, x[3]}, "Ba": {s.Ba, x[4]}, "Sr": {s.Sr, x[5]}, "HC": {s.Forms.HCO3, x[6]}, "CaC": {s.CaCO3, x[9]}, "CaHC": {s.CaHCO3, x[10]}, "CaS": {s.CaSO4, x[11]}, "CaOH": {s.CaOH, x[12] / aw}, "MgC": {s.MgCO3, x[13]}, "MgHC": {s.MgHCO3, x[14]}, "MgS": {s.MgSO4, x[15]}, "MgS2": {s.MgSO42, x[16]}, "MgOH": {s.MgOH, x[17] / aw}, "BaS": {s.BaSO4, x[18]}, "SrS": {s.SrSO4, x[19]}, "BaOH": {s.BaOH, x[20] / aw}, "BaC": {s.BaCO3, x[21]}, "BaHC": {s.BaHCO3, x[22]}, "SrOH": {s.SrOH, x[23] / aw}, "SrC": {s.SrCO3, x[24]}, "SrHC": {s.SrHCO3, x[25]}} {
			if math.Abs(p[0]/p[1]-1) > 1e-5 {
				t.Fatalf("pH %g %s %v", x[0], name, p)
			}
		}
	}
}

func TestMultiComplexMolalWaterActivityPHREEQCReference(t *testing.T) {
	b, err := os.ReadFile("reference/multicomplex/golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n")[1:] {
		fields := strings.Fields(line)
		x := make([]float64, 34)
		for i, field := range fields {
			x[i], err = strconv.ParseFloat(field, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
		aw := math.Pow(10, x[33])
		cat := x[1] + x[9] + x[10] + x[11] + x[12]
		mgt := x[2] + x[13] + x[14] + x[15] + x[16] + x[17]
		bat := x[4] + x[18] + x[20] + x[21] + x[22]
		srt := x[5] + x[19] + x[23] + x[24] + x[25]
		ct := x[6] + x[7] + x[8] + x[9] + x[10] + x[13] + x[14] + x[21] + x[22] + x[24] + x[25]
		st := x[3] + x[11] + x[15] + 2*x[16] + x[18] + x[19]
		s, err := laboratoryMultiComplexSolveExtendedMolal(x[0], cat, mgt, bat, srt, ct, st, .001, .002)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(s.WaterActivity-aw) > 1e-10 || math.Abs(s.CalciteSI-x[29]) > 1e-5 || math.Abs(s.GypsumSI-x[30]) > 1e-5 || math.Abs(s.BariteSI-x[31]) > 1e-5 || math.Abs(s.CelestiteSI-x[32]) > 1e-5 {
			t.Fatal("molal activity/SI mismatch", x[0], s, aw)
		}
		for name, pair := range map[string][2]float64{"CaOH": {s.CaOH, x[12]}, "MgOH": {s.MgOH, x[17]}, "BaOH": {s.BaOH, x[20]}, "SrOH": {s.SrOH, x[23]}, "CO2": {s.Forms.CO2, x[8]}} {
			if math.Abs(pair[0]/pair[1]-1) > 1e-5 {
				t.Fatalf("pH %g %s %v", x[0], name, pair)
			}
		}
	}
}

func TestMultiComplexThermodynamicBasisPinned(t *testing.T) {
	b, err := os.ReadFile("reference/temperature/phreeqc.dat")
	if err != nil {
		t.Fatal(err)
	}
	db := string(b)
	for _, block := range []string{
		"Ca+2 + H2O = CaOH+ + H+\n\t-log_k -12.78",
		"Ca+2 + CO3-2 = CaCO3\n\t-log_k 3.224; -delta_h 3.545 kcal\n\t-analytic -1228.732 -0.29944 35512.75 485.818",
		"Ca+2 + CO3-2 + H+ = CaHCO3+\n\t-log_k 10.91; -delta_h 4.38 kcal\n\t-analytic -6.009 3.377e-2 2044",
		"Ca+2 + SO4-2 = CaSO4\n\t-log_k 2.25",
		"Mg+2 + H2O = MgOH+ + H+\n\t-log_k -11.44",
		"Mg+2 + CO3-2 = MgCO3\n\t-log_k 2.98\n\t-delta_h 2.713 kcal\n\t-analytic 0.991 0.00667",
		"Mg+2 + H+ + CO3-2 = MgHCO3+\n\t-log_k 11.399\n\t-delta_h -2.771 kcal\n\t-analytic 48.6721 0.03252849 -2614.335 -18.00263 563713.9",
		"Mg+2 + SO4-2 = MgSO4\n\t-gamma 0 0.2\n\t-log_k 2.42; -delta_h 19 kJ\n\t-analytical_expression 0 9.64e-3 -136",
		"SO4-2 + MgSO4 = Mg(SO4)2-2\n\t-gamma 7 0.047\n\t-log_k 0.52; -delta_h -13.6 kJ\n\t-analytical_expression 0 -1.51e-3 0 0 8.604e4",
		"Ba+2 + SO4-2 = BaSO4\n\t-log_k 2.7",
		"Sr+2 + SO4-2 = SrSO4\n\t-log_k 2.29",
	} {
		if !strings.Contains(db, block) {
			t.Fatalf("pinned PHREEQC basis block is absent: %q", block)
		}
	}
	if !strings.Contains(laboratoryMultiComplexBasis, "I≤0,1") || !strings.Contains(laboratoryMultiComplexBasis, "a(H₂O)=1") {
		t.Fatal("calculation domain is not explicit")
	}
}

func TestBaSrExpansionBasisPinnedAndImplemented(t *testing.T) {
	b, err := os.ReadFile("reference/temperature/phreeqc.dat")
	if err != nil {
		t.Fatal(err)
	}
	db := string(b)
	for _, block := range []string{
		"Ba+2 + H2O = BaOH+ + H+\n\t-log_k -13.47",
		"Ba+2 + CO3-2 = BaCO3\n\t-log_k 2.71",
		"Ba+2 + HCO3- = BaHCO3+\n\t-log_k 0.982",
		"Sr+2 + H2O = SrOH+ + H+\n\t-log_k -13.29",
		"Sr+2 + CO3-2 + H+ = SrHCO3+\n\t-log_k 11.509",
		"Sr+2 + CO3-2 = SrCO3\n\t-log_k 2.81",
	} {
		if !strings.Contains(db, block) {
			t.Fatalf("Ba/Sr candidate block is absent: %q", block)
		}
	}
	if !strings.Contains(laboratoryBaSrComplexBasis, "карбонатные") || !strings.Contains(laboratoryBaSrComplexBasis, "гидроксокомплексы") {
		t.Fatal("implemented Ba/Sr basis is not explicit")
	}
}
func TestMultiComplexFourBalances(t *testing.T) {
	s, e := laboratoryMultiComplexSolveExtended(9, .002, .001, 1e-5, 2e-5, .01, .01, .001)
	if e != nil {
		t.Fatal(e)
	}
	for name, p := range map[string][2]float64{"Ca": {s.Ca + s.CaCO3 + s.CaHCO3 + s.CaSO4 + s.CaOH, .002}, "Mg": {s.Mg + s.MgCO3 + s.MgHCO3 + s.MgSO4 + s.MgSO42 + s.MgOH, .001}, "Ba": {s.Ba + s.BaSO4 + s.BaCO3 + s.BaHCO3 + s.BaOH, 1e-5}, "Sr": {s.Sr + s.SrSO4 + s.SrCO3 + s.SrHCO3 + s.SrOH, 2e-5}, "C": {s.Forms.TotalCarbon + s.CaCO3 + s.CaHCO3 + s.MgCO3 + s.MgHCO3 + s.BaCO3 + s.BaHCO3 + s.SrCO3 + s.SrHCO3, .01}, "S": {s.SO4 + s.CaSO4 + s.MgSO4 + 2*s.MgSO42 + s.BaSO4 + s.SrSO4, .01}} {
		if math.Abs(p[0]/p[1]-1) > 1e-10 {
			t.Fatal(name, p)
		}
	}
	if s.CaSO4 <= 0 || s.MgSO4 <= 0 || s.MgSO42 <= 0 || s.BaSO4 <= 0 || s.SrSO4 <= 0 || s.BaCO3 <= 0 || s.BaHCO3 <= 0 || s.BaOH <= 0 || s.SrCO3 <= 0 || s.SrHCO3 <= 0 || s.SrOH <= 0 || s.Ca >= .002 || s.Mg >= .001 {
		t.Fatal("complexes ineffective")
	}
}
func TestMultiComplexZeroIonsAndGuards(t *testing.T) {
	s, e := laboratoryMultiComplexSolve(8, .001, 0, .001, 0, .001)
	if e != nil || s.Mg != 0 || s.SO4 != 0 || s.CaSO4 != 0 || s.MgSO4 != 0 || s.MgSO42 != 0 {
		t.Fatal(s, e)
	}
	for _, ph := range []float64{math.NaN(), math.Inf(1), -1, 15, 14} {
		if _, e := laboratoryMultiComplexSolve(ph, .001, .001, .001, .001, .001); e == nil {
			t.Fatal("accepted pH", ph)
		}
	}
}
func TestMultiComplexInputAndReport(t *testing.T) {
	v := carbonDaviesTestValues()
	v["ca_total"] = "40.078"
	v["mg_total"] = "24.305"
	v["so4_total"] = "96.06"
	v["tic"] = "12.0111"
	before := map[string]string{}
	for k, x := range v {
		before[k] = x
	}
	v["ba"] = "1.37327"
	v["sr"] = "0.8762"
	before["ba"] = v["ba"]
	before["sr"] = v["sr"]
	r, _, w := laboratoryMultiComplexResults(v)
	if len(r) != 4 || len(w) == 0 || !reflect.DeepEqual(v, before) {
		t.Fatal("result/provenance")
	}
	text := laboratorySaturationTextForModel(v, "multicomplex")
	if !strings.Contains(text, "MgSO₄(aq)") || !strings.Contains(text, "Mg(SO₄)₂²⁻") || !strings.Contains(text, "BaSO₄(aq)") || !strings.Contains(text, "Барит") || !strings.Contains(text, "Целестин") {
		t.Fatal(text)
	}
	for k, bad := range map[string]string{"mg_total": "", "so4_total": "", "ba": "bad", "sample_temp": "60", "tic": "0", "cl": "50000"} {
		c := map[string]string{}
		for a, b := range before {
			c[a] = b
		}
		c[k] = bad
		if _, e := laboratoryMultiComplexFromValues(c); e == nil {
			t.Fatal("accepted", k)
		}
	}
}
