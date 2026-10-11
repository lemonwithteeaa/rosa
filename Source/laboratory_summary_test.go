//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestOverallSummaryCountsDirectionsInsteadOfBullets(t *testing.T) {
	text := "РАЗДЕЛ: КОРРОЗИЯ\n• pH требует внимания\n• O2 требует внимания\n• исходные данные\nРАЗДЕЛ: СОЛИ\n• Ca\n• Mg\n• SO4"
	sections, attention := laboratoryOverallSummaryCounts(text)
	if sections != 2 || attention != 1 {
		t.Fatalf("got %d sections, %d attention", sections, attention)
	}
}

func TestOverallSummaryIgnoresUnknownAndRepeatedHeadings(t *testing.T) {
	text := "РАЗДЕЛ: КОРРОЗИЯ\r\nРАЗДЕЛ: КОРРОЗИЯ\r\nРАЗДЕЛ: НЕИЗВЕСТНЫЙ\r\nтребует внимания"
	sections, attention := laboratoryOverallSummaryCounts(text)
	if sections != 1 || attention != 0 {
		t.Fatalf("got %d sections, %d attention", sections, attention)
	}
}

func TestOverallSummaryWithoutSections(t *testing.T) {
	sections, attention := laboratoryOverallSummaryCounts("• исходные данные\n• требует внимания")
	if sections != 0 || attention != 0 {
		t.Fatal(sections, attention)
	}
}

func TestSummarizeLaboratoryInfluence(t *testing.T) {
	items := []laboratoryInfluence{
		{Name: "Коррозия", Level: 0, BaseFilled: 0, BaseTotal: 7},
		{Name: "Соли", Level: 1, BaseFilled: 4, BaseTotal: 5},
		{Name: "АСПО", Level: 2, BaseFilled: 2, BaseTotal: 4},
		{Name: "Мехпримеси", Level: 3, BaseFilled: 4, BaseTotal: 4},
	}
	got := summarizeLaboratoryInfluence(items)
	if got.Directions != 4 || got.NoData != 1 || got.Low != 1 || got.Moderate != 1 || got.Elevated != 1 {
		t.Fatalf("unexpected distribution: %+v", got)
	}
	if got.BaseFilled != 10 || got.BaseTotal != 20 {
		t.Fatalf("unexpected coverage: %+v", got)
	}
}

func TestLaboratoryCoveragePercent(t *testing.T) {
	cases := []struct {
		filled, total, want int
	}{
		{0, 0, 0}, {0, 7, 0}, {3, 6, 50}, {2, 3, 67}, {8, 7, 100},
	}
	for _, tc := range cases {
		if got := laboratoryCoveragePercent(tc.filled, tc.total); got != tc.want {
			t.Fatalf("%d/%d: got %d, want %d", tc.filled, tc.total, got, tc.want)
		}
	}
}

func TestLaboratoryInfluenceChartResponsiveGeometry(t *testing.T) {
	if got := laboratoryInfluenceChartColumns(900); got != 2 {
		t.Fatalf("wide chart columns = %d", got)
	}
	if got := laboratoryInfluenceChartColumns(620); got != 1 {
		t.Fatalf("narrow chart columns = %d", got)
	}
	if got := laboratoryInfluenceChartHeight(900, 6); got != 132 {
		t.Fatalf("wide chart height = %d", got)
	}
	if got := laboratoryInfluenceChartHeight(620, 6); got != 234 {
		t.Fatalf("narrow chart height = %d", got)
	}
}

func TestLaboratoryOverviewFitsStatus(t *testing.T) {
	for _, width := range []int32{394, 620, 900} {
		if got := laboratoryInfluenceStatusWidth(width); got < 154 {
			t.Fatalf("status badge is too narrow at %d: %d", width, got)
		}
	}
}

func TestLaboratoryWidthsUseAvailableSpace(t *testing.T) {
	for _, contentW := range []int32{720, 894, 1650} {
		sourceW, searchW := laboratoryMetadataWidths(contentW)
		if sourceW < 160 || searchW < 160 || sourceW+searchW > contentW-374 {
			t.Fatalf("metadata width %d: source=%d search=%d", contentW, sourceW, searchW)
		}
	}
	for _, columnW := range []int32{240, 371, 520} {
		editW, unitW := laboratoryFieldWidths(columnW)
		if editW < 100 || unitW < 100 || editW+unitW+8 > columnW {
			t.Fatalf("field width %d: edit=%d unit=%d", columnW, editW, unitW)
		}
	}
}

func TestLaboratoryFieldMarkerCodes(t *testing.T) {
	cases := []struct {
		group, key, want string
	}{
		{"corrosion", "mineralization", "Б/О"},
		{"corrosion", "ph", "Б/О"},
		{"microbiology", "srb", "Б"},
		{"microbiology", "iron_bacteria", "Д"},
	}
	for _, tc := range cases {
		if got := laboratoryFieldMarkerCode(tc.group, tc.key); got != tc.want {
			t.Fatalf("%s/%s: got %q want %q", tc.group, tc.key, got, tc.want)
		}
	}
}

func TestLaboratoryFieldMarkerCaptions(t *testing.T) {
	for _, code := range []string{"Б", "Д", "Б/О", "Д/О", "Т", "Т/О"} {
		caption := laboratoryFieldMarkerCaption(code)
		if caption == "" || strings.Contains(caption, "не определён") {
			t.Fatalf("marker %q has no user-facing caption: %q", code, caption)
		}
	}
	if got := laboratoryFieldMarkerCode("scale", "mineralization"); got != "Б/О" {
		t.Fatalf("scale/mineralization must be a shared baseline indicator, got %q", got)
	}
}

func TestLaboratoryMovedMetadataDoesNotOverlap(t *testing.T) {
	for _, contentW := range []int32{1450, 1650, 1900} {
		objectW, sourceW := laboratoryWideMetadataWidths(contentW)
		if objectW < 360 {
			t.Fatalf("content %d: object field too narrow: %d", contentW, objectW)
		}
		if sourceW < 180 {
			t.Fatalf("content %d: source field too narrow: %d", contentW, sourceW)
		}
		metadataRight := int32(242) + contentW - 576
		sourceRight := int32(376) + objectW + 14 + 150 + sourceW
		if sourceRight > metadataRight {
			t.Fatalf("content %d: metadata overlaps action button: %d > %d", contentW, sourceRight, metadataRight)
		}
	}
}

func TestLaboratoryLegendCaptionsHaveEnoughWidth(t *testing.T) {
	wants := map[string]int32{
		"дополнительный":       120,
		"базовый, повторяется": 170,
		"доп., повторяется":    154,
	}
	for caption, minimum := range wants {
		if got := laboratoryLegendCaptionWidth(caption); got < minimum {
			t.Fatalf("%q is clipped: width %d, need at least %d", caption, got, minimum)
		}
	}
}

func TestLaboratoryInfluenceDisplayNameWrapsLongCaptions(t *testing.T) {
	if got := laboratoryInfluenceDisplayName("Механические примеси"); got != "Механические\nпримеси" {
		t.Fatalf("mechanical impurities caption wrap = %q", got)
	}
	if got := laboratoryInfluenceDisplayName("Газы и гидраты"); got != "Газы и\nгидраты" {
		t.Fatalf("gases/hydrates caption wrap = %q", got)
	}
	if got := laboratoryInfluenceDisplayName("Солеотложения"); got != "Солеотложения" {
		t.Fatalf("unexpected rewrite of short caption: %q", got)
	}
}

func TestLaboratoryRC165MultilineInfluenceCaptionDoesNotUseSingleLine(t *testing.T) {
	flags := laboratoryInfluenceDrawFlags(DT_WORDBREAK)
	if flags&DT_SINGLELINE != 0 {
		t.Fatalf("multiline caption still uses DT_SINGLELINE: %#x", flags)
	}
	if flags&DT_WORDBREAK == 0 {
		t.Fatalf("multiline caption lost DT_WORDBREAK: %#x", flags)
	}
}

func TestLaboratoryInfluenceRowHeightKeepsSixItemsVisible(t *testing.T) {
	if got := laboratoryInfluenceRowHeight(2, 6); got < 42 {
		t.Fatalf("two-column six-item row height must allow two-line caption: %d", got)
	}
	if got := laboratoryInfluenceRowHeight(3, 6); got != 34 {
		t.Fatalf("three-column layout should stay on default height, got %d", got)
	}
}

func TestLaboratoryInfluenceTwoColumnLabelWidthIsWideEnough(t *testing.T) {
	if got := laboratoryInfluenceLabelWidth(420, 2); got < 210 {
		t.Fatalf("two-column label width too narrow: %d", got)
	}
	if got := laboratoryInfluenceStatusWidthForLayout(420, 2); got > 138 {
		t.Fatalf("two-column status width too wide: %d", got)
	}
}

func TestLaboratoryInfluenceChartHeightUsesActualRowHeight(t *testing.T) {
	width := int32(760) // two-column layout for six directions
	rows := int32(3)
	want := int32(8) + rows*laboratoryInfluenceRowHeight(2, 6) + 22
	if got := laboratoryInfluenceChartHeight(width, 6); got < want {
		t.Fatalf("chart height %d is too small for three visible rows; need at least %d", got, want)
	}
}

func TestLaboratoryRC179InfluenceChartHeightMatchesRenderer(t *testing.T) {
	w := int32(900)
	cols := laboratoryInfluenceChartColumns(w)
	rows := (int32(6) + cols - 1) / cols
	want := int32(8) + rows*laboratoryInfluenceRowHeight(cols, 6) + 22
	if got := laboratoryInfluenceChartHeight(w, 6); got != want {
		t.Fatalf("height=%d want=%d", got, want)
	}
}
