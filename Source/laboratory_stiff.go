//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

var (
	pCreatePenStiff = gdi32.NewProc("CreatePen")
	pPolylineStiff  = gdi32.NewProc("Polyline")
)

const (
	PS_SOLID_STIFF = 0
	PS_DASH_STIFF  = 1
	DT_RIGHT_STIFF = 0x00000002
)

type laboratoryStiffProfile struct {
	Cations [3]float64 // Ca, Mg, Na+K, meq/L
	Anions  [3]float64 // HCO3+CO3, SO4, Cl, meq/L
	OK      bool
	Approx  bool
}

func laboratoryStiffProfileFromValues(values map[string]string) laboratoryStiffProfile {
	ca, caOK := laboratoryEquivalentMillimoles(values, "ca", 40.078, 2)
	mg, mgOK := laboratoryEquivalentMillimoles(values, "mg", 24.305, 2)
	nak, nakOK, approximate := laboratorySulinNaKEquivalent(values)
	hco3, hco3OK := laboratoryEquivalentMillimoles(values, "hco3", 61.0168, 1)
	co3, co3OK := laboratoryEquivalentMillimoles(values, "co3", 60.008, 2)
	so4, so4OK := laboratoryEquivalentMillimoles(values, "so4", 96.06, 2)
	cl, clOK := laboratoryEquivalentMillimoles(values, "cl", 35.453, 1)
	if !(caOK && mgOK && nakOK && hco3OK && so4OK && clOK) {
		return laboratoryStiffProfile{}
	}
	carbonate := hco3
	if co3OK {
		carbonate += co3
	}
	return laboratoryStiffProfile{
		Cations: [3]float64{ca, mg, nak},
		Anions:  [3]float64{carbonate, so4, cl},
		OK:      true,
		Approx:  approximate,
	}
}

func laboratoryCurrentScaleCanonicalValues() map[string]string {
	group, ok := labGroupByCode("scale")
	if !ok {
		return map[string]string{}
	}
	return canonicalizeLaboratoryValues(group, labDrafts["scale"], labUnitDrafts["scale"])
}

func laboratoryCurrentPassForStiff() LabPassRecord {
	return LabPassRecord{
		PassID:     labActivePassID,
		ObjectKey:  strings.TrimSpace(getText(labObjectEdit)),
		Date:       strings.TrimSpace(getText(labDateEdit)),
		ScaleModel: labScaleModelCode,
		Groups:     cloneLabGroups(labDrafts),
		Units:      cloneLabGroups(labUnitDrafts),
	}
}

func laboratoryPreviousScaleCanonicalValues() (map[string]string, bool) {
	current := laboratoryCurrentPassForStiff()
	passes := repositoryLabPassesForObject(current.ObjectKey)
	prev, ok := laboratoryPreviousSavedPass(current, passes)
	if !ok {
		return nil, false
	}
	group, ok := labGroupByCode("scale")
	if !ok {
		return nil, false
	}
	return canonicalizeLaboratoryValues(group, prev.Groups["scale"], prev.Units["scale"]), true
}

func stiffMax(profile laboratoryStiffProfile) float64 {
	maxv := 0.0
	for _, v := range profile.Cations {
		if v > maxv {
			maxv = v
		}
	}
	for _, v := range profile.Anions {
		if v > maxv {
			maxv = v
		}
	}
	return maxv
}

func drawLaboratoryStiffChart(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	rc := di.RcItem
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rc)), uintptr(brushLight))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), rgb(43, 46, 50))

	drawStiffText(di.HDC, RECT{rc.Left + 12, rc.Top + 8, rc.Right - 12, rc.Top + 30}, "Stiff-профиль воды, мг-экв/л", 0|DT_SINGLELINE|DT_NOPREFIX, hFontBold)

	current := laboratoryStiffProfileFromValues(laboratoryCurrentScaleCanonicalValues())
	if !current.OK {
		drawStiffText(di.HDC, RECT{rc.Left + 12, rc.Top + 40, rc.Right - 12, rc.Bottom - 10}, "Для диаграммы нужны Ca²⁺, Mg²⁺, Na/K, HCO₃⁻, SO₄²⁻ и Cl⁻. Диаграмма — визуальный химический отпечаток, а не самостоятельный прогноз солеотложения.", 0|DT_WORDBREAK|DT_NOPREFIX, hFont)
		return
	}

	previousValues, hasPrevious := laboratoryPreviousScaleCanonicalValues()
	previous := laboratoryStiffProfile{}
	if hasPrevious {
		previous = laboratoryStiffProfileFromValues(previousValues)
		hasPrevious = previous.OK
	}

	left := rc.Left + 112
	right := rc.Right - 112
	top := rc.Top + 50
	bottom := rc.Bottom - 32
	if right-left < 220 || bottom-top < 90 {
		return
	}
	center := (left + right) / 2
	rowY := [3]int32{top + 12, (top + bottom) / 2, bottom - 12}

	maxv := stiffMax(current)
	if hasPrevious {
		maxv = math.Max(maxv, stiffMax(previous))
	}
	if maxv <= 0 {
		maxv = 1
	}
	// Add headroom and round to a readable engineering scale.
	maxv *= 1.15
	halfW := float64(right - center - 16)
	scale := halfW / maxv

	// Axis and row guides.
	axisPen, _, _ := pCreatePenStiff.Call(PS_SOLID_STIFF, 1, rgb(150, 160, 170))
	oldPen, _, _ := pSelectObject.Call(uintptr(di.HDC), axisPen)
	axisPts := []POINT{{X: center, Y: top}, {X: center, Y: bottom}}
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&axisPts[0])), uintptr(len(axisPts)))
	for _, y := range rowY {
		pts := []POINT{{X: left, Y: y}, {X: right, Y: y}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
	}
	pSelectObject.Call(uintptr(di.HDC), oldPen)
	pDeleteObject.Call(axisPen)

	// Labels.
	cats := []string{"Ca²⁺", "Mg²⁺", "Na⁺+K⁺"}
	ans := []string{"HCO₃⁻+CO₃²⁻", "SO₄²⁻", "Cl⁻"}
	for i, y := range rowY {
		drawStiffText(di.HDC, RECT{rc.Left + 8, y - 11, left - 8, y + 12}, cats[i], DT_RIGHT_STIFF|DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX, hFont)
		drawStiffText(di.HDC, RECT{right + 8, y - 11, rc.Right - 8, y + 12}, ans[i], 0|DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX, hFont)
	}

	drawProfile := func(profile laboratoryStiffProfile, color uintptr, style uintptr, width uintptr) {
		pts := make([]POINT, 0, 7)
		for i := 0; i < 3; i++ {
			x := center - int32(math.Round(profile.Cations[i]*scale))
			pts = append(pts, POINT{X: x, Y: rowY[i]})
		}
		for i := 2; i >= 0; i-- {
			x := center + int32(math.Round(profile.Anions[i]*scale))
			pts = append(pts, POINT{X: x, Y: rowY[i]})
		}
		pts = append(pts, pts[0])
		pen, _, _ := pCreatePenStiff.Call(style, width, color)
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
		pSelectObject.Call(uintptr(di.HDC), old)
		pDeleteObject.Call(pen)
	}

	if hasPrevious {
		drawProfile(previous, rgb(105, 115, 125), PS_DASH_STIFF, 1)
	}
	drawProfile(current, rgb(15, 48, 82), PS_SOLID_STIFF, 2)

	footer := fmt.Sprintf("Шкала до %.1f мг-экв/л. Текущий анализ — сплошная линия", maxv)
	if hasPrevious {
		footer += "; предыдущий — пунктир"
	}
	if current.Approx {
		footer += ". Na+K оценены как натриевый эквивалент"
	}
	drawStiffText(di.HDC, RECT{rc.Left + 12, rc.Bottom - 26, rc.Right - 12, rc.Bottom - 6}, footer+".", 0|DT_SINGLELINE|DT_NOPREFIX, hFont)
}

func drawStiffText(hdc syscall.Handle, rect RECT, text string, flags uintptr, font syscall.Handle) {
	buf, _ := syscall.UTF16FromString(text)
	if len(buf) == 0 {
		return
	}
	old := uintptr(0)
	if font != 0 {
		old, _, _ = pSelectObject.Call(uintptr(hdc), uintptr(font))
	}
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&rect)), flags)
	if old != 0 {
		pSelectObject.Call(uintptr(hdc), old)
	}
}
