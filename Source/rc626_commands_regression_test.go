//go:build windows

package main

import "testing"

func TestRC626ClassificationApprovalDoesNotShareExitCommand(t *testing.T) {
	if ID_ASSESS_CONFIRM_FULL == ID_HEADER_EXIT {
		t.Fatalf("classification approval command %d collides with Exit", ID_ASSESS_CONFIRM_FULL)
	}
	if ID_ASSESS_CONFIRM_FULL == ID_HEADER_APPROVE {
		t.Fatalf("classification approval command %d collides with user switch", ID_ASSESS_CONFIRM_FULL)
	}
}
