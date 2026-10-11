//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

func chooseDynamogramExcelPath(defaultName string) string {
	buf := make([]uint16, 32768)
	name, _ := syscall.UTF16FromString(defaultName)
	copy(buf, name)
	filter := utf16.Encode([]rune("Книга Excel (*.xlsx)\x00*.xlsx\x00Все файлы (*.*)\x00*.*\x00\x00"))
	of := OPENFILENAMEW{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain,
		LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)),
		LpstrDefExt: p("xlsx"), LpstrTitle: p("Выгрузить динамограмму в Excel"),
		Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT,
	}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	return path
}

func safeDynamogramExportName(a DynamogramAnalysis) string {
	name := strings.TrimSpace(a.ObjectKey)
	if name == "" {
		name = "динамограмма"
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, name)
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return name + "_динамо.xlsx"
}

func exportCurrentDynamogramExcelUI() {
	if currentDyn == nil || len(currentDynPoints) == 0 {
		msg("Экспорт динамограммы", "Сначала загрузите и обработайте динамограмму.", MB_OK|MB_ICONWARNING)
		return
	}
	path := chooseDynamogramExcelPath(safeDynamogramExportName(*currentDyn))
	if path == "" {
		return
	}
	if err := writeDynamogramXLSX(path, *currentDyn, currentDynPoints); err != nil {
		msg("Экспорт динамограммы", "Не удалось создать книгу Excel: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Экспорт динамограммы", "Книга Excel сохранена. Она содержит исходные точки X/Y, единицы измерения, шаг шкал и редактируемый график.", MB_OK|MB_ICONINFORMATION)
	openExternal(path)
}

func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func xlsxInlineCell(ref, value string, style int) string {
	space := ""
	if strings.TrimSpace(value) != value {
		space = ` xml:space="preserve"`
	}
	return fmt.Sprintf(`<c r="%s" t="inlineStr" s="%d"><is><t%s>%s</t></is></c>`, ref, style, space, xmlText(value))
}

func xlsxNumberCell(ref string, value float64, style int) string {
	return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, style, strconv.FormatFloat(value, 'g', 15, 64))
}

func niceAxisStep(minimum, maximum float64) float64 {
	rangeValue := maximum - minimum
	if !(rangeValue > 0) || math.IsNaN(rangeValue) || math.IsInf(rangeValue, 0) {
		return 0.2
	}
	raw := rangeValue / 5
	power := math.Pow(10, math.Floor(math.Log10(raw)))
	normalized := raw / power
	multiplier := 1.0
	if normalized > 5 {
		multiplier = 10
	} else if normalized > 2 {
		multiplier = 5
	} else if normalized > 1 {
		multiplier = 2
	}
	return multiplier * power
}

func writeDynamogramXLSX(path string, a DynamogramAnalysis, pts []DynamogramPoint) error {
	if len(pts) == 0 {
		return fmt.Errorf("нет точек контура")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	zw := zip.NewWriter(f)
	add := func(name, body string) error {
		w, e := zw.Create(name)
		if e != nil {
			return e
		}
		_, e = w.Write([]byte(body))
		return e
	}

	xMin, xMax, yMin, yMax := 0.0, 1.0, 0.0, 1.0
	xUnit, yUnit := "норм.", "норм."
	xUpperCol, yUpperCol, yLowerCol := "B", "C", "D"
	if a.Calibrated {
		xMin, xMax, yMin, yMax = a.XMin, a.XMax, a.YMin, a.YMax
		xUnit, yUnit = strings.TrimSpace(a.XUnit), strings.TrimSpace(a.YUnit)
		if xUnit == "" {
			xUnit = "ед."
		}
		if yUnit == "" {
			yUnit = "ед."
		}
		xUpperCol, yUpperCol, yLowerCol = "E", "F", "G"
	}
	xStep, yStep := niceAxisStep(xMin, xMax), niceAxisStep(yMin, yMax)
	firstDataRow, lastDataRow := 9, 8+len(pts)

	var rows strings.Builder
	rows.WriteString(`<row r="1" ht="28" customHeight="1">` + xlsxInlineCell("A1", "РОСА — динамограмма", 1) + `</row>`)
	rows.WriteString(`<row r="2">` + xlsxInlineCell("A2", "Объект", 2) + xlsxInlineCell("B2", strings.TrimSpace(a.ObjectKey), 0) + `</row>`)
	rows.WriteString(`<row r="3">` + xlsxInlineCell("A3", "Источник", 2) + xlsxInlineCell("B3", filepath.Base(a.SourceFile), 0) + `</row>`)
	rows.WriteString(`<row r="4">` + xlsxInlineCell("A4", "Дата экспорта", 2) + xlsxInlineCell("B4", time.Now().Format("02.01.2006 15:04"), 0) + `</row>`)
	rows.WriteString(`<row r="5">` + xlsxInlineCell("A5", "Оси", 2) + xlsxInlineCell("B5", "X — ход, "+xUnit+"; Y — нагрузка, "+yUnit, 0) + `</row>`)
	rows.WriteString(`<row r="6">` + xlsxInlineCell("A6", "Шаг шкалы", 2) + xlsxInlineCell("B6", fmt.Sprintf("X = %g %s; Y = %g %s", xStep, xUnit, yStep, yUnit), 0) + `</row>`)
	headers := []string{"№", "X норм.", "Верхняя ветвь норм.", "Нижняя ветвь норм.", "X, " + xUnit, "Верхняя ветвь, " + yUnit, "Нижняя ветвь, " + yUnit}
	rows.WriteString(`<row r="8" ht="30" customHeight="1">`)
	for i, header := range headers {
		rows.WriteString(xlsxInlineCell(string(rune('A'+i))+"8", header, 3))
	}
	rows.WriteString(`</row>`)
	for i, point := range pts {
		row := firstDataRow + i
		rows.WriteString(fmt.Sprintf(`<row r="%d">`, row))
		rows.WriteString(xlsxNumberCell("A"+strconv.Itoa(row), float64(i+1), 0))
		rows.WriteString(xlsxNumberCell("B"+strconv.Itoa(row), point.XNorm, 0))
		rows.WriteString(xlsxNumberCell("C"+strconv.Itoa(row), point.UpperNorm, 0))
		rows.WriteString(xlsxNumberCell("D"+strconv.Itoa(row), point.LowerNorm, 0))
		if a.Calibrated {
			rows.WriteString(xlsxNumberCell("E"+strconv.Itoa(row), point.X, 0))
			rows.WriteString(xlsxNumberCell("F"+strconv.Itoa(row), point.Upper, 0))
			rows.WriteString(xlsxNumberCell("G"+strconv.Itoa(row), point.Lower, 0))
		}
		rows.WriteString(`</row>`)
	}

	sheet := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<dimension ref="A1:G%d"/><sheetViews><sheetView workbookViewId="0"><pane ySplit="8" topLeftCell="A9" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>
<cols><col min="1" max="1" width="8" customWidth="1"/><col min="2" max="7" width="23" customWidth="1"/></cols><sheetData>%s</sheetData>
<mergeCells count="1"><mergeCell ref="A1:G1"/></mergeCells><drawing r:id="rId1"/></worksheet>`, lastDataRow, rows.String())

	xTitle := "X — ход (" + xUnit + ")"
	yTitle := "Y — нагрузка (" + yUnit + ")"
	chartTitle := func(value string) string {
		return `<c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="ru-RU"/><a:t>` + xmlText(value) + `</a:t></a:r></a:p></c:rich></c:tx><c:layout/><c:overlay val="0"/></c:title>`
	}
	series := func(index int, name, yCol string) string {
		return fmt.Sprintf(`<c:ser><c:idx val="%d"/><c:order val="%d"/><c:tx><c:v>%s</c:v></c:tx><c:spPr><a:ln w="25400"><a:solidFill><a:srgbClr val="CB1B2B"/></a:solidFill></a:ln></c:spPr><c:xVal><c:numRef><c:f>'Данные'!$%s$%d:$%s$%d</c:f></c:numRef></c:xVal><c:yVal><c:numRef><c:f>'Данные'!$%s$%d:$%s$%d</c:f></c:numRef></c:yVal><c:smooth val="0"/></c:ser>`, index, index, xmlText(name), xUpperCol, firstDataRow, xUpperCol, lastDataRow, yCol, firstDataRow, yCol, lastDataRow)
	}
	chart := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><c:chart>` + chartTitle("Динамограмма") + `<c:plotArea><c:layout/><c:scatterChart><c:scatterStyle val="line"/><c:varyColors val="0"/>` + series(0, "Верхняя ветвь", yUpperCol) + series(1, "Нижняя ветвь", yLowerCol) + `<c:dLbls><c:showLegendKey val="0"/><c:showVal val="0"/><c:showCatName val="0"/><c:showSerName val="0"/></c:dLbls><c:axId val="48650112"/><c:axId val="48672768"/></c:scatterChart>
<c:valAx><c:axId val="48650112"/><c:scaling><c:orientation val="minMax"/><c:min val="` + strconv.FormatFloat(xMin, 'g', 15, 64) + `"/><c:max val="` + strconv.FormatFloat(xMax, 'g', 15, 64) + `"/></c:scaling><c:delete val="0"/><c:axPos val="b"/>` + chartTitle(xTitle) + `<c:numFmt formatCode="0.###" sourceLinked="0"/><c:majorTickMark val="out"/><c:tickLblPos val="nextTo"/><c:crossAx val="48672768"/><c:crosses val="autoZero"/><c:majorUnit val="` + strconv.FormatFloat(xStep, 'g', 15, 64) + `"/></c:valAx>
<c:valAx><c:axId val="48672768"/><c:scaling><c:orientation val="minMax"/><c:min val="` + strconv.FormatFloat(yMin, 'g', 15, 64) + `"/><c:max val="` + strconv.FormatFloat(yMax, 'g', 15, 64) + `"/></c:scaling><c:delete val="0"/><c:axPos val="l"/>` + chartTitle(yTitle) + `<c:majorGridlines/><c:numFmt formatCode="0.###" sourceLinked="0"/><c:majorTickMark val="out"/><c:tickLblPos val="nextTo"/><c:crossAx val="48650112"/><c:crosses val="autoZero"/><c:crossBetween val="midCat"/><c:majorUnit val="` + strconv.FormatFloat(yStep, 'g', 15, 64) + `"/></c:valAx></c:plotArea><c:legend><c:legendPos val="b"/><c:overlay val="0"/></c:legend><c:plotVisOnly val="1"/><c:dispBlanksAs val="gap"/></c:chart></c:chartSpace>`

	parts := map[string]string{
		"[Content_Types].xml":                 `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/><Override PartName="/xl/drawings/drawing1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/><Override PartName="/xl/charts/chart1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/></Types>`,
		"_rels/.rels":                         `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/></Relationships>`,
		"docProps/core.xml":                   `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><dc:title>Динамограмма РОСА</dc:title><dc:creator>РОСА</dc:creator><dcterms:created xsi:type="dcterms:W3CDTF">` + time.Now().UTC().Format(time.RFC3339) + `</dcterms:created></cp:coreProperties>`,
		"docProps/app.xml":                    `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes"><Application>РОСА</Application></Properties>`,
		"xl/workbook.xml":                     `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView/></bookViews><sheets><sheet name="Данные" sheetId="1" r:id="rId1"/></sheets><calcPr calcId="191029" fullCalcOnLoad="1"/></workbook>`,
		"xl/_rels/workbook.xml.rels":          `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":                       `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="3"><font><sz val="11"/><name val="Calibri"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="16"/><name val="Calibri"/></font><font><b/><color rgb="FF0F3052"/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FFCB1B2B"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FF0F3052"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="2"><border/><border><left style="thin"><color rgb="FFD3DBE4"/></left><right style="thin"><color rgb="FFD3DBE4"/></right><top style="thin"><color rgb="FFD3DBE4"/></top><bottom style="thin"><color rgb="FFD3DBE4"/></bottom></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf><xf numFmtId="0" fontId="2" fillId="0" borderId="0" xfId="0" applyFont="1"/><xf numFmtId="0" fontId="1" fillId="3" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`,
		"xl/worksheets/sheet1.xml":            sheet,
		"xl/worksheets/_rels/sheet1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/></Relationships>`,
		"xl/drawings/drawing1.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><xdr:twoCellAnchor><xdr:from><xdr:col>8</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from><xdr:to><xdr:col>17</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>24</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to><xdr:graphicFrame macro=""><xdr:nvGraphicFramePr><xdr:cNvPr id="2" name="График динамограммы"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><xdr:xfrm/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:twoCellAnchor></xdr:wsDr>`,
		"xl/drawings/_rels/drawing1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/></Relationships>`,
		"xl/charts/chart1.xml":                chart,
	}
	order := []string{"[Content_Types].xml", "_rels/.rels", "docProps/core.xml", "docProps/app.xml", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/_rels/sheet1.xml.rels", "xl/drawings/drawing1.xml", "xl/drawings/_rels/drawing1.xml.rels", "xl/charts/chart1.xml"}
	for _, name := range order {
		if err := add(name, parts[name]); err != nil {
			_ = zw.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}
