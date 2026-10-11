//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

type laboratoryMixingCurvePoint struct {
	Fraction float64
	SI       [4]float64
	Present  [4]bool
}

var laboratoryMixingMineralNames = [4]string{"Кальцит", "Гипс", "Барит", "Целестин"}

const laboratoryMixingRiskThreshold = 0.10

type laboratoryMixingRiskRange struct {
	Mineral  string
	StartPct int
	EndPct   int
}

// laboratoryMixingRiskRanges summarizes contiguous sampled fractions where SI
// exceeds the working screening threshold. It does not claim precipitation; the
// range is only a thermodynamic screening interval for the fixed-pH sensitivity.
func laboratoryMixingRiskRanges(points []laboratoryMixingCurvePoint, threshold float64) []laboratoryMixingRiskRange {
	var out []laboratoryMixingRiskRange
	for i, name := range laboratoryMixingMineralNames {
		in := false
		start := 0
		last := 0
		for _, p := range points {
			pct := int(math.Round(p.Fraction * 100))
			hit := p.Present[i] && p.SI[i] > threshold
			if hit && !in {
				in = true
				start = pct
			}
			if hit {
				last = pct
			}
			if !hit && in {
				out = append(out, laboratoryMixingRiskRange{Mineral: name, StartPct: start, EndPct: last})
				in = false
			}
		}
		if in {
			out = append(out, laboratoryMixingRiskRange{Mineral: name, StartPct: start, EndPct: last})
		}
	}
	return out
}

func laboratoryMixingRiskText(values map[string]string) string {
	points, ph, err := laboratoryWaterMixingCurve(values)
	if err != nil {
		return "Диапазоны риска по смешению пока не рассчитаны: " + err.Error()
	}
	ranges := laboratoryMixingRiskRanges(points, laboratoryMixingRiskThreshold)
	if len(ranges) == 0 {
		return fmt.Sprintf("При фиксированном измеренном pH %.3f ни один рассчитанный минерал не превышает рабочий порог SI > +%.1f на сетке 0–100 %% воды 2. Это не доказывает отсутствие осадка.", ph, laboratoryMixingRiskThreshold)
	}
	lines := []string{fmt.Sprintf("Скрининговые диапазоны SI > +%.1f при фиксированном измеренном pH %.3f:", laboratoryMixingRiskThreshold, ph)}
	for _, r := range ranges {
		if r.StartPct == r.EndPct {
			lines = append(lines, fmt.Sprintf("• %s: около %d %% воды 2.", r.Mineral, r.StartPct))
		} else {
			lines = append(lines, fmt.Sprintf("• %s: примерно %d–%d %% воды 2.", r.Mineral, r.StartPct, r.EndPct))
		}
	}
	lines = append(lines, "Это диапазоны термодинамической предпосылки на дискретной сетке 5 %, а не прогноз фактической массы или скорости осадка.")
	return strings.Join(lines, "\n")
}

// laboratoryWaterMixingCurve performs a transparent screening sweep. The pH is
// held at the measured pH of the selected mixture because pH is not a
// conservative quantity and must not be silently averaged. Therefore this
// curve is a sensitivity view, not a full equilibrium path for every fraction.
func laboratoryWaterMixingCurve(values map[string]string) ([]laboratoryMixingCurvePoint, float64, error) {
	ph, ok := parseLabNumber(values["mix_ph"])
	if !ok || ph < 0 || ph > 14 {
		return nil, 0, fmt.Errorf("для графика нужен измеренный pH выбранной смеси")
	}
	points := make([]laboratoryMixingCurvePoint, 0, 21)
	for pct := 0; pct <= 100; pct += 5 {
		v := cloneStringMap(values)
		usePct := pct
		if usePct == 0 {
			usePct = 1
		} else if usePct == 100 {
			usePct = 99
		}
		v["mix_percent"] = fmt.Sprintf("%d", usePct)
		results, _, warnings := laboratoryWaterMixingResults(v)
		if len(results) == 0 {
			if len(warnings) > 0 {
				return nil, ph, fmt.Errorf("%s", warnings[0])
			}
			return nil, ph, fmt.Errorf("расчёт кривой не выполнен")
		}
		p := laboratoryMixingCurvePoint{Fraction: float64(pct) / 100}
		for _, r := range results {
			for i, name := range laboratoryMixingMineralNames {
				if r.Mineral == name {
					p.SI[i] = r.SI
					p.Present[i] = true
				}
			}
		}
		points = append(points, p)
	}
	return points, ph, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func drawLaboratoryMixingChart(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	rc := di.RcItem
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rc)), uintptr(brushLight))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), rgb(43, 46, 50))
	drawStiffText(di.HDC, RECT{rc.Left + 12, rc.Top + 8, rc.Right - 12, rc.Top + 30}, "Совместимость двух вод: чувствительность SI к доле воды 2", DT_SINGLELINE|DT_NOPREFIX, hFontBold)

	values := laboratoryCurrentScaleCanonicalValues()
	points, ph, err := laboratoryWaterMixingCurve(values)
	if err != nil {
		drawStiffText(di.HDC, RECT{rc.Left + 12, rc.Top + 38, rc.Right - 12, rc.Bottom - 10}, "График пока недоступен: "+err.Error()+". Он строится только для полностью заданного сценария смешения.", DT_WORDBREAK|DT_NOPREFIX, hFont)
		return
	}

	left, right := rc.Left+58, rc.Right-18
	top, bottom := rc.Top+52, rc.Bottom-44
	if right-left < 260 || bottom-top < 80 {
		return
	}
	minSI, maxSI := math.Inf(1), math.Inf(-1)
	for _, p := range points {
		for i := range p.SI {
			if !p.Present[i] {
				continue
			}
			if p.SI[i] < minSI {
				minSI = p.SI[i]
			}
			if p.SI[i] > maxSI {
				maxSI = p.SI[i]
			}
		}
	}
	if math.IsInf(minSI, 0) || math.IsInf(maxSI, 0) {
		return
	}
	if minSI > -0.25 {
		minSI = -0.25
	}
	if maxSI < 0.25 {
		maxSI = 0.25
	}
	pad := (maxSI - minSI) * 0.08
	minSI -= pad
	maxSI += pad
	yOf := func(si float64) int32 {
		return bottom - int32(math.Round((si-minSI)/(maxSI-minSI)*float64(bottom-top)))
	}
	xOf := func(f float64) int32 { return left + int32(math.Round(f*float64(right-left))) }

	axisPen, _, _ := pCreatePenStiff.Call(PS_SOLID_STIFF, 1, rgb(150, 160, 170))
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), axisPen)
	axes := []POINT{{X: left, Y: top}, {X: left, Y: bottom}, {X: right, Y: bottom}}
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&axes[0])), uintptr(len(axes)))
	if minSI <= 0 && maxSI >= 0 {
		y0 := yOf(0)
		line := []POINT{{X: left, Y: y0}, {X: right, Y: y0}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), uintptr(len(line)))
	}
	if minSI <= laboratoryMixingRiskThreshold && maxSI >= laboratoryMixingRiskThreshold {
		yRisk := yOf(laboratoryMixingRiskThreshold)
		line := []POINT{{X: left, Y: yRisk}, {X: right, Y: yRisk}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), uintptr(len(line)))
		drawStiffText(di.HDC, RECT{left + 4, yRisk - 18, right - 4, yRisk - 2}, "рабочий порог SI +0,1", DT_RIGHT_STIFF|DT_SINGLELINE|DT_NOPREFIX, hFont)
	}
	pSelectObject.Call(uintptr(di.HDC), old)
	pDeleteObject.Call(axisPen)

	colors := []uintptr{rgb(15, 48, 82), rgb(170, 60, 60), rgb(110, 85, 20), rgb(60, 110, 85)}
	for i := 0; i < 4; i++ {
		pts := make([]POINT, 0, len(points))
		for _, p := range points {
			if p.Present[i] {
				pts = append(pts, POINT{X: xOf(p.Fraction), Y: yOf(p.SI[i])})
			}
		}
		if len(pts) < 2 {
			continue
		}
		pen, _, _ := pCreatePenStiff.Call(PS_SOLID_STIFF, 2, colors[i])
		prev, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
		pSelectObject.Call(uintptr(di.HDC), prev)
		pDeleteObject.Call(pen)
	}

	drawStiffText(di.HDC, RECT{left - 50, bottom - 10, left - 4, bottom + 14}, fmt.Sprintf("%.2f", minSI), DT_RIGHT_STIFF|DT_SINGLELINE|DT_NOPREFIX, hFont)
	drawStiffText(di.HDC, RECT{left - 50, top - 10, left - 4, top + 14}, fmt.Sprintf("%.2f", maxSI), DT_RIGHT_STIFF|DT_SINGLELINE|DT_NOPREFIX, hFont)
	drawStiffText(di.HDC, RECT{left, bottom + 6, right, bottom + 28}, "0%                         доля воды 2                         100%", DT_SINGLELINE|DT_NOPREFIX, hFont)

	selected, _ := parseLabNumber(values["mix_percent"])
	if selected > 0 && selected < 100 {
		x := xOf(selected / 100)
		pen, _, _ := pCreatePenStiff.Call(PS_DASH_STIFF, 1, rgb(90, 90, 90))
		prev, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
		line := []POINT{{X: x, Y: top}, {X: x, Y: bottom}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), uintptr(len(line)))
		pSelectObject.Call(uintptr(di.HDC), prev)
		pDeleteObject.Call(pen)
	}

	riskSummary := laboratoryMixingRiskRanges(points, laboratoryMixingRiskThreshold)
	riskText := "порог +0,1 не превышен"
	if len(riskSummary) > 0 {
		riskText = fmt.Sprintf("диапазонов SI > +0,1: %d", len(riskSummary))
	}
	footer := fmt.Sprintf("Скрининг при фиксированном pH %.3f; %s. pH удержан измеренным для выбранной смеси; это не полный равновесный путь.", ph, riskText)
	drawStiffText(di.HDC, RECT{rc.Left + 12, rc.Bottom - 36, rc.Right - 12, rc.Bottom - 4}, footer, DT_WORDBREAK|DT_NOPREFIX, hFont)
}

var _ syscall.Handle
