//go:build windows

package main

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// reportLaboratoryMethodCandidates proposes investigations and alternatives,
// never a field recipe or an automatically approved action.
func reportLaboratoryMethodCandidates(pass LabPassRecord) string {
	var lines []string
	for _, code := range []string{"scale", "corrosion", "aspo", "solids", "microbiology", "gases"} {
		if !laboratoryPassHasGroupData(pass, code) {
			continue
		}
		group, ok := labGroupByCode(code)
		if !ok {
			continue
		}
		group = laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := canonicalizeLaboratoryValues(group, pass.Groups[code], pass.Units[code])
		assessment := laboratoryGroupSummary(group, values, pass.ScaleModel)
		if assessment.Level < 2 {
			continue
		}
		switch code {
		case "scale":
			results, _, warnings := laboratorySaturationResults(values, pass.ScaleModel)
			var minerals []string
			for _, result := range results {
				if result.SI > 0.1 {
					minerals = append(minerals, fmt.Sprintf("%s (SI %.2f)", result.Mineral, result.SI))
				}
			}
			if len(minerals) > 0 {
				lines = append(lines, "Солеотложения: расчёт указывает на пересыщение по "+strings.Join(minerals, ", ")+". Рассмотреть ингибирование солеотложений, корректировку режима смешения вод и, при подтверждённом осадке, регламентированное удаление отложений. До выбора реагента уточнить минералогию осадка, совместимость с пластовой водой, рабочие температуру и давление.")
			} else if len(warnings) > 0 {
				lines = append(lines, "Солеотложения: перед выбором ингибитора необходимо уточнить ионный состав и контроль баланса зарядов; оценка насыщения ограничена качеством исходных данных.")
			}
		case "corrosion":
			lines = append(lines, "Коррозия: рассмотреть подбор и опытную проверку ингибитора коррозии с контролем остаточного реагента, контрольные пластинки/зонды и обследование толщины стенки. Если выявлен H₂S или CO₂, отдельно проверить газовую фазу и совместимость реагента; дозировку назначать только по испытаниям.")
		case "aspo":
			lines = append(lines, "АСПО: рассмотреть испытания растворителей и диспергаторов, ингибирование парафиноотложений, а также механическое/тепловое удаление при допустимости по оборудованию. Проверить температуру отложения, состав осадка и совместимость составов; способ обработки выбирать после лабораторной оценки.")
		case "solids":
			lines = append(lines, "Механические примеси: установить происхождение и фракционный состав твёрдой фазы, затем сравнить фильтрацию/сепарацию, технологическую промывку и меры по ограничению выноса песка. Зафиксировать динамику концентрации и характер повреждений; конкретную компоновку ГНО определяют вне РОСА.")
		case "microbiology":
			lines = append(lines, "Микробиологические признаки: подтвердить активность микроорганизмов и наличие биоплёнки профильными методами. При подтверждении рассмотреть программу биоцидной обработки и контроль эффективности; выбор состава и дозы — после испытаний совместимости.")
		case "gases":
			lines = append(lines, "Газовый фактор: сопоставить состав газа, давление и температурный профиль с режимом работы. При подтверждённом риске гидратообразования рассмотреть контроль точки охлаждения и испытания ингибитора гидратов; меры против газового влияния оценивать по фактическим режимным данным.")
		}
	}
	if len(lines) == 0 {
		return "Выбор метода пока не обоснован: подтвердить предполагаемое осложнение профильным анализом, сопоставить его с причинами отказов и техрежимом. Отсутствие подтверждённых сигналов не означает отсутствие осложнения."
	}
	return strings.Join(lines, "\r\n")
}

func reportLaboratoryActionConclusion(pass LabPassRecord) string {
	var b strings.Builder
	b.WriteString("ОЦЕНКА ПО ИМЕЮЩИМСЯ ДАННЫМ\r\n")
	b.WriteString(laboratoryPassOverallShortConclusion(pass) + "\r\n")
	for _, code := range []string{"scale", "corrosion", "aspo", "solids", "microbiology", "gases"} {
		if !laboratoryPassHasGroupData(pass, code) {
			continue
		}
		group, ok := labGroupByCode(code)
		if !ok {
			continue
		}
		group = laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := canonicalizeLaboratoryValues(group, pass.Groups[code], pass.Units[code])
		assessment := laboratoryGroupSummary(group, values, pass.ScaleModel)
		status := "оценка ограничена"
		if assessment.Level >= 2 {
			status = "есть признаки для проверки"
		} else if assessment.Level == 1 {
			status = "по проверенным признакам отклонений не выявлено"
		}
		fmt.Fprintf(&b, "%s: %s; базовые поля %d/%d.\r\n", group.Name, status, assessment.BaseFilled, assessment.BaseTotal)
	}
	b.WriteString("\r\nЧТО РАССМОТРЕТЬ ДЛЯ СНИЖЕНИЯ ОСЛОЖНЕНИЙ\r\n")
	b.WriteString(reportLaboratoryMethodCandidates(pass) + "\r\n")
	b.WriteString("\r\nКОНТРОЛЬ ПЕРЕД РЕШЕНИЕМ\r\n")
	b.WriteString("Сопоставить лабораторные данные с точкой и датой отбора, техрежимом, историей отказов и ранее применёнными мероприятиями. Для предложенных методов определить показатель контроля до/после, провести лабораторные испытания и утвердить параметры технологом. Предложение не является подтверждением причины отказа или готовой рецептурой.\r\n")
	return reportCleanUserText(b.String())
}

var reportWorkbookRowRE = regexp.MustCompile(`(?s)<row r="(\d+)"[^>]*>(.*?)</row>`)

func reportWorkbookCellText(row, col string) string {
	re := regexp.MustCompile(`<c r="` + regexp.QuoteMeta(col) + `\d+"[^>]*>.*?<t(?: [^>]*)?>(.*?)</t>.*?</c>`)
	match := re.FindStringSubmatch(row)
	if len(match) < 2 {
		return ""
	}
	return html.UnescapeString(match[1])
}

// The readable laboratory sheet is a small navigable calculation dashboard.
// Every result links to the actual formula on the immutable-report calculation
// sheet. It deliberately does not duplicate historical raw-data dumps.
func fullReportLaboratoryDashboardXML(r FullReportRecord) string {
	pass := reportLabSnapshotInputs(fullReportSection(r, "section_2"))
	calcXML := fullReportCalculationSheetXML(r)
	var rows, merges strings.Builder
	row := 1
	appendRow := func(height int, cells ...string) {
		fmt.Fprintf(&rows, `<row r="%d" ht="%d" customHeight="1">%s</row>`, row, height, strings.Join(cells, ""))
		row++
	}
	merged := func(text string, style, height int) {
		ref := strconv.Itoa(row)
		appendRow(height, xlsxInlineCell("A"+ref, text, style))
		fmt.Fprintf(&merges, `<mergeCell ref="A%s:F%s"/>`, ref, ref)
	}
	merged("ЛАБОРАТОРИЯ · РАСЧЁТНЫЕ ПОКАЗАТЕЛИ", 5, 29)
	appendRow(25, xlsxInlineCell("A2", "Объект", 7), xlsxInlineCell("B2", r.ObjectLabel, 0))
	merges.WriteString(`<mergeCell ref="B2:F2"/>`)
	appendRow(25, xlsxInlineCell("A3", "Дата анализа", 7), xlsxInlineCell("B3", firstNonEmpty(pass.Date, "не указана"), 0), xlsxInlineCell("D3", "Модель", 7), xlsxInlineCell("E3", firstNonEmpty(pass.ScaleModel, "не указана"), 0))
	appendRow(52, xlsxInlineCell("A4", "Источник", 7), xlsxInlineCell("B4", firstNonEmpty(pass.Source, "не указан"), 0))
	merges.WriteString(`<mergeCell ref="B4:F4"/>`)
	row++
	merged("ЧИСЛОВЫЕ РАСЧЁТЫ · РЕЗУЛЬТАТЫ СВЯЗАНЫ С ФОРМУЛАМИ", 6, 25)
	hdr := strconv.Itoa(row)
	appendRow(34,
		xlsxInlineCell("A"+hdr, "Направление", 6),
		xlsxInlineCell("B"+hdr, "Расчётный показатель", 6),
		xlsxInlineCell("C"+hdr, "Основание", 6),
		xlsxInlineCell("D"+hdr, "Результат", 6),
		xlsxInlineCell("E"+hdr, "Единица", 6),
		xlsxInlineCell("F"+hdr, "Проверка расчёта", 6),
	)
	frozenAt := row - 1
	section := "Солеотложения"
	count := 0
	for _, match := range reportWorkbookRowRE.FindAllStringSubmatch(calcXML, -1) {
		if len(match) < 3 {
			continue
		}
		number, body := match[1], match[2]
		label := strings.TrimSpace(reportWorkbookCellText(body, "A"))
		if label == "" {
			continue
		}
		switch {
		case strings.HasPrefix(label, "КОРРОЗИЯ"):
			section = "Коррозия"
		case strings.HasPrefix(label, "АСПО"):
			section = "АСПО"
		case strings.HasPrefix(label, "МЕХПРИМЕСИ"):
			section = "Мехпримеси"
		case strings.HasPrefix(label, "ГАЗЫ"):
			section = "Газ и гидраты"
		case strings.HasPrefix(label, "РАСЧЁТНЫЕ ПОКАЗАТЕЛИ ОТЧЁТА"):
			section = "Иное"
		}
		if section == "Иное" {
			break
		}
		resultRE := regexp.MustCompile(`<c r="F` + regexp.QuoteMeta(number) + `"[^>]*><f>(.*?)</f><v>(.*?)</v></c>`)
		vals := resultRE.FindStringSubmatch(body)
		if len(vals) < 3 {
			continue
		}
		cached, err := strconv.ParseFloat(vals[2], 64)
		if err != nil || math.IsNaN(cached) || math.IsInf(cached, 0) {
			continue
		}
		if count >= 130 {
			break
		}
		dest := strconv.Itoa(row)
		unit := reportWorkbookCellText(body, "D")
		if strings.Contains(label, "→ c") {
			unit = "моль/л / вклад I"
		}
		if unit == "" {
			unit = "—"
		}
		note := reportWorkbookCellText(body, "G")
		if len([]rune(note)) > 120 {
			note = string([]rune(note)[:120]) + "…"
		}
		style := 8
		if cached != 0 && math.Abs(cached) < 0.001 {
			style = 9
		}
		appendRow(37,
			xlsxInlineCell("A"+dest, section, 0),
			xlsxInlineCell("B"+dest, label, 7),
			xlsxInlineCell("C"+dest, "'Расчёты'!F"+number, 0),
			calculationFormulaCell("D"+dest, "'Расчёты'!F"+number, cached, style),
			xlsxInlineCell("E"+dest, unit, 0),
			xlsxInlineCell("F"+dest, note, 0),
		)
		count++
	}
	if count == 0 {
		merged("Расчётные показатели отсутствуют: необходимы числовые результаты лабораторного анализа.", 7, 31)
	}
	row++
	merged("ВЫВОД И ВОЗМОЖНЫЕ МЕРОПРИЯТИЯ", 6, 25)
	conclusion := reportLaboratoryActionConclusion(pass)
	noteCount := 0
	for _, raw := range strings.Split(strings.ReplaceAll(conclusion, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if noteCount >= 17 {
			break
		}
		if strings.HasPrefix(line, "ОЦЕНКА ПО ") || strings.HasPrefix(line, "ЧТО РАССМОТРЕТЬ") || strings.HasPrefix(line, "КОНТРОЛЬ ПЕРЕД") {
			merged(line, 7, 21)
		} else {
			height := 35
			if len([]rune(line)) > 325 {
				height = 85
			} else if len([]rune(line)) > 190 {
				height = 60
			}
			merged(line, 0, height)
		}
		noteCount++
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="` + strconv.Itoa(frozenAt) + `" topLeftCell="A` + strconv.Itoa(frozenAt+1) + `" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="23" customWidth="1"/><col min="2" max="2" width="45" customWidth="1"/><col min="3" max="3" width="25" customWidth="1"/><col min="4" max="4" width="19" customWidth="1"/><col min="5" max="5" width="20" customWidth="1"/><col min="6" max="6" width="60" customWidth="1"/></cols><sheetData>` + rows.String() + `</sheetData><mergeCells count="` + strconv.Itoa(strings.Count(merges.String(), "<mergeCell ")) + `">` + merges.String() + `</mergeCells></worksheet>`
}

// Approved historical content is immutable. This presentation-only adapter
// makes old single-line plans readable in new DOCX/PDF exports.
var reportOldPlanLine = regexp.MustCompile(`^(\d+)\. (.*?) — статус (.*?); срок (.*?); ответственный (.*?); комментарий (.*?)\.$`)

func reportNormalizeHistoricalPlan(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var out []string
	teachingNote := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if groups := reportOldPlanLine.FindStringSubmatch(line); len(groups) == 7 {
			out = append(out, "", "МЕРОПРИЯТИЕ "+groups[1]+". "+groups[2],
				"Срок: "+groups[4]+"; статус: "+groups[3]+".",
				"Ответственный: "+groups[5]+".")
			if strings.Contains(strings.ToLower(groups[6]), "учебный план контроля") {
				teachingNote = true
			} else if strings.TrimSpace(groups[6]) != "" && strings.TrimSpace(groups[6]) != "—" {
				out = append(out, "Контроль выполнения: "+strings.TrimSuffix(groups[6], ".")+".")
			}
			continue
		}
		if strings.HasPrefix(line, "Взаимодействие методов: ") {
			if teachingNote {
				out = append(out, "Условие ОПИ: уточнить контрольные параметры и совместимость до начала работ.")
				teachingNote = false
			}
			out = append(out, "", "ВЗАИМОДЕЙСТВИЕ МЕТОДОВ", strings.TrimPrefix(line, "Взаимодействие методов: "))
			continue
		}
		if strings.HasPrefix(line, "Ограничение прогноза: Синтетические данные") {
			out = append(out, "Прогноз учебного объекта требует проверки по промысловым данным до внедрения.")
			continue
		}
		out = append(out, raw)
	}
	if teachingNote {
		out = append(out, "Условие ОПИ: уточнить контрольные параметры и совместимость до начала работ.")
	}
	return strings.Join(out, "\r\n")
}
