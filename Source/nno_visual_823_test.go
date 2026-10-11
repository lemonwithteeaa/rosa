//go:build windows

package main

import "testing"

func TestNNO823LegendRowsStayInOwnPanel(t *testing.T) {
	for _, height := range []int32{180, 220, 300, 480} {
		panel := RECT{Left: 810, Top: 5, Right: 1080, Bottom: height - 5}
		rows := nnoDistributionLegendRows(panel)
		if rows[0].Top < panel.Top+25 || rows[3].Bottom > panel.Bottom {
			t.Fatalf("rows exceed panel height %d: %+v", height, rows)
		}
		for i, row := range rows {
			if row.Bottom <= row.Top || row.Right <= row.Left {
				t.Fatalf("empty legend row %d at height %d: %+v", i, height, row)
			}
			if i != 0 && row.Top != rows[i-1].Bottom {
				t.Fatalf("legend rows overlap or leave gaps: %d %+v", height, rows)
			}
		}
	}
}

func TestNNO823ExpandedClickUsesSamePlotGeometryAsPaint(t *testing.T) {
	prevPaint, prevPick, prevKind := nnoExpandedPainting, nnoExpandedPicking, nnoExpandedKind
	defer func() { nnoExpandedPainting, nnoExpandedPicking, nnoExpandedKind = prevPaint, prevPick, prevKind }()
	rc := RECT{Left: 0, Top: 0, Right: 1050, Bottom: 510}
	for _, kind := range []int{0, 1} {
		nnoExpandedKind = kind
		nnoExpandedPainting, nnoExpandedPicking = true, false
		paint := nnoRollingPlot(rc)
		if kind == 1 {
			paint = nnoDistributionPlot(rc)
		}
		nnoExpandedPainting, nnoExpandedPicking = false, true
		pick := nnoRollingPlot(rc)
		if kind == 1 {
			pick = nnoDistributionPlot(rc)
		}
		if paint != pick {
			t.Fatalf("chart %d: paint %+v, click %+v", kind, paint, pick)
		}
	}
}

func TestNNO823ExpandedCanSelectQuartilesAndBothCentres(t *testing.T) {
	rc := RECT{Left: 0, Top: 0, Right: 1050, Bottom: 510}
	groups := []int{0}
	var summaries [nnoMethodCount]nnoCycleStats
	summaries[0] = nnoCycleStats{Count: 25, Min: 100, Q1: 300, Median: 480, Mean: 540, Q3: 730, Max: 900, Defined: true}
	plot := nnoDistributionPlot(rc)
	x := plot.Left + (plot.Right-plot.Left)/2
	for _, tc := range []struct {
		kind  int
		value float64
	}{
		{nnoDistributionQ1, 300}, {nnoDistributionMedian, 480},
		{nnoDistributionMean, 540}, {nnoDistributionQ3, 730},
	} {
		y := plot.Bottom - int32(tc.value/1000*float64(plot.Bottom-plot.Top))
		got, ok := nnoPickDistributionMarkerExtended(rc, groups, 1000, x, y, false, summaries, true)
		if !ok || got.Kind != tc.kind {
			t.Fatalf("kind %d value %.1f: %+v ok=%v", tc.kind, tc.value, got, ok)
		}
	}
}
