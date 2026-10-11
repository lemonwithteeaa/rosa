//go:build windows

package main

import "testing"

func TestRC395BackgroundStartsBelowCompleteHeader(t *testing.T) {
	if authHeaderHeight != 112 {
		t.Fatalf("header height drifted: %d", authHeaderHeight)
	}
	// A common widescreen viewport must crop the source from the top, not from
	// both top and bottom. This keeps the ground/pumpjacks/RITEK mark visible.
	sx, sy, cw, ch := authBackgroundCoverCrop(1920, 968)
	if sx != 0 || cw != authBackgroundSourceWidth {
		t.Fatalf("unexpected horizontal crop for widescreen: sx=%d cw=%d", sx, cw)
	}
	if sy <= 0 || sy+ch != authBackgroundSourceHeight {
		t.Fatalf("background must be bottom anchored: sy=%d ch=%d", sy, ch)
	}
}

func TestRC395BackgroundNarrowViewportKeepsRightEdge(t *testing.T) {
	sx, sy, cw, ch := authBackgroundCoverCrop(1024, 900)
	if sy != 0 || ch != authBackgroundSourceHeight {
		t.Fatalf("unexpected vertical crop for tall viewport: sy=%d ch=%d", sy, ch)
	}
	if sx < 0 || sx+cw != authBackgroundSourceWidth {
		t.Fatalf("right edge must stay anchored: sx=%d cw=%d", sx, cw)
	}
}
