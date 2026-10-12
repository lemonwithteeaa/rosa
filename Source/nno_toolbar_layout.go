package main

// The fund-analytics toolbar is a single horizontal rail.  Layout is computed
// independently from Win32 so that all breakpoints can be tested on Linux.
type nnoToolbarCell struct{ X, W int32 }
type nnoToolbarArrangement struct {
	Cells [8]nnoToolbarCell
	Gaps  [7]int32
	Mode  int // 0: compact, 1: medium, 2: full captions
}

// The first two slots are the dates. The rest are: operation mode, data source,
// update, audit, password-protected method, object forecast.
func nnoToolbarArrange(x, width int32) nnoToolbarArrangement {
	var a nnoToolbarArrangement
	// Below 877 px the eight native controls cannot retain readable minimum
	// widths in a single row. Use two complete rows, rather than stretching
	// controls beyond their parent or shrinking date boxes until they clip.
	if width < 877 {
		a.Mode = -1
		minimum := [2][4]int32{
			{96, 96, 176, 138},    // from, to, lift, source
			{132, 138, 196, 142}, // update, audit, rules, forecast
		}
		weights := [2][4]int32{{1, 1, 2, 2}, {1, 1, 2, 1}}
		for row := 0; row < 2; row++ {
			const gap int32 = 8
			var base, totalWeight int32
			for i := 0; i < 4; i++ {
				base += minimum[row][i]
				totalWeight += weights[row][i]
			}
			base += 3 * gap
			extra, assigned := max32(0, width-base), int32(0)
			xpos := x
			for i := 0; i < 4; i++ {
				add := extra * weights[row][i] / totalWeight
				if i == 3 {
					add = extra - assigned
				}
				assigned += add
				a.Cells[row*4+i] = nnoToolbarCell{X: xpos, W: minimum[row][i] + add}
				xpos += minimum[row][i] + add
				if i < 3 {
					xpos += gap
					a.Gaps[row*4+i] = gap
				}
			}
		}
		return a
	}
	minimum := [8]int32{88, 88, 120, 105, 88, 99, 136, 105}
	a.Gaps = [7]int32{10, 8, 6, 6, 6, 6, 6}
	if width >= 1310 {
		a.Mode = 2
		minimum = [8]int32{116, 116, 170, 145, 134, 155, 230, 180}
		a.Gaps = [7]int32{12, 10, 10, 8, 8, 8, 8}
	} else if width >= 1071 {
		a.Mode = 1
		minimum = [8]int32{98, 98, 150, 122, 112, 124, 178, 140}
		a.Gaps = [7]int32{10, 8, 7, 6, 6, 6, 6}
	}
	var minTotal int32
	for _, w := range minimum {
		minTotal += w
	}
	for _, gap := range a.Gaps {
		minTotal += gap
	}
	if width < minTotal {
		// Do not let controls overlap. A too-small host can clip the rail,
		// but never mixes control coordinates into another control's area.
		width = minTotal
	}
	weight := [8]int32{1, 1, 3, 2, 2, 2, 3, 2}
	var sumWeight int32
	for _, w := range weight {
		sumWeight += w
	}
	extra, used := width-minTotal, int32(0)
	cur := x
	for i, w := range minimum {
		add := extra * weight[i] / sumWeight
		if i == len(minimum)-1 {
			add = extra - used
		}
		used += add
		a.Cells[i] = nnoToolbarCell{X: cur, W: w + add}
		cur += w + add
		if i < len(a.Gaps) {
			cur += a.Gaps[i]
		}
	}
	return a
}
