//go:build windows

package main

import "testing"

// The laboratory page uses three tooltip ranges: labels 0..9, edits 10..19,
// model controls 20..21, and field badges 22..31. A shorter state table used
// to panic during WM_CREATE and made the application exit immediately after a
// successful login.
func TestLaboratoryStartupTooltipCapacity(t *testing.T) {
	const highestUsedSlot = 31
	if len(labFieldTipText) <= highestUsedSlot {
		t.Fatalf("tooltip text table has %d slots; need at least %d", len(labFieldTipText), highestUsedSlot+1)
	}
	if len(labFieldTipAdded) <= highestUsedSlot {
		t.Fatalf("tooltip state table has %d slots; need at least %d", len(labFieldTipAdded), highestUsedSlot+1)
	}
	if len(labFieldTipText) != len(labFieldTipAdded) {
		t.Fatalf("tooltip tables differ: text=%d state=%d", len(labFieldTipText), len(labFieldTipAdded))
	}
}
