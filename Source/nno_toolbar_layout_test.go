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

func TestNNOToolbarTwoRowsDoNotClipAtNarrowWidths(t *testing.T) {
	for _, width := range []int32{720, 740, 800, 850, 876} {
		a := nnoToolbarArrange(242, width)
		if a.Mode != -1 {
			t.Fatalf("width=%d should use two rows, mode=%d", width, a.Mode)
		}
		for row := 0; row < 2; row++ {
			start := row * 4
			for i := start; i < start+4; i++ {
				c := a.Cells[i]
				if c.W < 92 || c.X < 242 || c.X+c.W > 242+width {
					t.Fatalf("width=%d row=%d control=%d outside viewport: %+v", width, row, i, c)
				}
				if i > start && c.X < a.Cells[i-1].X+a.Cells[i-1].W+6 {
					t.Fatalf("width=%d row=%d controls overlap at %d", width, row, i)
				}
			}
			last := a.Cells[start+3]
			if last.X+last.W != 242+width {
				t.Fatalf("width=%d row=%d has overflow or unused space", width, row)
			}
		}
	}
}
