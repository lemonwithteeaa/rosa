//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLaboratoryFieldRowsDoNotOverlap(t *testing.T) {
	if laboratoryFieldEditOffset < laboratoryFieldLabelH {
		t.Fatalf("edit starts inside label: offset=%d labelH=%d", laboratoryFieldEditOffset, laboratoryFieldLabelH)
	}
	if laboratoryFieldEditOffset+laboratoryFieldEditH > laboratoryFieldRowStep {
		t.Fatalf("edit overlaps next row: editBottom=%d rowStep=%d", laboratoryFieldEditOffset+laboratoryFieldEditH, laboratoryFieldRowStep)
	}
}

func TestLaboratoryConceptUsesWiderRightPanelWithoutStarvingInputs(t *testing.T) {
	for _, contentW := range []int32{1010, 1280, 1400, 1600} {
		fieldsW, readinessW := laboratoryConceptWidths(contentW, false)
		if readinessW <= 0 {
			t.Fatalf("readiness card missing at width %d", contentW)
		}
		if readinessW > 700 {
			t.Fatalf("readiness card too wide at %d: %d", contentW, readinessW)
		}
		if fieldsW < 560 {
			t.Fatalf("input matrix starved at width %d: fields=%d readiness=%d", contentW, fieldsW, readinessW)
		}
		if contentW >= 1280 && readinessW < contentW*38/100 {
			t.Fatalf("right panel did not expand enough at width %d: %d", contentW, readinessW)
		}
		if fieldsW+readinessW+laboratoryConceptGap != contentW {
			t.Fatalf("layout width mismatch at %d", contentW)
		}
	}
}

func TestLaboratorySideActionsStayInsideFieldEnvelope(t *testing.T) {
	for _, height := range []int32{220, 268, 292, 320} {
		top := int32(500)
		bottom := top + height
		g := laboratorySideActions(top, bottom)
		if g.InfoH < 0 {
			t.Fatalf("height %d: negative info height %d", height, g.InfoH)
		}
		if g.PrimaryY < top || g.SecondaryY <= g.PrimaryY {
			t.Fatalf("height %d: invalid action order: primary=%d secondary=%d", height, g.PrimaryY, g.SecondaryY)
		}
		if g.SecondaryY+g.ButtonH > bottom {
			t.Fatalf("height %d: actions leave field envelope: bottom=%d envelope=%d", height, g.SecondaryY+g.ButtonH, bottom)
		}
	}
}

func TestLaboratoryFieldRowsHaveVisibleAir(t *testing.T) {
	if laboratoryFieldEditGap < 4 {
		t.Fatalf("default label/edit gap too tight: %d", laboratoryFieldEditGap)
	}
	if laboratoryFieldRowGap < 4 {
		t.Fatalf("default row gap too tight: %d", laboratoryFieldRowGap)
	}
	if laboratoryConceptGap < 18 {
		t.Fatalf("concept panel gap too tight: %d", laboratoryConceptGap)
	}
}

func TestLaboratoryRC132TitleAndActionsStayBelowGlobalWarning(t *testing.T) {
	warningBottom := int32(WARNING_Y + WARNING_H)
	for _, clientH := range []int32{820, 960, 1080, 1200} {
		v := laboratoryVerticalLayoutForHeight(clientH)
		top := py(v.TitleY)
		if top < warningBottom+4 {
			t.Fatalf("h=%d: laboratory title/actions intrude into global warning: top=%d warningBottom=%d", clientH, top, warningBottom)
		}
	}
}

func TestLaboratoryRC132SummaryNeverOverlapsMetadata(t *testing.T) {
	for _, clientH := range []int32{820, 960, 1080, 1200} {
		v := laboratoryVerticalLayoutForHeight(clientH)
		wideY := laboratorySummaryVisualY(v, true)
		if wideY < v.ObjectY+30+8 {
			t.Fatalf("h=%d: wide summary overlaps one-row metadata: summary=%d objectBottom=%d", clientH, wideY, v.ObjectY+30)
		}
		narrowY := laboratorySummaryVisualY(v, false)
		metadataBottom := max32(v.ObjectY+30, v.SourceY+28)
		if narrowY < metadataBottom+8 {
			t.Fatalf("h=%d: narrow summary overlaps metadata: summary=%d metadataBottom=%d", clientH, narrowY, metadataBottom)
		}
	}
}

func TestLaboratoryRC132PagerAndInputsAreSeparatedFromSummary(t *testing.T) {
	for _, clientH := range []int32{820, 960, 1080} {
		v := laboratoryVerticalLayoutForHeight(clientH)
		m := laboratoryFieldLayoutForHeight(clientH)
		if m.PagerOffset < v.SummaryH+4 {
			t.Fatalf("h=%d: pager starts too close to summary: offset=%d summaryH=%d", clientH, m.PagerOffset, v.SummaryH)
		}
		if m.PagerOffset+m.PagerH >= m.FieldBaseOffset {
			t.Fatalf("h=%d: pager overlaps first field row: pagerBottom=%d fieldBase=%d", clientH, m.PagerOffset+m.PagerH, m.FieldBaseOffset)
		}
		if m.EditOffset < m.LabelH || m.EditOffset+m.EditH > m.RowStep {
			t.Fatalf("h=%d: compact field geometry overlaps: %#v", clientH, m)
		}
	}
}

func TestLaboratoryRC132KeepsEnlargedHistoryAt1080pClassHeight(t *testing.T) {
	for _, clientH := range []int32{960, 1000, 1080} {
		v := laboratoryVerticalLayoutForHeight(clientH)
		m := laboratoryFieldLayoutForHeight(clientH)
		summaryY := laboratorySummaryVisualY(v, true)
		inputTop := py(summaryY + m.FieldBaseOffset)
		inputBottom := inputTop + 4*m.RowStep + m.EditOffset + m.EditH
		historyH := clientH - 24 - (inputBottom + m.HistoryGap)
		minimum := int32(225)
		if clientH >= 1000 {
			minimum = 265
		}
		if clientH >= 1080 {
			minimum = 350
		}
		if historyH < minimum {
			t.Fatalf("h=%d: enlarged previous-analysis area regressed: history=%d minimum=%d inputBottom=%d", clientH, historyH, minimum, inputBottom)
		}
	}
}

func TestLaboratoryRC132ShortClientStillKeepsHistoryDrawer(t *testing.T) {
	const clientH int32 = 820
	v := laboratoryVerticalLayoutForHeight(clientH)
	m := laboratoryFieldLayoutForHeight(clientH)
	summaryY := laboratorySummaryVisualY(v, true)
	inputTop := py(summaryY + m.FieldBaseOffset)
	inputBottom := inputTop + 4*m.RowStep + m.EditOffset + m.EditH
	historyH := clientH - 24 - (inputBottom + m.HistoryGap)
	if historyH < 130 {
		t.Fatalf("short-client previous-analysis drawer too shallow: history=%d inputBottom=%d", historyH, inputBottom)
	}
}

func TestLaboratoryRC132WideMetadataReservesSourceCaptionAndEdit(t *testing.T) {
	for _, contentW := range []int32{1450, 1480, 1600} {
		objectW, sourceW := laboratoryWideMetadataWidths(contentW)
		if objectW < 340 {
			t.Fatalf("content=%d: object edit too narrow: %d", contentW, objectW)
		}
		if sourceW < 180 {
			t.Fatalf("content=%d: source edit too narrow: %d", contentW, sourceW)
		}
		if laboratorySourceLabelW < 170 {
			t.Fatalf("source caption envelope unexpectedly narrow: %d", laboratorySourceLabelW)
		}
	}
}

func TestLaboratoryCompactActionsFitCompressedInputEnvelope(t *testing.T) {
	g := laboratorySideActions(400, 600)
	if g.ButtonH > 34 {
		t.Fatalf("compact action buttons were not reduced proportionally: %d", g.ButtonH)
	}
	if g.PrimaryY < 400 || g.SecondaryY+g.ButtonH > 600 {
		t.Fatalf("compact action stack clips its envelope: %#v", g)
	}
}

func TestLaboratoryRC132UpperBandsDoNotOverlap(t *testing.T) {
	for _, clientH := range []int32{820, 960, 1080, 1200} {
		v := laboratoryVerticalLayoutForHeight(clientH)
		if v.LegendY < v.TitleY+42+2 {
			t.Fatalf("h=%d: legend overlaps title: title=[%d,%d] legendY=%d", clientH, v.TitleY, v.TitleY+42, v.LegendY)
		}
		if v.TabsY < v.LegendY+v.LegendH+4 {
			t.Fatalf("h=%d: first tab row overlaps legend: legendBottom=%d tabsY=%d", clientH, v.LegendY+v.LegendH, v.TabsY)
		}
		if v.TabsSecondY < v.TabsY+v.TabH+4 {
			t.Fatalf("h=%d: second tab row overlaps first: firstBottom=%d secondY=%d", clientH, v.TabsY+v.TabH, v.TabsSecondY)
		}
		if v.ObjectY < v.TabsSecondY+v.TabH+6 {
			t.Fatalf("h=%d: metadata row overlaps second tabs: tabBottom=%d objectY=%d", clientH, v.TabsSecondY+v.TabH, v.ObjectY)
		}
	}
}

func TestLaboratoryRC135CompactLabelsAndEditsAreNotClipped(t *testing.T) {
	for _, clientH := range []int32{820, 960, 1080} {
		m := laboratoryFieldLayoutForHeight(clientH)
		if m.LabelH < 22 {
			t.Fatalf("h=%d: label height clips UI font: %d", clientH, m.LabelH)
		}
		if m.EditH < 24 {
			t.Fatalf("h=%d: edit height too small: %d", clientH, m.EditH)
		}
		if m.EditOffset < m.LabelH+1 {
			t.Fatalf("h=%d: edit touches caption: label=%d offset=%d", clientH, m.LabelH, m.EditOffset)
		}
	}
}

func TestLaboratoryRC135HistoryKeepsBottomSafetyMargin(t *testing.T) {
	if laboratoryBottomSafeMargin < 30 {
		t.Fatalf("bottom safety margin too small: %d", laboratoryBottomSafeMargin)
	}
}

func TestLaboratoryRC145PagerHasVisibleSafetyBandBeforeFirstField(t *testing.T) {
	for _, clientH := range []int32{820, 960, 1080, 1200} {
		m := laboratoryFieldLayoutForHeight(clientH)
		gap := m.FieldBaseOffset - (m.PagerOffset + m.PagerH)
		if gap < 10 {
			t.Fatalf("h=%d: pager/first-field safety band too small: %d px", clientH, gap)
		}
	}
}

func TestLaboratoryRC145NewHorizontalProportionsArePreserved(t *testing.T) {
	for _, contentW := range []int32{1280, 1450, 1600} {
		fieldsW, readinessW := laboratoryConceptWidths(contentW, true)
		if fieldsW < 560 {
			t.Fatalf("content=%d: input matrix became too narrow after vertical shift: %d", contentW, fieldsW)
		}
		if readinessW < 440 {
			t.Fatalf("content=%d: scale/readiness panel lost the widened proportion: %d", contentW, readinessW)
		}
	}
}

func TestLaboratoryRC146ActuallyChangesHorizontalSizes(t *testing.T) {
	fieldsW, readinessW := laboratoryConceptWidths(1440, true)
	if readinessW < 700 {
		t.Fatalf("readiness/method panel is still too narrow: %d", readinessW)
	}
	if fieldsW > 722 {
		t.Fatalf("input matrix was not narrowed enough: %d", fieldsW)
	}
	editW, unitW := laboratoryFieldWidths(340)
	if editW > 180 {
		t.Fatalf("value edit is still too long: %d", editW)
	}
	if unitW > 116 {
		t.Fatalf("unit selector is unexpectedly wide: %d", unitW)
	}
}

func TestLaboratoryRC148FirstRowCannotOverlapPager(t *testing.T) {
	for _, clientH := range []int32{820, 900, 960, 1080, 1200} {
		m := laboratoryFieldLayoutForHeight(clientH)
		pagerBottom := m.PagerOffset + m.PagerH
		fieldBase := max32(m.FieldBaseOffset, pagerBottom+18)
		if fieldBase-pagerBottom < 18 {
			t.Fatalf("h=%d: first row overlaps pager: base=%d pagerBottom=%d", clientH, fieldBase, pagerBottom)
		}
	}
}

func TestLaboratoryRC161LongCaptionGetsTwoLineEnvelope(t *testing.T) {
	got := laboratoryFieldLabelHeightForText(22, "Окислительно-восстановительный потенциал (ОВП)")
	if got < 42 {
		t.Fatalf("long ORP caption height = %d, want >= 42", got)
	}
	if other := laboratoryFieldLabelHeightForText(22, "Температура пробы"); other != 22 {
		t.Fatalf("short caption unexpectedly expanded: %d", other)
	}
}

func TestLaboratoryRC162TabsDoNotUseClippingHeight(t *testing.T) {
	for _, h := range []int32{820, 1080} {
		v := laboratoryVerticalLayoutForHeight(h)
		if v.TabH < 40 {
			t.Fatalf("h=%d: tab height still clips wrapped labels: %d", h, v.TabH)
		}
		if v.TabsSecondY < v.TabsY+v.TabH+4 {
			t.Fatalf("h=%d: tab rows overlap: first=%d second=%d height=%d", h, v.TabsY, v.TabsSecondY, v.TabH)
		}
		if v.ObjectY < v.TabsSecondY+v.TabH+4 {
			t.Fatalf("h=%d: metadata overlaps second tab row", h)
		}
	}
	if got := laboratoryTabFirstRowCount(900); got != 3 {
		t.Fatalf("narrow row count = %d, want 3", got)
	}
	if got := laboratoryTabFirstRowCount(1200); got != 4 {
		t.Fatalf("wide row count = %d, want 4", got)
	}
}

func TestLaboratoryAnalysisHeadingsUseFullCenteredCompactCaptions(t *testing.T) {
	b, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`caption = "Механические\nпримеси"`,
		`caption = "Газы и\nгидраты"`,
		`longCaption := utf8.RuneCountInString(caption) > 25`,
		`if f.Key == "h2s" || longCaption`,
		`if longCaption && hFontHeaderSmall != 0`,
		`centerListHeaders(labHistoryList, 8)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("laboratory heading guard missing %q", want)
		}
	}
}

func TestLaboratoryRC163OverviewLongLabelsAreNotStarved(t *testing.T) {
	if got := laboratoryInfluenceLabelWidth(700, 1); got < 205 {
		t.Fatalf("one-column overview label too narrow: %d", got)
	}
	if got := laboratoryInfluenceStatusWidthForLayout(700, 1); got > 190 {
		t.Fatalf("status consumed caption space: %d", got)
	}
	label := laboratoryInfluenceLabelWidth(360, 2)
	status := laboratoryInfluenceStatusWidthForLayout(360, 2)
	if label+status >= 350 {
		t.Fatalf("two-column overview leaves no room for coverage: label=%d status=%d", label, status)
	}
}

func TestLaboratoryRC169AspoSolidsCaptionGetsThreeLineEnvelope(t *testing.T) {
	label := laboratoryFieldDisplayLabel(LabFieldDef{Key: "solids", Label: "Твёрдая фаза АСПО", Unit: "%"})
	if !strings.Contains(label, "\n") {
		t.Fatalf("ASPO solids label must be multiline: %q", label)
	}
	if got := laboratoryFieldLabelHeightForText(22, label); got < 60 {
		t.Fatalf("ASPO solids label height = %d, want >= 60", got)
	}
}

func TestLaboratoryRC179LongDisplayLabelsReserveEnoughHeight(t *testing.T) {
	for _, group := range labGroups {
		for _, field := range group.Fields {
			label := laboratoryFieldDisplayLabel(field)
			runes := utf8.RuneCountInString(strings.ReplaceAll(label, "\n", ""))
			h := laboratoryFieldLabelHeightForText(22, label)
			if runes > 68 && h < 78 {
				t.Fatalf("%s/%s: very long label height %d for %q", group.Code, field.Key, h, label)
			}
			if runes > 48 && h < 60 {
				t.Fatalf("%s/%s: long label height %d for %q", group.Code, field.Key, h, label)
			}
			if strings.Count(label, "\n") >= 2 && h < 60 {
				t.Fatalf("%s/%s: multiline label height %d for %q", group.Code, field.Key, h, label)
			}
		}
	}
}

func TestLaboratoryRC179CompactCorrosionLabels(t *testing.T) {
	cases := map[string]string{"ssc_test": "SSC", "hic_test": "HIC", "material_hardness_weld": "ЗТВ", "inhibitor_type_ngvj": "НГВЖ", "inhibitor_brand_ngvj": "НГВЖ"}
	for key, token := range cases {
		got := laboratoryFieldDisplayLabel(LabFieldDef{Key: key, Label: "исходная длинная подпись"})
		if !strings.Contains(got, token) {
			t.Fatalf("%s compact label %q does not contain %q", key, got, token)
		}
	}
}

func TestLaboratoryRC180WidthAwareLabelsDoNotClip(t *testing.T) {
	cases := []struct {
		width int32
		text  string
		minH  int32
	}{
		{150, "Количественная ПЦР (qPCR) — кислотообразующие бактерии", 60},
		{170, "Окислительно-восстановительный потенциал (ОВП)", 42},
		{180, "Твёрдая фаза\nасфальтосмолопарафиновых\nотложений (АСПО)", 60},
	}
	for _, tc := range cases {
		if got := laboratoryFieldLabelHeightForWidth(22, tc.text, tc.width); got < tc.minH {
			t.Fatalf("width %d: height %d < %d for %q", tc.width, got, tc.minH, tc.text)
		}
	}
}

func TestLaboratoryRC180OverallGeometryKeepsConclusionAndHistoryVisible(t *testing.T) {
	for _, h := range []int32{800, 900, 1080, 1200} {
		for _, mass := range []bool{false, true} {
			m := laboratoryOverallVisualMetricsForHeight(h, 510, mass)
			if m.TrendH < 72 {
				t.Fatalf("h=%d mass=%v: trend too short: %+v", h, mass, m)
			}
			if m.ConclusionH < 56 {
				t.Fatalf("h=%d mass=%v: conclusion too short: %+v", h, mass, m)
			}
			if m.HistoryH < 80 {
				t.Fatalf("h=%d mass=%v: history disappeared: %+v", h, mass, m)
			}
			if m.HistoryY+m.HistoryH > h-laboratoryBottomSafeMargin {
				t.Fatalf("h=%d mass=%v: history outside client: %+v", h, mass, m)
			}
		}
	}
}

func TestLaboratoryRC180ExplicitBreaksProtectUnbreakableRussianTerms(t *testing.T) {
	cases := map[string]int{
		"orp":      2,
		"qpcr_apb": 2,
		"solids":   3,
	}
	for key, minBreaks := range cases {
		got := laboratoryFieldDisplayLabel(LabFieldDef{Key: key, Label: "fallback"})
		if strings.Count(got, "\n") < minBreaks {
			t.Fatalf("%s needs explicit wrapping, got %q", key, got)
		}
	}
}

func TestLaboratoryRC180OverallGeometryHasNoVerticalOverlap(t *testing.T) {
	for _, h := range []int32{800, 900, 1080, 1200} {
		for _, mass := range []bool{false, true} {
			m := laboratoryOverallVisualMetricsForHeight(h, 510, mass)
			conclusionTitleY := int32(510) + m.TrendH + 6
			conclusionY := conclusionTitleY + 24
			conclusionBottom := conclusionY + m.ConclusionH
			if m.HistoryY < conclusionBottom+14 {
				t.Fatalf("h=%d mass=%v: conclusion/history overlap: conclusionBottom=%d historyY=%d %+v", h, mass, conclusionBottom, m.HistoryY, m)
			}
		}
	}
}
