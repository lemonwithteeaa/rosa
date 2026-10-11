//go:build windows

package main

import "testing"

func TestComplexLegendUsesTwoColumnsAfterFourObjects(t *testing.T) {
	tests := []struct{ count, columns, rows int }{{0, 1, 0}, {4, 1, 4}, {5, 2, 3}, {8, 2, 4}, {9, 2, 5}}
	for _, test := range tests {
		columns, rows := complexLegendLayout(test.count)
		if columns != test.columns || rows != test.rows {
			t.Fatalf("count=%d: got %d columns/%d rows, want %d/%d", test.count, columns, rows, test.columns, test.rows)
		}
	}
}

func TestComplexChartPlotRemainsVisibleWithLongLegend(t *testing.T) {
	plot, ok := complexChartPlotRect(RECT{Left: 0, Top: 0, Right: 1760, Bottom: 405}, 12)
	if !ok || plot.Bottom-plot.Top < 65 || plot.Right-plot.Left < 180 {
		t.Fatalf("plot hidden in a large window: %#v ok=%v", plot, ok)
	}
}

func TestBundledDemoWatercutCoversNamedAndAdditionalExamples(t *testing.T) {
	for name, want := range map[string]string{
		"01_01_12-01-2022_070_процентов.xlsx":        "070",
		"52_01_Бешкульское_17_70_ПРОЦЕНТОВ.xlsx":     "70",
		"71_ДОПОЛНИТЕЛЬНЫЙ_УЧЕБНЫЙ_ПРИМЕР_71.xlsx":   "25",
		"81_ДОПОЛНИТЕЛЬНЫЙ_УЧЕБНЫЙ_ПРИМЕР_81.xlsx":   "30",
		"100_ДОПОЛНИТЕЛЬНЫЙ_УЧЕБНЫЙ_ПРИМЕР_100.xlsx": "80",
	} {
		got, ok := bundledDemoWatercut(name)
		if !ok || got != want {
			t.Fatalf("%s: got %q/%v, want %q", name, got, ok, want)
		}
	}
}
