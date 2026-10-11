//go:build windows

package main

import (
	"math"
	"strings"
	"testing"
)

func TestLaboratoryWaterQCChargeBalanceNearZero(t *testing.T) {
	values := map[string]string{
		"ca": "100.195", "mg": "60.7625", "na": "229.89769", "k": "0",
		"cl": "354.53", "so4": "240.15", "hco3": "0", "mineralization": "0.98553519",
	}
	qc := laboratoryWaterQualityQC(values)
	if !qc.ChargeBalanceOK {
		t.Fatal("charge balance not calculated")
	}
	if math.Abs(qc.ChargeBalancePct) > 0.2 {
		t.Fatalf("unexpected balance: %.4f", qc.ChargeBalancePct)
	}
}

func TestLaboratoryWaterQCHardness(t *testing.T) {
	values := map[string]string{"ca": "40.078", "mg": "24.305"}
	qc := laboratoryWaterQualityQC(values)
	if !qc.HardnessOK {
		t.Fatal("hardness not calculated")
	}
	if math.Abs(qc.HardnessMeqL-4) > 1e-6 {
		t.Fatalf("hardness meq/L = %g", qc.HardnessMeqL)
	}
	if math.Abs(qc.HardnessCaCO3MgL-200) > 1e-6 {
		t.Fatalf("hardness as CaCO3 = %g", qc.HardnessCaCO3MgL)
	}
}

func TestLaboratoryWaterQCTDSComparisonAndText(t *testing.T) {
	values := map[string]string{
		"ca": "40.078", "mg": "24.305", "na": "22.989769", "k": "39.0983",
		"cl": "35.453", "so4": "96.06", "hco3": "61.0168", "mineralization": "0.318999869",
	}
	qc := laboratoryWaterQualityQC(values)
	if !qc.TDSComparisonOK {
		t.Fatal("TDS comparison not calculated")
	}
	text := laboratoryWaterQualityQCText(values)
	for _, token := range []string{"Ионный баланс", "TDS-контроль", "Расчётная жёсткость"} {
		if !strings.Contains(text, token) {
			t.Fatalf("QC text missing %q: %s", token, text)
		}
	}
}
