//go:build windows

package main

import (
	"archive/zip"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const laboratoryWordMaxSize = 64 * 1024 * 1024

type laboratoryWordMetadata struct {
	Object  string
	Date    string
	Group   string
	Source  string
	Comment string
}

func laboratoryWordMetadataValue(value string) string {
	return strings.TrimSpace(strings.TrimLeft(value, "№#:=—–- "))
}

func assignLaboratoryWordMetadata(metadata *laboratoryWordMetadata, key, value string) bool {
	key = normalizeLabHeader(key)
	value = laboratoryWordMetadataValue(value)
	if value == "" {
		return false
	}
	switch {
	case key == "объект" || key == "ключ объекта" || key == "код объекта" || key == "шифр объекта" || key == "скважина" || key == "номер скважины" || key == "№ скважины" || key == "№ скв":
		if metadata.Object == "" {
			metadata.Object = value
		}
	case key == "дата" || key == "дата анализа" || key == "дата исследования" || key == "дата отбора" || key == "дата протокола":
		if metadata.Date == "" {
			metadata.Date = value
		}
	case key == "раздел" || key == "направление анализа" || key == "тип анализа" || key == "группа" || key == "осложнение":
		if metadata.Group == "" {
			metadata.Group = value
		}
	case key == "источник" || key == "источник данных" || key == "протокол" || key == "номер протокола" || key == "лаборатория":
		if metadata.Source == "" {
			metadata.Source = value
		}
	case key == "комментарий" || key == "примечание" || key == "комментарий лаборатории":
		if metadata.Comment == "" {
			metadata.Comment = value
		}
	default:
		return false
	}
	return true
}

func collectLaboratoryWordMetadata(rows [][]string, headerRow int) laboratoryWordMetadata {
	var metadata laboratoryWordMetadata
	datePattern := regexp.MustCompile(`(?i)\b(\d{1,2}[./-]\d{1,2}[./-]\d{4}|\d{4}[./-]\d{1,2}[./-]\d{1,2})\b`)
	protocolPattern := regexp.MustCompile(`(?i)(?:протокол|номер протокола)\s*(?:№|N)?\s*[:#-]?\s*([^,;\r\n]+)`)
	wellPattern := regexp.MustCompile(`(?i)(?:скважина|скв\.)\s*(?:№|N)?\s*[:#-]?\s*([\p{L}\d._/-]+)`)
	for i := 0; i < headerRow && i < len(rows); i++ {
		row := rows[i]
		if len(row) >= 2 && assignLaboratoryWordMetadata(&metadata, row[0], row[1]) {
			continue
		}
		line := strings.TrimSpace(strings.Join(row, " "))
		if line == "" {
			continue
		}
		for _, delimiter := range []string{":", "—", "–", " - "} {
			if parts := strings.SplitN(line, delimiter, 2); len(parts) == 2 && assignLaboratoryWordMetadata(&metadata, parts[0], parts[1]) {
				break
			}
		}
		if metadata.Date == "" {
			metadata.Date = datePattern.FindString(line)
		}
		if metadata.Source == "" {
			if match := protocolPattern.FindStringSubmatch(line); len(match) == 2 {
				metadata.Source = "Протокол " + laboratoryWordMetadataValue(match[1])
			}
		}
		if metadata.Object == "" {
			if match := wellPattern.FindStringSubmatch(line); len(match) == 2 {
				metadata.Object = laboratoryWordMetadataValue(match[1])
			}
		}
	}
	return metadata
}

func enrichLaboratoryWordRows(rows [][]string) [][]string {
	if len(rows) < 2 {
		return rows
	}
	headerRow := detectLaboratoryHeaderRow(rows)
	if headerRow < 0 || headerRow >= len(rows) {
		return rows
	}
	metadata := collectLaboratoryWordMetadata(rows, headerRow)
	head := append([]string(nil), rows[headerRow]...)
	originalColumns := len(head)
	type metadataColumn struct {
		header  string
		aliases []string
		value   string
	}
	columns := []metadataColumn{
		{"Ключ объекта", []string{"ключ объекта", "шифр объекта", "код объекта", "объект", "скважина", "номер скважины", "№ скв"}, metadata.Object},
		{"Дата", []string{"дата", "дата анализа", "дата исследования", "дата отбора"}, metadata.Date},
		{"Раздел", []string{"раздел", "направление анализа", "тип анализа", "группа", "осложнение"}, metadata.Group},
		{"Источник", []string{"источник", "источник данных", "протокол", "номер протокола", "лаборатория"}, metadata.Source},
		{"Комментарий", []string{"комментарий", "комментарий лаборатории", "примечание", "примечания"}, metadata.Comment},
	}
	indices := make([]int, len(columns))
	for i, column := range columns {
		indices[i] = labHeaderIndex(head, column.aliases...)
		if indices[i] < 0 && column.value != "" {
			indices[i] = len(head)
			head = append(head, column.header)
		}
	}
	enriched := make([][]string, len(rows))
	for i := range rows {
		enriched[i] = append([]string(nil), rows[i]...)
	}
	enriched[headerRow] = head
	for i := headerRow + 1; i < len(enriched); i++ {
		// Table rows retain all cells, including empty trailing cells. Narrative
		// paragraphs below the table must not be converted into fake analyses.
		if len(enriched[i]) < originalColumns {
			continue
		}
		for len(enriched[i]) < len(head) {
			enriched[i] = append(enriched[i], "")
		}
		for j, column := range columns {
			index := indices[j]
			if index >= 0 && index < len(enriched[i]) && strings.TrimSpace(enriched[i][index]) == "" && column.value != "" {
				enriched[i][index] = column.value
			}
		}
	}
	return enriched
}

func laboratoryWordRowsFromText(text string) [][]string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		delim := detectDelimiter(line)
		parts := strings.Split(line, string(delim))
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rows = append(rows, parts)
	}
	return rows
}

func readLaboratoryDOCX(path string) ([][]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > laboratoryWordMaxSize {
		return nil, fmt.Errorf("лабораторный DOCX больше 64 МБ; разделите документ перед импортом")
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("DOCX повреждён или не является документом Word: %w", err)
	}
	defer zr.Close()
	var document *zip.File
	for _, file := range zr.File {
		if file.Name == "word/document.xml" {
			document = file
			break
		}
	}
	if document == nil {
		return nil, fmt.Errorf("в DOCX не найден основной текст документа")
	}
	if document.UncompressedSize64 > 32*1024*1024 {
		return nil, fmt.Errorf("текстовая часть DOCX больше 32 МБ; разделите документ")
	}
	r, err := document.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	rows, err := parseLaboratoryDOCXXML(io.LimitReader(r, 32*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать структуру DOCX: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("в DOCX не найдена таблица или строки лабораторных данных")
	}
	return enrichLaboratoryWordRows(rows), nil
}

func parseLaboratoryDOCXXML(r io.Reader) ([][]string, error) {
	decoder := xml.NewDecoder(r)
	var rows [][]string
	var row []string
	var cell, paragraph strings.Builder
	inRow, inCell, inParagraph := false, false, false
	appendParagraph := func() {
		value := strings.TrimSpace(paragraph.String())
		paragraph.Reset()
		if value != "" && !inRow {
			rows = append(rows, laboratoryWordRowsFromText(value)...)
		}
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "tr":
				appendParagraph()
				inRow = true
				row = nil
			case "tc":
				inCell = true
				cell.Reset()
			case "p":
				inParagraph = true
				if !inCell {
					paragraph.Reset()
				}
			case "tab":
				if inCell {
					cell.WriteByte('\t')
				} else if inParagraph {
					paragraph.WriteByte('\t')
				}
			case "br", "cr":
				if inCell {
					cell.WriteByte(' ')
				} else if inParagraph {
					paragraph.WriteByte('\n')
				}
			}
		case xml.CharData:
			if inCell {
				cell.Write([]byte(value))
			} else if inParagraph {
				paragraph.Write([]byte(value))
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "tc":
				row = append(row, strings.TrimSpace(cell.String()))
				cell.Reset()
				inCell = false
			case "tr":
				if len(row) > 0 {
					rows = append(rows, append([]string(nil), row...))
				}
				row = nil
				inRow = false
			case "p":
				if !inCell {
					appendParagraph()
				}
				inParagraph = false
			}
		}
	}
	appendParagraph()
	return enrichLaboratoryWordRows(rows), nil
}

func laboratoryTextScore(text string) int {
	n := normalizeLabHeader(text)
	score := utf8.RuneCountInString(text) / 20
	for _, word := range []string{"объект", "скваж", "дата", "показател", "значен", "результат", "протокол", "ph", "корроз", "микробиолог"} {
		if strings.Contains(n, word) {
			score += 100
		}
	}
	return score
}

func extractLegacyDOCUTF16(data []byte, offset int) string {
	var best, current strings.Builder
	flush := func() {
		value := strings.TrimSpace(current.String())
		if laboratoryTextScore(value) > laboratoryTextScore(best.String()) {
			best.Reset()
			best.WriteString(value)
		}
		current.Reset()
	}
	for i := offset; i+1 < len(data); i += 2 {
		r := rune(binary.LittleEndian.Uint16(data[i : i+2]))
		if r == '\r' || r == '\n' || r == '\t' || unicode.IsPrint(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return best.String()
}

func extractLegacyDOCSingleByte(data []byte) string {
	var best []byte
	start := -1
	flush := func(end int) {
		if start >= 0 && end-start > len(best) {
			best = append(best[:0], data[start:end]...)
		}
		start = -1
	}
	for i, value := range data {
		printable := value == '\r' || value == '\n' || value == '\t' || (value >= 0x20 && value <= 0x7e) || value >= 0xc0
		if printable {
			if start < 0 {
				start = i
			}
		} else {
			flush(i)
		}
	}
	flush(len(data))
	return decodeCP1251(best)
}

func readLaboratoryDOC(path string) ([][]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > laboratoryWordMaxSize {
		return nil, fmt.Errorf("лабораторный DOC больше 64 МБ; разделите документ перед импортом")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	candidates := []string{
		extractLegacyDOCUTF16(data, 0),
		extractLegacyDOCUTF16(data, 1),
		extractLegacyDOCSingleByte(data),
	}
	best := ""
	for _, candidate := range candidates {
		if laboratoryTextScore(candidate) > laboratoryTextScore(best) {
			best = candidate
		}
	}
	rows := laboratoryWordRowsFromText(best)
	if len(rows) < 2 {
		return nil, fmt.Errorf("в DOC не удалось выделить таблицу лабораторных данных; откройте старый документ в Word и сохраните как DOCX")
	}
	return rows, nil
}
