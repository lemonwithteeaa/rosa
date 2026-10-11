package main

import "testing"

func TestPageInputIsolationAcrossTransitionAndSubview(t *testing.T) {
	tests := []struct {
		name                                     string
		owner, active, controlScope, activeScope int
		visible, transitioning, want             bool
	}{
		{"active shared action", 5, 5, pageSubviewShared, pageSubviewMRPFleet, true, false, true},
		{"old page queued click", 3, 5, pageSubviewShared, pageSubviewMRPFleet, true, false, false},
		{"hidden control queued click", 5, 5, pageSubviewShared, pageSubviewMRPFleet, false, false, false},
		{"same page old inner tab", 3, 3, pageSubviewAssessmentTech, pageSubviewAssessmentCompare, true, false, false},
		{"current inner tab", 3, 3, pageSubviewAssessmentCompare, pageSubviewAssessmentCompare, true, false, true},
		{"switch in progress", 5, 5, pageSubviewMRPFleet, pageSubviewMRPFleet, true, true, false},
		{"startup", 5, 0, pageSubviewMRPFleet, pageSubviewMRPFleet, true, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pageInputMayBeHandled(tc.owner, tc.active, tc.visible, tc.transitioning, pageSubviewMatches(tc.controlScope, tc.activeScope))
			if got != tc.want {
				t.Fatalf("input allowed=%v want=%v", got, tc.want)
			}
		})
	}
}
