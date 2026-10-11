package main

import "testing"

func TestSectionNoticeFitsEverySupportedWidth(t *testing.T) {
	for _, width := range []int32{1180, 1280, 1366, 1600, 1920, 2560} {
		textWidth, labelX := sectionNoticeLayout(width)
		labelWidth := width - labelX - 12
		if textWidth < 0 || 58+textWidth+14 > labelX {
			t.Fatalf("warning text overlaps teaching-data note at width %d", width)
		}
		if labelX < 12 || labelWidth < 288 || labelX+labelWidth > width-12 {
			t.Fatalf("teaching-data note outside warning strip at width %d", width)
		}
	}
}
