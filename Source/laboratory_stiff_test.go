//go:build windows

package main

import (
	"math"
	"testing"
)

func TestLaboratoryStiffProfileUsesMeqPerLiter(t *testing.T) {
	values := map[string]string{
		"ca": "40.078", "mg": "24.305", "na": "22.989769", "k": "39.0983",
		"hco3": "61.0168", "so4": "48.03", "cl": "35.453",
	}
	p := laboratoryStiffProfileFromValues(values)
	if !p.OK {
		t.Fatal("stiff profile not calculated")
	}
	wantCations := []float64{2, 2, 2}
	wantAnions := []float64{1, 1, 1}
	for i, want := range wantCations {
		if math.Abs(p.Cations[i]-want) > 1e-6 {
			t.Fatalf("cation %d = %g, want %g", i, p.Cations[i], want)
		}
	}
	for i, want := range wantAnions {
		if math.Abs(p.Anions[i]-want) > 1e-6 {
			t.Fatalf("anion %d = %g, want %g", i, p.Anions[i], want)
		}
	}
}

func TestLaboratoryStiffRequiresCoreIonSet(t *testing.T) {
	p := laboratoryStiffProfileFromValues(map[string]string{"ca": "40", "mg": "20"})
	if p.OK {
		t.Fatal("incomplete ion set must not produce a Stiff profile")
	}
}
