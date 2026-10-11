//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unsafe"
)

type complexMetric struct {
	label, group, key, unit, implication string
}

var complexMetrics = []complexMetric{
	{"Минерализация", "scale", "mineralization", "г/л", "рост солесодержания может усилить склонность к отложениям при изменении температуры и давления"},
	{"Кальций", "scale", "ca", "мг/л", "рост кальция может повысить риск карбонатных или сульфатных отложений"},
	{"Сульфаты", "scale", "so4", "мг/л", "рост сульфатов может повысить склонность к сульфатным отложениям"},
	{"Скорость коррозии", "corrosion", "corrosion_rate", "мм/год", "рост скорости коррозии может говорить об ухудшении защиты; проверьте ингибирование и условия отбора"},
	{"Парафины", "aspo", "paraffins", "% масс.", "рост парафинов может увеличить склонность к АСПО при охлаждении потока"},
	{"Механические примеси", "solids", "suspended", "мг/л", "рост взвешенных веществ может повышать нагрузку на оборудование и фильтры"},
	{"Магний Mg²⁺", "scale", "mg", "мг/л", "изменение магния оцените вместе с составом воды и насыщением"},
	{"Натрий Na⁺", "scale", "na", "мг/л", "изменение натрия может отражать смену состава воды"},
	{"Калий K⁺", "scale", "k", "мг/л", "изменение калия может отражать смену состава воды"},
	{"Барий Ba²⁺", "scale", "ba", "мг/л", "барий оцените вместе с сульфатами и насыщением барита"},
	{"Стронций Sr²⁺", "scale", "sr", "мг/л", "стронций оцените вместе с сульфатами и насыщением целестина"},
	{"Карбонат CO₃²⁻", "scale", "co3", "мг/л", "карбонат оцените вместе с pH и кальцием"},
	{"Гидрокарбонат HCO₃⁻", "scale", "hco3", "мг/л", "гидрокарбонат оцените вместе с pH, кальцием и выделением CO₂"},
	{"Хлорид Cl⁻", "scale", "cl", "мг/л", "хлориды оцените вместе с составом воды и коррозией"},
	{"Кислород O₂", "corrosion", "o2", "мг/л", "кислород поддерживает коррозию при наличии водной фазы"},
	{"Углекислый газ CO₂", "corrosion", "co2", "мг/л", "CO₂ оцените вместе с pH и обводнённостью"},
	{"Сероводород H₂S", "corrosion", "h2s", "мг/л", "H₂S повышает коррозионную опасность в водной фазе"},
	{"Обводнённость (контекст техрежима)", "corrosion", "watercut", "%", "рост доли воды меняет контакт металла с водной фазой и условия коррозии"},
}

type complexValue struct {
	date  string
	value float64
}

func complexMetricValues(passes []LabPassRecord, metric complexMetric) []complexValue {
	byDate := map[string]float64{}
	// Обводнённость — режимный показатель. Даты лабораторных диагностик и
	// даты загрузки техрежима нельзя выдавать за даты замеров обводнённости.
	// Поэтому для неё используем только актуальную запись техрежима и не
	// строим ложную горизонтальную линию по датам лабораторных паспортов.
	if metric.key == "watercut" && len(passes) > 0 {
		// Prefer the live technical-regime value for this exact object. Some
		// imported/teaching diagnoses use a readable caption instead of the
		// persistent key, so also try the globally pinned object when it refers to
		// the current group.
		state, found := laboratorySelectedTechState(passes[0].ObjectKey)
		if !found && normalizeLaboratoryObjectKey(selectedObjectKey) == normalizeLaboratoryObjectKey(passes[0].ObjectKey) {
			state, found = laboratorySelectedTechState(selectedObjectKey)
		}
		if found {
			if v, valid := parseLabNumber(state.WaterCut); valid && v >= 0 && v <= 100 {
				date := "дата не указана"
				if measuredAt, err := time.Parse(time.RFC3339, state.ImportedAt); err == nil {
					date = measuredAt.Format("02.01.2006")
				}
				return []complexValue{{date: date, value: v}}
			}
		}
		// A saved diagnosis contains the contextual snapshot that was actually
		// used at that date. This is the reliable fallback when the current
		// technical-regime key has changed or the record came from a teaching
		// import. Preserve every dated snapshot instead of showing an empty chart.
		for _, pass := range passes {
			v, valid := parseLabNumber(pass.SharedValues["watercut"])
			if !valid {
				for _, values := range pass.Groups {
					if v, valid = parseLabNumber(values["watercut"]); valid {
						break
					}
				}
			}
			if valid && v >= 0 && v <= 100 && strings.TrimSpace(pass.Date) != "" {
				byDate[pass.Date] = v
			}
		}
	}
	for _, pass := range passes {
		group, ok := labGroupByCode(metric.group)
		if !ok {
			continue
		}
		values := canonicalizeLaboratoryValues(group, pass.Groups[metric.group], pass.Units[metric.group])
		v, ok := parseLabNumber(values[metric.key])
		if !ok && metric.key == "watercut" {
			// Historical passes may have kept the contextual tech-regime value in
			// another group or only in the shared-value snapshot.
			v, ok = parseLabNumber(pass.SharedValues["watercut"])
			if !ok {
				for _, groupValues := range pass.Groups {
					if v, ok = parseLabNumber(groupValues["watercut"]); ok {
						break
					}
				}
			}
		}
		if ok && strings.TrimSpace(pass.Date) != "" {
			byDate[pass.Date] = v
		}
	}
	result := make([]complexValue, 0, len(byDate))
	for date, value := range byDate {
		result = append(result, complexValue{date, value})
	}
	sort.Slice(result, func(i, j int) bool { return parseDate(result[i].date).Before(parseDate(result[j].date)) })
	return result
}

func complexDateAssessment(passes []LabPassRecord) string {
	var b strings.Builder
	b.WriteString("\r\nИЗМЕНЕНИЯ ПО ДАТАМ И ВОЗМОЖНЫЕ ПРЕДПОСЫЛКИ\r\n")
	compared, worsened := 0, 0
	for _, metric := range complexMetrics {
		values := complexMetricValues(passes, metric)
		if len(values) < 2 {
			continue
		}
		first, last := values[0], values[len(values)-1]
		compared++
		delta := last.value - first.value
		word := "без существенного изменения"
		if delta > 0 {
			word = "рост"
			worsened++
		} else if delta < 0 {
			word = "снижение"
		}
		fmt.Fprintf(&b, "%s: %s %.3g → %s %.3g %s; %s.\r\n", metric.label, first.date, first.value, last.date, last.value, metric.unit, word)
		if delta > 0 {
			fmt.Fprintf(&b, "  Возможная предпосылка: %s.\r\n", metric.implication)
		}
	}
	if compared == 0 {
		b.WriteString("Нет двух разных дат с числовыми значениями одного показателя. Динамику и предпосылки ухудшения пока оценить нельзя.\r\n")
	}
	b.WriteString("\r\nКОРОТКИЙ ПРОГНОЗ\r\n")
	if worsened == 0 {
		b.WriteString("Устойчивого роста по сопоставимым показателям не видно. Продолжите контроль тем же методом и в тех же единицах.\r\n")
	} else {
		fmt.Fprintf(&b, "Если текущий режим сохранится, %d растущих показателей могут продолжить увеличиваться. Проверьте их в следующей диагностике и сопоставьте с температурой, давлением, дозированием и историей ремонтов. Линейное числовое продолжение по немногим датам ненадёжно.\r\n", worsened)
	}
	return b.String()
}

func drawComplexInfographic(di *DRAWITEMSTRUCT, selected []LabPassRecord) {
	rc := di.RcItem
	brush, _, _ := pCreateSolidBrush.Call(rgb(248, 250, 253))
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rc)), brush)
	pDeleteObject.Call(brush)
	draw := func(s string, r RECT, bold bool) {
		font := hFont
		if bold {
			font = hFontBold
		}
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		pSetTextColor.Call(uintptr(di.HDC), rgb(22, 51, 82))
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	draw("ПОКАЗАТЕЛЬ И ДИНАМИКА ПО ДАТАМ", RECT{Left: rc.Left + 18, Top: rc.Top + 12, Right: rc.Right - 18, Bottom: rc.Top + 42}, true)
	if len(selected) < 2 {
		draw("Выберите минимум две диагностики и нажмите «Проанализировать выбранные».", RECT{Left: rc.Left + 18, Top: rc.Top + 50, Right: rc.Right - 18, Bottom: rc.Top + 82}, false)
		return
	}
	metric := complexSelectedMetric()
	if groups := complexPassGroups(selected); len(groups) > 1 {
		lineGroups := complexLineGroups(groups, metric)
		if len(lineGroups) == 0 {
			draw(metric.label+": нет объектов минимум с двумя разными датами. Одиночные точки не отображаются как ложная динамика.", RECT{Left: rc.Left + 18, Top: rc.Top + 54, Right: rc.Right - 18, Bottom: rc.Top + 84}, false)
			return
		}
		drawComplexMultiInfographic(di, lineGroups, metric)
		return
	}
	threshold, hasThreshold := laboratoryMetricThreshold(metric.group, metric.key)
	values := complexMetricValues(selected, metric)
	if len(values) < 2 {
		draw(metric.label+": нет двух разных дат с числовыми значениями.", RECT{Left: rc.Left + 18, Top: rc.Top + 54, Right: rc.Right - 18, Bottom: rc.Top + 84}, false)
		return
	}
	first, last := values[0], values[len(values)-1]
	change := last.value - first.value
	direction := "без изменения"
	color := rgb(39, 91, 143)
	if change > 0 {
		direction = "рост"
		color = rgb(32, 98, 170)
	} else if change < 0 {
		direction = "снижение"
		color = rgb(23, 125, 85)
	}
	draw(fmt.Sprintf("%s · %d дат · %s: %.3g → %.3g %s", metric.label, len(values), direction, first.value, last.value, metric.unit), RECT{Left: rc.Left + 18, Top: rc.Top + 44, Right: rc.Right - 18, Bottom: rc.Top + 78}, true)
	plot := RECT{Left: rc.Left + 72, Top: rc.Top + 80, Right: rc.Right - 54, Bottom: rc.Bottom - 50}
	if plot.Right-plot.Left < 200 || plot.Bottom-plot.Top < 80 {
		draw("Увеличьте окно для графика.", RECT{Left: rc.Left + 18, Top: rc.Top + 90, Right: rc.Right - 18, Bottom: rc.Bottom - 12}, false)
		return
	}
	minV, maxV := values[0].value, values[0].value
	for _, v := range values[1:] {
		if v.value < minV {
			minV = v.value
		}
		if v.value > maxV {
			maxV = v.value
		}
	}
	if hasThreshold {
		if threshold.value < minV {
			minV = threshold.value
		}
		if threshold.value > maxV {
			maxV = threshold.value
		}
	}
	if maxV-minV < 1e-9 {
		minV -= 1
		maxV += 1
	} else {
		pad := (maxV - minV) * 0.15
		minV -= pad
		maxV += pad
	}
	axisPen, _, _ := pCreatePenStiff.Call(PS_SOLID_STIFF, 1, rgb(170, 185, 204))
	oldAxis, _, _ := pSelectObject.Call(uintptr(di.HDC), axisPen)
	axes := []POINT{{X: plot.Left, Y: plot.Top}, {X: plot.Left, Y: plot.Bottom}, {X: plot.Right, Y: plot.Bottom}}
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&axes[0])), uintptr(len(axes)))
	if oldAxis != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldAxis)
	}
	pDeleteObject.Call(axisPen)
	if hasThreshold {
		y := plot.Bottom - int32((threshold.value-minV)/(maxV-minV)*float64(plot.Bottom-plot.Top))
		guide, _, _ := pCreatePenStiff.Call(2, 1, rgb(195, 39, 54)) // PS_DOT: threshold
		previous, _, _ := pSelectObject.Call(uintptr(di.HDC), guide)
		line := []POINT{{X: plot.Left, Y: y}, {X: plot.Right, Y: y}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), 2)
		pSelectObject.Call(uintptr(di.HDC), previous)
		pDeleteObject.Call(guide)
		// Keep the threshold caption beside its line and away from the Y-axis
		// extrema. Previously both texts occupied the same top-left strip.
		draw(threshold.caption(metric.unit), RECT{Left: max32(plot.Left+120, plot.Right-360), Top: y - 25, Right: plot.Right - 6, Bottom: y - 2}, true)
	} else {
		draw("Для этого показателя единый числовой порог не задан; оцените сочетание условий.", RECT{Left: plot.Left + 120, Top: plot.Top - 24, Right: plot.Right, Bottom: plot.Top - 1}, false)
	}
	linePen, _, _ := pCreatePenStiff.Call(PS_SOLID_STIFF, 3, color)
	oldLine, _, _ := pSelectObject.Call(uintptr(di.HDC), linePen)
	points := make([]POINT, len(values))
	for i, v := range values {
		x := plot.Left + int32(i)*(plot.Right-plot.Left)/int32(len(values)-1)
		y := plot.Bottom - int32((v.value-minV)/(maxV-minV)*float64(plot.Bottom-plot.Top))
		points[i] = POINT{X: x, Y: y}
	}
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&points[0])), uintptr(len(points)))
	if oldLine != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldLine)
	}
	pDeleteObject.Call(linePen)
	for i, v := range values {
		pt := points[i]
		dot := RECT{Left: pt.X - 5, Top: pt.Y - 5, Right: pt.X + 6, Bottom: pt.Y + 6}
		dotColor := color
		if hasThreshold && threshold.exceeded(v.value) {
			dotColor = rgb(195, 39, 54)
		}
		b, _, _ := pCreateSolidBrush.Call(dotColor)
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&dot)), b)
		pDeleteObject.Call(b)
		step := 1
		if len(values) > 8 {
			step = (len(values) + 7) / 8
		}
		if i%step == 0 || i == len(values)-1 {
			draw(fmt.Sprintf("%.3g", v.value), RECT{Left: pt.X - 35, Top: pt.Y - 32, Right: pt.X + 45, Bottom: pt.Y - 9}, true)
			draw(v.date, RECT{Left: pt.X - 40, Top: plot.Bottom + 8, Right: pt.X + 55, Bottom: plot.Bottom + 32}, false)
		}
	}
	draw("Причину изменения проверьте по режиму и протоколам; рекомендации приведены ниже.", RECT{Left: rc.Left + 18, Top: rc.Bottom - 23, Right: rc.Right - 18, Bottom: rc.Bottom - 3}, false)
}

func absComplex(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
func maxComplex(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
