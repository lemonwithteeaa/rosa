//go:build windows

package main

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func reportXMLText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(value)
}

func writeReportZip(path string, files map[string]string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, createErr := zw.Create(name)
		if createErr != nil {
			_ = zw.Close()
			return createErr
		}
		if _, writeErr := w.Write([]byte(body)); writeErr != nil {
			_ = zw.Close()
			return writeErr
		}
	}
	return zw.Close()
}

type laboratoryReportStyle int

const (
	reportStyleNormal laboratoryReportStyle = iota
	reportStyleTitle
	reportStyleTechnologist
	reportStyleSection
	reportStyleTechnology
	reportStyleLabel
	reportStyleLow
	reportStyleModerate
	reportStyleElevated
	reportStyleNoData
	reportStyleAction
)

func laboratoryTechnologyHeading(line string) bool {
	upper := strings.ToUpper(strings.TrimSpace(line))
	return upper == "ТЕХНОЛОГИЧЕСКИЙ ИТОГ" ||
		strings.HasPrefix(upper, "ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО СОЛЕОТЛОЖЕНИЯМ") ||
		strings.HasPrefix(upper, "ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО CO₂-РАВНОВЕСИЮ")
}

func laboratoryReportSectionHeading(line string) bool {
	upper := strings.ToUpper(strings.TrimSpace(line))
	if upper == "" {
		return false
	}
	for _, heading := range []string{
		"СВОДНЫЙ АНАЛИЗ", "РЕКОМЕНДАЦИИ ПО РЯДУ ОТКАЗОВ", "ИСХОДНЫЕ СВЕДЕНИЯ", "ГНО И РЕЖИМ", "ПРИЧИНА ОТКАЗА", "РАССЛЕДОВАНИЕ И ЗАКЛЮЧЕНИЕ", "ПРИЗНАКИ И ПРЕДПОСЫЛКИ", "СОПУТСТВУЮЩИЕ ОСЛОЖНЕНИЯ", "РЕМОНТ, ПРИЧИНЫ И МЕТОДЫ", "КОМПЛЕКТАЦИЯ ГНО", "КАБЕЛЬ, ГИДРОЗАЩИТА И ТМС", "РЕКОМЕНДАЦИИ К МЕРОПРИЯТИЯМ", "ПОКАЗАТЕЛИ", "ВЫВОД", "КОММЕНТАРИЙ ПОЛЬЗОВАТЕЛЯ", "ОБЩИЙ ВЫВОД",
		"ИСХОДНЫЕ ДАННЫЕ РАЗДЕЛА", "СВОД ОБЩИХ ПОЛЕЙ", "КОНТРОЛЬ ИСТОЧНИКОВ И АКТУАЛЬНОСТИ",
		"ВИЗУАЛЬНАЯ СВОДКА ПО НАПРАВЛЕНИЯМ", "РАСШИРЕННАЯ ОЦЕНКА ВЛИЯНИЯ НА КОРРОЗИЮ",
		"СВОД ПО НАПРАВЛЕНИЯМ", "КОМПЛЕКСНАЯ ИНЖЕНЕРНАЯ ИНТЕРПРЕТАЦИЯ",
		"ПРИЛОЖЕНИЕ. РАСЧЁТЫ, ФОРМУЛЫ И ПРИМЕРЫ", "МЕТОДИКА", "ФОРМУЛЫ И ПРАВИЛА",
		"ПРИМЕР РАСЧЁТА ПО ТЕКУЩИМ ДАННЫМ", "РАСЧЁТ СРАЗУ ПО ВСЕМ МОДЕЛЯМ", "ПЕРЕМЕННЫЕ", "ГРАНИЦЫ ПРИМЕНИМОСТИ", "ТЕКУЩИЕ ИСХОДНЫЕ ДАННЫЕ", "ВЗАИМОДЕЙСТВИЕ МЕТОДОВ", "ОЦЕНКА ПО ИМЕЮЩИМСЯ ДАННЫМ", "ЧТО РАССМОТРЕТЬ ДЛЯ СНИЖЕНИЯ ОСЛОЖНЕНИЙ", "КОНТРОЛЬ ПЕРЕД РЕШЕНИЕМ",
		"ОЦЕНКА РИСКА", "РАСЧЁТ НЕ ВЫПОЛНЕН ПОЛНОСТЬЮ", "ПОЛНОТА ИСХОДНЫХ ДАННЫХ", "ПРАКТИЧЕСКАЯ РЕКОМЕНДАЦИЯ",
		"ВЫБРАННАЯ МОДЕЛЬ", "ПОДСТАНОВКА И РЕЗУЛЬТАТ",
	} {
		if upper == heading || strings.HasPrefix(upper, heading+":") {
			return true
		}
	}
	return strings.HasPrefix(upper, "РАЗДЕЛ:") || strings.HasPrefix(upper, "РАЗДЕЛ ") || strings.HasPrefix(upper, "ПРИЛОЖЕНИЕ.") || strings.HasPrefix(upper, "НАПРАВЛЕНИЕ:") || upper == "ИСХОДНЫЕ ЛАБОРАТОРНЫЕ ПОКАЗАТЕЛИ" || upper == "СКВОЗНЫЕ / ОБЩИЕ ПОКАЗАТЕЛИ" || upper == "ДОПОЛНИТЕЛЬНЫЕ ИМПОРТИРОВАННЫЕ ПОКАЗАТЕЛИ" || upper == "ПОДРОБНЫЕ СОБЫТИЯ" || upper == "ПАРАМЕТРЫ РЕАЛИЗАЦИИ" || upper == "ПЛАН ИСПОЛНЕНИЯ И КОНТРОЛЯ" || upper == "ПОЛНАЯ СВОДКА РАЗДЕЛА 7" || upper == "ПРИМЕЧАНИЕ"
}

func laboratoryReportLabelLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{
		"Объект:", "Дата анализа:", "Дата формирования отчёта:", "Раздел:", "Источник / протокол:",
		"Автор:", "Комментарий:", "Заполнено направлений:", "Заполнено показателей:",
		"Обязательный базовый уровень данных:", "Базовый минимум данных:", "Дополнительные исследования:",
		"Контекст техрежима:", "Общие условия:", "Уровень влияния:", "Достоверность:", "Вывод:", "Ограничение:",
		"Что происходит:", "Что означают именно эти данные:", "Чего пока нельзя утверждать:", "Что проверить следующим действием:", "Следующее действие:",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func laboratoryReportLineStyle(line string, index int) laboratoryReportStyle {
	trimmed := strings.TrimSpace(line)
	upper := strings.ToUpper(trimmed)
	if index == 0 && trimmed != "" {
		return reportStyleTitle
	}
	if strings.HasPrefix(upper, "МЕРОПРИЯТИЕ ") && strings.Contains(upper, ". ") {
		return reportStyleAction
	}
	if laboratoryTechnologistHeading(line) {
		return reportStyleTechnologist
	}
	if laboratoryTechnologyHeading(line) {
		return reportStyleTechnology
	}
	if strings.Contains(line, " | ") {
		switch {
		case strings.Contains(upper, "ПОВЫШ") || strings.Contains(upper, "ВЫСОК"):
			return reportStyleElevated
		case strings.Contains(upper, "УМЕРЕН") || strings.Contains(upper, "СРЕДН"):
			return reportStyleModerate
		case strings.Contains(upper, "НИЗК"):
			return reportStyleLow
		case strings.Contains(upper, "НЕТ ДАННЫХ") || strings.Contains(upper, "НЕ ОЦЕНЕН") || strings.Contains(upper, "НЕ ОПРЕДЕЛ"):
			return reportStyleNoData
		}
	}
	if laboratoryReportSectionHeading(line) {
		return reportStyleSection
	}
	if strings.HasPrefix(trimmed, "• ") {
		return reportStyleLabel
	}
	formula := strings.TrimSpace(line)
	for _, prefix := range []string{"Kнас =", "SI =", "IAP =", "log₁₀", "ΔT =", "Контрольные условия:", "Расчётный блок"} {
		if strings.HasPrefix(formula, prefix) {
			return reportStyleLabel
		}
	}
	if laboratoryReportLabelLine(line) {
		return reportStyleLabel
	}
	return reportStyleNormal
}

func laboratoryDOCXRunProperties(style laboratoryReportStyle) (string, string) {
	font := `<w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman" w:cs="Times New Roman"/><w:sz w:val="24"/>`
	paragraph := `<w:pPr><w:spacing w:before="0" w:after="60" w:line="240" w:lineRule="auto"/></w:pPr>`
	switch style {
	case reportStyleTitle:
		font = `<w:b/>` + font
		paragraph = `<w:pPr><w:jc w:val="center"/><w:keepNext/><w:keepLines/><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>`
	case reportStyleAction:
		font = `<w:b/><w:color w:val="153C6A"/>` + font
		paragraph = `<w:pPr><w:keepNext/><w:keepLines/><w:shd w:val="clear" w:fill="EDF2F8"/><w:spacing w:before="120" w:after="70" w:line="240" w:lineRule="auto"/></w:pPr>`
	case reportStyleTechnologist, reportStyleSection, reportStyleTechnology, reportStyleLabel:
		font = `<w:b/>` + font
		if style != reportStyleLabel {
			paragraph = `<w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="140" w:after="75" w:line="240" w:lineRule="auto"/></w:pPr>`
		}
	case reportStyleLow, reportStyleModerate, reportStyleElevated, reportStyleNoData:
		font = `<w:b/>` + font
	}
	return paragraph, font
}

func writeLaboratoryDOCXReport(path, report string) error {
	report = reportCleanUserText(report)
	var paragraphs strings.Builder
	lines := strings.Split(strings.ReplaceAll(report, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if rows, end, common, ok := assessmentExportParseBlock(lines, i); ok {
			paragraphs.WriteString(`<w:p><w:pPr><w:keepNext/><w:spacing w:before="110" w:after="65" w:line="240" w:lineRule="auto"/></w:pPr><w:r><w:rPr><w:b/><w:i/><w:color w:val="173D6C"/><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/></w:rPr><w:t>` + reportXMLText(line) + `</w:t></w:r></w:p>`)
			paragraphs.WriteString(assessmentApprovalDOCXTable(rows, common))
			i = end - 1
			continue
		}
		if strings.TrimSpace(line) == "" {
			paragraphs.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="100" w:lineRule="auto"/></w:pPr></w:p>`)
			continue
		}
		style := laboratoryReportLineStyle(line, i)
		pPr, rPr := laboratoryDOCXRunProperties(style)
		if i == 1 && strings.HasPrefix(line, "Отчёт: ") {
			pPr = `<w:pPr><w:jc w:val="center"/><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>`
			_, rPr = laboratoryDOCXRunProperties(reportStyleNormal)
			paragraphs.WriteString(`<w:p>` + pPr + `<w:r><w:rPr>` + rPr + `</w:rPr><w:t xml:space="preserve">` + reportXMLText(line) + `</w:t></w:r></w:p>`)
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "• ") {
			// A measurement is one paragraph: keep its label, value and unit
			// on the same page. Emphasise the indicator name, not the number.
			pPr = `<w:pPr><w:keepLines/><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>`
			if colon := strings.LastIndex(line, ": "); colon > 2 {
				label := reportXMLText(line[:colon+1])
				value := reportXMLText(line[colon+1:])
				paragraphs.WriteString(`<w:p>` + pPr + `<w:r><w:rPr><w:b/><w:i/><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/></w:rPr><w:t xml:space="preserve">` + label + `</w:t></w:r><w:r><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/></w:rPr><w:t xml:space="preserve">` + value + `</w:t></w:r></w:p>`)
				continue
			}
		}
		if style == reportStyleSection && (strings.TrimSpace(line) == "ПЛАН ИСПОЛНЕНИЯ И КОНТРОЛЯ" || strings.TrimSpace(line) == "ВЗАИМОДЕЙСТВИЕ МЕТОДОВ" || strings.TrimSpace(line) == "ЧТО РАССМОТРЕТЬ ДЛЯ СНИЖЕНИЯ ОСЛОЖНЕНИЙ") {
			rPr = `<w:b/><w:i/><w:color w:val="153C6A"/><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/>`
		}
		if style == reportStyleSection && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "РАЗДЕЛ ") {
			rPr = `<w:b/><w:i/><w:color w:val="B41426"/><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/>`
		}
		if style == reportStyleNormal || style == reportStyleLabel {
			if colon := strings.Index(line, ": "); colon > 0 && colon < 45 && !strings.HasPrefix(line, "• ") {
				label, value := reportXMLText(line[:colon+1]), reportXMLText(line[colon+1:])
				paragraphs.WriteString(`<w:p>` + pPr + `<w:r><w:rPr><w:b/><w:i/><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/></w:rPr><w:t xml:space="preserve">` + label + `</w:t></w:r><w:r><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/></w:rPr><w:t xml:space="preserve">` + value + `</w:t></w:r></w:p>`)
				continue
			}
		}
		paragraphs.WriteString(`<w:p>` + pPr + `<w:r><w:rPr>` + rPr + `</w:rPr><w:t xml:space="preserve">` + reportXMLText(line) + `</w:t></w:r></w:p>`)
	}
	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` + paragraphs.String() + `<w:sectPr><w:footerReference w:type="default" r:id="rId1"/><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1134"/></w:sectPr></w:body></w:document>`
	footer := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:jc w:val="right"/><w:spacing w:line="240" w:lineRule="auto"/></w:pPr><w:fldSimple w:instr="PAGE"><w:r><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/></w:rPr><w:t>1</w:t></w:r></w:fldSimple></w:p></w:ftr>`
	return writeReportZip(path, map[string]string{
		"[Content_Types].xml":          `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/></Types>`,
		"_rels/.rels":                  `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":            document,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/></Relationships>`,
		"word/footer1.xml":             footer,
	})
}

func laboratoryXLSXStyleIndex(style laboratoryReportStyle) int {
	return int(style)
}

func writeLaboratoryXLSXReport(path, report string) error {
	lines := laboratoryExcelReportLines(report)
	report = strings.Join(lines, "\r\n")
	sheet := fullReportXLSXSheetXML(report)
	styles := fullReportXLSXStylesXML()
	return writeReportZip(path, map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Отчёт" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              styles,
		"xl/worksheets/sheet1.xml":   sheet,
	})
}

type laboratoryPDFFallbackError struct {
	Cause              error
	DOCXPath, XLSXPath string
	DOCXErr, XLSXErr   error
}

func (e *laboratoryPDFFallbackError) Error() string {
	if e == nil {
		return "PDF не сформирован"
	}
	message := "PDF не сформирован"
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	if e.DOCXPath != "" && e.XLSXPath != "" {
		message += "\n\nДанные отчёта не потеряны. Резервные файлы сохранены:\nDOCX: " + e.DOCXPath + "\nExcel: " + e.XLSXPath
	} else {
		if e.DOCXPath != "" {
			message += "\n\nDOCX сохранён: " + e.DOCXPath
		}
		if e.XLSXPath != "" {
			message += "\n\nExcel сохранён: " + e.XLSXPath
		}
	}
	if e.DOCXErr != nil || e.XLSXErr != nil {
		message += "\n\nНе все резервные форматы удалось сохранить."
	}
	message += "\n\nДля PDF проверьте установку Microsoft Word и возможность открыть DOCX обычным двойным щелчком."
	return message
}

func laboratoryAvailableFallbackPath(pdfPath, ext string) string {
	base := strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + "_резервный"
	candidate := base + ext
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 2; i <= 99; i++ {
		candidate = fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return base + "_новый" + ext
}

func preserveLaboratoryReportFallbacks(pdfPath, tmpDocx, report string) (docxPath, xlsxPath string, docxErr, xlsxErr error) {
	docxPath = laboratoryAvailableFallbackPath(pdfPath, ".docx")
	xlsxPath = laboratoryAvailableFallbackPath(pdfPath, ".xlsx")
	data, err := os.ReadFile(tmpDocx)
	if err != nil {
		docxErr = err
		docxPath = ""
	} else if err = os.WriteFile(docxPath, data, 0o600); err != nil {
		docxErr = err
		docxPath = ""
	}
	if err = writeLaboratoryXLSXReport(xlsxPath, report); err != nil {
		xlsxErr = err
		xlsxPath = ""
	}
	return
}

const laboratoryWordPDFPowerShell = `param(
  [Parameter(Mandatory=$true)][string]$Docx,
  [Parameter(Mandatory=$true)][string]$Pdf
)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
if (-not (Test-Path -LiteralPath $Docx -PathType Leaf)) { throw 'Temporary DOCX was not found.' }
$docxItem = Get-Item -LiteralPath $Docx
if ($docxItem.Length -le 0) { throw 'Temporary DOCX is empty.' }
$word = $null
$documents = $null
$doc = $null
try {
  $word = New-Object -ComObject Word.Application
  if ($null -eq $word) { throw 'Word.Application COM object was not created.' }
  $word.Visible = $false
  $word.DisplayAlerts = 0
  $documents = $word.Documents
  if ($null -eq $documents) { throw 'Word.Documents collection is unavailable.' }
  # Open a read-only copy. Explicit arguments are more stable than relying on
  # Word defaults on machines with add-ins and non-standard Trust Center settings.
  $doc = $documents.Open($Docx, $false, $true, $false)
  if ($null -eq $doc) { throw 'Word returned an empty Document object.' }
  $doc.ExportAsFixedFormat($Pdf, 17)
} finally {
  if ($null -ne $doc) {
    try { $doc.Close(0) } catch {}
    try { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($doc) } catch {}
  }
  if ($null -ne $documents) {
    try { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($documents) } catch {}
  }
  if ($null -ne $word) {
    try { $word.Quit() } catch {}
    try { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($word) } catch {}
  }
  [GC]::Collect()
  [GC]::WaitForPendingFinalizers()
  [GC]::Collect()
  [GC]::WaitForPendingFinalizers()
}
if (-not (Test-Path -LiteralPath $Pdf -PathType Leaf)) { throw 'Word did not create the PDF file.' }
$pdfItem = Get-Item -LiteralPath $Pdf
if ($pdfItem.Length -le 0) { throw 'Word created an empty PDF file.' }
`

func validateLaboratoryGeneratedDOCX(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("временный DOCX не найден: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return fmt.Errorf("временный DOCX не создан или пуст")
	}
	return nil
}

func laboratoryPowerShellErrorText(output []byte) string {
	text := strings.ReplaceAll(strings.TrimSpace(string(output)), "\r\n", "\n")
	if text == "" {
		return "без дополнительного сообщения PowerShell"
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "CategoryInfo") || strings.HasPrefix(line, "FullyQualifiedErrorId") {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "object reference") || strings.Contains(lower, "nullreference"):
			return "Microsoft Word не смог открыть сформированный DOCX через системный интерфейс"
		case strings.Contains(lower, "word.application"):
			return "Microsoft Word не удалось запустить через системный интерфейс"
		case strings.Contains(lower, "word.documents"):
			return "Microsoft Word запущен, но коллекция документов недоступна"
		case strings.Contains(lower, "temporary docx was not found") || strings.Contains(lower, "temporary docx is empty"):
			return "промежуточный DOCX не был создан корректно"
		case strings.Contains(lower, "word returned an empty document"):
			return "Microsoft Word не открыл сформированный DOCX"
		case strings.Contains(lower, "word did not create the pdf") || strings.Contains(lower, "word created an empty pdf"):
			return "Microsoft Word не создал корректный PDF"
		}
		if len([]rune(line)) > 280 {
			runes := []rune(line)
			line = string(runes[:280]) + "…"
		}
		return line
	}
	return "Word завершил преобразование с ошибкой"
}

func runLaboratoryWordPDFConversion(tmpDir, docx, pdf string) error {
	scriptPath := filepath.Join(tmpDir, "export_word_pdf.ps1")
	if err := os.WriteFile(scriptPath, []byte(laboratoryWordPDFPowerShell), 0o600); err != nil {
		return fmt.Errorf("не удалось подготовить преобразование PDF: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-Docx", docx, "-Pdf", pdf)
	// RC367: экспорт PDF не должен показывать пользователю синее окно
	// PowerShell. Флаг CREATE_NO_WINDOW скрывает консоль на уровне Windows;
	// -WindowStyle Hidden оставлен как дополнительная защита.
	cmd.SysProcAttr = hiddenProcessAttributes()
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("преобразование PDF превысило 60 секунд; Word не ответил")
	}
	if err != nil {
		return fmt.Errorf("Microsoft Word не смог преобразовать DOCX в PDF: %s", laboratoryPowerShellErrorText(output))
	}
	return nil
}

func writeLaboratoryPDFReport(path, report string) error {
	// Формируем тот же стилизованный Word-отчёт во временном DOCX, затем просим
	// установленный Microsoft Word экспортировать его в PDF. Конвертация выполняется
	// отдельным .ps1-файлом: это надёжнее для путей с пробелами и кириллицей, чем
	// передача большого сценария через PowerShell -Command.
	tmpDir, err := os.MkdirTemp("", "rosa_lab_report_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tmpDocx := filepath.Join(tmpDir, "rosa_report.docx")
	if err = writeLaboratoryDOCXReport(tmpDocx, report); err != nil {
		return fmt.Errorf("не удалось сформировать промежуточный DOCX: %w", err)
	}
	if err = validateLaboratoryGeneratedDOCX(tmpDocx); err != nil {
		return err
	}
	absDocx, err := filepath.Abs(tmpDocx)
	if err != nil {
		return err
	}
	absPDF, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(absPDF), 0o755); err != nil {
		return fmt.Errorf("не удалось подготовить папку отчёта: %w", err)
	}
	_ = os.Remove(absPDF) // Do not accept a stale PDF left from a previous attempt.
	if err = runLaboratoryWordPDFConversion(tmpDir, absDocx, absPDF); err != nil {
		_ = os.Remove(absPDF) // Remove a partial/invalid PDF before reporting fallback files.
		docxPath, xlsxPath, docxErr, xlsxErr := preserveLaboratoryReportFallbacks(absPDF, absDocx, report)
		return &laboratoryPDFFallbackError{Cause: err, DOCXPath: docxPath, XLSXPath: xlsxPath, DOCXErr: docxErr, XLSXErr: xlsxErr}
	}
	info, statErr := os.Stat(absPDF)
	if statErr != nil || info.IsDir() || info.Size() == 0 {
		return fmt.Errorf("Microsoft Word завершил работу без готового PDF-файла")
	}
	return nil
}

func laboratoryReportVisualizationForPass(pass LabPassRecord) string {
	items := laboratoryInfluenceData(pass)
	stats := summarizeLaboratoryInfluence(items)
	lines := []string{"ВИЗУАЛЬНАЯ СВОДКА ПО НАПРАВЛЕНИЯМ"}
	for _, item := range items {
		state := strings.ToUpper(strings.TrimSpace(item.State))
		if state == "" || state == "НЕТ ОЦЕНКИ" {
			state = "НЕТ ДАННЫХ"
		}
		lines = append(lines, fmt.Sprintf("%s | %s | База %d/%d · %d%%", item.Name, state, item.BaseFilled, item.BaseTotal, laboratoryCoveragePercent(item.BaseFilled, item.BaseTotal)))
	}
	lines = append(lines, fmt.Sprintf("Базовые данные всего: %d/%d · %d%%", stats.BaseFilled, stats.BaseTotal, laboratoryCoveragePercent(stats.BaseFilled, stats.BaseTotal)))
	return strings.Join(lines, "\n")
}

func laboratoryReportVisualizationForGroup(group LabGroupDef, values map[string]string, modelCode string) string {
	group = laboratoryScaleInputGroup(group, modelCode)
	assessment := laboratoryGroupSummary(group, values, modelCode)
	state := strings.ToUpper(strings.TrimSpace(assessment.State))
	if state == "" || state == "НЕТ ОЦЕНКИ" {
		state = "НЕТ ДАННЫХ"
	}
	return fmt.Sprintf("ВИЗУАЛЬНАЯ СВОДКА ПО НАПРАВЛЕНИЯМ\n%s | %s | База %d/%d · %d%%", group.Name, state, assessment.BaseFilled, assessment.BaseTotal, laboratoryCoveragePercent(assessment.BaseFilled, assessment.BaseTotal))
}

// Keep all text, including spaces and Unicode, while fitting explanatory text
// within Excel's fixed row-height limit. A continuation uses the next row.
func laboratoryExcelReportLines(report string) []string {
	report = strings.ReplaceAll(strings.ReplaceAll(report, "\r\n", "\n"), "\r", "\n")
	var out []string
	for _, line := range strings.Split(report, "\n") {
		runes := []rune(line)
		for len(runes) > 900 {
			cut := 900
			for i := 899; i >= 450; i-- {
				if runes[i] == ' ' || runes[i] == '\t' {
					cut = i + 1
					break
				}
			}
			out = append(out, string(runes[:cut]))
			runes = runes[cut:]
		}
		out = append(out, string(runes))
	}
	return out
}
