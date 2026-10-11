//go:build windows

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
)

const (
	BI_RGB_STIFF         = 0
	DIB_RGB_COLORS_STIFF = 0
)

type laboratoryBitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type laboratoryBitmapInfo struct {
	Header laboratoryBitmapInfoHeader
	Colors [1]uint32
}

type laboratoryInfographicGeometry struct {
	Width, Height   int32
	Margin          int32
	InfluenceTop    int32
	InfluenceHeight int32
	TrendTop        int32
	TrendHeight     int32
	SummaryTop      int32
	SummaryHeight   int32
}

func laboratoryInfographicGeometryForDPI(scalePercent int, scopeCode string) laboratoryInfographicGeometry {
	if scalePercent < 100 {
		scalePercent = 100
	}
	scale := func(v int32) int32 { return v * int32(scalePercent) / 100 }
	g := laboratoryInfographicGeometry{
		Width:           scale(1400),
		Margin:          scale(24),
		InfluenceTop:    scale(24),
		InfluenceHeight: scale(170),
		TrendHeight:     scale(330),
	}
	if scopeCode != laboratoryScopeObject {
		g.TrendHeight = scale(410)
	}
	g.TrendTop = g.InfluenceTop + g.InfluenceHeight + scale(16)
	g.SummaryTop = g.TrendTop + g.TrendHeight + scale(16)
	g.SummaryHeight = scale(150)
	g.Height = g.SummaryTop + g.SummaryHeight + g.Margin
	return g
}

func laboratoryInfographicSemanticSummary(pass LabPassRecord, scopeCode string, metric laboratoryTrendMetric, passes []LabPassRecord, now time.Time) string {
	q := laboratoryPassQCSummaryFor(pass, now)
	scope := laboratoryScopeCaption(scopeCode, pass.ObjectKey, passes)
	filter := laboratoryRiskFilterCaption(labOverviewRiskFilterCode)
	return fmt.Sprintf("РОСА — ПАСПОРТ ИНФОГРАФИКИ\nОбъект / якорь: %s · дата анализа: %s · источник: %s\nОбласть: %s · фильтр: %s · показатель: %s (%s)\nQC: данные по %d из 6 направлений; базовый набор полностью заполнен по %d; ошибок %d; предупреждений %d; свежесть: %s.\nОграничение: инфографика отражает сохранённые фактические данные и расчётные признаки. Отсутствие оценки не означает низкий риск; dose-response не интерполирует оптимальную дозу.", emptyLabText(pass.ObjectKey), emptyLabText(pass.Date), emptyLabText(pass.Source), scope, filter, metric.Label, metric.Unit, q.DirectionsWithData, q.DirectionsBaselineComplete, q.ValidationErrors, q.ValidationWarnings, q.FreshnessLabel)
}

func drawLaboratoryInfographicSummary(dc uintptr, rect RECT, text string) {
	pFillRect.Call(dc, uintptr(unsafe.Pointer(&rect)), uintptr(brushLight))
	pFrameRect.Call(dc, uintptr(unsafe.Pointer(&rect)), uintptr(brushBorder))
	inner := rect
	inner.Left += 12
	inner.Right -= 12
	inner.Top += 10
	inner.Bottom -= 8
	pSetBkMode.Call(dc, TRANSPARENT)
	pSetTextColor.Call(dc, rgb(15, 48, 82))
	old, _, _ := pSelectObject.Call(dc, uintptr(hFont))
	pDrawTextW.Call(dc, uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&inner)), DT_WORDBREAK|DT_NOPREFIX)
	if old != 0 {
		pSelectObject.Call(dc, old)
	}
}

func renderLaboratoryInfographicPNG(path string, dpiPercent int) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("не задан путь для PNG")
	}
	g := laboratoryInfographicGeometryForDPI(dpiPercent, labOverviewScopeCode)
	dc, _, _ := pCreateCompatibleDC.Call(0)
	if dc == 0 {
		return fmt.Errorf("не удалось создать графический контекст")
	}
	defer pDeleteDC.Call(dc)

	bmi := laboratoryBitmapInfo{}
	bmi.Header.Size = uint32(unsafe.Sizeof(laboratoryBitmapInfoHeader{}))
	bmi.Header.Width = g.Width
	bmi.Header.Height = -g.Height // top-down DIB
	bmi.Header.Planes = 1
	bmi.Header.BitCount = 32
	bmi.Header.Compression = BI_RGB_STIFF
	bmi.Header.SizeImage = uint32(g.Width * g.Height * 4)
	var bits unsafe.Pointer
	bmp, _, _ := pCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS_STIFF, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return fmt.Errorf("не удалось создать bitmap для инфографики")
	}
	defer pDeleteObject.Call(bmp)
	old, _, _ := pSelectObject.Call(dc, bmp)
	if old != 0 {
		defer pSelectObject.Call(dc, old)
	}

	full := RECT{Left: 0, Top: 0, Right: g.Width, Bottom: g.Height}
	pFillRect.Call(dc, uintptr(unsafe.Pointer(&full)), uintptr(brushWhite))

	influence := DRAWITEMSTRUCT{HDC: syscall.Handle(dc), RcItem: RECT{
		Left: g.Margin, Top: g.InfluenceTop, Right: g.Width - g.Margin, Bottom: g.InfluenceTop + g.InfluenceHeight,
	}}
	drawLaboratoryInfluenceChart(&influence)
	trend := DRAWITEMSTRUCT{HDC: syscall.Handle(dc), RcItem: RECT{
		Left: g.Margin, Top: g.TrendTop, Right: g.Width - g.Margin, Bottom: g.TrendTop + g.TrendHeight,
	}}
	drawLaboratoryTrendChart(&trend)
	pass := ensureActiveLabPass()
	metricIndex, _, _ := pSendMessageW.Call(uintptr(labTrendMetricCombo), CB_GETCURSEL, 0, 0)
	metric := laboratoryTrendMetricByIndex(int(metricIndex))
	passes := repositoryLabPassesForObject(pass.ObjectKey)
	if labOverviewScopeCode != laboratoryScopeObject {
		passes = repositoryLatestSavedLabPasses()
	}
	summary := laboratoryInfographicSemanticSummary(*pass, labOverviewScopeCode, metric, passes, time.Now())
	drawLaboratoryInfographicSummary(dc, RECT{Left: g.Margin, Top: g.SummaryTop, Right: g.Width - g.Margin, Bottom: g.SummaryTop + g.SummaryHeight}, summary)

	// DIB is BGRA. Convert to an opaque NRGBA image for PNG encoding.
	raw := unsafe.Slice((*byte)(bits), int(g.Width*g.Height*4))
	img := image.NewNRGBA(image.Rect(0, 0, int(g.Width), int(g.Height)))
	for y := 0; y < int(g.Height); y++ {
		for x := 0; x < int(g.Width); x++ {
			i := (y*int(g.Width) + x) * 4
			b, gg, r := raw[i], raw[i+1], raw[i+2]
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: gg, B: b, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, img); err != nil {
		return err
	}
	return nil
}

func defaultLaboratoryInfographicPNGName() string {
	key := safeFilePart(strings.TrimSpace(getText(labObjectEdit)))
	if key == "" {
		key = "выборка"
	}
	return "РОСА_инфографика_" + key + "_" + time.Now().Format("20060102") + ".png"
}
