package main

import "testing"

func TestPageControlVisibilityOwnership(t *testing.T) {
	for _, tc := range []struct {
		name      string
		owner     int
		active    int
		requested bool
		preparing bool
		want      bool
	}{
		{"startup laboratory initialization", 3, 0, true, false, false},
		{"startup dashboard initialization", 1, 0, true, false, false},
		{"active page requested visible", 3, 3, true, false, true},
		{"target page preparing", 3, 3, true, true, false},
		{"other page requested visible", 3, 1, true, false, false},
		{"active page requested hidden", 3, 3, false, false, false},
		{"no owner", 0, 0, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pageControlMayBeVisible(tc.owner, tc.active, tc.requested, tc.preparing); got != tc.want {
				t.Fatalf("owner=%d active=%d requested=%t: got %t, want %t", tc.owner, tc.active, tc.requested, got, tc.want)
			}
		})
	}
}
