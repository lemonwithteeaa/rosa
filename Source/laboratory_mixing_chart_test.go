//go:build windows

package main

import "testing"

func TestLaboratoryWaterMixingCurveCoversFractions(t *testing.T) {
	v := mixingInputs()
	points, _, err := laboratoryWaterMixingCurve(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 21 {
		t.Fatalf("points=%d", len(points))
	}
	if points[0].Fraction != 0 || points[len(points)-1].Fraction != 1 {
		t.Fatal("fraction range incomplete")
	}
}

func TestLaboratoryWaterMixingCurveDoesNotAveragePH(t *testing.T) {
	v := mixingInputs()
	v["mix_ph"] = "8.25"
	_, ph, err := laboratoryWaterMixingCurve(v)
	if err != nil {
		t.Fatal(err)
	}
	if ph != 8.25 {
		t.Fatalf("pH changed: %v", ph)
	}
}
