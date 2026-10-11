//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type infographicExcelMetric struct {
	Label, Group, Key, Unit, Implication string
}

func allInfographicExcelMetrics() []infographicExcelMetric {
	metrics := make([]infographicExcelMetric, 0, len(complexMetrics)+len(laboratoryTrendMetrics))
	seen := map[string]bool{}
	for _, metric := range complexMetrics {
		metrics = append(metrics, infographicExcelMetric{metric.label, metric.group, metric.key, metric.unit, metric.implication})
		seen[metric.group+"|"+metric.key] = true
	}
	for _, metric := range laboratoryTrendMetrics {
		key := metric.Group + "|" + metric.Key
		if seen[key] {
			continue
		}
		metrics = append(metrics, infographicExcelMetric{metric.Label, metric.Group, metric.Key, metric.Unit, ""})
		seen[key] = true
	}
	return metrics
}

func infographicExcelAutoPath() (string, error) {
	dir := rosaDesktopDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "РОСА_графики_ДЛЯ_ПРОВЕРКИ_И_ИЗМЕНЕНИЙ_"+time.Now().Format("20060102_150405")+".xlsx"), nil
}

// Exports are intentionally detached working copies.  A chemist may edit and
// save them, but those edits are never read back into the application.
func rosaDesktopDir() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		for _, candidate := range []string{
			filepath.Join(home, "Desktop"),
			filepath.Join(home, "Рабочий стол"),
			filepath.Join(home, "OneDrive", "Desktop"),
		} {
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				return candidate
			}
		}
		return filepath.Join(home, "Desktop")
	}
	return filepath.Join(appRootDir(), "exports")
}

func notifyDetachedExcelCopy(path string) {
	msg("Рабочая копия Excel", "Исходные данные и расчёты в РОСА остаются без изменений.\n\nExcel-файл сохранён как отдельная рабочая копия на рабочем столе:\n"+path+"\n\nИзменённые или удалённые расчётные формулы будут подсвечены в файле. Эти правки не загружаются обратно в программу.", MB_OK|MB_ICONWARNING)
}

func exportComplexInfographicExcel(passes []LabPassRecord) {
	passes = append([]LabPassRecord(nil), passes...)
	go exportComplexInfographicExcelWorker(passes)
}

func exportComplexInfographicExcelWorker(passes []LabPassRecord) {
	result := complexExportResult{}
	path, err := infographicExcelAutoPath()
	if err == nil {
		trendType := laboratoryComboSelection(complexTrendTypeCombo)
		err = writeInfographicDependenciesXLSX(path, passes, complexTrendEnabled, trendType)
	}
	result.path, result.err = path, err
	complexExcelDone <- result
	if complexWindowHandle != 0 {
		pPostMessageW.Call(uintptr(complexWindowHandle), wmComplexExcelDone, 0, 0)
	}
}

func exportLaboratoryInfographicExcel() {
	go exportLaboratoryInfographicExcelWorker()
}

func exportLaboratoryInfographicExcelWorker() {
	objectKey := strings.TrimSpace(getText(labObjectEdit))
	if objectKey == "" {
		objectKey = strings.TrimSpace(selectedObjectKey)
	}
	passes := repositoryLabPassesForObject(objectKey)
	if len(passes) == 0 {
		msg("Экспорт инфографики", "Для выбранного объекта нет сохранённых диагностик.", MB_OK|MB_ICONWARNING)
		return
	}
	path, err := infographicExcelAutoPath()
	if err != nil {
		msg("Графики в Excel", "Не удалось подготовить файл Excel: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	if err = writeInfographicDependenciesXLSX(path, passes, false, 0); err != nil {
		msg("Графики в Excel", "Не удалось создать Excel: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	notifyDetachedExcelCopy(path)
	openExternal(path)
}

func excelColumnName(index int) string {
	if index < 1 {
		return "A"
	}
	var out string
	for index > 0 {
		index--
		out = string(rune('A'+index%26)) + out
		index /= 26
	}
	return out
}

func infographicSheetName(index int, label string) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\\`, r) || r < 32 {
			return ' '
		}
		return r
	}, strings.TrimSpace(label))
	name := fmt.Sprintf("%02d %s", index, clean)
	runes := []rune(name)
	if len(runes) > 31 {
		name = string(runes[:31])
	}
	return name
}

func excelSheetFormulaName(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func infographicTrendlineXML(enabled bool, trendType int) string {
	if !enabled {
		return ""
	}
	style := `<c:spPr><a:ln w="44450"><a:solidFill><a:srgbClr val="CB1B2B"/></a:solidFill></a:ln></c:spPr>`
	switch trendType {
	case 1:
		return `<c:trendline>` + style + `<c:trendlineType val="log"/></c:trendline>`
	case 2:
		return `<c:trendline>` + style + `<c:trendlineType val="exp"/></c:trendline>`
	case 3:
		return `<c:trendline>` + style + `<c:trendlineType val="poly"/><c:order val="2"/></c:trendline>`
	case 4:
		return `<c:trendline>` + style + `<c:trendlineType val="movingAvg"/><c:period val="2"/></c:trendline>`
	default:
		return `<c:trendline>` + style + `<c:trendlineType val="linear"/></c:trendline>`
	}
}

// Excel may open a hand-built chart before it has recalculated external cell
// references. Embedding the point cache is therefore essential: the diagram is
// visible immediately on entry and remains linked to the underlying cells.
func infographicNumberCacheXML(values []float64, formatCode string) string {
	var b strings.Builder
	b.WriteString(`<c:numCache><c:formatCode>` + reportXMLText(formatCode) + `</c:formatCode><c:ptCount val="` + strconv.Itoa(len(values)) + `"/>`)
	for i, value := range values {
		b.WriteString(`<c:pt idx="` + strconv.Itoa(i) + `"><c:v>` + strconv.FormatFloat(value, 'g', -1, 64) + `</c:v></c:pt>`)
	}
	b.WriteString(`</c:numCache>`)
	return b.String()
}

func infographicChartTitle(value string) string {
	return `<c:title><c:tx><c:rich><a:bodyPr/><a:p><a:pPr><a:defRPr/></a:pPr><a:r><a:t>` + reportXMLText(value) + `</a:t></a:r></a:p></c:rich></c:tx></c:title>`
}

func infographicExcelDateSerial(value string) (float64, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "02.01.2006 15:04", "02.01.2006", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, strings.TrimSpace(value), time.Local); err == nil {
			base := time.Date(1899, 12, 30, 0, 0, 0, 0, parsed.Location())
			return parsed.Sub(base).Hours() / 24, true
		}
	}
	return 0, false
}

func infographicExcelAxisBounds(dates []string, series [][]complexValue) (float64, float64, float64, float64) {
	xMin, xMax := 0.0, 0.0
	for _, date := range dates {
		serial, ok := infographicExcelDateSerial(date)
		if !ok {
			continue
		}
		if xMin == 0 || serial < xMin {
			xMin = serial
		}
		if xMax == 0 || serial > xMax {
			xMax = serial
		}
	}
	if xMin == xMax {
		xMin--
		xMax++
	}
	yMin, yMax := 0.0, 0.0
	hasY := false
	for _, values := range series {
		for _, value := range values {
			if !hasY || value.value < yMin {
				yMin = value.value
			}
			if !hasY || value.value > yMax {
				yMax = value.value
			}
			hasY = true
		}
	}
	if !hasY {
		yMin, yMax = 0, 1
	} else if yMin == yMax {
		padding := yMin * 0.1
		if padding < 0 {
			padding = -padding
		}
		if padding == 0 {
			padding = 1
		}
		yMin -= padding
		yMax += padding
	} else {
		padding := (yMax - yMin) * 0.08
		yMin -= padding
		yMax += padding
	}
	return xMin, xMax, yMin, yMax
}

func writeInfographicDependenciesXLSX(path string, passes []LabPassRecord, trendEnabled bool, trendType int) error {
	if len(passes) == 0 {
		return fmt.Errorf("нет выбранных диагностик")
	}
	metrics := allInfographicExcelMetrics()
	groups := complexPassGroups(passes)
	if len(groups) == 0 {
		return fmt.Errorf("не распознаны объекты")
	}

	files := map[string]string{}
	var contentTypes, workbookSheets, workbookRels, summaryRows strings.Builder
	contentTypes.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`)
	workbookSheets.WriteString(`<sheet name="Свод" sheetId="1" r:id="rId1"/>`)
	workbookRels.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>`)
	summaryRows.WriteString(`<row r="1" ht="30" customHeight="1">` + xlsxInlineCell("A1", "РОСА — зависимости инфографики", 1) + `</row>`)
	summaryRows.WriteString(`<row r="2">` + xlsxInlineCell("A2", "Дата экспорта", 2) + xlsxInlineCell("B2", time.Now().Format("02.01.2006 15:04"), 0) + `</row>`)
	summaryRows.WriteString(`<row r="3" ht="34" customHeight="1">` + xlsxInlineCell("A3", "РАБОЧАЯ КОПИЯ", 4) + xlsxInlineCell("B3", "Изменения в этом Excel-файле не загружаются обратно в РОСА.", 2) + `</row>`)
	summaryRows.WriteString(`<row r="4">` + xlsxInlineCell("A4", "№", 3) + xlsxInlineCell("B4", "Показатель", 3) + xlsxInlineCell("C4", "Единица", 3) + xlsxInlineCell("D4", "Объектов с данными", 3) + xlsxInlineCell("E4", "Точек", 3) + xlsxInlineCell("F4", "Лист", 3) + `</row>`)

	for metricIndex, metric := range metrics {
		sheetID := metricIndex + 2
		sheetName := infographicSheetName(metricIndex+1, metric.Label)
		workbookSheets.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, reportXMLText(sheetName), sheetID, sheetID))
		workbookRels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, sheetID, sheetID))
		contentTypes.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, sheetID))

		metricDef := complexMetric{label: metric.Label, group: metric.Group, key: metric.Key, unit: metric.Unit, implication: metric.Implication}
		series := make([][]complexValue, len(groups))
		chartSeriesValues := make([][]complexValue, len(groups))
		allDates := map[string]bool{}
		points, objectsWithData, chartableSeries := 0, 0, 0
		for i, group := range groups {
			series[i] = complexMetricValues(group, metricDef)
			if len(series[i]) > 0 {
				objectsWithData++
			}
			for _, value := range series[i] {
				if _, validDate := infographicExcelDateSerial(value.date); validDate {
					chartSeriesValues[i] = append(chartSeriesValues[i], value)
				}
			}
			if len(chartSeriesValues[i]) >= 2 {
				chartableSeries++
			}
			points += len(series[i])
			for _, value := range series[i] {
				if _, validDate := infographicExcelDateSerial(value.date); validDate {
					allDates[value.date] = true
				}
			}
		}
		dates := make([]string, 0, len(allDates))
		for date := range allDates {
			dates = append(dates, date)
		}
		sort.Slice(dates, func(i, j int) bool { return parseDate(dates[i]).Before(parseDate(dates[j])) })

		summaryRow := metricIndex + 5
		summaryRows.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s%s%s</row>`, summaryRow,
			xlsxNumberCell("A"+strconv.Itoa(summaryRow), float64(metricIndex+1), 0),
			xlsxInlineCell("B"+strconv.Itoa(summaryRow), metric.Label, 0),
			xlsxInlineCell("C"+strconv.Itoa(summaryRow), metric.Unit, 0),
			xlsxNumberCell("D"+strconv.Itoa(summaryRow), float64(objectsWithData), 0),
			xlsxNumberCell("E"+strconv.Itoa(summaryRow), float64(points), 0),
			xlsxInlineCell("F"+strconv.Itoa(summaryRow), sheetName, 0)))

		var rows strings.Builder
		lastColumn := len(groups) + 1
		threshold, hasThreshold := laboratoryMetricThreshold(metric.Group, metric.Key)
		rows.WriteString(`<row r="1" ht="32" customHeight="1">` + xlsxInlineCell("A1", metric.Label, 1))
		if hasThreshold {
			rows.WriteString(xlsxInlineCell("B1", strings.ToUpper(threshold.caption(metric.Unit)), 6))
		}
		rows.WriteString(`</row>`)
		rows.WriteString(`<row r="2">` + xlsxInlineCell("A2", "Единица", 2) + xlsxInlineCell("B2", metric.Unit, 0) + `</row>`)
		if metric.Implication != "" {
			rows.WriteString(`<row r="3" ht="30" customHeight="1">` + xlsxInlineCell("A3", "Смысл", 2) + xlsxInlineCell("B3", metric.Implication, 0) + `</row>`)
		}
		rows.WriteString(`<row r="4" ht="30" customHeight="1">` + xlsxInlineCell("A4", "Дата", 3))
		for i, group := range groups {
			name := readableObjectKey(group[0].ObjectKey)
			rows.WriteString(xlsxInlineCell(excelColumnName(i+2)+"4", name, 3))
		}
		rows.WriteString(`</row>`)
		valueMaps := make([]map[string]float64, len(series))
		for i := range series {
			valueMaps[i] = map[string]float64{}
			for _, value := range series[i] {
				valueMaps[i][value.date] = value.value
			}
		}
		for rowOffset, date := range dates {
			row := rowOffset + 5
			dateCell := xlsxInlineCell("A"+strconv.Itoa(row), date, 0)
			if serial, ok := infographicExcelDateSerial(date); ok {
				dateCell = xlsxNumberCell("A"+strconv.Itoa(row), serial, 5)
			}
			rows.WriteString(fmt.Sprintf(`<row r="%d">`, row) + dateCell)
			for seriesIndex := range groups {
				if value, ok := valueMaps[seriesIndex][date]; ok {
					rows.WriteString(xlsxNumberCell(excelColumnName(seriesIndex+2)+strconv.Itoa(row), value, 0))
				}
			}
			rows.WriteString(`</row>`)
		}
		if len(dates) == 0 {
			rows.WriteString(`<row r="5">` + xlsxInlineCell("A5", "По выбранным диагностикам нет числовых данных.", 0) + `</row>`)
		}

		drawingTag := ""
		if len(dates) > 0 && chartableSeries > 0 {
			drawingTag = `<drawing r:id="rId1"/>`
			contentTypes.WriteString(fmt.Sprintf(`<Override PartName="/xl/drawings/drawing%d.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/><Override PartName="/xl/charts/chart%d.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`, sheetID, sheetID))
			files[fmt.Sprintf("xl/worksheets/_rels/sheet%d.xml.rels", sheetID)] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="/xl/drawings/drawing%d.xml" Id="rId1"/></Relationships>`, sheetID)
			files[fmt.Sprintf("xl/drawings/drawing%d.xml", sheetID)] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"><xdr:oneCellAnchor><xdr:from><xdr:col>%d</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>6</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from><xdr:ext cx="10800000" cy="6000000"/><xdr:graphicFrame><xdr:nvGraphicFramePr><xdr:cNvPr id="1" name="%s"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><xdr:xfrm/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:oneCellAnchor></xdr:wsDr>`, lastColumn+1, reportXMLText("График "+metric.Label))
			files[fmt.Sprintf("xl/drawings/_rels/drawing%d.xml.rels", sheetID)] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="/xl/charts/chart%d.xml" Id="rId1"/></Relationships>`, sheetID)

			formulaSheet := excelSheetFormulaName(sheetName)
			var chartSeries strings.Builder
			palette := []string{"2062AA", "198B67", "9653AE", "CD7519", "298F99", "73583A", "627080", "4B7221"}
			seriesOrder := 0
			// Each series has its own compact X/Y columns. Sparse observations
			// must never turn a line into isolated markers on a shared date grid.
			var helperRows strings.Builder
			for i := range groups {
				values := chartSeriesValues[i]
				if len(values) < 2 {
					continue
				}
				sort.SliceStable(values, func(a, b int) bool { return parseDate(values[a].date).Before(parseDate(values[b].date)) })
				xCol := excelColumnName(lastColumn + 2 + 2*i)
				yCol := excelColumnName(lastColumn + 3 + 2*i)
				var xCache, yCache []float64
				for _, point := range values {
					serial, _ := infographicExcelDateSerial(point.date)
					xCache = append(xCache, serial)
					yCache = append(yCache, point.value)
					row := 10000 + len(xCache) - 1
					sourceRow := 5
					for dateIndex, date := range dates {
						if date == point.date {
							sourceRow = 5 + dateIndex
							break
						}
					}
					// Values are cached for immediate display and formulas preserve
					// edits made to the visible source table in Excel.
					helperRows.WriteString(fmt.Sprintf(`<row r="%d">%s%s</row>`, row,
						fmt.Sprintf(`<c r="%s%d" s="5"><f>A%d</f><v>%s</v></c>`, xCol, row, sourceRow, strconv.FormatFloat(serial, 'g', -1, 64)),
						fmt.Sprintf(`<c r="%s%d"><f>%s%d</f><v>%s</v></c>`, yCol, row, excelColumnName(i+2), sourceRow, strconv.FormatFloat(point.value, 'g', -1, 64))))
				}
				// Use literal caches in the chart itself. Excel renders these immediately
				// and does not recalculate thousands of sparse formulas on opening.
				var xLit, yLit strings.Builder
				xLit.WriteString(fmt.Sprintf(`<c:numLit><c:formatCode>dd.mm.yyyy</c:formatCode><c:ptCount val="%d"/>`, len(xCache)))
				yLit.WriteString(fmt.Sprintf(`<c:numLit><c:formatCode>0.###</c:formatCode><c:ptCount val="%d"/>`, len(yCache)))
				for j := range xCache {
					xLit.WriteString(fmt.Sprintf(`<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, j, strconv.FormatFloat(xCache[j], 'g', -1, 64)))
					yLit.WriteString(fmt.Sprintf(`<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, j, strconv.FormatFloat(yCache[j], 'g', -1, 64)))
				}
				xLit.WriteString(`</c:numLit>`)
				yLit.WriteString(`</c:numLit>`)
				chartSeries.WriteString(fmt.Sprintf(`<c:ser><c:idx val="%d"/><c:order val="%d"/><c:tx><c:strRef><c:f>%s!$%s$4</c:f></c:strRef></c:tx><c:spPr><a:ln w="38100"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:prstDash val="solid"/></a:ln></c:spPr><c:marker><c:symbol val="circle"/><c:size val="5"/></c:marker>%s<c:xVal>%s</c:xVal><c:yVal>%s</c:yVal><c:smooth val="0"/></c:ser>`, seriesOrder, seriesOrder, formulaSheet, excelColumnName(i+2), palette[i%len(palette)], infographicTrendlineXML(trendEnabled, trendType), xLit.String(), yLit.String()))
				seriesOrder++
			}
			// Chart uses literal caches above; do not materialize thousands of
			// hidden formula rows, which made Excel recalculate and appear frozen.
			xMin, xMax, yMin, yMax := infographicExcelAxisBounds(dates, chartSeriesValues)
			xScale := fmt.Sprintf(`<c:scaling><c:orientation val="minMax"/><c:min val="%s"/><c:max val="%s"/></c:scaling>`, strconv.FormatFloat(xMin, 'g', -1, 64), strconv.FormatFloat(xMax, 'g', -1, 64))
			yScale := fmt.Sprintf(`<c:scaling><c:orientation val="minMax"/><c:min val="%s"/><c:max val="%s"/></c:scaling>`, strconv.FormatFloat(yMin, 'g', -1, 64), strconv.FormatFloat(yMax, 'g', -1, 64))
			chartTitle := metric.Label + ", " + metric.Unit + " · линейные зависимости по исходным ячейкам"
			files[fmt.Sprintf("xl/charts/chart%d.xml", sheetID)] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:chart>%s<c:plotArea><c:scatterChart><c:scatterStyle val="lineMarker"/><c:varyColors val="0"/>%s<c:axId val="500"/><c:axId val="100"/></c:scatterChart><c:valAx><c:axId val="500"/>%s<c:delete val="0"/><c:axPos val="b"/><c:title><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>Дата</a:t></a:r></a:p></c:rich></c:tx></c:title><c:numFmt formatCode="dd.mm.yyyy" sourceLinked="0"/><c:majorTickMark val="out"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:crossAx val="100"/><c:crosses val="min"/></c:valAx><c:valAx><c:axId val="100"/>%s<c:delete val="0"/><c:axPos val="l"/><c:title><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>Значение</a:t></a:r></a:p></c:rich></c:tx></c:title><c:majorGridlines/><c:numFmt formatCode="0.###" sourceLinked="0"/><c:majorTickMark val="out"/><c:tickLblPos val="nextTo"/><c:crossAx val="500"/><c:crosses val="min"/><c:crossBetween val="midCat"/></c:valAx></c:plotArea><c:legend><c:legendPos val="b"/><c:overlay val="0"/></c:legend><c:plotVisOnly val="1"/><c:dispBlanksAs val="span"/></c:chart></c:chartSpace>`, infographicChartTitle(chartTitle), chartSeries.String(), xScale, yScale)
		}

		hiddenColumns := fmt.Sprintf(`<col min="%d" max="%d" hidden="1"/>`, lastColumn+2, lastColumn+1+2*len(groups))
		sheet := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetViews><sheetView workbookViewId="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="22" customWidth="1"/><col min="2" max="%d" width="28" customWidth="1"/>%s</cols><sheetData>%s</sheetData>%s</worksheet>`, lastColumn, hiddenColumns, rows.String(), drawingTag)
		files[fmt.Sprintf("xl/worksheets/sheet%d.xml", sheetID)] = sheet
	}
	contentTypes.WriteString(`</Types>`)
	styleRelID := len(metrics) + 2
	workbookRels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, styleRelID))
	files["[Content_Types].xml"] = contentTypes.String()
	files["_rels/.rels"] = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
	files["xl/workbook.xml"] = `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView/></bookViews><sheets>` + workbookSheets.String() + `</sheets><calcPr calcId="191029"/></workbook>`
	files["xl/_rels/workbook.xml.rels"] = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + workbookRels.String() + `</Relationships>`
	files["xl/styles.xml"] = `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="1"><numFmt numFmtId="164" formatCode="dd.mm.yyyy"/></numFmts><fonts count="5"><font><name val="Segoe UI"/><sz val="11"/></font><font><b/><color rgb="FFFFFFFF"/><name val="Segoe UI"/><sz val="15"/></font><font><b/><color rgb="FF0F3052"/><name val="Segoe UI"/><sz val="11"/></font><font><b/><color rgb="FFFFFFFF"/><name val="Segoe UI"/><sz val="10.5"/></font><font><b/><color rgb="FFCB1B2B"/><name val="Segoe UI"/><sz val="12"/></font></fonts><fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF12385F"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFC4353D"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="2"><border/><border><left style="thin"><color rgb="FFD9E1E8"/></left><right style="thin"><color rgb="FFD9E1E8"/></right><top style="thin"><color rgb="FFD9E1E8"/></top><bottom style="thin"><color rgb="FFD9E1E8"/></bottom></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="7"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"><alignment wrapText="1" vertical="top"/></xf><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="2" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="3" fillId="2" borderId="1" xfId="0"><alignment horizontal="center" vertical="center" wrapText="1"/></xf><xf numFmtId="0" fontId="3" fillId="3" borderId="1" xfId="0"><alignment horizontal="center" vertical="center" wrapText="1"/></xf><xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/><xf numFmtId="0" fontId="4" fillId="0" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`
	files["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="7" customWidth="1"/><col min="2" max="2" width="34" customWidth="1"/><col min="3" max="6" width="20" customWidth="1"/></cols><sheetData>` + summaryRows.String() + `</sheetData><mergeCells count="1"><mergeCell ref="A1:F1"/></mergeCells></worksheet>`
	return writeReportZip(path, files)
}
