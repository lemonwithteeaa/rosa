//go:build windows

package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestPHREEQCWaterActivityReference(t *testing.T) {
	b, err := os.ReadFile("reference/water_activity/golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 5 {
		t.Fatalf("water-activity golden must contain four controls, got %d", len(lines)-1)
	}
	previous := 1.0
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) != 4 {
			t.Fatal(line)
		}
		ionicStrength, e1 := strconv.ParseFloat(fields[1], 64)
		activity, e2 := strconv.ParseFloat(fields[2], 64)
		if e1 != nil || e2 != nil || ionicStrength <= 0 || ionicStrength > .100001 || activity <= 0 || activity > previous {
			t.Fatalf("invalid water-activity control: %q", line)
		}
		previous = activity
	}
	if !strings.Contains(laboratoryMultiComplexBasis, "a(H₂O)=1") {
		t.Fatal("working-model water-activity limitation must remain explicit")
	}
}
