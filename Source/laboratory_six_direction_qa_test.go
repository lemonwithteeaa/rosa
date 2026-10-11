//go:build windows

package main

import "testing"

func TestLAB90SixDirectionsAreWiredToRealLogic(t *testing.T) {
	if issues := laboratorySixDirectionQAIssues(); len(issues) != 0 {
		t.Fatalf("six-direction laboratory QA failed: %v", issues)
	}
}
