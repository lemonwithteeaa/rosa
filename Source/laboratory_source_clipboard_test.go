//go:build windows

package main

import "testing"

func TestRC150SourceMetadataUpdatesInMemoryWithoutChangingOtherPassFields(t *testing.T) {
	p := &LabPassRecord{PassID: "LP-1", ObjectKey: "OBJ", Date: "18.09.2026", Source: "old", Status: labPassDraftStatus}
	applyLaboratorySourceToPass(p, "  Протокол № 77  ")
	if p.Source != "Протокол № 77" {
		t.Fatalf("source was not normalized: %q", p.Source)
	}
	if p.PassID != "LP-1" || p.ObjectKey != "OBJ" || p.Date != "18.09.2026" || p.Status != labPassDraftStatus {
		t.Fatalf("source edit unexpectedly changed pass identity: %#v", p)
	}
}

func TestRC150StandardClipboardShortcutMapping(t *testing.T) {
	cases := []struct {
		key         uintptr
		ctrl, shift bool
		want        editShortcutAction
	}{
		{VK_A, true, false, editShortcutSelectAll},
		{VK_C, true, false, editShortcutCopy},
		{VK_V, true, false, editShortcutPaste},
		{VK_X, true, false, editShortcutCut},
		{VK_INSERT, true, false, editShortcutCopy},
		{VK_INSERT, false, true, editShortcutPaste},
		{VK_DELETE, false, true, editShortcutCut},
		{VK_C, false, false, editShortcutNone},
	}
	for _, tc := range cases {
		if got := standardEditShortcutAction(tc.key, tc.ctrl, tc.shift); got != tc.want {
			t.Fatalf("key=%d ctrl=%v shift=%v: got %v want %v", tc.key, tc.ctrl, tc.shift, got, tc.want)
		}
	}
}
