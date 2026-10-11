//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type laboratoryTrendMetric struct {
	Code, Label, Group, Key, Unit string
}

var laboratoryTrendMetrics = []laboratoryTrendMetric{
	{"mineralization", "Минерализация", "scale", "mineralization", "г/л"},
	{"ph", "pH", "corrosion", "ph", "ед."},
	{"iron_total", "Железо общее", "corrosion", "iron_total", "мг/л"},
	{"chlorides", "Хлориды", "corrosion", "chlorides", "мг/л"},
	{"orp", "ОВП", "corrosion", "orp", "мВ"},
	{"viscosity", "Вязкость", "aspo", "viscosity", "мПа·с"},
	{"suspended", "Механические примеси", "solids", "suspended", "мг/л"},
	{"h2s", "H₂S", "gases", "h2s", "мг/л"},
}

func laboratoryTrendMetricByIndex(index int) laboratoryTrendMetric {
	if index < 0 || index >= len(laboratoryTrendMetrics) {
		return laboratoryTrendMetrics[0]
	}
	return laboratoryTrendMetrics[index]
}

func laboratorySelectedObjectKeysText(text string) []string {
	repl := strings.NewReplacer("\r", ";", "\n", ";", ",", ";")
	parts := strings.Split(repl.Replace(text), ";")
	seen := map[string]bool{}
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		key := normalizeLaboratoryObjectKey(part)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func laboratoryRiskFilterCaption(code string) string {
	for i, candidate := range laboratoryRiskFilterCodes {
		if candidate == code && i < len(laboratoryRiskFilterNames) {
			return laboratoryRiskFilterNames[i]
		}
	}
	return laboratoryRiskFilterNames[0]
}

func laboratoryChartObjectLabel(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "—"
	}
	runes := []rune(key)
	if len(runes) <= 16 {
		return key
	}
	return "…" + string(runes[len(runes)-15:])
}

func laboratoryTrendValue(pass LabPassRecord, metric laboratoryTrendMetric) (float64, bool) {
	group, ok := labGroupByCode(metric.Group)
	if !ok {
		return 0, false
	}
	group = laboratoryScaleInputGroup(group, pass.ScaleModel)
	values := canonicalizeLaboratoryValues(group, pass.Groups[group.Code], pass.Units[group.Code])
	n, ok := parseLabNumber(values[metric.Key])
	return n, ok
}

type laboratoryChartPoint struct {
	Label     string
	Value     float64
	ObjectKey string
	Severity  int
	Date      string
}

func laboratoryPersonalTrendPoints(objectKey string, metric laboratoryTrendMetric, passes []LabPassRecord) []laboratoryChartPoint {
	key := normalizeLaboratoryObjectKey(objectKey)
	var rows []LabPassRecord
	for _, p := range passes {
		if p.Status == labPassSavedStatus && normalizeLaboratoryObjectKey(p.ObjectKey) == key {
			if _, ok := laboratoryTrendValue(p, metric); ok {
				rows = append(rows, p)
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		di, dj := parseDate(rows[i].Date), parseDate(rows[j].Date)
		if di.Equal(dj) {
			return rows[i].UpdatedAt < rows[j].UpdatedAt
		}
		return di.Before(dj)
	})
	out := make([]laboratoryChartPoint, 0, len(rows))
	for _, p := range rows {
		if v, ok := laboratoryTrendValue(p, metric); ok {
			out = append(out, laboratoryChartPoint{Label: p.Date, Value: v, ObjectKey: p.ObjectKey, Date: p.Date})
		}
	}
	return out
}

func laboratoryPassSeverity(pass LabPassRecord) int {
	severity := 0
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		g := laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := canonicalizeLaboratoryValues(g, pass.Groups[g.Code], pass.Units[g.Code])
		assessment := laboratoryGroupSummary(g, values, pass.ScaleModel)
		if assessment.HasData && assessment.Level > severity {
			severity = assessment.Level
		}
	}
	return severity
}

var laboratoryTrendSortNames = []string{
	"Сортировка: по объекту",
	"Сортировка: значение ↑",
	"Сортировка: значение ↓",
	"Сортировка: по выраженности признака",
}

func laboratoryGroupTrendPointsSorted(keys []string, metric laboratoryTrendMetric, passes []LabPassRecord, sortMode int) []laboratoryChartPoint {
	latest := laboratoryLatestSavedPassesForKeys(keys, passes)
	out := make([]laboratoryChartPoint, 0, len(latest))
	for _, p := range latest {
		if v, ok := laboratoryTrendValue(p, metric); ok {
			out = append(out, laboratoryChartPoint{Label: p.ObjectKey, Value: v, ObjectKey: p.ObjectKey, Severity: laboratoryPassSeverity(p), Date: p.Date})
		}
	}
	switch sortMode {
	case 1:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Value == out[j].Value {
				return strings.Compare(out[i].ObjectKey, out[j].ObjectKey) < 0
			}
			return out[i].Value < out[j].Value
		})
	case 2:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Value == out[j].Value {
				return strings.Compare(out[i].ObjectKey, out[j].ObjectKey) < 0
			}
			return out[i].Value > out[j].Value
		})
	case 3:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Severity == out[j].Severity {
				if out[i].Value == out[j].Value {
					return strings.Compare(out[i].ObjectKey, out[j].ObjectKey) < 0
				}
				return out[i].Value > out[j].Value
			}
			return out[i].Severity > out[j].Severity
		})
	default:
		sort.SliceStable(out, func(i, j int) bool { return strings.Compare(out[i].ObjectKey, out[j].ObjectKey) < 0 })
	}
	return out
}

func laboratoryGroupTrendPoints(keys []string, metric laboratoryTrendMetric, passes []LabPassRecord) []laboratoryChartPoint {
	return laboratoryGroupTrendPointsSorted(keys, metric, passes, 0)
}

func laboratoryTrendSortIndex() int {
	if labTrendSortCombo == 0 {
		return 0
	}
	index, _, _ := pSendMessageW.Call(uintptr(labTrendSortCombo), CB_GETCURSEL, 0, 0)
	if int(index) < 0 || int(index) >= len(laboratoryTrendSortNames) {
		return 0
	}
	return int(index)
}

func laboratorySelectedDetailObject() string {
	if labDetailObjectCombo == 0 {
		return ""
	}
	index, _, _ := pSendMessageW.Call(uintptr(labDetailObjectCombo), CB_GETCURSEL, 0, 0)
	if int(index) < 0 {
		return ""
	}
	buf := make([]uint16, 1024)
	pSendMessageW.Call(uintptr(labDetailObjectCombo), CB_GETLBTEXT, uintptr(index), uintptr(unsafe.Pointer(&buf[0])))
	return normalizeLaboratoryObjectKey(syscall.UTF16ToString(buf))
}

func laboratoryRefreshDetailObjects() {
	if labDetailObjectCombo == 0 {
		return
	}
	previous := laboratorySelectedDetailObject()
	keys := laboratoryScopeKeysForCurrentUI(labOverviewScopeCode, strings.TrimSpace(getText(labObjectEdit)), labPasses)
	pSendMessageW.Call(uintptr(labDetailObjectCombo), CB_RESETCONTENT, 0, 0)
	selectedIndex := 0
	for i, key := range keys {
		pSendMessageW.Call(uintptr(labDetailObjectCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(key))))
		if normalizeLaboratoryObjectKey(key) == previous {
			selectedIndex = i
		}
	}
	if len(keys) > 0 {
		pSendMessageW.Call(uintptr(labDetailObjectCombo), CB_SETCURSEL, uintptr(selectedIndex), 0)
	}
}

func laboratoryDetailLine(objectKey string, metric laboratoryTrendMetric, passes []LabPassRecord) string {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return "Детализация: выберите объект."
	}
	latest := laboratoryLatestSavedPassesForKeys([]string{key}, passes)
	if len(latest) == 0 {
		return "Детализация: " + key + " — сохранённого анализа нет."
	}
	p := latest[0]
	valueText := "нет значения"
	if v, ok := laboratoryTrendValue(p, metric); ok {
		valueText = fmt.Sprintf("%.4g %s", v, metric.Unit)
	}
	alerts := []string{}
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		g := laboratoryScaleInputGroup(group, p.ScaleModel)
		vals := canonicalizeLaboratoryValues(g, p.Groups[g.Code], p.Units[g.Code])
		a := laboratoryGroupSummary(g, vals, p.ScaleModel)
		if a.HasData && a.Level >= 2 {
			alerts = append(alerts, group.Name)
		}
	}
	alertText := "повышенных/пограничных признаков по заполненным направлениям нет"
	if len(alerts) > 0 {
		alertText = "требуют проверки: " + strings.Join(alerts, ", ")
	}
	return fmt.Sprintf("Детализация: %s · анализ %s · %s: %s · %s.", key, emptyLabText(p.Date), metric.Label, valueText, alertText)
}

func laboratoryScopeKeysForCurrentUI(scopeCode, selectedObject string, passes []LabPassRecord) []string {
	var keys []string
	if scopeCode == laboratoryScopeSelected {
		keys = laboratorySelectedObjectKeysText(getText(labScopeObjectsEdit))
	} else {
		keys = laboratoryScopeObjectKeys(scopeCode, selectedObject, passes)
	}
	if scopeCode == laboratoryScopeObject {
		return keys
	}
	return laboratoryFilterKeysByDirection(keys, passes, labOverviewRiskFilterCode)
}

func laboratoryInfographicExportSummary(scopeCode, selectedObject string, passes []LabPassRecord, metric laboratoryTrendMetric) string {
	keys := laboratoryScopeKeysForCurrentUI(scopeCode, selectedObject, passes)
	if scopeCode == laboratoryScopeObject {
		points := laboratoryPersonalTrendPoints(selectedObject, metric, passes)
		return fmt.Sprintf("ИНФОГРАФИКА / ТРЕНД\nОбласть: текущий объект %s. Показатель: %s. Точек истории: %d. Данные разных объектов не объединяются.", emptyLabText(selectedObject), metric.Label, len(points))
	}
	points := laboratoryGroupTrendPoints(keys, metric, passes)
	analytics := laboratoryMassAnalyticsForKeys(keys, passes, time.Now())
	return fmt.Sprintf(`ИНФОГРАФИКА / КОМПЛЕКСНАЯ ДИАГНОСТИКА
Область: %s. Фильтр осложнения: %s. Объектов в выборке: %d. Показатель: %s. Объектов с доступным значением: %d. Для каждого объекта используется последняя сохранённая диагностика.
%s
%s`, laboratoryScopeCaption(scopeCode, selectedObject, passes), laboratoryRiskFilterCaption(labOverviewRiskFilterCode), len(keys), metric.Label, len(points), laboratoryMassAnalyticsSummary(analytics), laboratoryMassDirectionSummary(analytics))
}

func laboratoryDrawKPIStrip(di *DRAWITEMSTRUCT, cards []laboratoryKPI, top, left, right int32) int32 {
	if di == nil || len(cards) == 0 || right <= left {
		return top
	}
	gap := int32(8)
	cardW := (right - left - gap*int32(len(cards)-1)) / int32(len(cards))
	cardH := int32(76)
	for i, card := range cards {
		x := left + int32(i)*(cardW+gap)
		r := RECT{Left: x, Top: top, Right: x + cardW, Bottom: top + cardH}
		bg, _, _ := pCreateSolidBrush.Call(rgb(245, 248, 251))
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&r)), bg)
		pDeleteObject.Call(bg)
		pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&r)), uintptr(brushBorder))
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
		pSetTextColor.Call(uintptr(di.HDC), rgb(96, 104, 112))
		labelRect := RECT{Left: r.Left + 8, Top: r.Top + 5, Right: r.Right - 8, Bottom: r.Top + 27}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(card.Label))), ^uintptr(0), uintptr(unsafe.Pointer(&labelRect)), DT_SINGLELINE|DT_NOPREFIX)
		pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
		valueRect := RECT{Left: r.Left + 8, Top: r.Top + 27, Right: r.Right - 8, Bottom: r.Top + 50}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(card.Value))), ^uintptr(0), uintptr(unsafe.Pointer(&valueRect)), DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(uintptr(di.HDC), uintptr(hFont))
		pSetTextColor.Call(uintptr(di.HDC), rgb(96, 104, 112))
		noteRect := RECT{Left: r.Left + 8, Top: r.Top + 50, Right: r.Right - 8, Bottom: r.Bottom - 4}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(card.Note))), ^uintptr(0), uintptr(unsafe.Pointer(&noteRect)), DT_WORDBREAK|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	return top + cardH
}

func laboratoryDrawDirectionBars(di *DRAWITEMSTRUCT, a laboratoryMassAnalytics, top, left, right int32) int32 {
	if di == nil || right <= left {
		return top
	}
	items := []struct{ code, label string }{
		{"corrosion", "Коррозия"}, {"scale", "Соли"}, {"aspo", "АСПО"},
		{"solids", "Мехпримеси"}, {"microbiology", "Микробиология"}, {"gases", "Газы/гидраты"},
	}
	gap := int32(10)
	colW := (right - left - gap) / 2
	rowH := int32(25)
	for i, item := range items {
		col := int32(i % 2)
		row := int32(i / 2)
		x := left + col*(colW+gap)
		y := top + row*rowH
		known, risk := a.DirectionKnown[item.code], a.DirectionRisk[item.code]
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFont))
		pSetTextColor.Call(uintptr(di.HDC), rgb(43, 46, 50))
		lr := RECT{Left: x, Top: y, Right: x + 128, Bottom: y + 21}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(item.label))), ^uintptr(0), uintptr(unsafe.Pointer(&lr)), DT_SINGLELINE|DT_NOPREFIX)
		track := RECT{Left: x + 132, Top: y + 7, Right: x + colW - 66, Bottom: y + 15}
		if track.Right > track.Left {
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&track)), uintptr(brushBorder))
			if known > 0 {
				fill := track
				fill.Right = fill.Left + (fill.Right-fill.Left)*int32(risk)/int32(known)
				b, _, _ := pCreateSolidBrush.Call(rgb(203, 27, 43))
				pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&fill)), b)
				pDeleteObject.Call(b)
			}
		}
		pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
		vr := RECT{Left: x + colW - 60, Top: y, Right: x + colW, Bottom: y + 21}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(fmt.Sprintf("%d/%d", risk, known)))), ^uintptr(0), uintptr(unsafe.Pointer(&vr)), DT_RIGHT_STIFF|DT_SINGLELINE|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	return top + 3*rowH + 4
}

func laboratoryDoseResponsePanelHeight(series laboratoryInhibitorDoseSeries, availableHeight int32) int32 {
	if len(series.Points) < 2 || availableHeight < 250 {
		return 0
	}
	if availableHeight < 340 {
		return 88
	}
	return 112
}

func drawLaboratoryDoseResponsePanel(di *DRAWITEMSTRUCT, rect RECT, series laboratoryInhibitorDoseSeries) {
	if di == nil || len(series.Points) < 2 || rect.Right-rect.Left < 320 || rect.Bottom-rect.Top < 72 {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rect)), uintptr(brushLight))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rect)), uintptr(brushBorder))
	draw := func(text string, r RECT, bold bool, flags uintptr) {
		font := hFont
		if bold {
			font = hFontBold
		}
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	draw(laboratoryInhibitorDoseSeriesCaption(series), RECT{Left: rect.Left + 8, Top: rect.Top + 4, Right: rect.Right - 8, Bottom: rect.Top + 24}, true, DT_SINGLELINE|DT_VCENTER)
	draw("Только фактические сопоставимые точки; без интерполяции и расчёта оптимальной дозы.", RECT{Left: rect.Left + 8, Top: rect.Top + 23, Right: rect.Right - 8, Bottom: rect.Top + 42}, false, DT_SINGLELINE|DT_VCENTER)
	plot := RECT{Left: rect.Left + 52, Top: rect.Top + 44, Right: rect.Right - 18, Bottom: rect.Bottom - 18}
	if plot.Bottom <= plot.Top || plot.Right <= plot.Left {
		return
	}
	minDose, maxDose := series.Points[0].Dose, series.Points[0].Dose
	for _, pt := range series.Points[1:] {
		if pt.Dose < minDose {
			minDose = pt.Dose
		}
		if pt.Dose > maxDose {
			maxDose = pt.Dose
		}
	}
	if math.Abs(maxDose-minDose) < 1e-9 {
		maxDose = minDose + 1
	}
	axisX := []POINT{{X: plot.Left, Y: plot.Bottom}, {X: plot.Right, Y: plot.Bottom}}
	axisY := []POINT{{X: plot.Left, Y: plot.Top}, {X: plot.Left, Y: plot.Bottom}}
	pen, _, _ := pCreatePenStiff.Call(PS_SOLID_STIFF, 1, rgb(96, 104, 112))
	oldPen, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&axisX[0])), uintptr(len(axisX)))
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&axisY[0])), uintptr(len(axisY)))
	for _, pt := range series.Points {
		x := plot.Left + int32((pt.Dose-minDose)/(maxDose-minDose)*float64(plot.Right-plot.Left))
		eff := math.Max(0, math.Min(100, pt.Efficiency))
		y := plot.Bottom - int32(eff/100*float64(plot.Bottom-plot.Top))
		dot := RECT{Left: x - 4, Top: y - 4, Right: x + 5, Bottom: y + 5}
		b, _, _ := pCreateSolidBrush.Call(rgb(31, 93, 145))
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&dot)), b)
		pDeleteObject.Call(b)
		label := fmt.Sprintf("%g / %.0f%%", pt.Dose, pt.Efficiency)
		draw(label, RECT{Left: x - 42, Top: y - 19, Right: x + 42, Bottom: y - 3}, false, DT_CENTER|DT_SINGLELINE)
	}
	if oldPen != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldPen)
	}
	pDeleteObject.Call(pen)
	draw("доза, мг/л", RECT{Left: plot.Right - 94, Top: plot.Bottom + 1, Right: plot.Right, Bottom: rect.Bottom - 1}, false, DT_RIGHT_STIFF|DT_SINGLELINE)
	draw("100%", RECT{Left: rect.Left + 4, Top: plot.Top - 6, Right: plot.Left - 4, Bottom: plot.Top + 10}, false, DT_RIGHT_STIFF|DT_SINGLELINE)
	draw("0%", RECT{Left: rect.Left + 4, Top: plot.Bottom - 8, Right: plot.Left - 4, Bottom: plot.Bottom + 8}, false, DT_RIGHT_STIFF|DT_SINGLELINE)
}

func drawLaboratoryTrendChart(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	metricCombo := labTrendMetricCombo
	if di.HwndItem == labInfographicTrend && labInfographicMetricCombo != 0 {
		metricCombo = labInfographicMetricCombo
	}
	metric := laboratoryTrendMetricByIndex(laboratoryComboSelection(metricCombo))
	selectedObject := strings.TrimSpace(getText(labObjectEdit))
	// RC185: heavy infographic queries now go through Repository. Personal mode
	// loads only one object's history; mass mode loads only the latest saved pass
	// per object. The full legacy/global slice remains available to older UI paths.
	var overviewPasses []LabPassRecord
	if labOverviewScopeCode == laboratoryScopeObject {
		overviewPasses = repositoryLabPassesForObject(selectedObject)
	} else {
		overviewPasses = repositoryLatestSavedLabPasses()
	}
	keys := laboratoryScopeKeysForCurrentUI(labOverviewScopeCode, selectedObject, overviewPasses)
	var pts []laboratoryChartPoint
	title := metric.Label + " — история объекта по диагностикам"
	if labOverviewScopeCode == laboratoryScopeObject {
		pts = laboratoryPersonalTrendPoints(selectedObject, metric, overviewPasses)
	} else {
		pts = laboratoryGroupTrendPointsSorted(keys, metric, overviewPasses, laboratoryTrendSortIndex())
		title = metric.Label + " — сравнение последних анализов группы"
	}
	drawText := func(s string, r RECT, bold bool, flags uintptr) {
		font := hFont
		if bold {
			font = hFontBold
		}
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	rc := di.RcItem
	drawText(title, RECT{Left: rc.Left + 10, Top: rc.Top + 6, Right: rc.Right - 10, Bottom: rc.Top + 28}, true, DT_SINGLELINE|DT_VCENTER)
	headerBottom := rc.Top + 32
	if labOverviewScopeCode == laboratoryScopeObject {
		cardsBottom := laboratoryDrawKPIStrip(di, laboratoryPersonalKPIs(selectedObject, overviewPasses), rc.Top+30, rc.Left+12, rc.Right-12)
		headerBottom = cardsBottom + 8
	} else {
		analytics := laboratoryMassAnalyticsForKeys(keys, overviewPasses, time.Now())
		cardsBottom := laboratoryDrawKPIStrip(di, laboratoryMassKPIs(analytics), rc.Top+30, rc.Left+12, rc.Right-12)
		barsBottom := laboratoryDrawDirectionBars(di, analytics, cardsBottom+8, rc.Left+12, rc.Right-12)
		attention := laboratoryAttentionForKeys(keys, overviewPasses, time.Now())
		attentionRect := RECT{Left: rc.Left + 12, Top: barsBottom + 4, Right: rc.Right - 12, Bottom: barsBottom + 38}
		drawText(laboratoryAttentionCaption(attention), attentionRect, true, DT_WORDBREAK)
		detailRect := RECT{Left: rc.Left + 12, Top: barsBottom + 40, Right: rc.Right - 12, Bottom: barsBottom + 78}
		drawText(laboratoryDetailLine(laboratorySelectedDetailObject(), metric, overviewPasses), detailRect, false, DT_WORDBREAK)
		headerBottom = barsBottom + 82
	}
	// In the approved individual infographic the lower panel is a compact
	// history summary. Line dependencies are built only in Complex diagnostics,
	// where the user deliberately selects comparable dates and objects.
	if di.HwndItem == labInfographicTrend {
		drawText("Линейные зависимости по выбранным датам доступны в «Комплексной диагностике». Сводные показатели выше показаны полностью.", RECT{Left: rc.Left + 12, Top: headerBottom + 8, Right: rc.Right - 12, Bottom: rc.Bottom - 12}, false, DT_WORDBREAK)
		return
	}
	var doseSeries laboratoryInhibitorDoseSeries
	hasDoseSeries := false
	if labOverviewScopeCode == laboratoryScopeObject {
		doseSeries, hasDoseSeries = laboratoryInhibitorDoseSeriesForObject(selectedObject, overviewPasses, ensureActiveLabPass())
	}
	doseH := int32(0)
	if hasDoseSeries {
		doseH = laboratoryDoseResponsePanelHeight(doseSeries, rc.Bottom-headerBottom)
	}
	// One point cannot show a trend or group comparison. Keep the useful
	// summary cards above and explain what additional analysis is needed.
	if len(pts) < 2 {
		if doseH > 0 {
			panel := RECT{Left: rc.Left + 12, Top: headerBottom + 8, Right: rc.Right - 12, Bottom: headerBottom + 8 + doseH}
			drawLaboratoryDoseResponsePanel(di, panel, doseSeries)
			return
		}
		panel := RECT{Left: rc.Left + 12, Top: headerBottom + 8, Right: rc.Right - 12, Bottom: rc.Bottom - 12}
		if panel.Bottom <= panel.Top+28 {
			drawText(fmt.Sprintf("Для графика «%s»: %d из %d диагностик со значением; нужны две.", metric.Label, len(pts), len(overviewPasses)), RECT{Left: rc.Left + 12, Top: headerBottom, Right: rc.Right - 12, Bottom: rc.Bottom - 4}, true, DT_SINGLELINE|DT_VCENTER)
			return
		}
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&panel)), uintptr(brushLight))
		pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&panel)), uintptr(brushBorder))
		inner := RECT{Left: panel.Left + 18, Top: panel.Top + 8, Right: panel.Right - 18, Bottom: panel.Bottom - 8}
		drawText(fmt.Sprintf("%s: значение есть в %d из %d сохранённых диагностик", metric.Label, len(pts), len(overviewPasses)), RECT{Left: inner.Left, Top: inner.Top, Right: inner.Right, Bottom: inner.Top + 26}, true, DT_SINGLELINE|DT_VCENTER)
		if inner.Bottom-inner.Top >= 60 {
			message := "Для графика нужны значения в двух диагностиках. Выберите другой показатель или загрузите ещё один анализ."
			if len(overviewPasses) == 0 {
				message = "Сохраните анализ для этого объекта, затем загрузите следующую диагностику."
			}
			drawText(message, RECT{Left: inner.Left, Top: inner.Top + 30, Right: inner.Right, Bottom: inner.Bottom}, false, DT_WORDBREAK)
		}
		return
	}
	minV, maxV := pts[0].Value, pts[0].Value
	for _, pt := range pts[1:] {
		if pt.Value < minV {
			minV = pt.Value
		}
		if pt.Value > maxV {
			maxV = pt.Value
		}
	}
	threshold, hasThreshold := laboratoryMetricThreshold(metric.Group, metric.Key)
	if hasThreshold {
		if threshold.value < minV {
			minV = threshold.value
		}
		if threshold.value > maxV {
			maxV = threshold.value
		}
	}
	if math.Abs(maxV-minV) < 1e-12 {
		maxV = minV + 1
	}
	plotBottomMargin := int32(54)
	if labOverviewScopeCode != laboratoryScopeObject {
		plotBottomMargin = 86
	}
	plot := RECT{Left: rc.Left + 54, Top: headerBottom + 6, Right: rc.Right - 20, Bottom: rc.Bottom - plotBottomMargin - doseH}
	if plot.Bottom-plot.Top < 70 {
		drawText("Недостаточно места для корректного графика. Сводные показатели выше показаны полностью.", RECT{Left: rc.Left + 12, Top: headerBottom + 8, Right: rc.Right - 12, Bottom: rc.Bottom - 42}, false, DT_WORDBREAK)
		return
	}
	pen, _, _ := pCreatePenStiff.Call(PS_SOLID_STIFF, 1, rgb(31, 93, 145))
	oldPen, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
	axis1 := []POINT{{X: plot.Left, Y: plot.Bottom}, {X: plot.Right, Y: plot.Bottom}}
	axis2 := []POINT{{X: plot.Left, Y: plot.Top}, {X: plot.Left, Y: plot.Bottom}}
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&axis1[0])), uintptr(len(axis1)))
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&axis2[0])), uintptr(len(axis2)))
	if hasThreshold {
		y := plot.Bottom - int32((threshold.value-minV)/(maxV-minV)*float64(plot.Bottom-plot.Top))
		boundaryPen, _, _ := pCreatePenStiff.Call(1, 2, rgb(195, 39, 54))
		previous, _, _ := pSelectObject.Call(uintptr(di.HDC), boundaryPen)
		boundary := []POINT{{X: plot.Left, Y: y}, {X: plot.Right, Y: y}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&boundary[0])), 2)
		pSelectObject.Call(uintptr(di.HDC), previous)
		pDeleteObject.Call(boundaryPen)
		// Place the caption at the threshold line on the right. This guarantees
		// that it cannot cover the maximum value printed next to the Y axis.
		drawText(threshold.caption(metric.Unit), RECT{Left: max32(plot.Left+120, plot.Right-360), Top: y - 22, Right: plot.Right - 6, Bottom: y - 2}, true, DT_RIGHT_STIFF|DT_SINGLELINE)
	}
	count := len(pts)
	prevX, prevY := int32(0), int32(0)
	for i, pt := range pts {
		x := plot.Left
		if count > 1 {
			x += int32(i) * (plot.Right - plot.Left) / int32(count-1)
		}
		y := plot.Bottom - int32((pt.Value-minV)/(maxV-minV)*float64(plot.Bottom-plot.Top))
		if labOverviewScopeCode == laboratoryScopeObject && i > 0 {
			line := []POINT{{X: prevX, Y: prevY}, {X: x, Y: y}}
			pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), uintptr(len(line)))
		}
		dot := RECT{Left: x - 3, Top: y - 3, Right: x + 4, Bottom: y + 4}
		pointColor := rgb(31, 93, 145)
		if hasThreshold && threshold.exceeded(pt.Value) {
			pointColor = rgb(195, 39, 54)
		}
		b, _, _ := pCreateSolidBrush.Call(pointColor)
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&dot)), b)
		pDeleteObject.Call(b)
		if labOverviewScopeCode != laboratoryScopeObject {
			step := 1
			if count > 12 {
				step = int(math.Ceil(float64(count) / 12.0))
			}
			if i%step == 0 || i == count-1 {
				labelW := int32(96)
				labelRect := RECT{Left: x - labelW/2, Top: plot.Bottom + 4, Right: x + labelW/2, Bottom: plot.Bottom + 34}
				drawText(laboratoryChartObjectLabel(pt.ObjectKey), labelRect, false, DT_CENTER|DT_WORDBREAK)
			}
		}
		prevX, prevY = x, y
	}
	if oldPen != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldPen)
	}
	pDeleteObject.Call(pen)
	drawText(fmt.Sprintf("%.3g", maxV), RECT{Left: rc.Left + 4, Top: plot.Top - 8, Right: plot.Left - 6, Bottom: plot.Top + 14}, false, DT_RIGHT_STIFF|DT_SINGLELINE)
	drawText(fmt.Sprintf("%.3g", minV), RECT{Left: rc.Left + 4, Top: plot.Bottom - 10, Right: plot.Left - 6, Bottom: plot.Bottom + 12}, false, DT_RIGHT_STIFF|DT_SINGLELINE)
	if doseH > 0 {
		panelBottom := rc.Bottom - plotBottomMargin + 4
		panelTop := panelBottom - doseH
		drawLaboratoryDoseResponsePanel(di, RECT{Left: rc.Left + 12, Top: panelTop, Right: rc.Right - 12, Bottom: panelBottom}, doseSeries)
	}
	footer := fmt.Sprintf("%d точек · %s", len(pts), metric.Unit)
	if hasThreshold {
		count := 0
		for _, pt := range pts {
			if threshold.exceeded(pt.Value) {
				count++
			}
		}
		footer += fmt.Sprintf(" · за рабочей границей: %d", count)
	} else {
		footer += " · числовой рабочей границы нет"
	}
	if labOverviewScopeCode != laboratoryScopeObject {
		footer += " · одна последняя точка на объект"
	}
	drawText(footer, RECT{Left: rc.Left + 12, Top: rc.Bottom - 34, Right: rc.Right - 12, Bottom: rc.Bottom - 8}, false, DT_SINGLELINE|DT_VCENTER)
}
