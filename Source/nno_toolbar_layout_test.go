package main

import "testing"

func TestNNOToolbarUsesFullWidthAndNeverOverlaps(t *testing.T) {
	for _, width := range []int32{910, 1000, 1070, 1071, 1096, 1309, 1310, 1427, 1600, 1920} {
		a := nnoToolbarArrange(242, width)
		for i, c := range a.Cells {
			if c.W < 80 {
				t.Fatalf("width=%d slot=%d width=%d", width, i, c.W)
			}
			if i > 0 && c.X <= a.Cells[i-1].X+a.Cells[i-1].W {
				t.Fatalf("overlap %d slot %d", width, i)
			}
		}
		last := a.Cells[7]
		if last.X+last.W != 242+width {
			t.Fatalf("unused or overflowing space: width=%d end=%d", width, last.X+last.W)
		}
		if a.Cells[1].X-a.Cells[0].X-a.Cells[0].W != a.Gaps[0] {
			t.Fatal("date separator lost")
		}
	}
}
func TestNNOToolbarModeTransitions(t *testing.T) {
	for _, tc := range []struct {
		width int32
		mode  int
	}{{910, 0}, {1070, 0}, {1071, 1}, {1309, 1}, {1310, 2}, {1427, 2}} {
		if mode := nnoToolbarArrange(0, tc.width).Mode; mode != tc.mode {
			t.Fatalf("width=%d mode=%d", tc.width, mode)
		}
	}
}
