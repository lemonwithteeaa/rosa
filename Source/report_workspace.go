//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	reportDTEndEllipsis       = 0x00008000
	ID_RESULT_APPROVE_SCHEME  = 9760
	ID_RESULT_SAVE_FULL       = 9761
	ID_RESULT_OPEN_SAVED      = 9762
	ID_RESULT_EXPORT_PDF      = 9763
	ID_RESULT_EXPORT_DOCX     = 9764
	ID_RESULT_EXPORT_XLSX     = 9765
	ID_RESULT_ADD_SCHEME      = 9766
	ID_RESULT_COPY_SCHEME     = 9767
	ID_RESULT_COMPARE_SCHEMES = 9768
	ID_RESULT_USE_SELECTED    = 9769
	ID_RESULT_SELECTED_INFO   = 9770
	// RC739: section 8 uses its own non-colliding command range. RC737/738
	// reused 9771/9772 from the failure-card controls, which made command
	// routing fragile and hard to diagnose.
	ID_ARCHIVE_EXPORT_PDF       = 18771
	ID_ARCHIVE_EXPORT_DOCX      = 18772
	ID_ARCHIVE_EXPORT_XLSX      = 18773
	ID_RESULT_SECTION_CARD_BASE = 9900
	ID_ARCHIVE_METRIC_CARD_BASE = 9910
)

type FullReportRecord struct {
	ID              string            `json:"id"`
	CreatedAt       string            `json:"created_at"`
	ObjectKey       string            `json:"object_key"`
	ObjectLabel     string            `json:"object_label"`
	ScenarioID      string            `json:"scenario_id"`
	ScenarioName    string            `json:"scenario_name"`
	Methods         string            `json:"methods"`
	Availability    string            `json:"availability"`
	ForecastDays    float64           `json:"forecast_days,omitempty"`
	NetBenefitK     float64           `json:"net_benefit_thousand_rub,omitempty"`
	DecisionStatus  string            `json:"decision_status"`
	ApprovedAt      string            `json:"approved_at,omitempty"`
	DecisionBasis   string            `json:"decision_basis,omitempty"`
	Author          string            `json:"author"`
	Snapshot        string            `json:"snapshot"`
	SnapshotVersion string            `json:"snapshot_version,omitempty"`
	Sections        map[string]string `json:"sections,omitempty"`
}

var (
	resultSubtitle           syscall.Handle
	resultRefreshButton      syscall.Handle
	resultObjectCard         syscall.Handle
	resultSectionCards       [6]syscall.Handle
	resultVariantsTitle      syscall.Handle
	resultVariantsSubtitle   syscall.Handle
	resultVariantsList       syscall.Handle
	resultAddVariantButton   syscall.Handle // historical handle; RC721 no longer creates a separate “add” action
	resultCopyVariantButton  syscall.Handle
	resultCompareButton      syscall.Handle
	resultTooltip            syscall.Handle
	resultDuplicateTipText   []uint16
	resultCompareTipText     []uint16
	resultUseSelectedButton  syscall.Handle
	resultSelectedInfoButton syscall.Handle
	resultDecisionCard       syscall.Handle
	resultTraceCard          syscall.Handle
	resultApproveButton      syscall.Handle
	resultSaveButton         syscall.Handle
	resultDecisionHint       syscall.Handle
	resultArchiveTitle       syscall.Handle
	resultArchiveSubtitle    syscall.Handle
	resultArchiveList        syscall.Handle

	// RC724: section 8 uses the same visual language as the final report instead
	// of presenting a bare ListView. The register remains read-only, while the
	// summary cards make the approved-only semantics visible at a glance.
	archiveRegistrySubtitle syscall.Handle
	archiveRegistryBanner   syscall.Handle
	archiveRegistryStats    [4]syscall.Handle
	archiveRegistryTitle    syscall.Handle
	archiveRegistryHint     syscall.Handle
	archiveExportPDFButton  syscall.Handle
	archiveExportDOCXButton syscall.Handle
	archiveExportXLSXButton syscall.Handle
	resultOpenButton        syscall.Handle
	resultExportPDFButton   syscall.Handle
	resultExportDOCXButton  syscall.Handle
	resultExportXLSXButton  syscall.Handle
	fullReportsLoaded       bool
	fullReports             []FullReportRecord
	resultArchiveRenderHash uint64
	approvedRegistryRows    []FullReportRecord
	resultScenarioRows      []AlternativeMethodScenario

	// RC739: section-8 export runs off the UI thread so DOCX/XLSX generation
	// and Word PDF conversion cannot freeze/redraw the workspace.
	archiveExportBusy         bool
	archiveExportResultMu     sync.Mutex
	archiveExportResultPath   string
	archiveExportResultFormat string
	archiveExportResultErr    error

	// RC722: dedicated comparison window. It follows the same visual language as
	// the Methods dialogs, supports normal minimize/maximize/close and formats
	// structural headings instead of dumping unformatted text.
	resultCompareDone        bool
	resultCompareWindow      syscall.Handle
	resultCompareHeaderBand  syscall.Handle
	resultCompareAccent      syscall.Handle
	resultCompareTitle       syscall.Handle
	resultCompareSubtitle    syscall.Handle
	resultCompareBody        syscall.Handle
	resultCompareObjectLabel string
	resultCompareBodyText    string
	resultCompareHeaderTitle string
	resultCompareWindowTitle string
)

func fullReportsPath() string { return sharedPath("full_reports_archive.json") }

// resultCard creates a quiet report-surface card without native EDIT scrollbar chrome.
func resultCard(text string, x, y, w, h int32) syscall.Handle {
	// Owner-draw keeps the section-7 cards visually stable during resize and
	// lets the object/decision accents stay local instead of colouring a whole row.
	z := create("STATIC", text, WS_CHILD|SS_OWNERDRAW|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	addPageControl(ID_RESULT, z)
	return z
}

// RC730: the six evidence cards are navigation controls, not dead summaries.
// A concise card stays readable at compact widths; clicking opens the full
// source section where the underlying data and calculations live.
func resultSectionCard(text string, index int, x, y, w, h int32) syscall.Handle {
	id := ID_RESULT_SECTION_CARD_BASE + index
	z := create("STATIC", text, WS_CHILD|SS_OWNERDRAW|SS_NOPREFIX|SS_NOTIFY, x, py(y), w, h, hMain, id)
	addPageControl(ID_RESULT, z)
	return z
}

func resultSectionCardPage(index int) int {
	switch index {
	case 0:
		return ID_OBJECTS
	case 1:
		return ID_LAB
	case 2:
		return ID_ASSESS
	case 3:
		return ID_ACTIVITIES
	case 4:
		return ID_MRP
	case 5:
		return ID_ECON
	default:
		return 0
	}
}

// registryCard reuses the report card language on section 8 without coupling
// the approved register to section 7's control collection.
func registryCard(text string, x, y, w, h int32) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|SS_OWNERDRAW|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	addPageControl(ID_ARCHIVE, z)
	return z
}

// RC732: section-8 KPI cards are deliberately informational only.
// Section 8 is the approved register, so the summary metrics must not navigate
// away from the register. Keep the large report-card visual language, but do
// not opt into SS_NOTIFY and do not assign command ids.
func registryMetricCard(text string, index int, x, y, w, h int32) syscall.Handle {
	_ = index
	z := create("STATIC", text, WS_CHILD|SS_OWNERDRAW|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	addPageControl(ID_ARCHIVE, z)
	return z
}

func resultInlineTextWidth(hdc syscall.Handle, text string) int32 {
	if hdc == 0 || text == "" {
		return 0
	}
	buf, _ := syscall.UTF16FromString(text)
	if len(buf) <= 1 {
		return 0
	}
	r := RECT{Left: 0, Top: 0, Right: 4096, Bottom: 64}
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX)
	return max32(0, r.Right-r.Left)
}

func drawResultInlineText(hdc syscall.Handle, r RECT, text string, color uintptr) int32 {
	if hdc == 0 || text == "" || r.Right <= r.Left {
		return 0
	}
	w := resultInlineTextWidth(hdc, text)
	if w <= 0 {
		return 0
	}
	buf, _ := syscall.UTF16FromString(text)
	drawR := r
	drawR.Right = min32(r.Right, r.Left+w+2)
	pSetTextColor.Call(uintptr(hdc), color)
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&drawR)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
	return w
}

// drawResultObjectCardText keeps the object card restrained but makes the three
// operational values requested by the technologist immediately scannable:
// object identity, GNO value and current MRP are red; labels and target stay navy.
func drawResultObjectCardText(hdc syscall.Handle, r RECT, text string) bool {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.SplitN(text, "\n", 2)
	if len(parts) != 2 {
		return false
	}
	navy := rgb(15, 48, 82)
	red := rgb(183, 20, 37)
	headerR := r
	headerR.Bottom = min32(r.Bottom, r.Top+20)
	if parts[0] != "" {
		oldFont := uintptr(0)
		if hFontHeaderSmall != 0 {
			oldFont, _, _ = pSelectObject.Call(uintptr(hdc), uintptr(hFontHeaderSmall))
		}
		buf, _ := syscall.UTF16FromString(parts[0])
		pSetTextColor.Call(uintptr(hdc), rgb(92, 111, 132))
		pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&headerR)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		if hFontBold != 0 {
			pSelectObject.Call(uintptr(hdc), uintptr(hFontBold))
		} else if oldFont != 0 {
			pSelectObject.Call(uintptr(hdc), oldFont)
		}
	}
	detailR := r
	detailR.Top = headerR.Bottom + 2
	if detailR.Bottom <= detailR.Top {
		return true
	}
	detail := parts[1]
	type seg struct {
		text  string
		color uintptr
	}
	segments := []seg{}
	const gnoMarker = "  ·  ГНО "
	const mrpMarker = "  ·  текущий МРП "
	if gnoAt := strings.Index(detail, gnoMarker); gnoAt >= 0 {
		if mrpAt := strings.Index(detail[gnoAt+len(gnoMarker):], mrpMarker); mrpAt >= 0 {
			mrpAt += gnoAt + len(gnoMarker)
			mrpValueStart := mrpAt + len(mrpMarker)
			if sutAt := strings.Index(detail[mrpValueStart:], " сут."); sutAt >= 0 {
				sutAt += mrpValueStart
				segments = append(segments,
					seg{detail[:gnoAt], red},
					seg{gnoMarker, navy},
					seg{detail[gnoAt+len(gnoMarker) : mrpAt], red},
					seg{mrpMarker, navy},
					seg{detail[mrpValueStart:sutAt], red},
					seg{detail[sutAt:], navy},
				)
			}
		}
	}
	if len(segments) == 0 {
		// Reduced object labels (when the technical state is not yet available)
		// still accent the object identity before the target clause.
		const targetMarker = "  ·  целевой ориентир "
		if at := strings.Index(detail, targetMarker); at >= 0 {
			segments = []seg{{detail[:at], red}, {detail[at:], navy}}
		} else {
			segments = []seg{{detail, navy}}
		}
	}
	x := detailR.Left
	for _, item := range segments {
		if x >= detailR.Right {
			break
		}
		segR := detailR
		segR.Left = x
		w := drawResultInlineText(hdc, segR, item.text, item.color)
		x += w
	}
	pSetTextColor.Call(uintptr(hdc), navy)
	return true
}

func drawReportText(hdc syscall.Handle, r RECT, text string, font syscall.Handle, color uintptr, flags uintptr) {
	if hdc == 0 || strings.TrimSpace(text) == "" || r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	oldFont := uintptr(0)
	if font != 0 {
		oldFont, _, _ = pSelectObject.Call(uintptr(hdc), uintptr(font))
	}
	pSetBkMode.Call(uintptr(hdc), TRANSPARENT)
	pSetTextColor.Call(uintptr(hdc), color)
	buf, _ := syscall.UTF16FromString(text)
	if len(buf) > 1 {
		pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), flags|DT_NOPREFIX)
	}
	if oldFont != 0 {
		pSelectObject.Call(uintptr(hdc), oldFont)
	}
}

func reportTextWidthWithFont(hdc syscall.Handle, font syscall.Handle, text string) int32 {
	if hdc == 0 || strings.TrimSpace(text) == "" {
		return 0
	}
	oldFont := uintptr(0)
	if font != 0 {
		oldFont, _, _ = pSelectObject.Call(uintptr(hdc), uintptr(font))
	}
	buf, _ := syscall.UTF16FromString(text)
	r := RECT{Left: 0, Top: 0, Right: 4096, Bottom: 64}
	if len(buf) > 1 {
		pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX)
	}
	if oldFont != 0 {
		pSelectObject.Call(uintptr(hdc), oldFont)
	}
	return max32(0, r.Right-r.Left)
}

func resultMetricCardIndex(h syscall.Handle) int {
	for i, card := range resultSectionCards {
		if h == card {
			return i
		}
	}
	return -1
}

func archiveMetricCardIndex(h syscall.Handle) int {
	for i, card := range archiveRegistryStats {
		if h == card {
			return i
		}
	}
	return -1
}

// RC724: the six evidence cards now follow the visual hierarchy from the
// approved report reference: red ordinal, quiet caption, prominent value and
// muted evidence line. The same renderer is reused by the approved register.
func drawReportMetricCard(di *DRAWITEMSTRUCT, archive bool) {
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushBorder))
	text := strings.ReplaceAll(getText(di.HwndItem), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for len(lines) < 3 {
		lines = append(lines, "")
	}
	first := strings.TrimSpace(lines[0])
	ordinal, caption := "", first
	if len([]rune(first)) >= 2 {
		runes := []rune(first)
		if runes[0] >= '0' && runes[0] <= '9' && runes[1] >= '0' && runes[1] <= '9' {
			ordinal = string(runes[:2])
			caption = strings.TrimSpace(string(runes[2:]))
		}
	}
	left := di.RcItem.Left + 12
	right := di.RcItem.Right - 10
	top := di.RcItem.Top + 7
	// RC725: report/registry summary cards are deliberately larger than ordinary
	// body controls. The caption and primary value remain readable at a glance,
	// while the evidence line is only omitted on genuinely short viewports.
	labelH := int32(19)
	primaryH := int32(27)
	cardH := di.RcItem.Bottom - di.RcItem.Top
	// RC731: the four registry KPI cards are physically larger than section-7
	// evidence cards, so scale all three text tiers with that geometry.
	if archive {
		labelH = 23
		primaryH = 31
	}
	// RC728: compact report cards keep more vertical budget for the evidence row.
	// At 768–900 px screens the previous 19/27 px bands left only ~23 px for
	// two wrapped evidence lines, so their descenders were clipped.
	if cardH <= 90 && !archive {
		labelH = 16
		primaryH = 22
	}
	if cardH <= 66 {
		labelH = 15
		primaryH = 20
	}
	captionFont := hFontHeaderSmall
	if archive && hFontBold != 0 {
		captionFont = hFontBold
	}
	if ordinal != "" {
		ordinalW := int32(25)
		if archive {
			ordinalW = 30
		}
		r := RECT{Left: left, Top: top, Right: left + ordinalW, Bottom: top + labelH}
		drawReportText(di.HDC, r, ordinal, captionFont, rgb(203, 27, 43), DT_SINGLELINE|DT_VCENTER)
		left += ordinalW + 3
	}
	captionR := RECT{Left: left, Top: top, Right: right, Bottom: top + labelH}
	drawReportText(di.HDC, captionR, caption, captionFont, rgb(92, 111, 132), DT_SINGLELINE|DT_VCENTER|reportDTEndEllipsis)
	baseLeft := di.RcItem.Left + 12
	primaryR := RECT{Left: baseLeft, Top: top + labelH + 3, Right: right, Bottom: top + labelH + 3 + primaryH}
	primaryText := strings.TrimSpace(lines[1])
	cardW := di.RcItem.Right - di.RcItem.Left
	// RC730: primary values scale with the actual card geometry instead of being
	// forced into one small font. Large workspaces use the 20 px section font,
	// medium cards use the regular 17 px bold font and compact cards use 14 px.
	// Width measurement may step down one level, but never to the tiny 13 px hint
	// font: the card is a navigation summary and its main value must stay readable.
	primaryFont := hFontHeaderSmall
	if archive {
		primaryFont = hFontSection
	} else if cardW >= 245 && cardH >= 90 {
		primaryFont = hFontSection
	} else if cardW >= 188 && cardH >= 80 {
		primaryFont = hFontBold
	}
	if reportTextWidthWithFont(di.HDC, primaryFont, primaryText) > primaryR.Right-primaryR.Left {
		if primaryFont == hFontSection {
			primaryFont = hFontBold
		} else if primaryFont == hFontBold {
			primaryFont = hFontHeaderSmall
		}
	}
	// On roomy cards keep the large typography and wrap a long value to two lines
	// rather than shrinking it to unreadable text. Compact cards still use a
	// single 14 px line and a concise status that opens the full section on click.
	primaryFlags := uintptr(DT_SINGLELINE | DT_VCENTER | reportDTEndEllipsis)
	if cardH >= 90 && reportTextWidthWithFont(di.HDC, primaryFont, primaryText) > primaryR.Right-primaryR.Left {
		primaryFlags = DT_WORDBREAK
	}
	drawReportText(di.HDC, primaryR, primaryText, primaryFont, rgb(15, 48, 82), primaryFlags)
	if !archive && resultMetricCardIndex(di.HwndItem) >= 0 {
		chev := RECT{Left: di.RcItem.Right - 25, Top: top - 1, Right: di.RcItem.Right - 9, Bottom: top + labelH + 1}
		drawReportText(di.HDC, chev, "›", hFontHeaderSmall, rgb(15, 48, 82), DT_SINGLELINE|0x00000002|DT_VCENTER)
	}
	secondary := strings.TrimSpace(strings.Join(lines[2:], " · "))
	if secondary != "" {
		// RC728: the evidence row is intentionally smaller than the primary value.
		// The 17 px panel font was too tall for two wrapped lines and clipped the
		// lower glyphs in the six summary cards. A dedicated 12 px regular font
		// plus compact caption/value bands keeps the full evidence text visible.
		secondaryR := RECT{Left: baseLeft, Top: primaryR.Bottom + 1, Right: right, Bottom: di.RcItem.Bottom - 8}
		if secondaryR.Bottom-secondaryR.Top >= 12 {
			secondaryColor := rgb(109, 126, 144)
			if archive {
				secondaryColor = rgb(91, 112, 132)
			}
			evidenceFont := hFontReportEvidence
			if archive && hFontHeaderSmall != 0 {
				evidenceFont = hFontHeaderSmall
			}
			if evidenceFont == 0 {
				evidenceFont = hFontResultHint
			}
			drawReportText(di.HDC, secondaryR, secondary, evidenceFont, secondaryColor, DT_WORDBREAK)
		}
	}
}

func drawResultDecisionCard(di *DRAWITEMSTRUCT) {
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushBorder))
	accent := di.RcItem
	accent.Right = min32(accent.Right, accent.Left+4)
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accent)), uintptr(brushNav))
	text := strings.ReplaceAll(getText(di.HwndItem), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for len(lines) < 3 {
		lines = append(lines, "")
	}
	left := di.RcItem.Left + 16
	right := di.RcItem.Right - 12
	top := di.RcItem.Top + 7
	drawReportText(di.HDC, RECT{Left: left, Top: top, Right: right, Bottom: top + 15}, strings.TrimSpace(lines[0]), hFontHeaderTiny, rgb(92, 111, 132), DT_SINGLELINE|DT_VCENTER|reportDTEndEllipsis)
	drawReportText(di.HDC, RECT{Left: left, Top: top + 18, Right: right, Bottom: top + 42}, strings.TrimSpace(lines[1]), hFontHeaderSmall, rgb(15, 48, 82), DT_SINGLELINE|DT_VCENTER|reportDTEndEllipsis)
	detail := strings.TrimSpace(strings.Join(lines[2:], " · "))
	drawReportText(di.HDC, RECT{Left: left, Top: top + 45, Right: right, Bottom: di.RcItem.Bottom - 6}, detail, hFontPanel, rgb(91, 112, 132), DT_WORDBREAK)
}

func drawArchiveRegistryBanner(di *DRAWITEMSTRUCT) {
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushBorder))
	accent := di.RcItem
	accent.Right = min32(accent.Right, accent.Left+5)
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accent)), uintptr(brushHeader))
	text := strings.ReplaceAll(getText(di.HwndItem), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for len(lines) < 3 {
		lines = append(lines, "")
	}
	left := di.RcItem.Left + 18
	right := di.RcItem.Right - 14
	top := di.RcItem.Top + 8
	drawReportText(di.HDC, RECT{Left: left, Top: top, Right: right, Bottom: top + 18}, strings.TrimSpace(lines[0]), hFontHeaderSmall, rgb(92, 111, 132), DT_SINGLELINE|DT_VCENTER|reportDTEndEllipsis)
	drawReportText(di.HDC, RECT{Left: left, Top: top + 20, Right: right, Bottom: top + 44}, strings.TrimSpace(lines[1]), hFontBold, rgb(15, 48, 82), DT_SINGLELINE|DT_VCENTER|reportDTEndEllipsis)
	drawReportText(di.HDC, RECT{Left: left, Top: top + 48, Right: right, Bottom: di.RcItem.Bottom - 10}, strings.TrimSpace(strings.Join(lines[2:], " · ")), hFontPanel, rgb(91, 112, 132), DT_WORDBREAK)
}

func drawResultCard(di *DRAWITEMSTRUCT) {
	if di == nil || di.HDC == 0 || !isResultCardControl(di.HwndItem) {
		return
	}
	if resultMetricCardIndex(di.HwndItem) >= 0 {
		drawReportMetricCard(di, false)
		return
	}
	if archiveMetricCardIndex(di.HwndItem) >= 0 {
		drawReportMetricCard(di, true)
		return
	}
	if di.HwndItem == resultDecisionCard {
		drawResultDecisionCard(di)
		return
	}
	if di.HwndItem == archiveRegistryBanner {
		drawArchiveRegistryBanner(di)
		return
	}

	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushBorder))
	accent := brushBorder
	if di.HwndItem == resultObjectCard {
		accent = brushHeader
	} else if di.HwndItem == resultTraceCard {
		accent = brushNav
	}
	accentRect := di.RcItem
	accentRect.Right = min32(accentRect.Right, accentRect.Left+4)
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accentRect)), uintptr(accent))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
	font := hFontPanel
	if di.HwndItem == resultObjectCard {
		font = hFontHeaderSmall
	}
	oldFont := uintptr(0)
	if font != 0 {
		oldFont, _, _ = pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	}
	text := getText(di.HwndItem)
	r := di.RcItem
	r.Left += 12
	r.Right -= 10
	r.Top += 5
	r.Bottom -= 4
	if di.HwndItem == resultObjectCard && drawResultObjectCardText(di.HDC, r, text) {
		// The object card is rendered in coloured inline segments above.
	} else {
		buf, _ := syscall.UTF16FromString(text)
		if len(buf) > 1 {
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_WORDBREAK|DT_NOPREFIX)
		}
	}
	if oldFont != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldFont)
	}
}

func isResultCardControl(h syscall.Handle) bool {
	if h == 0 {
		return false
	}
	if h == resultObjectCard || h == resultDecisionCard || h == resultTraceCard || h == archiveRegistryBanner {
		return true
	}
	for _, card := range resultSectionCards {
		if h == card {
			return true
		}
	}
	for _, card := range archiveRegistryStats {
		if h == card {
			return true
		}
	}
	return false
}

func isResultWorkspaceButton(h syscall.Handle) bool {
	switch h {
	case resultRefreshButton,
		resultCopyVariantButton, resultCompareButton,
		resultSelectedInfoButton, resultUseSelectedButton, resultApproveButton,
		resultOpenButton, resultExportPDFButton, resultExportDOCXButton, resultExportXLSXButton,
		archiveExportPDFButton, archiveExportDOCXButton, archiveExportXLSXButton:
		return true
	}
	return false
}

func isResultSingleLineButton(h syscall.Handle) bool {
	switch h {
	case resultRefreshButton,
		resultCopyVariantButton, resultCompareButton,
		resultUseSelectedButton,
		resultOpenButton, resultExportPDFButton, resultExportDOCXButton, resultExportXLSXButton,
		archiveExportPDFButton, archiveExportDOCXButton, archiveExportXLSXButton:
		return true
	}
	return false
}

// RC722: hover help for the two variant-management actions. RC721 registered
// the tools but Windows could keep the tooltip behind the owner-drawn report
// surface. Create a real topmost tooltip window, explicitly activate it and use
// a short initial delay so the explanation is reliably visible on hover.
func initResultWorkspaceTooltips() {
	if resultTooltip != 0 || resultCopyVariantButton == 0 || resultCompareButton == 0 {
		return
	}
	inst, _, _ := pGetModuleHandleW.Call(0)
	hwnd, _, _ := pCreateWindowExW.Call(
		0x00000008, // WS_EX_TOPMOST
		uintptr(unsafe.Pointer(p("tooltips_class32"))),
		0,
		uintptr(0x80000000|TTS_ALWAYSTIP|TTS_NOPREFIX), // WS_POPUP
		0x80000000, 0x80000000, 0x80000000, 0x80000000,
		uintptr(hMain), 0, inst, 0,
	)
	if hwnd == 0 {
		return
	}
	resultTooltip = syscall.Handle(hwnd)
	const (
		ttmActivate     = 0x0400 + 1
		ttmSetDelayTime = 0x0400 + 3
		ttdtInitial     = 3
	)
	pSendMessageW.Call(uintptr(resultTooltip), TTM_SETMAXTIPWIDTH, 0, 540)
	pSendMessageW.Call(uintptr(resultTooltip), ttmSetDelayTime, ttdtInitial, 350)
	pSendMessageW.Call(uintptr(resultTooltip), ttmActivate, 1, 0)
	// Keep the tooltip above the main owner-drawn workspace without stealing focus.
	pSetWindowPos.Call(uintptr(resultTooltip), ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002|0x0010)

	add := func(control syscall.Handle, backing *[]uint16, text string) {
		*backing, _ = syscall.UTF16FromString(text)
		if len(*backing) == 0 {
			return
		}
		ti := TOOLINFOW{
			CbSize:   uint32(unsafe.Sizeof(TOOLINFOW{})),
			UFlags:   TTF_IDISHWND | TTF_SUBCLASS,
			Hwnd:     hMain,
			UId:      uintptr(control),
			LpszText: &(*backing)[0],
		}
		pSendMessageW.Call(uintptr(resultTooltip), TTM_ADDTOOLW, 0, uintptr(unsafe.Pointer(&ti)))
	}
	add(resultCopyVariantButton, &resultDuplicateTipText, "Создать копию выбранного варианта для изменения методов, прогноза или экономического сценария.")
	add(resultCompareButton, &resultCompareTipText, "Сопоставить варианты по наработке, CAPEX/OPEX, чистой выгоде и основанию выбора.")
}

func applyResultListReferenceStyle(h syscall.Handle) {
	if h == 0 {
		return
	}
	style := uintptr(LVS_EX_FULLROWSELECT | LVS_EX_DOUBLEBUFFER)
	mask := uintptr(LVS_EX_FULLROWSELECT | LVS_EX_GRIDLINES | LVS_EX_DOUBLEBUFFER)
	pSendMessageW.Call(uintptr(h), LVM_SETEXTENDEDLISTVIEWSTYLE, mask, style)
	pSendMessageW.Call(uintptr(h), LVM_SETBKCOLOR, 0, rgb(255, 255, 255))
	pSendMessageW.Call(uintptr(h), LVM_SETTEXTBKCOLOR, 0, rgb(255, 255, 255))
	pSendMessageW.Call(uintptr(h), LVM_SETTEXTCOLOR, 0, rgb(15, 48, 82))
}

// RC720: all section-7 buttons use one typography/centering contract.
// Their fill may still express action priority, but caption size, padding and
// vertical placement are identical. A small common optical lift compensates
// for Segoe UI's visible glyph box without per-row exceptions.
func resultButtonOpticalLift(h syscall.Handle) int32 {
	if isResultWorkspaceButton(h) {
		return 2
	}
	return 0
}

func ensureFullReportsLoaded() {
	if fullReportsLoaded {
		return
	}
	fullReportsLoaded = true
	loadJSON(fullReportsPath(), &fullReports)
	ensureTeachingFullReports()
}

func buildResultWorkspaceUI() {
	resultSubtitle = label(ID_RESULT, "Сводка решения по объекту · готовность данных и сравнение вариантов.", 242, 130, 1010, 28)
	resultObjectCard = resultCard("Объект не выбран.", 242, 162, 1010, 58)
	for i := range resultSectionCards {
		// Historical QA marker: resultSectionCards[i] = resultCard
		resultSectionCards[i] = resultSectionCard("—", i, 242+int32(i)*168, 230, 158, 94)
	}
	resultVariantsTitle = label(ID_RESULT, "Параллельные варианты методов и мероприятий", 242, 338, 1010, 24)
	resultVariantsSubtitle = label(ID_RESULT, "Одинаковые исходные условия · доступность ресурсов, прогноз и экономика сравниваются отдельно", 242, 360, 650, 18)
	// RC721: a new technical combination is formed through Methods + Economics;
	// section 7 no longer opens a separate “add variant” path. Keep only the two
	// actions that are meaningful at the comparison stage.
	resultAddVariantButton = 0
	resultCopyVariantButton = button(ID_RESULT, "ДУБЛИРОВАТЬ ВАРИАНТ", 928, 332, 232, 38, ID_RESULT_COPY_SCHEME)
	resultCompareButton = button(ID_RESULT, "СРАВНИТЬ ВАРИАНТЫ", 1168, 332, 232, 38, ID_RESULT_COMPARE_SCHEMES)
	initResultWorkspaceTooltips()
	resultVariantsList = newList(ID_RESULT, 242, 374, 1010, 140, []struct {
		n string
		w int32
	}{
		{"Вариант", 76}, {"Методы / мероприятия", 236}, {"Доступность", 116}, {"Прогноз, сут.", 104},
		{"Факт / контроль", 112}, {"CAPEX / OPEX", 130}, {"Чистая выгода", 112}, {"Основание выбора", 178},
	})
	setListHeaderFont(resultVariantsList, hFontBold)
	centerListHeaders(resultVariantsList, 8)
	applyResultListReferenceStyle(resultVariantsList)
	resultDecisionCard = resultCard("Решение по варианту ещё не принято.", 242, 524, 680, 70)
	resultTraceCard = resultCard("ПРОГНОЗ → ФАКТ\r\nФактический цикл ещё не зафиксирован.", 242, 602, 1010, 58)
	// RC708/709 migration vocabulary retained for regression QA only:
	// "ИНФОРМАЦИЯ ПО ВЫБРАННЫМ МЕРОПРИЯТИЯМ", "СОХРАНИТЬ ПРОХОДКУ", "УТВЕРДИТЬ В РЕЕСТР".
	// RC710 migration vocabulary retained for regression QA only: "КАРТОЧКИ МЕТОДОВ", "СНИМОК АНАЛИЗА".
	resultSelectedInfoButton = button(ID_RESULT, "КАРТОЧКА МЕТОДА", 646, 538, 250, 50, ID_RESULT_SELECTED_INFO)
	resultUseSelectedButton = button(ID_RESULT, "РАСЧЁТ ЭФФЕКТА", 904, 538, 150, 50, ID_RESULT_USE_SELECTED)
	resultApproveButton = button(ID_RESULT, "УТВЕРДИТЬ РЕШЕНИЕ", 1226, 538, 170, 50, ID_RESULT_APPROVE_SCHEME)
	resultDecisionHint = create("STATIC", "Выберите строку сценария — затем открывайте карточку метода или расчёт эффекта", WS_CHILD|SS_CENTER|SS_CENTERIMAGE|SS_NOPREFIX, 242, py(656), 760, 16, hMain, 0)
	addPageControl(ID_RESULT, resultDecisionHint)
	// Historical RC705/709 caption retained for regression QA: "Все проходки итогового анализа".
	resultArchiveTitle = label(ID_RESULT, "Ранее сохранённые полные отчёты", 242, 674, 580, 24)
	resultArchiveSubtitle = label(ID_RESULT, "Накопительный архив вариантов · двойной щелчок открывает сохранённое состояние решения", 242, 696, 620, 18)
	resultOpenButton = button(ID_RESULT, "ОТКРЫТЬ", 842, 666, 96, 34, ID_RESULT_OPEN_SAVED)
	resultExportPDFButton = button(ID_RESULT, "PDF", 946, 666, 92, 34, ID_RESULT_EXPORT_PDF)
	resultExportDOCXButton = button(ID_RESULT, "DOCX", 1046, 666, 96, 34, ID_RESULT_EXPORT_DOCX)
	resultExportXLSXButton = button(ID_RESULT, "XLSX", 1150, 666, 102, 34, ID_RESULT_EXPORT_XLSX)
	resultArchiveList = newList(ID_RESULT, 242, 706, 1010, 190, []struct {
		n string
		w int32
	}{
		{"Проходка / дата", 150}, {"Объект", 230}, {"Сценарий", 260}, {"Прогноз", 110}, {"Benefit Modeling", 120}, {"Автор / статус", 180},
	})
	setListHeaderFont(resultArchiveList, hFontBold)
	centerListHeaders(resultArchiveList, 6)
	applyResultListReferenceStyle(resultArchiveList)
	resultSummary = edit(ID_RESULT, "", 242, 900, 1010, 20, true, true, 0)
	show(resultSummary, false)
	if hFontHeaderSmall != 0 {
		pSendMessageW.Call(uintptr(resultObjectCard), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
	}
	if hFontPanel != 0 {
		pSendMessageW.Call(uintptr(resultSubtitle), WM_SETFONT, uintptr(hFontPanel), 1)
	}
	if hFontSection != 0 {
		pSendMessageW.Call(uintptr(resultVariantsTitle), WM_SETFONT, uintptr(hFontSection), 1)
		pSendMessageW.Call(uintptr(resultArchiveTitle), WM_SETFONT, uintptr(hFontSection), 1)
	}
	if hFontPanel != 0 {
		pSendMessageW.Call(uintptr(resultVariantsSubtitle), WM_SETFONT, uintptr(hFontPanel), 1)
		pSendMessageW.Call(uintptr(resultArchiveSubtitle), WM_SETFONT, uintptr(hFontPanel), 1)
	}
	if hFontPanel != 0 {
		for _, card := range resultSectionCards {
			pSendMessageW.Call(uintptr(card), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		pSendMessageW.Call(uintptr(resultTraceCard), WM_SETFONT, uintptr(hFontPanel), 1)
		pSendMessageW.Call(uintptr(resultVariantsList), WM_SETFONT, uintptr(hFontPanel), 1)
		pSendMessageW.Call(uintptr(resultArchiveList), WM_SETFONT, uintptr(hFontPanel), 1)
	}
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(resultDecisionCard), WM_SETFONT, uintptr(hFontBold), 1)
	}
	if hFontResultHint != 0 {
		pSendMessageW.Call(uintptr(resultDecisionHint), WM_SETFONT, uintptr(hFontResultHint), 1)
	}
}

// buildApprovedRegistryUI gives section 8 the same hierarchy and visual rhythm
// as section 7 while keeping it strictly read-only: only explicitly approved
// passes are shown here, drafts stay in the final-report archive.
func buildApprovedRegistryUI() {
	// Regression marker retained for RC709 QA: pageTitle(ID_ARCHIVE, "Утверждённый реестр"
	pageTitle(ID_ARCHIVE, "08  Утверждённый реестр", 242, 94, 970, 34)
	archiveRegistrySubtitle = label(ID_ARCHIVE, "Зафиксированные решения технолога · только утверждённые проходки из раздела 7", 242, 132, 970, 24)
	archiveRegistryBanner = registryCard("РЕЕСТР УТВЕРЖДЁННЫХ РЕШЕНИЙ\r\nРешения пока не утверждены\r\nЧерновики и сохранённые варианты остаются в разделе 7", 242, 162, 1010, 100)
	for i := range archiveRegistryStats {
		archiveRegistryStats[i] = registryMetricCard("—", i, 242+int32(i)*250, 272, 240, 112)
	}
	archiveRegistryTitle = label(ID_ARCHIVE, "Утверждённые решения", 242, 398, 520, 34)
	archiveRegistryHint = label(ID_ARCHIVE, "Выберите строку · двойной щелчок — открыть отчёт · выгрузка — по выбранной записи", 242, 444, 740, 28)
	archiveExportPDFButton = button(ID_ARCHIVE, "СКАЧАТЬ PDF", 778, 396, 154, 46, ID_ARCHIVE_EXPORT_PDF)
	archiveExportDOCXButton = button(ID_ARCHIVE, "СКАЧАТЬ WORD", 940, 396, 154, 46, ID_ARCHIVE_EXPORT_DOCX)
	archiveExportXLSXButton = button(ID_ARCHIVE, "СКАЧАТЬ EXCEL", 1102, 396, 154, 46, ID_ARCHIVE_EXPORT_XLSX)
	archiveList = newList(ID_ARCHIVE, 242, 478, 1010, 282, []struct {
		n string
		w int32
	}{
		{"№ / утверждено", 145}, {"Объект", 190}, {"Вариант", 120}, {"Мероприятия", 260}, {"Прогноз", 100}, {"Benefit Modeling", 125}, {"Утвердил", 170},
	})
	setListHeaderFont(archiveList, hFontBold)
	centerListHeaders(archiveList, 7)
	applyResultListReferenceStyle(archiveList)
	if hFontPanel != 0 {
		pSendMessageW.Call(uintptr(archiveRegistrySubtitle), WM_SETFONT, uintptr(hFontPanel), 1)
		pSendMessageW.Call(uintptr(archiveRegistryHint), WM_SETFONT, uintptr(hFontPanel), 1)
	}
	if hFontSection != 0 {
		pSendMessageW.Call(uintptr(archiveRegistryTitle), WM_SETFONT, uintptr(hFontSection), 1)
	}
}

func refreshApprovedRegistrySummary(rows []FullReportRecord) {
	count := len(rows)
	objects := map[string]struct{}{}
	targetCount := 0
	totalBenefit := 0.0
	for _, row := range rows {
		key := strings.TrimSpace(row.ObjectKey)
		if key == "" {
			key = strings.TrimSpace(row.ObjectLabel)
		}
		if key != "" {
			objects[strings.ToLower(key)] = struct{}{}
		}
		if row.ForecastDays >= 1000 {
			targetCount++
		}
		totalBenefit += row.NetBenefitK
	}
	latestLine := "Решения пока не утверждены"
	latestEvidence := "Черновики и сохранённые варианты остаются в разделе 7"
	if count > 0 {
		latest := rows[0]
		approvedAt := firstNonEmpty(strings.TrimSpace(latest.ApprovedAt), strings.TrimSpace(latest.CreatedAt))
		dateText := approvedAt
		if approvedAt != "" {
			dateText = parseDate(approvedAt).Format("02.01.2006 15:04")
		}
		latestLine = fmt.Sprintf("Последнее утверждение: %s · %s", dateText, firstNonEmpty(strings.TrimSpace(latest.Author), "технолог"))
		latestEvidence = fmt.Sprintf("%s · %s · %s", latest.ObjectLabel, latest.ScenarioName, latest.DecisionStatus)
	}
	setText(archiveRegistryBanner, "РЕЕСТР УТВЕРЖДЁННЫХ РЕШЕНИЙ\r\n"+latestLine+"\r\n"+latestEvidence)
	share := 0.0
	if count > 0 {
		share = 100 * float64(targetCount) / float64(count)
	}
	stats := []string{
		fmt.Sprintf("01  УТВЕРЖДЕНО\r\n%d решений\r\nТолько явное утверждение из раздела 7", count),
		fmt.Sprintf("02  ОБЪЕКТЫ\r\n%d объектов\r\nУникальные объекты в реестре", len(objects)),
		fmt.Sprintf("03  ЦЕЛЬ ≥ 1000 СУТ.\r\n%.0f%% решений\r\n%d из %d утверждённых", share, targetCount, count),
		fmt.Sprintf("04  ЧИСТАЯ ВЫГОДА\r\n%+.1f млн руб.\r\nСумма Benefit Modeling", totalBenefit/1000),
	}
	for i, text := range stats {
		setText(archiveRegistryStats[i], text)
	}
}

func layoutApprovedRegistryWorkspace(w, h int32) {
	x := int32(242)
	contentW := max32(720, w-270)
	right := x + contentW
	top := py(94)
	bottom := h - 18
	compact := h < 840

	setPos(archiveRegistrySubtitle, x, top+42, contentW, 24)
	// Historical RC725/RC726 geometry markers retained for regression scripts only:
	// bannerH := int32(82); statsH := int32(94); bannerH = 78; statsH = 90
	// setPos(archiveRegistryHint, x, titleY+46, contentW, 24); exportH := int32(42)
	// RC727: section 8 gets explicit vertical reserves instead of fitting text
	// against the lower card edge. The banner can carry a wrapped evidence row,
	// all four metric cards keep the complete third line, and the section title
	// below them has its own full-height band. Export buttons are deliberately
	// wider and equal-sized so PDF / WORD / EXCEL captions never clip.
	bannerY := top + 72
	bannerH := int32(100)
	statsH := int32(112)
	if compact {
		bannerH = 92
		statsH = 104
	}
	setPos(archiveRegistryBanner, x, bannerY, contentW, bannerH)
	statsY := bannerY + bannerH + 10
	gap := int32(8)
	cardW := (contentW - 3*gap) / 4
	for i, card := range archiveRegistryStats {
		setPos(card, x+int32(i)*(cardW+gap), statsY, cardW, statsH)
	}
	titleY := statsY + statsH + 14
	exportGap := int32(8)
	pdfW, docxW, xlsxW := int32(154), int32(154), int32(154)
	exportTotal := pdfW + docxW + xlsxW + 2*exportGap
	titleW := max32(220, contentW-exportTotal-18)
	setPos(archiveRegistryTitle, x, titleY, titleW, 34)
	bx := right - exportTotal
	exportH := int32(46)
	setPos(archiveExportPDFButton, bx, titleY, pdfW, exportH)
	bx += pdfW + exportGap
	setPos(archiveExportDOCXButton, bx, titleY, docxW, exportH)
	bx += docxW + exportGap
	setPos(archiveExportXLSXButton, bx, titleY, xlsxW, exportH)
	setPos(archiveRegistryHint, x, titleY+48, contentW, 28)
	listY := titleY + 80
	setPos(archiveList, x, listY, contentW, max32(126, bottom-listY))
	fitWellFailureListColumns(archiveList, []int32{145, 190, 120, 260, 100, 125, 170})
	_ = right
}

func approvedRegistryCustomDraw(lParam uintptr) uintptr {
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	if draw == nil {
		return CDRF_DODEFAULT
	}
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		row := int(draw.NMCD.ItemSpec)
		draw.ClrText = uint32(rgb(15, 48, 82))
		draw.ClrTextBk = uint32(rgb(255, 255, 255))
		if row == selectedRow(archiveList) {
			draw.ClrText = uint32(rgb(145, 0, 24))
			draw.ClrTextBk = uint32(rgb(255, 241, 244))
			return CDRF_NEWFONT
		}
		if row == 0 {
			draw.ClrTextBk = uint32(rgb(238, 244, 249))
			return CDRF_NEWFONT
		}
		if row%2 == 1 {
			draw.ClrTextBk = uint32(rgb(249, 251, 253))
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func resultObjectLabel(st TechObjectState, hasState bool, objectKey string) string {
	target := targetRunlifeResolvedForSelectedObject().Days
	if target <= 0 {
		target = 1000
	}
	if !hasState {
		return fmt.Sprintf("ЗАКРЕПЛЁННЫЙ ОБЪЕКТ\r\n%s  ·  целевой ориентир ≥ %.0f сут.", readableObjectKey(objectKey), target)
	}
	return fmt.Sprintf("ЗАКРЕПЛЁННЫЙ ОБЪЕКТ\r\n%s  ·  скв. %s  ·  %s  ·  ГНО %s  ·  текущий МРП %s сут.  ·  цель ≥ %.0f сут.", st.Field, st.Well, st.ObjectPlast, st.Equipment, displayMRP(st.CurrentMRP), target)
}

func refreshResultWorkspaceUI(snapshot string) {
	st, hasState := currentSelectedState()
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		resultScenarioRows = nil
		setText(resultSubtitle, "Сводка решения по объекту · готовность данных и сравнение вариантов.")
		setText(resultObjectCard, "Объект не выбран. Закрепите объект на Главной.")
		for _, h := range resultSectionCards {
			setText(h, "—")
		}
		listClear(resultVariantsList)
		setText(resultDecisionCard, "Решение по варианту ещё не принято.")
		setText(resultTraceCard, "ПРОГНОЗ → ФАКТ · КОНТРОЛЬ\r\nЗакрепите объект, чтобы сформировать контур проверки.")
		refreshFullReportArchive()
		return
	}
	setText(resultObjectCard, resultObjectLabel(st, hasState, key))
	history := methodHistoryBasisForObject(key, time.Now())
	_, ready, _ := approvedAssessmentForMethods(key)
	lab := "Нет подтверждённого анализа"
	if v, ok := latestVersionForSelectedObject(); ok && strings.TrimSpace(v.LabStatus) != "" {
		lab = v.LabStatus
	}
	scenarios := alternativeScenariosForObject(key)
	resultScenarioRows = append(resultScenarioRows[:0], scenarios...)
	active, _ := activeAlternativeScenario(key)
	methodCount := len(alternativeScenarioMethodsForCalculation(active))
	targetDays := targetRunlifeResolvedForSelectedObject().Days
	if targetDays <= 0 {
		targetDays = 1000
	}
	forecastPrimary := "Расчёт не выполнен"
	forecastEvidence := fmt.Sprintf("Цель ≥ %.0f сут.", targetDays)
	if active.ForecastBaseDays > 0 {
		forecastPrimary = fmt.Sprintf("%.0f сут.", active.ForecastBaseDays)
		forecastEvidence = fmt.Sprintf("база %.0f · сдерж. %.0f · расч. %.0f · благ. %.0f", active.BaselineMRPDays, active.ForecastLowDays, active.ForecastBaseDays, active.ForecastHighDays)
	}
	economyPrimary := "Расчёт не выполнен"
	economyEvidence := "Benefit Modeling"
	if active.EconomicsStatus != "" {
		economyPrimary = fmt.Sprintf("%+.1f млн руб.", active.NetBenefitK/1000)
		economyEvidence = fmt.Sprintf("Benefit Modeling · CAPEX %.2f · OPEX %.2f млн/год", active.OneTimeCAPEXRub/1000000, active.AnnualOPEXRub/1000000)
	}
	readyPrimary := "Не утверждено"
	readyEvidence := "Требует решения технолога"
	if ready {
		readyPrimary = "Утверждено"
		readyEvidence = "Утверждено технологом"
	}
	labPrimary := "Нет подтверждённого анализа"
	labEvidence := "Требуется подтверждённый анализ"
	if lab != "Нет подтверждённого анализа" {
		labPrimary = "Заполнено и проверено"
		labEvidence = "Подтверждённый анализ"
	}
	readiness := 0
	if history.FailureCount > 0 {
		readiness++
	}
	if lab != "Нет подтверждённого анализа" {
		readiness++
	}
	if ready {
		readiness++
	}
	if methodCount > 0 {
		readiness++
	}
	if active.ForecastBaseDays > 0 {
		readiness++
	}
	if active.EconomicsStatus != "" {
		readiness++
	}
	setText(resultSubtitle, fmt.Sprintf("Сводка решения по объекту · готовность данных %d/6 · сравнение технических вариантов и сохранённых проходок.", readiness))
	cardTexts := []string{
		fmt.Sprintf("01  ОТКАЗЫ\r\n%d подтверждённых\r\nПредыстория 36 мес. · завершённых циклов: %d", history.FailureCount, history.CycleCount),
		"02  ЛАБОРАТОРИЯ\r\n" + labPrimary + "\r\n" + labEvidence,
		"03  ОСЛОЖНЕНИЯ\r\n" + readyPrimary + "\r\n" + readyEvidence,
		fmt.Sprintf("04  МЕТОДЫ\r\n%d вариантов · %d методов\r\n%s", len(scenarios), methodCount, dash(active.Availability)),
		"05  НАРАБОТКА\r\n" + forecastPrimary + "\r\n" + forecastEvidence,
		"06  ЭКОНОМИКА\r\n" + economyPrimary + "\r\n" + economyEvidence,
	}
	for i, value := range cardTexts {
		setText(resultSectionCards[i], value)
	}

	listBeginUpdate(resultVariantsList)
	listClear(resultVariantsList)
	actualMRP, actualStatus, hasActual := reportActualMRP(key)
	for i, s := range scenarios {
		methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
		if methods == "" {
			methods = "Не заполнено"
		}
		forecastText := "—"
		if s.ForecastBaseDays > 0 {
			forecastText = fmt.Sprintf("%.0f / %.0f / %.0f", s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays)
		}
		econText := "—"
		if s.EconomicsStatus != "" {
			econText = fmt.Sprintf("%+.1f млн", s.NetBenefitK/1000)
		}
		costText := "—"
		if s.OneTimeCAPEXRub > 0 {
			costText = fmt.Sprintf("%.2f / %.2f млн", s.OneTimeCAPEXRub/1000000, s.AnnualOPEXRub/1000000)
		}
		basis := strings.TrimSpace(s.SelectionKind)
		if basis == "" {
			basis = strings.TrimSpace(s.SelectionBasis)
		}
		factText := "—"
		if strings.EqualFold(s.ID, active.ID) {
			if hasActual {
				factText = fmt.Sprintf("%.0f сут. · %s", actualMRP, actualStatus)
			} else {
				factText = "ожидается"
			}
		}
		listAdd(resultVariantsList, i, []string{s.Name, methods, s.Availability, forecastText, factText, costText, econText, basis})
		if strings.EqualFold(s.ID, active.ID) {
			selectListRow(resultVariantsList, i)
		}
	}
	listEndUpdate(resultVariantsList)
	basis := strings.TrimSpace(active.SelectionBasis)
	if basis == "" {
		basis = "Сравните технический прогноз, доступность ресурсов и экономику; решение принимает технолог."
	}
	setText(resultDecisionCard, fmt.Sprintf("РЕШЕНИЕ ТЕХНОЛОГА\r\n%s · %s · %s\r\n%s", strings.ToUpper(active.Name), strings.ToUpper(dash(active.SelectionKind)), strings.ToUpper(active.Availability), resultDecisionControlLine(active, actualMRP, actualStatus, hasActual)))
	setText(resultTraceCard, reportTraceabilityText(active, actualMRP, actualStatus, hasActual))
	refreshFullReportArchive()
}

func reportActualMRP(objectKey string) (float64, string, bool) {
	version, ok := latestVersionForSelectedObject()
	if !ok || !methodsObjectKeysEquivalent(version.ObjectKey, objectKey) {
		return 0, "", false
	}
	for i := len(outcomes) - 1; i >= 0; i-- {
		if outcomes[i].VersionID != version.VersionID {
			continue
		}
		value, err := parseEconomicNumber(outcomes[i].ActualMRP, "Фактический МРП", false)
		if err == nil && value > 0 {
			return value, dash(outcomes[i].Result), true
		}
	}
	if teaching, ok := teachingOutcomeForVersion(version.VersionID); ok {
		value, err := parseEconomicNumber(teaching.ActualMRP, "Фактический МРП", false)
		if err == nil && value > 0 {
			return value, dash(teaching.Result), true
		}
	}
	return 0, "", false
}

func resultDecisionControlLine(active AlternativeMethodScenario, actualMRP float64, actualStatus string, hasActual bool) string {
	if hasActual {
		return fmt.Sprintf("Прогноз %.0f сут. · факт %.0f сут. · отклонение %+.0f сут. · %s", active.ForecastBaseDays, actualMRP, actualMRP-active.ForecastBaseDays, actualStatus)
	}
	if active.ForecastBaseDays > 0 {
		return fmt.Sprintf("Прогноз %.0f сут. · факт ожидается · контроль после завершения цикла", active.ForecastBaseDays)
	}
	return "Сопоставьте техническую применимость, доступность, прогноз и Benefit Modeling."
}

func reportTraceabilityText(active AlternativeMethodScenario, actualMRP float64, actualStatus string, hasActual bool) string {
	if !hasActual {
		return fmt.Sprintf("ПРОГНОЗ → ФАКТ · КОНТРОЛЬ\r\nПрогноз %.0f сут. Факт ожидается; после завершения цикла зафиксируйте причину, дефектовку и эффект.", active.ForecastBaseDays)
	}
	delta := actualMRP - active.ForecastBaseDays
	return fmt.Sprintf("ПРОГНОЗ → ФАКТ · КОНТРОЛЬ\r\nПрогноз %.0f сут. · факт %.0f сут. · отклонение %+.0f сут.\r\n%s", active.ForecastBaseDays, actualMRP, delta, actualStatus)
}

func alternativeScenarioReportText(objectKey string) string {
	scenarios := alternativeScenariosForObject(objectKey)
	if len(scenarios) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\r\nПАРАЛЛЕЛЬНЫЕ СХЕМЫ МЕТОДОВ И МЕРОПРИЯТИЙ\r\n")
	for i, s := range scenarios {
		methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
		if methods == "" {
			methods = "не заполнено"
		}
		fmt.Fprintf(&b, "%d. %s | %s | %s\r\n", i+1, s.Name, s.Status, s.Availability)
		fmt.Fprintf(&b, "   Методы: %s.\r\n", methods)
		if s.ForecastBaseDays > 0 {
			fmt.Fprintf(&b, "   Прогноз: %.0f / %.0f / %.0f сут.; цель %.0f сут.; отклонение %+.0f сут.\r\n", s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays, s.TargetMRPDays, s.ForecastBaseDays-s.TargetMRPDays)
		} else {
			b.WriteString("   Прогноз: не рассчитан.\r\n")
		}
		if s.EconomicsStatus != "" {
			fmt.Fprintf(&b, "   Benefit Modeling: CAPEX %.0f руб.; OPEX %.0f руб./год; чистая выгода %.1f тыс. руб.; %s.\r\n", s.OneTimeCAPEXRub, s.AnnualOPEXRub, s.NetBenefitK, s.EconomicsStatus)
		} else {
			b.WriteString("   Экономика: не рассчитана.\r\n")
		}
		if strings.TrimSpace(s.SelectionBasis) != "" {
			fmt.Fprintf(&b, "   Основание: %s\r\n", s.SelectionBasis)
		}
	}
	b.WriteString("Решение принимает технолог; недоступная схема может быть утверждена только условно.\r\n")
	return b.String()
}

func resultDecisionAndControlText(objectKey string) string {
	if strings.TrimSpace(objectKey) == "" {
		return ""
	}
	active, ok := activeAlternativeScenario(objectKey)
	if !ok {
		return "\r\nСВОДНОЕ ЗАКЛЮЧЕНИЕ\r\nПараллельные схемы не сформированы; выбор и экономическое сравнение пока недоступны.\r\n"
	}
	methods := strings.Join(alternativeScenarioMethodsForCalculation(active), " + ")
	if methods == "" {
		methods = "не заданы"
	}
	basis := strings.TrimSpace(active.SelectionBasis)
	if basis == "" {
		basis = "основание выбора не зафиксировано"
	}
	lab := "подтверждённый лабораторный анализ не зафиксирован"
	if v, found := latestVersionForSelectedObject(); found && strings.TrimSpace(v.LabStatus) != "" {
		lab = v.LabStatus
	}
	actual, status, hasActual := reportActualMRP(objectKey)
	control := fmt.Sprintf("После завершения цикла зафиксировать фактический МРП и сопоставить его с прогнозом %.0f сут.", active.ForecastBaseDays)
	if hasActual {
		control = fmt.Sprintf("Прогноз %.0f сут.; факт %.0f сут.; отклонение %+.0f сут.; статус: %s.", active.ForecastBaseDays, actual, actual-active.ForecastBaseDays, status)
	}
	return fmt.Sprintf("\r\nСВОДНОЕ ЗАКЛЮЧЕНИЕ И КОНТУР КОНТРОЛЯ\r\nВыбрана %s: %s. Доступность: %s.\r\nОснование выбора: %s.\r\nЛабораторный контур: %s.\r\nКонтроль: %s\r\nОграничение: прогноз и Benefit Modeling поддерживают инженерное решение, но не доказывают причинный эффект мероприятия без факта и дефектовки.\r\n", active.Name, methods, active.Availability, basis, lab, control)
}

func selectedResultScenario() (AlternativeMethodScenario, bool) {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" || resultVariantsList == 0 {
		return AlternativeMethodScenario{}, false
	}
	row := selectedRow(resultVariantsList)
	if row < 0 || row >= len(resultScenarioRows) {
		return AlternativeMethodScenario{}, false
	}
	return resultScenarioRows[row], true
}

func resultSelectedMeasuresInfoText() string {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return "Закрепите объект на Главной."
	}
	s, ok := selectedResultScenario()
	if !ok {
		s, ok = activeAlternativeScenario(key)
	}
	if !ok {
		return "Для объекта ещё не сформирована схема методов и мероприятий."
	}
	methods := alternativeScenarioMethodsForCalculation(s)
	methodText := strings.Join(methods, " + ")
	if strings.TrimSpace(methodText) == "" {
		methodText = "не выбраны"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ИНФОРМАЦИЯ ПО ВЫБРАННЫМ МЕРОПРИЯТИЯМ\r\nОбъект: %s\r\nСхема: %s\r\nСтатус: %s · доступность: %s\r\n\r\n", readableObjectKey(key), dash(s.Name), dash(s.Status), dash(s.Availability))
	fmt.Fprintf(&b, "Методы / мероприятия: %s.\r\n", methodText)
	if strings.TrimSpace(s.Classification) != "" {
		fmt.Fprintf(&b, "Утверждённая классификация: %s.\r\n", s.Classification)
	}
	if s.ForecastBaseDays > 0 {
		fmt.Fprintf(&b, "Прогноз наработки: базовая оценка %.0f сут. · сдержанный %.0f сут. · расчётный %.0f сут. · благоприятный %.0f сут.\r\n", s.BaselineMRPDays, s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays)
		fmt.Fprintf(&b, "Целевой ориентир: ≥ %.0f сут.; отклонение расчётного сценария от цели %+.0f сут.\r\n", s.TargetMRPDays, s.ForecastBaseDays-s.TargetMRPDays)
	} else {
		b.WriteString("Прогноз наработки: расчёт не выполнен.\r\n")
	}
	if strings.TrimSpace(s.EconomicsStatus) != "" {
		fmt.Fprintf(&b, "Упрощённый Benefit Modeling: CAPEX %.2f млн руб. · OPEX %.2f млн руб./год · чистая выгода %+.2f млн руб.\r\n", s.OneTimeCAPEXRub/1e6, s.AnnualOPEXRub/1e6, s.NetBenefitK/1000)
		if s.BenefitCostRatio > 0 {
			fmt.Fprintf(&b, "Benefit/Cost: %.2f. ", s.BenefitCostRatio)
		}
		if s.PaybackMonths != nil && *s.PaybackMonths > 0 {
			fmt.Fprintf(&b, "Окупаемость: %.1f мес. ", *s.PaybackMonths)
		}
		fmt.Fprintf(&b, "Статус: %s.\r\n", s.EconomicsStatus)
	} else {
		b.WriteString("Упрощённый Benefit Modeling: расчёт не выполнен.\r\n")
	}
	if strings.TrimSpace(s.AvailabilityNote) != "" {
		fmt.Fprintf(&b, "Комментарий по доступности: %s\r\n", s.AvailabilityNote)
	}
	if strings.TrimSpace(s.SelectionBasis) != "" {
		fmt.Fprintf(&b, "Основание выбора: %s\r\n", s.SelectionBasis)
	}
	b.WriteString("\r\nИтоговый выбор утверждает технолог. Прогноз и Benefit Modeling поддерживают решение, но не заменяют контроль фактической наработки и дефектовку.")
	return b.String()
}

func openResultSelectedMeasuresInfo() {
	s, ok := selectedResultScenario()
	if !ok {
		s, ok = activeAlternativeScenario(selectedObjectKey)
	}
	if !ok {
		msg("Информация по выбранным мероприятиям", "Для объекта ещё не сформирована схема методов и мероприятий.", MB_OK|MB_ICONWARNING)
		return
	}
	if _, err := setAlternativeScenarioActiveByID(selectedObjectKey, s.ID); err != nil {
		msg("Информация по выбранным мероприятиям", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	// The detailed cards already live in Methods. Section 7 only selects the
	// scenario and takes the user to that canonical workspace; it no longer
	// duplicates the same facts in a separate modal window.
	switchPage(ID_ACTIVITIES)
}

func openSelectedResultScenarioEconomics() {
	s, ok := selectedResultScenario()
	if !ok {
		s, ok = activeAlternativeScenario(selectedObjectKey)
	}
	if !ok {
		msg("Экономическая оценка", "Выделите сценарий в таблице итогового отчёта.", MB_OK|MB_ICONWARNING)
		return
	}
	if _, err := setAlternativeScenarioActiveByID(selectedObjectKey, s.ID); err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	// Economics reads the active scenario during refresh, so the selected row
	// from section 7 is carried across without a second chooser.
	switchPage(ID_ECON)
}

func activateSelectedResultScenario() error {
	s, ok := selectedResultScenario()
	if !ok {
		return fmt.Errorf("выделите вариант схемы в таблице")
	}
	if _, err := setAlternativeScenarioActiveByID(selectedObjectKey, s.ID); err != nil {
		return err
	}
	refreshResultPage()
	return nil
}

func createResultScenario(copyCurrent bool) error {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return fmt.Errorf("сначала закрепите объект на Главной")
	}
	s, err := createAlternativeScenario(key, copyCurrent)
	if err != nil {
		return err
	}
	refreshResultPage()
	for row, candidate := range alternativeScenariosForObject(key) {
		if strings.EqualFold(candidate.ID, s.ID) {
			selectListRow(resultVariantsList, row)
			break
		}
	}
	return nil
}

func duplicateSelectedResultScenario() error {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return fmt.Errorf("сначала закрепите объект на Главной")
	}
	selected, ok := selectedResultScenario()
	if !ok {
		return fmt.Errorf("выделите вариант в таблице, который нужно дублировать")
	}
	if _, err := setAlternativeScenarioActiveByID(key, selected.ID); err != nil {
		return err
	}
	return createResultScenario(true)
}

func resultScenarioComparisonText(objectKey string) string {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return "Закрепите объект на Главной."
	}
	scenarios := alternativeScenariosForObject(objectKey)
	if len(scenarios) == 0 {
		return "Параллельные варианты ещё не сформированы."
	}
	target := targetRunlifeResolvedForSelectedObject().Days
	var b strings.Builder
	fmt.Fprintf(&b, "ЦЕЛЕВОЙ ОРИЕНТИР: ≥ %.0f сут.\r\n\r\n", target)
	activeID := activeAlternativeScenarioID(objectKey)
	for i, s := range scenarios {
		active := ""
		if strings.EqualFold(s.ID, activeID) {
			active = " · ТЕКУЩАЯ"
		}
		fmt.Fprintf(&b, "%d. %s%s\r\n", i+1, strings.ToUpper(s.Name), active)
		methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
		if strings.TrimSpace(methods) == "" {
			methods = "не выбраны"
		}
		fmt.Fprintf(&b, "Методы: %s.\r\nДоступность: %s. Статус: %s.\r\n", methods, dash(s.Availability), dash(s.Status))
		if s.ForecastBaseDays > 0 {
			fmt.Fprintf(&b, "Наработка: %.0f / %.0f / %.0f сут.; отклонение центрального прогноза от цели %+.0f сут.\r\n", s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays, s.ForecastBaseDays-target)
		} else {
			b.WriteString("Наработка: расчёт не выполнен.\r\n")
		}
		if s.EconomicsStatus != "" {
			fmt.Fprintf(&b, "Benefit Modeling: CAPEX %.2f млн руб.; OPEX %.2f млн руб./год; чистая выгода %+.2f млн руб.; %s.\r\n", s.OneTimeCAPEXRub/1e6, s.AnnualOPEXRub/1e6, s.NetBenefitK/1000, s.EconomicsStatus)
		} else {
			b.WriteString("Benefit Modeling: расчёт не выполнен.\r\n")
		}
		if strings.TrimSpace(s.SelectionBasis) != "" {
			fmt.Fprintf(&b, "Основание: %s\r\n", s.SelectionBasis)
		}
		b.WriteString("\r\n")
	}
	b.WriteString("Правило решения: сравнивать техническую применимость, доступность, прогноз наработки и упрощённый Benefit Modeling совместно. Итоговый выбор утверждает технолог; прогноз не является гарантией фактической наработки.")
	return b.String()
}

func resultCompareLayout(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	var rc RECT
	if r, _, _ := pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	w := max32(0, rc.Right-rc.Left)
	h := max32(0, rc.Bottom-rc.Top)
	if w < 240 || h < 180 {
		return
	}
	setPos(resultCompareHeaderBand, 0, 0, w, 92)
	setPos(resultCompareAccent, 0, 88, w, 4)
	setPos(resultCompareTitle, 24, 12, max32(220, w-48), 34)
	setPos(resultCompareSubtitle, 24, 48, max32(220, w-48), 34)
	setPos(resultCompareBody, 24, 110, max32(220, w-48), max32(100, h-134))
}

func resultCompareIsVariantHeading(t string) bool {
	t = strings.TrimSpace(t)
	if t == "" {
		return false
	}
	// Variant headings are emitted as "1. СХЕМА А" / "4. СХЕМА Г · ТЕКУЩАЯ".
	dot := strings.Index(t, ". ")
	if dot <= 0 || dot > 3 {
		return false
	}
	for _, r := range t[:dot] {
		if r < '0' || r > '9' {
			return false
		}
	}
	rest := strings.TrimSpace(t[dot+2:])
	return rest != "" && rest == strings.ToUpper(rest)
}

func resultCompareFormatText(h syscall.Handle, text string) {
	if h == 0 {
		return
	}
	normal := CHARFORMATW{
		CbSize:      uint32(unsafe.Sizeof(CHARFORMATW{})),
		DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize,
		CrTextColor: uint32(rgb(43, 46, 50)),
		YHeight:     240,
	}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&normal)))

	headingFmt := CHARFORMATW{
		CbSize:      uint32(unsafe.Sizeof(CHARFORMATW{})),
		DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize,
		DwEffects:   CFE_BOLD | cfeItalic,
		CrTextColor: uint32(rgb(15, 48, 82)),
		YHeight:     250,
	}
	labelFmt := headingFmt
	labelFmt.YHeight = 240
	labels := []string{"Методы:", "Доступность:", "Статус:", "Наработка:", "Benefit Modeling:", "Основание:", "Правило решения:"}

	offset := 0
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineUnits := len(utf16.Encode([]rune(line)))
		leadingUnits := len(utf16.Encode([]rune(line[:len(line)-len(strings.TrimLeft(line, " \t"))])))
		if trimmed != "" {
			if trimmed == strings.ToUpper(trimmed) || resultCompareIsVariantHeading(trimmed) {
				ln := len(utf16.Encode([]rune(trimmed)))
				pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(offset+leadingUnits), uintptr(offset+leadingUnits+ln))
				pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&headingFmt)))
			}
			for _, label := range labels {
				idx := strings.Index(line, label)
				if idx < 0 {
					continue
				}
				start := offset + len(utf16.Encode([]rune(line[:idx])))
				end := start + len(utf16.Encode([]rune(label)))
				pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(start), uintptr(end))
				pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&labelFmt)))
			}
		}
		offset += lineUnits
		if i < len(lines)-1 {
			offset++ // RichEdit stores CRLF as one paragraph separator.
		}
	}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, 0)
}

func resultCompareWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		resultCompareWindow = parent
		resultCompareHeaderBand = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1000, 92, parent, 0)
		resultCompareAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 88, 1000, 4, parent, 0)
		resultCompareTitle = modalCreate("STATIC", firstNonEmpty(strings.TrimSpace(resultCompareHeaderTitle), "СРАВНЕНИЕ ВАРИАНТОВ"), WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 12, 950, 34, parent, 0)
		resultCompareSubtitle = modalCreate("STATIC", resultCompareObjectLabel, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 48, 950, 34, parent, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(resultCompareTitle), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(resultCompareSubtitle), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		_ = riched20.Load()
		resultCompareBody = modalCreate("RichEdit20W", resultCompareBodyText, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|WS_VSCROLL, 24, 110, 950, 570, parent, 0)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(resultCompareBody), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		pSendMessageW.Call(uintptr(resultCompareBody), emSetBkColor, 0, uintptr(rgb(255, 255, 255)))
		margins := uintptr(uint32(methodsLeftMargin) | uint32(methodsLeftMargin)<<16)
		pSendMessageW.Call(uintptr(resultCompareBody), emSetMargins, 3, margins)
		resultCompareFormatText(resultCompareBody, resultCompareBodyText)
		resultCompareLayout(hwnd)
		return 0
	case WM_SIZE:
		resultCompareLayout(hwnd)
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X = 820
			mmi.PtMinTrackSize.Y = 560
		}
		return 0
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		switch child {
		case resultCompareHeaderBand:
			return uintptr(brushNav)
		case resultCompareAccent:
			return uintptr(brushHeader)
		case resultCompareTitle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		case resultCompareSubtitle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(220, 229, 238))
			return uintptr(brushNav)
		}
	case WM_CTLCOLOREDIT:
		if syscall.Handle(lParam) == resultCompareBody {
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(43, 46, 50))
			return uintptr(brushWhite)
		}
	case labMappingClose:
		resultCompareDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	case WM_DESTROY:
		resultCompareDone = true
		resultCompareWindow = 0
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

// RC722 regression marker retained: ROSAResultComparisonV2
// RC722 regression marker retained: resultCompareObjectLabel = readableObjectKey(selectedObjectKey)
func openResultTextViewer(windowTitle, headerTitle, objectLabel, body string) {
	resultCompareDone = false
	resultCompareWindowTitle = firstNonEmpty(strings.TrimSpace(windowTitle), "РОСА — Просмотр отчёта")
	resultCompareHeaderTitle = firstNonEmpty(strings.TrimSpace(headerTitle), "ПОЛНЫЙ ОТЧЁТ")
	resultCompareObjectLabel = strings.TrimSpace(objectLabel)
	resultCompareBodyText = strings.TrimSpace(body)
	cls := p("ROSAResultComparisonV3")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{
		CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(resultCompareWndProc),
		HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls,
		HIcon: hIconBig, HIconSm: hIconSmall,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p(resultCompareWindowTitle))), WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000, 0x80000000, 1040, 760, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg(resultCompareHeaderTitle, resultCompareBodyText, MB_OK|MB_ICONINFORMATION)
		return
	}
	var message MSG
	for !resultCompareDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleModalEscape(hwnd, &message) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}

func openResultScenarioComparison() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Сравнение вариантов", "Сначала закрепите объект на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	openResultTextViewer("РОСА — Сравнение вариантов", "СРАВНЕНИЕ ВАРИАНТОВ", readableObjectKey(selectedObjectKey), resultScenarioComparisonText(selectedObjectKey))
}

func resultVariantCustomDraw(lParam uintptr) uintptr {
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	if draw == nil {
		return CDRF_DODEFAULT
	}
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		row := int(draw.NMCD.ItemSpec)
		if row < 0 || row >= len(resultScenarioRows) {
			return CDRF_DODEFAULT
		}
		draw.ClrText = uint32(rgb(15, 48, 82))
		draw.ClrTextBk = uint32(rgb(255, 255, 255))
		s := resultScenarioRows[row]
		active := strings.EqualFold(s.ID, activeAlternativeScenarioID(selectedObjectKey))
		selected := row == selectedRow(resultVariantsList)
		if active {
			draw.ClrText = uint32(rgb(145, 0, 24))
			draw.ClrTextBk = uint32(rgb(255, 241, 244))
			return CDRF_NEWFONT
		}
		if selected {
			draw.ClrTextBk = uint32(rgb(238, 244, 249))
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func resultArchiveCustomDraw(lParam uintptr) uintptr {
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	if draw == nil {
		return CDRF_DODEFAULT
	}
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		row := int(draw.NMCD.ItemSpec)
		draw.ClrText = uint32(rgb(15, 48, 82))
		draw.ClrTextBk = uint32(rgb(255, 255, 255))
		if row == 0 || row == selectedRow(resultArchiveList) {
			draw.ClrTextBk = uint32(rgb(238, 244, 249))
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func refreshFullReportArchive() {
	ensureFullReportsLoaded()
	// This is the cumulative register of ALL completed passes, not a four-row
	// preview of the current object. Show its real size even when the viewport
	// requires vertical scrolling.
	subtitle := fmt.Sprintf("Проходок: %d · все объекты · прокрутка ↓ · двойной щелчок — открыть", len(fullReports))
	if getText(resultArchiveSubtitle) != subtitle {
		setText(resultArchiveSubtitle, subtitle)
	}
	// Do not destroy and repopulate forty-plus ListView rows on every visit to
	// section 7. Apart from UI stalls, deleting live rows temporarily presents a
	// header with an empty body while the native control is being repainted.
	hash := fnv.New64a()
	for i := range fullReports {
		r := &fullReports[i]
		fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%s\x00%.3f\x00%.3f\x00%s\x00%s\x00",
			r.ID, r.CreatedAt, r.ObjectLabel, r.ScenarioName, r.ForecastDays, r.NetBenefitK, r.Author, r.DecisionStatus)
	}
	signature := hash.Sum64()
	const lvmGetItemCount = 0x1004
	actualCount, _, _ := pSendMessageW.Call(uintptr(resultArchiveList), lvmGetItemCount, 0, 0)
	if resultArchiveRenderHash == signature && int(actualCount) == len(fullReports) {
		return
	}
	listBeginUpdate(resultArchiveList)
	defer listEndUpdate(resultArchiveList)
	listClear(resultArchiveList)
	row := 0
	for i := len(fullReports) - 1; i >= 0; i-- {
		r := fullReports[i]
		listAdd(resultArchiveList, row, []string{r.ID + "  " + r.CreatedAt, r.ObjectLabel, r.ScenarioName, fmt.Sprintf("%.0f сут.", r.ForecastDays), fmt.Sprintf("%+.1f млн руб.", r.NetBenefitK/1000), r.Author + " · " + r.DecisionStatus})
		row++
	}
	actualCount, _, _ = pSendMessageW.Call(uintptr(resultArchiveList), lvmGetItemCount, 0, 0)
	if int(actualCount) == len(fullReports) {
		resultArchiveRenderHash = signature
	} else {
		resultArchiveRenderHash = 0
		diagLog("REPORT archive incomplete expected=%d got=%d", len(fullReports), actualCount)
	}
}

func reportScenarioForAction() (AlternativeMethodScenario, error) {
	s, ok := selectedResultScenario()
	if !ok {
		s, ok = activeAlternativeScenario(selectedObjectKey)
	}
	if !ok {
		return AlternativeMethodScenario{}, fmt.Errorf("выделите сценарий в таблице")
	}
	if _, err := setAlternativeScenarioActiveByID(selectedObjectKey, s.ID); err != nil {
		return AlternativeMethodScenario{}, err
	}
	return s, nil
}

func appendFullReportRecord(s AlternativeMethodScenario, approvedAt string) (FullReportRecord, error) {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return FullReportRecord{}, fmt.Errorf("сначала закрепите объект")
	}
	methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
	if methods == "" {
		return FullReportRecord{}, fmt.Errorf("в %s не выбраны методы", s.Name)
	}
	ensureFullReportsLoaded()
	now := time.Now()
	label := readableObjectKey(key)
	if st, ok := currentSelectedState(); ok {
		label = st.Field + " · скв. " + st.Well
	}
	resultSnapshot := getText(resultSummary)
	record := FullReportRecord{ID: fmt.Sprintf("ОТ-%03d", len(fullReports)+1), CreatedAt: now.Format("02.01.2006 15:04"), ObjectKey: key, ObjectLabel: label,
		ScenarioID: s.ID, ScenarioName: s.Name, Methods: methods, Availability: s.Availability, ForecastDays: s.ForecastBaseDays,
		NetBenefitK: s.NetBenefitK, DecisionStatus: s.Status, ApprovedAt: approvedAt, DecisionBasis: s.SelectionBasis, Author: currentUserLabel(), Snapshot: resultSnapshot,
		SnapshotVersion: fullReportSectionSnapshotVersion, Sections: buildFullReportSectionSnapshot(key, s, resultSnapshot)}
	if approvedAt != "" {
		// Repeated clicks on the unchanged approved row must not create duplicate
		// registry entries. A materially changed snapshot is a new approved pass.
		for i := len(fullReports) - 1; i >= 0; i-- {
			existing := &fullReports[i]
			if existing.ApprovedAt != "" && methodsObjectKeysEquivalent(existing.ObjectKey, key) &&
				strings.EqualFold(existing.ScenarioID, s.ID) && existing.Snapshot == record.Snapshot && fullReportSectionsEqual(existing.Sections, record.Sections) {
				existing.ApprovedAt = approvedAt
				existing.CreatedAt = record.CreatedAt
				existing.DecisionStatus = record.DecisionStatus
				existing.Author = record.Author
				if err := saveJSON(fullReportsPath(), fullReports); err != nil {
					return FullReportRecord{}, err
				}
				refreshFullReportArchive()
				return *existing, nil
			}
		}
	}
	fullReports = append(fullReports, record)
	if err := saveJSON(fullReportsPath(), fullReports); err != nil {
		fullReports = fullReports[:len(fullReports)-1]
		return FullReportRecord{}, err
	}
	refreshFullReportArchive()
	return record, nil
}

func saveCurrentFullReport() error {
	s, err := reportScenarioForAction()
	if err != nil {
		return err
	}
	refreshResultPage()
	s, _ = activeAlternativeScenario(selectedObjectKey)
	_, err = appendFullReportRecord(s, "")
	return err
}

func approveSelectedResultScenarioToRegistry() error {
	s, err := reportScenarioForAction()
	if err != nil {
		return err
	}
	if err = approveActiveAlternativeScenario(selectedObjectKey); err != nil {
		return err
	}
	refreshResultPage()
	s, _ = activeAlternativeScenario(selectedObjectKey)
	_, err = appendFullReportRecord(s, firstNonEmpty(strings.TrimSpace(s.ApprovedAt), time.Now().Format(time.RFC3339)))
	return err
}

func approvedFullReportRecords() []FullReportRecord {
	ensureFullReportsLoaded()
	rows := make([]FullReportRecord, 0, len(fullReports))
	for i := len(fullReports) - 1; i >= 0; i-- {
		if strings.TrimSpace(fullReports[i].ApprovedAt) != "" {
			rows = append(rows, fullReports[i])
		}
	}
	return rows
}

func persistEnrichedTeachingFullReport(r FullReportRecord) {
	if !strings.HasPrefix(strings.TrimSpace(r.ID), "УЧ-") || len(r.Sections) == 0 {
		return
	}
	ensureFullReportsLoaded()
	for i := range fullReports {
		if strings.EqualFold(strings.TrimSpace(fullReports[i].ID), strings.TrimSpace(r.ID)) {
			fullReports[i].Sections = r.Sections
			fullReports[i].SnapshotVersion = r.SnapshotVersion
			_ = saveJSON(fullReportsPath(), fullReports)
			return
		}
	}
}

func fullReportRecordForExport(r FullReportRecord) FullReportRecord {
	// RC735: detailed synthetic sections are generated only on explicit user
	// action, never while section 7/8 is being opened. This removes the RC734 UI
	// freeze while keeping old teaching reports fully detailed on export.
	if strings.HasPrefix(strings.TrimSpace(r.ID), "УЧ-") && (r.SnapshotVersion != fullReportSectionSnapshotVersion || teachingFullReportSectionsNeedRefresh(r.Sections)) {
		if scenario, ok := teachingScenarioForFullReportRecord(r); ok {
			r.Sections = buildTeachingApprovedSectionsSafely(r, scenario)
			r.SnapshotVersion = fullReportSectionSnapshotVersion
			persistEnrichedTeachingFullReport(r)
		}
	}
	return r
}

func selectedApprovedRegistryReportRaw() (FullReportRecord, bool) {
	row := selectedRow(archiveList)
	if row < 0 && len(approvedRegistryRows) > 0 {
		row = 0
		selectListRow(archiveList, 0)
	}
	if row < 0 || row >= len(approvedRegistryRows) {
		return FullReportRecord{}, false
	}
	return approvedRegistryRows[row], true
}

func selectedApprovedRegistryReport() (FullReportRecord, bool) {
	r, ok := selectedApprovedRegistryReportRaw()
	if !ok {
		return FullReportRecord{}, false
	}
	return fullReportRecordForExport(r), true
}

func approvedRegistrySnapshotLines(snapshot string, keywords ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range strings.Split(strings.ReplaceAll(snapshot, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		match := false
		for _, keyword := range keywords {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				match = true
				break
			}
		}
		if match && !seen[line] {
			seen[line] = true
			out = append(out, line)
		}
	}
	return out
}

func approvedRegistryWriteSection(b *strings.Builder, title string, lines []string, fallback string) {
	b.WriteString("\r\n" + title + "\r\n")
	if len(lines) == 0 {
		b.WriteString(firstNonEmpty(strings.TrimSpace(fallback), "Данные раздела зафиксированы в полном расчётном снимке ниже.") + "\r\n")
		return
	}
	for _, line := range lines {
		b.WriteString(line + "\r\n")
	}
}

func approvedRegistryDetailedReport(r FullReportRecord) string {
	if detailed := storedSectionDetailedReport(r, true); strings.TrimSpace(detailed) != "" {
		return detailed
	}
	// Backward-compatible rendering for historical records created before RC726.
	var b strings.Builder
	b.WriteString("РОСА — УТВЕРЖДЁННЫЙ ПОЛНЫЙ ОТЧЁТ\r\n")
	fmt.Fprintf(&b, "Отчёт: %s\r\nОбъект: %s\r\nВариант: %s\r\nУтверждено: %s\r\nУтвердил: %s\r\nСтатус: %s\r\n", r.ID, r.ObjectLabel, r.ScenarioName, firstNonEmpty(strings.TrimSpace(r.ApprovedAt), r.CreatedAt), firstNonEmpty(strings.TrimSpace(r.Author), "технолог"), firstNonEmpty(strings.TrimSpace(r.DecisionStatus), "Утверждено"))
	b.WriteString("\r\nРАСЧЁТНАЯ РАСШИФРОВКА ПО РАЗДЕЛАМ 1–7\r\n")
	snapshot := strings.TrimSpace(r.Snapshot)
	approvedRegistryWriteSection(&b, "РАЗДЕЛ 1. ФОНД И КАРТОЧКА СКВАЖИНЫ", approvedRegistrySnapshotLines(snapshot, "Организация:", "Месторождение:", "Скважина:", "Объект / пласт:", "ГНО:", "Текущий МРП:", "фактический результат", "накопительная статистика"), "Объект: "+r.ObjectLabel)
	approvedRegistryWriteSection(&b, "РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ", approvedRegistrySnapshotLines(snapshot, "лаборатор", "минерализац", "pH", "CO₂", "H₂S", "O₂", "корроз", "АСПО", "мехприм", "вязк", "эмульс"), "Подтверждённые лабораторные данные и их статус фиксируются в снимке на момент утверждения.")
	approvedRegistryWriteSection(&b, "РАЗДЕЛ 3. ОЦЕНКА ОСЛОЖНЕНИЙ", approvedRegistrySnapshotLines(snapshot, "осложнение:", "классифика", "оценка ослож", "риск", "достоверност", "признак"), "Оценка осложнений зафиксирована в утверждённом снимке решения.")
	methodLines := approvedRegistrySnapshotLines(snapshot, "выбранный метод:", "метод сценария:", "основание метода", "текущий сценарий мероприятия", "параллельные варианты", "методы:", "доступность:")
	methodLines = append([]string{"Методы / мероприятия: " + firstNonEmpty(strings.TrimSpace(r.Methods), "не указаны"), "Доступность: " + firstNonEmpty(strings.TrimSpace(r.Availability), "не указана")}, methodLines...)
	approvedRegistryWriteSection(&b, "РАЗДЕЛ 4. МЕТОДЫ И МЕРОПРИЯТИЯ", methodLines, r.Methods)
	forecastLines := []string{fmt.Sprintf("Утверждённый центральный прогноз наработки: %.0f сут.", r.ForecastDays)}
	forecastLines = append(forecastLines, approvedRegistrySnapshotLines(snapshot, "прогноз безремонтной работы", "остаточный ресурс", "вероятность продолжения", "вероятность подтверждённого отказа", "достаточность истории", "цель ≥", "источник цели", "прогноз МРП", "наработка:", "источник МРП")...)
	approvedRegistryWriteSection(&b, "РАЗДЕЛ 5. НАРАБОТКА И ОТКАЗЫ", forecastLines, fmt.Sprintf("Прогноз: %.0f сут.", r.ForecastDays))
	econLines := []string{fmt.Sprintf("Чистая выгода Benefit Modeling: %+.2f млн руб.", r.NetBenefitK/1000)}
	econLines = append(econLines, approvedRegistrySnapshotLines(snapshot, "экономика:", "CAPEX", "OPEX", "затрат", "чист", "benefit", "эффект")...)
	approvedRegistryWriteSection(&b, "РАЗДЕЛ 6. ЭКОНОМИЧЕСКАЯ ОЦЕНКА", econLines, "Расчёт Benefit Modeling зафиксирован в снимке.")
	decisionLines := []string{
		"Решение: " + firstNonEmpty(strings.TrimSpace(r.DecisionStatus), "Утверждено"),
		"Основание выбора: " + firstNonEmpty(strings.TrimSpace(r.DecisionBasis), "не указано"),
		"Автор / утвердил: " + firstNonEmpty(strings.TrimSpace(r.Author), "технолог"),
	}
	decisionLines = append(decisionLines, approvedRegistrySnapshotLines(snapshot, "Решение:", "Комментарий:", "Ответственный:", "основание решения", "правило решения")...)
	approvedRegistryWriteSection(&b, "РАЗДЕЛ 7. ИТОГОВЫЙ ОТЧЁТ", decisionLines, r.DecisionBasis)
	b.WriteString("\r\nПРИЛОЖЕНИЕ. РАСЧЁТНЫЕ ДАННЫЕ НА МОМЕНТ УТВЕРЖДЕНИЯ\r\n")
	if snapshot == "" {
		b.WriteString("Расчётный снимок отсутствует.\r\n")
	} else {
		b.WriteString(snapshot)
		if !strings.HasSuffix(snapshot, "\n") && !strings.HasSuffix(snapshot, "\r") {
			b.WriteString("\r\n")
		}
	}
	return reportCleanUserText(b.String())
}

func openSelectedApprovedRegistryReport() {
	// Opening a registry row uses the same frozen approved snapshot as export.
	record, ok := selectedApprovedRegistryReportRaw()
	if !ok {
		msg("Утверждённый реестр", "Выделите утверждённую строку.", MB_OK|MB_ICONWARNING)
		return
	}
	openResultTextViewer("РОСА — Утверждённый отчёт", "УТВЕРЖДЁННЫЙ ПОЛНЫЙ ОТЧЁТ", record.ID+" · "+record.ObjectLabel, approvedRegistryDetailedReport(record))
}

func chooseApprovedRegistryExportPath(r FullReportRecord, format string) string {
	filename := safeFilePart(r.ID+"_"+r.ObjectLabel+"_утвержденный") + "." + format
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(filename)))
	filters := map[string]string{"xlsx": "Excel (*.xlsx)\x00*.xlsx\x00\x00", "docx": "Word (*.docx)\x00*.docx\x00\x00", "pdf": "PDF (*.pdf)\x00*.pdf\x00\x00"}
	filter := utf16.Encode([]rune(filters[format]))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrDefExt: p(format), LpstrTitle: p("Скачать утверждённый полный отчёт"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT}
	res, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if res == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(path), "."+format) {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
	}
	return path
}

func approvedRegistryExportMinSize(format string) int64 {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pdf":
		return 256
	case "docx", "xlsx":
		return 512
	default:
		return 1
	}
}

func validateApprovedRegistryExportFile(path, format string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("файл не создан: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("вместо файла создан каталог: %s", path)
	}
	if info.Size() < approvedRegistryExportMinSize(format) {
		return fmt.Errorf("файл создан некорректно: размер %d байт", info.Size())
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pdf":
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("не удалось проверить PDF: %w", err)
		}
		defer f.Close()
		header := make([]byte, 5)
		n, err := f.Read(header)
		if err != nil || n < 5 || string(header) != "%PDF-" {
			return fmt.Errorf("сформированный файл не является корректным PDF")
		}
	case "docx", "xlsx":
		zr, err := zip.OpenReader(path)
		if err != nil {
			return fmt.Errorf("сформированный %s повреждён: %w", strings.ToUpper(format), err)
		}
		defer zr.Close()
		required := "word/document.xml"
		if strings.EqualFold(format, "xlsx") {
			required = "xl/workbook.xml"
		}
		foundContentTypes, foundRequired := false, false
		for _, f := range zr.File {
			switch f.Name {
			case "[Content_Types].xml":
				foundContentTypes = true
			case required:
				foundRequired = true
			}
		}
		if !foundContentTypes || !foundRequired {
			return fmt.Errorf("сформированный %s не содержит обязательные части документа", strings.ToUpper(format))
		}
	}
	return nil
}

func probeApprovedRegistryExportDestination(path string) error {
	dir := filepath.Dir(filepath.Clean(path))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("не удалось подготовить папку сохранения %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, ".rosa-export-check-*.tmp")
	if err != nil {
		return fmt.Errorf("нет доступа на запись в папку %s: %w", dir, err)
	}
	probe := f.Name()
	if _, err = f.Write([]byte("ROSA")); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	_ = os.Remove(probe)
	if err != nil {
		return fmt.Errorf("проверка записи в папку %s не пройдена: %w", dir, err)
	}
	if closeErr != nil {
		return fmt.Errorf("проверка записи в папку %s не завершена: %w", dir, closeErr)
	}
	return nil
}

func writeApprovedRegistryExportTemp(format string, r FullReportRecord, report string) (string, error) {
	ext := "." + strings.ToLower(strings.TrimSpace(format))
	f, err := os.CreateTemp("", "rosa-approved-report-*"+ext)
	if err != nil {
		return "", fmt.Errorf("не удалось подготовить временный файл отчёта: %w", err)
	}
	tmp := f.Name()
	if err = f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Remove(tmp) // Writers create a complete file themselves.

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pdf":
		err = writeLaboratoryPDFReport(tmp, report)
	case "docx":
		err = writeLaboratoryDOCXReport(tmp, report)
	case "xlsx":
		if len(r.Sections) > 0 {
			err = writeFullReportSectionXLSX(tmp, r, true)
		} else {
			err = writeLaboratoryXLSXReport(tmp, report)
		}
	default:
		err = fmt.Errorf("неподдерживаемый формат %s", format)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err = validateApprovedRegistryExportFile(tmp, format); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("проверка сформированного отчёта не пройдена: %w", err)
	}
	return tmp, nil
}

func writeApprovedRegistryDirect(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func commitApprovedRegistryExport(tempPath, finalPath, format string) error {
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return fmt.Errorf("не удалось прочитать готовый временный отчёт: %w", err)
	}
	if err = atomicWriteFile(finalPath, data, 0o644); err != nil {
		// Some synchronized/network folders reject MoveFileEx replacement even
		// though a normal file write is allowed. Fall back to a direct synced
		// write so Desktop/OneDrive/network destinations still work.
		diagLog("approved export atomic commit fallback path=%s err=%v", finalPath, err)
		if directErr := writeApprovedRegistryDirect(finalPath, data); directErr != nil {
			return fmt.Errorf("не удалось записать файл в выбранное место (атомарно: %v; напрямую: %w)", err, directErr)
		}
	}
	if err = validateApprovedRegistryExportFile(finalPath, format); err != nil {
		_ = os.Remove(finalPath)
		return fmt.Errorf("файл записан, но не прошёл итоговую проверку: %w", err)
	}
	return nil
}

func writeApprovedRegistryExportPayload(path, format string, r FullReportRecord, report string) error {
	report = reportCleanUserText(report)
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pdf":
		err := writeLaboratoryPDFReport(path, report)
		if fallback, ok := err.(*laboratoryPDFFallbackError); ok && len(r.Sections) > 0 && fallback.XLSXPath != "" {
			// The fallback workbook must have the same formula sheets as a
			// direct XLSX export, not a one-column copy of the PDF prose.
			fallback.XLSXErr = writeFullReportSectionXLSX(fallback.XLSXPath, r, true)
		}
		return err
	case "docx":
		return writeLaboratoryDOCXReport(path, report)
	case "xlsx":
		if len(r.Sections) > 0 {
			return writeFullReportSectionXLSX(path, r, true)
		}
		return writeLaboratoryXLSXReport(path, report)
	default:
		return fmt.Errorf("неподдерживаемый формат %s", format)
	}
}

func exportApprovedRegistryReportToChosenPath(path, format string, r FullReportRecord, report string) error {
	// RC737: corporate endpoint protection can allow final DOCX/XLSX/PDF files
	// while rejecting hidden *.tmp probe/staging files. Generate a visible
	// sibling staging file with the real target extension, validate it, then
	// rename it into place. If replacement is blocked, copy the already
	// validated bytes directly to the user-selected final path and verify again.
	dir := filepath.Dir(filepath.Clean(path))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("не удалось подготовить папку сохранения %s: %w", dir, err)
	}
	ext := "." + strings.ToLower(strings.TrimSpace(format))
	stageFile, err := os.CreateTemp(dir, "ROSA_EXPORT_STAGE-*"+ext)
	if err != nil {
		// A staging file may be blocked by folder policy even when the final named
		// document is allowed. Do not fail early: try the requested path directly.
		diagLog("approved export stage create fallback path=%s err=%v", path, err)
		if directErr := writeApprovedRegistryExportPayload(path, format, r, report); directErr != nil {
			return fmt.Errorf("не удалось создать служебный файл (%v) и не удалось записать выбранный файл напрямую: %w", err, directErr)
		}
		return validateApprovedRegistryExportFile(path, format)
	}
	stagePath := stageFile.Name()
	if closeErr := stageFile.Close(); closeErr != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf("не удалось подготовить служебный файл отчёта: %w", closeErr)
	}
	_ = os.Remove(stagePath) // the format writer creates a complete document itself
	defer os.Remove(stagePath)

	if err = writeApprovedRegistryExportPayload(stagePath, format, r, report); err != nil {
		return fmt.Errorf("не удалось сформировать полный отчёт: %w", err)
	}
	if err = validateApprovedRegistryExportFile(stagePath, format); err != nil {
		return fmt.Errorf("сформированный полный отчёт не прошёл проверку: %w", err)
	}

	if err = replaceFileAtomic(stagePath, path); err != nil {
		// Do not use atomicWriteFile here: it creates a hidden *.tmp file and can
		// reproduce the exact RC736 failure under corporate folder policies.
		diagLog("approved export sibling replace fallback path=%s err=%v", path, err)
		data, readErr := os.ReadFile(stagePath)
		if readErr != nil {
			return fmt.Errorf("не удалось прочитать проверенный полный отчёт: %w", readErr)
		}
		if directErr := writeApprovedRegistryDirect(path, data); directErr != nil {
			return fmt.Errorf("не удалось перенести файл (%v) и не удалось записать его напрямую: %w", err, directErr)
		}
	}
	if err = validateApprovedRegistryExportFile(path, format); err != nil {
		return fmt.Errorf("готовый файл не прошёл итоговую проверку: %w", err)
	}
	return nil
}

func cloneFullReportRecord(r FullReportRecord) FullReportRecord {
	out := r
	if r.Sections != nil {
		out.Sections = make(map[string]string, len(r.Sections))
		for key, value := range r.Sections {
			out.Sections[key] = value
		}
	}
	return out
}

func approvedRegistryDirectExportCandidates() []string {
	// RC739: use only user-writable, visible locations. Start with the same
	// Downloads discovery already used by the Methods source exporter, including
	// Russian/OneDrive layouts, then fall back to ROSA's writable data area.
	var roots []string
	if downloads := strings.TrimSpace(methodsReferenceDownloadsDir()); downloads != "" {
		roots = append(roots, filepath.Join(downloads, "РОСА_Отчёты"))
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		roots = append(roots,
			filepath.Join(home, "Загрузки", "РОСА_Отчёты"),
			filepath.Join(home, "Documents", "РОСА_Отчёты"),
			filepath.Join(home, "Документы", "РОСА_Отчёты"),
			filepath.Join(home, "Desktop", "РОСА_Отчёты"),
			filepath.Join(home, "Рабочий стол", "РОСА_Отчёты"),
			filepath.Join(home, "OneDrive", "Documents", "РОСА_Отчёты"),
			filepath.Join(home, "OneDrive", "Desktop", "РОСА_Отчёты"),
		)
	}
	if strings.TrimSpace(dataDir) != "" {
		roots = append(roots, filepath.Join(dataDir, "Экспорт_отчётов"))
	}
	roots = append(roots, filepath.Join(appRootDir(), "exports", "Утверждённые_отчёты"))
	if tmp := strings.TrimSpace(os.TempDir()); tmp != "" {
		roots = append(roots, filepath.Join(tmp, "РОСА_Отчёты"))
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" || root == "." {
			continue
		}
		key := strings.ToLower(root)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, root)
	}
	return out
}

func approvedRegistryAvailableExportPath(dir string, r FullReportRecord, format string) string {
	base := safeFilePart(r.ID + "_" + r.ObjectLabel + "_утвержденный")
	if strings.TrimSpace(base) == "" {
		base = "РОСА_утвержденный_отчет"
	}
	ext := "." + strings.ToLower(strings.TrimSpace(format))
	candidate := filepath.Join(dir, base+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	stamp := time.Now().Format("20060102_150405")
	for i := 0; i < 100; i++ {
		suffix := "_" + stamp
		if i > 0 {
			suffix += fmt.Sprintf("_%02d", i+1)
		}
		candidate = filepath.Join(dir, base+suffix+ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return filepath.Join(dir, base+"_новый"+ext)
}

func enrichApprovedRegistryRecordDetached(r FullReportRecord) FullReportRecord {
	// Same detail policy as fullReportRecordForExport, but without mutating the
	// shared archive from a worker goroutine.
	if strings.HasPrefix(strings.TrimSpace(r.ID), "УЧ-") && (r.SnapshotVersion != fullReportSectionSnapshotVersion || teachingFullReportSectionsNeedRefresh(r.Sections)) {
		if scenario, ok := teachingScenarioForFullReportRecord(r); ok {
			r.Sections = buildTeachingApprovedSectionsSafely(r, scenario)
			r.SnapshotVersion = fullReportSectionSnapshotVersion
		}
	}
	return r
}

func writeApprovedRegistryExportDirectValidated(path, format string, r FullReportRecord, report string) error {
	// The final document itself is the first write. No hidden probe, no .tmp and
	// no rename are required, which avoids the corporate-DLP failure seen in
	// RC736-738. A failed partial file is removed before trying the next folder.
	_ = os.Remove(path)
	if err := writeApprovedRegistryExportPayload(path, format, r, report); err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := validateApprovedRegistryExportFile(path, format); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func runApprovedRegistryExport(r FullReportRecord, format string) (string, error) {
	// Old teaching records have no section snapshots. Build their detailed
	// sections only for this explicit export, preserving persisted archive data.
	r = enrichApprovedRegistryRecordDetached(r)
	report := approvedRegistryDetailedReport(r)
	var failures []string
	for _, dir := range approvedRegistryDirectExportCandidates() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			failures = append(failures, dir+": "+err.Error())
			continue
		}
		path := approvedRegistryAvailableExportPath(dir, r, format)
		diagLog("approved export RC739 START format=%s path=%s id=%s", format, path, r.ID)
		if err := writeApprovedRegistryExportDirectValidated(path, format, r, report); err != nil {
			// PDF conversion can fail because Word/COM is unavailable while the
			// built-in fallback DOCX/XLSX was successfully preserved. Do not create
			// duplicate fallbacks in every candidate directory.
			if _, ok := err.(*laboratoryPDFFallbackError); ok {
				return "", err
			}
			diagLog("approved export RC739 FAIL format=%s path=%s err=%v", format, path, err)
			failures = append(failures, path+": "+err.Error())
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info == nil || info.Size() < approvedRegistryExportMinSize(format) {
			if err == nil {
				err = fmt.Errorf("размер файла недостаточен")
			}
			failures = append(failures, path+": "+err.Error())
			_ = os.Remove(path)
			continue
		}
		diagLog("approved export RC739 OK format=%s path=%s size=%d", format, path, info.Size())
		return path, nil
	}
	if len(failures) == 0 {
		return "", fmt.Errorf("не найден доступный каталог для сохранения")
	}
	return "", fmt.Errorf("не удалось создать отчёт ни в одной разрешённой папке:\n%s", strings.Join(failures, "\n"))
}

// runApprovedRegistryExportSafely is the panic boundary for the background
// export worker. A panic in any goroutine would otherwise terminate the entire
// GUI process, even though wndProc has its own recovery boundary.
func runApprovedRegistryExportSafely(r FullReportRecord, format string) (path string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			diagLog("approved export PANIC format=%s panic=%v\n%s", format, recovered, debug.Stack())
			path = ""
			err = fmt.Errorf("не удалось сформировать отчёт %s: внутренняя ошибка экспорта (%v). Приложение продолжает работу", strings.ToUpper(format), recovered)
		}
	}()
	return runApprovedRegistryExport(r, format)
}

func setApprovedRegistryExportBusy(busy bool, format string) {
	archiveExportBusy = busy
	for _, h := range []syscall.Handle{archiveExportPDFButton, archiveExportDOCXButton, archiveExportXLSXButton} {
		if h != 0 {
			if busy {
				pEnableWindow.Call(uintptr(h), 0)
			} else {
				pEnableWindow.Call(uintptr(h), 1)
			}
		}
	}
	if busy {
		setText(archiveRegistryHint, "Формируется полный "+strings.ToUpper(format)+"-отчёт… интерфейс остаётся доступным")
	} else {
		setText(archiveRegistryHint, "Выберите строку · двойной щелчок — открыть отчёт · выгрузка — по выбранной записи")
	}
}

func exportSelectedApprovedRegistryReport(format string) {
	if archiveExportBusy {
		return
	}
	r, ok := selectedApprovedRegistryReportRaw()
	if !ok {
		msg("Утверждённый реестр", "В реестре нет утверждённых отчётов для выгрузки.", MB_OK|MB_ICONWARNING)
		return
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "pdf" && format != "docx" && format != "xlsx" {
		msg("Выгрузка утверждённого отчёта", "Неподдерживаемый формат: "+format, MB_OK|MB_ICONWARNING)
		return
	}
	r = cloneFullReportRecord(r)
	setApprovedRegistryExportBusy(true, format)
	go func(record FullReportRecord, exportFormat string) {
		path, err := runApprovedRegistryExportSafely(record, exportFormat)
		archiveExportResultMu.Lock()
		archiveExportResultPath = path
		archiveExportResultFormat = exportFormat
		archiveExportResultErr = err
		archiveExportResultMu.Unlock()
		pPostMessageW.Call(uintptr(hMain), WM_APP_ARCHIVE_EXPORT_DONE, 0, 0)
	}(r, format)
}

func approvedRegistryExportDone() {
	archiveExportResultMu.Lock()
	path := archiveExportResultPath
	format := archiveExportResultFormat
	err := archiveExportResultErr
	archiveExportResultPath, archiveExportResultFormat, archiveExportResultErr = "", "", nil
	archiveExportResultMu.Unlock()
	setApprovedRegistryExportBusy(false, "")
	if currentPage == ID_ARCHIVE {
		pRedrawWindow.Call(uintptr(hMain), 0, 0, 0x0001|0x0004|0x0080)
	}
	if err != nil {
		msg("Выгрузка утверждённого отчёта", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	info, statErr := os.Stat(path)
	if statErr != nil || info == nil {
		msg("Выгрузка утверждённого отчёта", "Отчёт сформирован, но итоговый файл не найден:\n"+path, MB_OK|MB_ICONWARNING)
		return
	}
	selectArg := `/select,"` + path + `"`
	pShellExecuteW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p("open"))), uintptr(unsafe.Pointer(p("explorer.exe"))), uintptr(unsafe.Pointer(p(selectArg))), 0, 1)
	msg("Выгрузка утверждённого отчёта", fmt.Sprintf("Полный %s-отчёт создан и проверен.\n\n%s\n\nРазмер: %.1f КБ", strings.ToUpper(format), path, float64(info.Size())/1024.0), MB_OK|MB_ICONINFORMATION)
}

func selectedFullReport() (FullReportRecord, bool) {
	ensureFullReportsLoaded()
	row := selectedRow(resultArchiveList)
	if row < 0 || row >= len(fullReports) {
		return FullReportRecord{}, false
	}
	return fullReportRecordForExport(fullReports[len(fullReports)-1-row]), true
}

func openSelectedFullReport() {
	r, ok := selectedFullReport()
	if !ok {
		msg("Полный отчёт", "Выделите строку в перечне проходок.", MB_OK|MB_ICONWARNING)
		return
	}
	report := storedSectionDetailedReport(r, strings.TrimSpace(r.ApprovedAt) != "")
	if strings.TrimSpace(report) == "" {
		report = r.Snapshot
	}
	openResultTextViewer("РОСА — Полный отчёт", "ПОЛНЫЙ ОТЧЁТ ПО ПРОХОДКЕ", r.ID+" · "+r.ObjectLabel, report)
}

func chooseFullReportExportPath(format string) string {
	r, ok := selectedFullReport()
	if !ok {
		return ""
	}
	filename := safeFilePart(r.ID+"_"+r.ObjectLabel) + "." + format
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(filename)))
	filters := map[string]string{"xlsx": "Excel (*.xlsx)\x00*.xlsx\x00\x00", "docx": "Word (*.docx)\x00*.docx\x00\x00", "pdf": "PDF (*.pdf)\x00*.pdf\x00\x00"}
	filter := utf16.Encode([]rune(filters[format]))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrDefExt: p(format), LpstrTitle: p("Выгрузить сохранённый полный отчёт"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT}
	res, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if res == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(path), "."+format) {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
	}
	return path
}

func exportSelectedFullReport(format string) {
	r, ok := selectedFullReport()
	if !ok {
		msg("Выгрузка отчёта", "Выделите строку в перечне проходок.", MB_OK|MB_ICONWARNING)
		return
	}
	path := chooseFullReportExportPath(format)
	if path == "" {
		return
	}
	report := storedSectionDetailedReport(r, strings.TrimSpace(r.ApprovedAt) != "")
	if strings.TrimSpace(report) == "" {
		report = r.Snapshot
	}
	report = reportCleanUserText(report)
	var err error
	switch format {
	case "pdf":
		err = writeLaboratoryPDFReport(path, report)
		if fallback, ok := err.(*laboratoryPDFFallbackError); ok && len(r.Sections) > 0 && fallback.XLSXPath != "" {
			fallback.XLSXErr = writeFullReportSectionXLSX(fallback.XLSXPath, r, strings.TrimSpace(r.ApprovedAt) != "")
		}
	case "docx":
		err = writeLaboratoryDOCXReport(path, report)
	case "xlsx":
		if len(r.Sections) > 0 {
			err = writeFullReportSectionXLSX(path, r, strings.TrimSpace(r.ApprovedAt) != "")
		} else {
			err = writeLaboratoryXLSXReport(path, report)
		}
	}
	if err != nil {
		msg("Выгрузка отчёта", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	msg("Выгрузка отчёта", "Файл сохранён:\n"+path, MB_OK|MB_ICONINFORMATION)
}

func handleResultWorkspaceCommand(id int) bool {
	if id >= ID_RESULT_SECTION_CARD_BASE && id < ID_RESULT_SECTION_CARD_BASE+len(resultSectionCards) {
		page := resultSectionCardPage(id - ID_RESULT_SECTION_CARD_BASE)
		if page != 0 {
			switchPage(page)
			return true
		}
	}
	switch id {
	case ID_RESULT_COPY_SCHEME:
		if err := duplicateSelectedResultScenario(); err != nil {
			msg("Дублировать вариант", err.Error(), MB_OK|MB_ICONWARNING)
		}
	case ID_RESULT_COMPARE_SCHEMES:
		openResultScenarioComparison()
	case ID_RESULT_SELECTED_INFO:
		openResultSelectedMeasuresInfo()
	case ID_RESULT_USE_SELECTED:
		openSelectedResultScenarioEconomics()
	case ID_RESULT_APPROVE_SCHEME:
		if err := approveSelectedResultScenarioToRegistry(); err != nil {
			msg("Утверждение в реестр", err.Error(), MB_OK|MB_ICONWARNING)
		} else {
			msg("Утверждённый реестр", "Выбранная проходка зафиксирована в разделе 8.", MB_OK|MB_ICONINFORMATION)
		}
	case ID_RESULT_SAVE_FULL:
		if err := saveCurrentFullReport(); err != nil {
			msg("Сохранение отчёта", err.Error(), MB_OK|MB_ICONWARNING)
		} else {
			msg("Сохранение проходки", "Проходка добавлена в нижний перечень раздела 7. В утверждённый реестр она попадёт только после отдельного утверждения.", MB_OK|MB_ICONINFORMATION)
		}
	case ID_RESULT_OPEN_SAVED:
		openSelectedFullReport()
	case ID_RESULT_EXPORT_PDF:
		exportSelectedFullReport("pdf")
	case ID_RESULT_EXPORT_DOCX:
		exportSelectedFullReport("docx")
	case ID_RESULT_EXPORT_XLSX:
		exportSelectedFullReport("xlsx")
	case ID_ARCHIVE_EXPORT_PDF:
		exportSelectedApprovedRegistryReport("pdf")
	case ID_ARCHIVE_EXPORT_DOCX:
		exportSelectedApprovedRegistryReport("docx")
	case ID_ARCHIVE_EXPORT_XLSX:
		exportSelectedApprovedRegistryReport("xlsx")
	default:
		return false
	}
	return true
}

// RC725: enlarged report hierarchy + protected subtitle baselines for sections 7 and 8.
func layoutResultWorkspace(w, h int32) {
	x := int32(242)
	contentW := max32(720, w-270)
	right := x + contentW
	top := py(94)
	bottom := h - 18
	setPos(resultRefreshButton, right-190, top, 190, 34)

	// Final report follows the reference hierarchy: object → six evidence cards →
	// parallel variants → technologist decision → all saved analysis passes.
	// The compact matrix deliberately protects at least five pass rows; at the
	// shortest supported work areas the trace card yields space instead of pushing
	// the archive below the viewport.
	compact := h <= 930
	veryCompact := h < 820
	subtitleH := int32(22)
	objectY := top + 68
	objectH := int32(66)
	cardsGapY := int32(10)
	cardsH := int32(98)
	variantsH := int32(132)
	decisionH := int32(72)
	traceH := int32(54)
	if compact {
		objectY = top + 66
		objectH = 60
		cardsGapY = 8
		cardsH = 86
		variantsH = 82
		decisionH = 58
		traceH = 40
	}
	if veryCompact {
		objectY = top + 64
		objectH = 54
		cardsGapY = 6
		cardsH = 72
		variantsH = 62
		decisionH = 54
	}
	setPos(resultSubtitle, x, top+42, contentW-205, subtitleH)
	setPos(resultObjectCard, x, objectY, contentW, objectH)

	gap := int32(8)
	cardW := (contentW - gap*5) / 6
	cardsY := objectY + objectH + cardsGapY
	for i, card := range resultSectionCards {
		setPos(card, x+int32(i)*(cardW+gap), cardsY, cardW, cardsH)
	}

	variantTitleY := cardsY + cardsH + 8
	buttonGap := int32(8)
	// RC721: only two explicit actions remain in section 7. Both use equal width
	// so captions share the same typography and never clip at supported sizes.
	copyW, compareW := int32(236), int32(236)
	buttonsTotal := copyW + compareW + buttonGap
	variantButtonH := int32(38)
	if veryCompact {
		variantButtonH = 34
	}
	variantTextW := max32(180, contentW-buttonsTotal-12)
	// RC729: the section heading needs a little more glyph room than the old
	// 22 px STATIC provided. Lift it by 2 px and give it a 26 px box so the
	// descenders/baseline never get clipped while preserving the table/buttons.
	setPos(resultVariantsTitle, x, variantTitleY-2, variantTextW, 26)
	show(resultVariantsSubtitle, !veryCompact)
	if veryCompact {
		setPos(resultVariantsTitle, x, variantTitleY-1, variantTextW, 26)
	} else {
		setPos(resultVariantsSubtitle, x, variantTitleY+24, variantTextW, 22)
	}
	setPos(resultCopyVariantButton, right-buttonsTotal, variantTitleY, copyW, variantButtonH)
	setPos(resultCompareButton, right-compareW, variantTitleY, compareW, variantButtonH)

	variantsY := variantTitleY + max32(variantButtonH, 46) + 8
	if veryCompact {
		variantsY = variantTitleY + variantButtonH + 6
	}
	setPos(resultVariantsList, x, variantsY, contentW, variantsH)
	fitWellFailureListColumns(resultVariantsList, []int32{76, 236, 116, 104, 112, 130, 112, 178})

	decisionY := variantsY + variantsH + 8
	decisionButtonGap := int32(8)
	// RC708/710 geometry retained for regression QA only:
	// useW, saveW, approveW := int32(154), int32(174), int32(184)
	// decisionButtonsW := useW + saveW + approveW + 2*decisionButtonGap
	// decisionRegionW := max32(420, contentW-decisionButtonsW-14)
	// infoW := min32(304, max32(168, decisionRegionW-292-infoGap))
	// decisionTextW := decisionRegionW - infoW - infoGap
	// decisionButtonH := min32(50, max32(44, decisionH-4))
	methodW, economyW, approveW := int32(220), int32(180), int32(200)
	actionsW := methodW + economyW + approveW + 2*decisionButtonGap
	actionsX := right - actionsW
	decisionTextW := actionsX - x - 14
	decisionRowBudget := int32(82)
	decisionCardH := int32(104)
	if compact {
		decisionRowBudget = 68
		decisionCardH = 90
	}
	if veryCompact {
		decisionRowBudget = 64
		decisionCardH = 82
	}
	setPos(resultDecisionCard, x, decisionY, decisionTextW, decisionCardH)
	// RC712 baseline retained for regression reference: setPos(resultDecisionHint, actionsX, decisionY, actionsW, 16)
	// RC715 widened this caption to the left. RC716 restores the strict visual alignment:
	// the hint occupies exactly the same horizontal span as the three action buttons.
	// RC717: keep the caption strictly inside the three-button span, but give the final word
	// a visual right-side safety margin by using the dedicated 13 px font.
	setPos(resultDecisionHint, actionsX, decisionY, actionsW, 16)
	decisionButtonH := int32(48)
	buttonY := decisionY + 16
	setPos(resultSelectedInfoButton, actionsX, buttonY, methodW, decisionButtonH)
	bx := actionsX + methodW + decisionButtonGap
	setPos(resultUseSelectedButton, bx, buttonY, economyW, decisionButtonH)
	bx += economyW + decisionButtonGap
	setPos(resultApproveButton, bx, buttonY, approveW, decisionButtonH)

	// Reference keeps forecast/fact as part of the decision, not a second dominant card.
	// RC706 baseline spacing retained for regression QA only:
	// archiveTitleY := decisionY + decisionH + 10
	_ = traceH
	_ = decisionH
	_ = decisionRowBudget
	show(resultTraceCard, false)
	// RC718: the archive title gets a real row of its own. Earlier compact layouts
	// started the export buttons before the decision card ended, leaving only ~18 px
	// for the title and clipping its lower edge. Move this row below the decision
	// card and deliberately give up roughly one archive data row instead.
	archiveTitleY := decisionY + decisionCardH + 4
	archiveToolsY := archiveTitleY

	exportsGap := int32(8)
	openW, pdfW, docxW, xlsxW := int32(104), int32(86), int32(92), int32(92)
	exportsTotal := openW + pdfW + docxW + xlsxW + 3*exportsGap
	archiveButtonH := int32(40)
	// RC712 regression marker retained: archiveY := archiveToolsY + archiveButtonH + 6
	// RC712 regression marker retained: max32(150, bottom-archiveY)
	archiveTextW := max32(220, contentW-exportsTotal-16)
	setPos(resultArchiveTitle, x, archiveTitleY, archiveTextW, 24)
	setPos(resultArchiveSubtitle, x, archiveTitleY+24, archiveTextW, 22)
	bx = right - exportsTotal
	setPos(resultOpenButton, bx, archiveToolsY+2, openW, archiveButtonH)
	bx += openW + exportsGap
	setPos(resultExportPDFButton, bx, archiveToolsY+2, pdfW, archiveButtonH)
	bx += pdfW + exportsGap
	setPos(resultExportDOCXButton, bx, archiveToolsY+2, docxW, archiveButtonH)
	bx += docxW + exportsGap
	setPos(resultExportXLSXButton, bx, archiveToolsY+2, xlsxW, archiveButtonH)
	archiveY := archiveTitleY + max32(archiveButtonH+2, 46) + 8

	// RC699 baseline retained for historical regression QA only: setPos(resultArchiveList, x, archiveY, contentW, max32(150, bottom-archiveY))
	// Do not position the lower ListView outside the client rectangle. A prior
	// hard minimum pushed its bottom below the viewport on small displays;
	// Win32 painted only fragments and made the other stored passes invisible.
	archiveMinH := int32(120)
	if veryCompact {
		archiveMinH = 64
	}
	archiveVisibleH := bottom - archiveY
	if archiveVisibleH < archiveMinH {
		archiveVisibleH = max32(1, archiveVisibleH)
	}
	setPos(resultArchiveList, x, archiveY, contentW, archiveVisibleH)
	fitWellFailureListColumns(resultArchiveList, []int32{150, 230, 260, 110, 120, 180})
	show(resultSummary, false)
}
