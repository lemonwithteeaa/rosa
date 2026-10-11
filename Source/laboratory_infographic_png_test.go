//go:build windows

package main

import "testing"

func TestLaboratoryRC177InfographicGeometryFitsDPI100125150(t *testing.T) {
	for _, dpi := range []int{100, 125, 150} {
		for _, scope := range []string{laboratoryScopeObject, laboratoryScopeAll} {
			g := laboratoryInfographicGeometryForDPI(dpi, scope)
			if g.Width <= 0 || g.Height <= 0 {
				t.Fatalf("dpi %d scope %s: invalid canvas %+v", dpi, scope, g)
			}
			if g.InfluenceTop+g.InfluenceHeight >= g.TrendTop {
				t.Fatalf("dpi %d scope %s: influence overlaps trend: %+v", dpi, scope, g)
			}
			if g.TrendTop+g.TrendHeight > g.SummaryTop {
				t.Fatalf("dpi %d scope %s: trend overlaps summary: %+v", dpi, scope, g)
			}
			if g.SummaryTop+g.SummaryHeight > g.Height-g.Margin+1 {
				t.Fatalf("dpi %d scope %s: summary clipped: %+v", dpi, scope, g)
			}
			if scope != laboratoryScopeObject && g.TrendHeight <= g.InfluenceHeight {
				t.Fatalf("dpi %d: mass trend area too short: %+v", dpi, g)
			}
		}
	}
}

func TestLaboratoryInfographicLiveLayoutFitsSixRowsAtCommonWidths(t *testing.T) {
	for _, width := range []int32{800, 1180, 1366, 1600, 1920} {
		chartH, trendTop, trendH := laboratoryInfographicLiveLayout(width, 900)
		columns := laboratoryInfluenceChartColumns(width - 48)
		rows := (6 + columns - 1) / columns
		need := int32(8) + rows*laboratoryInfluenceRowHeight(columns, 6) + 27 + 48
		if chartH < need {
			t.Fatalf("width %d: summary panel %d px, needs %d px for six rows and captions", width, chartH, need)
		}
		if trendTop < 120+chartH+10 || trendH < 180 {
			t.Fatalf("width %d: invalid/nonresponsive trend area top=%d height=%d", width, trendTop, trendH)
		}
		if width >= 1000 && columns != 2 {
			t.Fatalf("width %d: expected readable two-column summary, got %d columns", width, columns)
		}
	}
}
