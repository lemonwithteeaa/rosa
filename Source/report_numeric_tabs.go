//go:build windows

package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Read values exclusively from the immutable approved section snapshot.
// A missing model input must stay blank rather than turning into a fabricated 0.
func reportSnapshotNumber(section, pattern string) (float64, bool) {
	re := regexp.MustCompile(pattern)
	found := re.FindStringSubmatch(section)
	if len(found) < 2 {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(found[1]), ",", "."), 64)
	return v, err == nil
}

func reportSnapshotNumberCell(ref string, v float64, present bool) string {
	if !present {
		return xlsxInlineCell(ref, "не сохранено", 0)
	}
	return xlsxNumberCell(ref, v, 4)
}

func reportScenarioTabXML(r FullReportRecord) string {
	section := fullReportSection(r, "section_5")
	read := func(pattern string) (float64, bool) { return reportSnapshotNumber(section, pattern) }
	base, baseOK := read(`(?m)^Базовая оценка без нового мероприятия:\s*([+-]?\d+(?:[.,]\d+)?)\s*сут`)
	low, lowOK := read(`(?m)^Сдержанный сценарий:\s*([+-]?\d+(?:[.,]\d+)?)\s*сут`)
	mid, midOK := read(`(?m)^Расчётный сценарий:\s*([+-]?\d+(?:[.,]\d+)?)\s*сут`)
	high, highOK := read(`(?m)^Благоприятный сценарий:\s*([+-]?\d+(?:[.,]\d+)?)\s*сут`)
	goal, goalOK := read(`(?m)^Целевой ориентир:\s*([+-]?\d+(?:[.,]\d+)?)\s*сут`)
	if !midOK && r.ForecastDays > 0 {
		mid, midOK = r.ForecastDays, true
	}
	var rows strings.Builder
	add := func(row, height int, cells ...string) {
		fmt.Fprintf(&rows, `<row r="%d" ht="%d" customHeight="1">%s</row>`, row, height, strings.Join(cells, ""))
	}
	add(1, 32, xlsxInlineCell("A1", "НАРАБОТКА · РАСЧЁТ СЦЕНАРИЕВ", 5))
	add(2, 24, xlsxInlineCell("A2", "Объект", 3), xlsxInlineCell("B2", r.ObjectLabel, 0))
	add(3, 36, xlsxInlineCell("A3", "База и сценарии — сохранённые выходы прогноза. Изменения к базе и цели вычисляет Excel; полная статистическая модель требует историю отказов.", 0))
	add(5, 32, xlsxInlineCell("A5", "Сценарий", 2), xlsxInlineCell("B5", "Наработка, сут.", 2), xlsxInlineCell("C5", "Прирост к базе, сут.", 2), xlsxInlineCell("D5", "Прирост к базе, %", 2), xlsxInlineCell("E5", "Отклонение от цели, сут.", 2), xlsxInlineCell("F5", "Тип значения", 2), xlsxInlineCell("G5", "Смысл", 2))
	type scenario struct {
		row   int
		name  string
		value float64
		ok    bool
		note  string
	}
	items := []scenario{{6, "Базовая оценка", base, baseOK, "Без нового мероприятия"}, {7, "Сдержанный", low, lowOK, "Неблагоприятная реализация"}, {8, "Расчётный", mid, midOK, "Центральный вариант"}, {9, "Благоприятный", high, highOK, "При выполнении условий"}}
	for _, it := range items {
		n := strconv.Itoa(it.row)
		cells := []string{xlsxInlineCell("A"+n, it.name, 3), reportSnapshotNumberCell("B"+n, it.value, it.ok)}
		if it.row != 6 && it.ok && baseOK && base != 0 {
			cells = append(cells, calculationFormulaCell("C"+n, "B"+n+"-$B$6", it.value-base, 8), calculationFormulaCell("D"+n, "IFERROR(B"+n+"/$B$6-1,0)", it.value/base-1, 10))
		}
		if it.ok && goalOK {
			cells = append(cells, calculationFormulaCell("E"+n, "B"+n+"-$B$11", it.value-goal, 8))
		}
		cells = append(cells, xlsxInlineCell("F"+n, "Оценка модели", 0), xlsxInlineCell("G"+n, it.note, 0))
		add(it.row, 26, cells...)
	}
	add(11, 28, xlsxInlineCell("A11", "Целевой ориентир, сут.", 3), reportSnapshotNumberCell("B11", goal, goalOK), xlsxInlineCell("G11", "Управленческий ориентир, не прогноз", 0))
	if baseOK && midOK && base != 0 && goalOK {
		add(13, 29, xlsxInlineCell("A13", "Проверка достижения цели", 3), calculationFormulaCell("B13", "IF(B8>=B11,1,0)", func() float64 {
			if mid >= goal {
				return 1
			}
			return 0
		}(), 8), xlsxInlineCell("G13", "1 — расчётный сценарий достиг цели; 0 — не достиг", 0))
	}
	add(16, 30, xlsxInlineCell("A16", "ИСТОЧНИК И ОГРАНИЧЕНИЯ", 2))
	add(17, 60, xlsxInlineCell("A17", "Для базового значения и трёх сценариев здесь не создаётся формула расчёта надёжности по неполным данным: это зафиксированные выходы модели из утверждённой записи. Excel пересчитывает только их арифметические производные.", 0))
	add(19, 24, xlsxInlineCell("A19", "Источник", 3), xlsxInlineCell("B19", "Раздел 5 сохранённого полного отчёта", 0))
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="5" topLeftCell="A6" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="32" customWidth="1"/><col min="2" max="2" width="21" customWidth="1"/><col min="3" max="5" width="26" customWidth="1"/><col min="6" max="6" width="23" customWidth="1"/><col min="7" max="7" width="57" customWidth="1"/></cols><sheetData>` + rows.String() + `</sheetData><mergeCells count="4"><mergeCell ref="A1:G1"/><mergeCell ref="A3:G3"/><mergeCell ref="A16:G16"/><mergeCell ref="A17:G17"/></mergeCells></worksheet>`
}

func reportEconomyTabXML(r FullReportRecord) string {
	section := fullReportSection(r, "section_6")
	capex, capOK := reportSnapshotNumber(section, `CAPEX:\s*([+-]?\d+(?:[.,]\d+)?)\s*млн`)
	opex, opOK := reportSnapshotNumber(section, `OPEX:\s*([+-]?\d+(?:[.,]\d+)?)\s*млн`)
	gross, grossOK := reportSnapshotNumber(section, `валовая выгода\s*=\s*чистая выгода\s*\+\s*затраты\s*=\s*([+-]?\d+(?:[.,]\d+)?)\s*млн`)
	net, netOK := reportSnapshotNumber(section, `Чистая выгода:\s*([+-]?\d+(?:[.,]\d+)?)\s*млн`)
	bcr, bcrOK := reportSnapshotNumber(section, `Benefit/Cost:\s*([+-]?\d+(?:[.,]\d+)?)`)
	payback, payOK := reportSnapshotNumber(section, `окупаемость:\s*([+-]?\d+(?:[.,]\d+)?)\s*мес`)
	if !netOK && r.NetBenefitK != 0 {
		net, netOK = r.NetBenefitK/1000, true
	}
	// Old records may not contain a separate gross number. Derive it strictly
	// from stored model net and stored first-year expenses, mark as a reconstruction.
	derivedGross := !grossOK && netOK && capOK && opOK
	if derivedGross {
		gross, grossOK = net+capex+opex, true
	}
	var rows strings.Builder
	add := func(n, h int, cells ...string) {
		fmt.Fprintf(&rows, `<row r="%d" ht="%d" customHeight="1">%s</row>`, n, h, strings.Join(cells, ""))
	}
	add(1, 32, xlsxInlineCell("A1", "ЭКОНОМИКА · ПРОВЕРЯЕМЫЕ ПОКАЗАТЕЛИ", 5))
	add(2, 24, xlsxInlineCell("A2", "Объект", 3), xlsxInlineCell("B2", r.ObjectLabel, 0))
	add(3, 39, xlsxInlineCell("A3", "Затраты первого года и проверочная чистая выгода связаны формулами Excel. Расчёт добычи, цен и денежного потока без сохранённых первичных входов не воспроизводится.", 0))
	add(5, 34, xlsxInlineCell("A5", "Показатель", 2), xlsxInlineCell("B5", "Значение", 2), xlsxInlineCell("C5", "Ед. измерения", 2), xlsxInlineCell("D5", "Источник", 2), xlsxInlineCell("E5", "Формула / контроль", 2))
	add(6, 28, xlsxInlineCell("A6", "CAPEX", 3), reportSnapshotNumberCell("B6", capex, capOK), xlsxInlineCell("C6", "млн руб. (разово)", 0), xlsxInlineCell("D6", "Снимок экономики", 0))
	add(7, 28, xlsxInlineCell("A7", "OPEX", 3), reportSnapshotNumberCell("B7", opex, opOK), xlsxInlineCell("C7", "млн руб./год", 0), xlsxInlineCell("D7", "Снимок экономики", 0))
	if capOK && opOK {
		add(8, 28, xlsxInlineCell("A8", "Затраты первого года", 3), calculationFormulaCell("B8", "SUM(B6:B7)", capex+opex, 8), xlsxInlineCell("C8", "млн руб.", 0), xlsxInlineCell("D8", "Формула Excel", 0), xlsxInlineCell("E8", "=CAPEX+OPEX", 0))
	}
	if grossOK {
		note := "Оценка из сохранённой модели"
		if derivedGross {
			note = "Восстановлено: чистая выгода + затраты"
		}
		add(10, 28, xlsxInlineCell("A10", "Валовая выгода", 3), reportSnapshotNumberCell("B10", gross, true), xlsxInlineCell("C10", "млн руб.", 0), xlsxInlineCell("D10", note, 0))
	}
	if grossOK && capOK && opOK {
		add(11, 28, xlsxInlineCell("A11", "Чистая выгода · проверка", 3), calculationFormulaCell("B11", "B10-B8", gross-capex-opex, 8), xlsxInlineCell("C11", "млн руб.", 0), xlsxInlineCell("D11", "Формула Excel", 0), xlsxInlineCell("E11", "=Валовая выгода−затраты", 0))
	} else if netOK {
		add(11, 28, xlsxInlineCell("A11", "Чистая выгода модели", 3), reportSnapshotNumberCell("B11", net, true), xlsxInlineCell("C11", "млн руб.", 0), xlsxInlineCell("D11", "Снимок экономики", 0))
	}
	if netOK {
		add(12, 28, xlsxInlineCell("A12", "Чистая выгода · сохранено", 3), reportSnapshotNumberCell("B12", net, true), xlsxInlineCell("C12", "млн руб.", 0), xlsxInlineCell("D12", "Исходный результат модели", 0))
		if grossOK && capOK && opOK {
			add(13, 28, xlsxInlineCell("A13", "Разница проверки", 3), calculationFormulaCell("B13", "B11-B12", gross-capex-opex-net, 8), xlsxInlineCell("C13", "млн руб.", 0), xlsxInlineCell("E13", "Из-за округления снимка до 0,01 млн может отличаться от 0", 0))
		}
	}
	if capOK && opOK && capex+opex > 0 {
		add(15, 28, xlsxInlineCell("A15", "Доля OPEX в затратах 1 года", 3), calculationFormulaCell("B15", "IFERROR(B7/B8,0)", opex/(capex+opex), 10), xlsxInlineCell("C15", "доля", 0), xlsxInlineCell("D15", "Формула Excel", 0))
	}
	if bcrOK {
		add(17, 28, xlsxInlineCell("A17", "Benefit/Cost модели", 3), reportSnapshotNumberCell("B17", bcr, true), xlsxInlineCell("C17", "коэффициент", 0), xlsxInlineCell("D17", "Сохранённый показатель; входы расчёта недоступны", 0))
	}
	if payOK {
		add(18, 28, xlsxInlineCell("A18", "Окупаемость модели", 3), reportSnapshotNumberCell("B18", payback, true), xlsxInlineCell("C18", "мес.", 0), xlsxInlineCell("D18", "Сохранённый показатель; без выдуманного пересчёта", 0))
	}
	add(21, 30, xlsxInlineCell("A21", "ОГРАНИЧЕНИЕ РАСЧЁТА", 2))
	add(22, 62, xlsxInlineCell("A22", "Показатели Benefit/Cost и окупаемости нельзя достоверно пересчитать по одному CAPEX/OPEX: требуются цена, дополнительная добыча, горизонт, налоги и методика дисконтирования. До появления таких исходников показаны утверждённые оценки модели.", 0))
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="5" topLeftCell="A6" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="38" customWidth="1"/><col min="2" max="2" width="25" customWidth="1"/><col min="3" max="3" width="24" customWidth="1"/><col min="4" max="4" width="50" customWidth="1"/><col min="5" max="5" width="67" customWidth="1"/></cols><sheetData>` + rows.String() + `</sheetData><mergeCells count="4"><mergeCell ref="A1:E1"/><mergeCell ref="A3:E3"/><mergeCell ref="A21:E21"/><mergeCell ref="A22:E22"/></mergeCells></worksheet>`
}
