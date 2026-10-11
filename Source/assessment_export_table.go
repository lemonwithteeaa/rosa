package main

import "strings"

// assessmentExportRow keeps each archived classification approval independent.
// It is intentionally built from the report snapshot text: exporting an older
// approved report must not replace its values with the current object state.
type assessmentExportRow struct {
	Complication string
	Program      string
	Technologist string
	Status       string
	Comment      string
	ApprovedBy   string
	ApprovedAt   string
}

func assessmentExportParseDecision(line string) (assessmentExportRow, bool) {
	var row assessmentExportRow
	line = strings.TrimSpace(line)
	marker := ""
	for _, prefix := range []string{"✓ ", "⚠ ", "… "} {
		if strings.HasPrefix(line, prefix) {
			marker = prefix
			line = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			break
		}
	}
	if marker == "" {
		return row, false
	}
	colon := strings.Index(line, ": ")
	if colon < 1 || !strings.Contains(line, "; технолог ") || !strings.Contains(line, "; программа ") {
		return row, false
	}
	row.Complication = strings.TrimSpace(line[:colon])
	parts := strings.Split(line[colon+2:], ";")
	mode := ""
	for _, part := range parts {
		part = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), "."))
		switch {
		case strings.HasPrefix(part, "утверждённый итог "):
			// Actual signed final result can differ from the technologist's draft.
		case strings.HasPrefix(part, "выбранный вариант "):
			// A draft is never labelled as approved.
		case strings.HasPrefix(part, "технолог "):
			row.Technologist = strings.TrimSpace(strings.TrimPrefix(part, "технолог "))
		case strings.HasPrefix(part, "программа "):
			row.Program = strings.TrimSpace(strings.TrimPrefix(part, "программа "))
		case strings.HasPrefix(part, "решение — "):
			mode = strings.TrimSpace(strings.TrimPrefix(part, "решение — "))
		}
	}
	if strings.Contains(line, "ОЖИДАЕТ ОБЩЕГО УТВЕРЖДЕНИЯ") || marker == "… " {
		row.Status = "Не утверждено"
	} else {
		row.Status = "Утверждено"
	}
	if strings.EqualFold(mode, "без корректировки") {
		row.Status += "; без корректировки"
	} else if mode != "" && mode != "—" {
		row.Status += "; " + mode
	}
	if row.Program == "" {
		row.Program = "Не указано"
	}
	if row.Technologist == "" {
		row.Technologist = "Не указано"
	}
	return row, true
}

// assessmentExportParseBlock parses only the exact legacy approval block. No
// changes to the saved data or to the interactive assessment page are needed.
func assessmentExportParseBlock(lines []string, start int) (rows []assessmentExportRow, end int, commonComment string, ok bool) {
	end = start
	if start < 0 || start >= len(lines) || strings.TrimSpace(lines[start]) != "КЛАССИФИКАЦИЯ ОСЛОЖНЕНИЙ — ПОДТВЕРЖДЕНИЕ ТЕХНОЛОГА" {
		return nil, start, "", false
	}
	end++
	for end < len(lines) {
		line := strings.TrimSpace(lines[end])
		if line == "" {
			if len(rows) > 0 {
				break
			}
			end++
			continue
		}
		if row, valid := assessmentExportParseDecision(line); valid {
			rows = append(rows, row)
			end++
			continue
		}
		if len(rows) > 0 {
			// This summary repeats each approval status already shown by the table.
			if strings.HasPrefix(line, "Полнота утверждения:") {
				end++
				continue
			}
			last := &rows[len(rows)-1]
			switch {
			case strings.HasPrefix(line, "Комментарий: "):
				last.Comment = strings.TrimSpace(strings.TrimPrefix(line, "Комментарий: "))
				end++
				continue
			case strings.HasPrefix(line, "Утвердил: "):
				value := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "Утвердил: ")), ".")
				authorDate := strings.SplitN(value, ";", 2)
				last.ApprovedBy = strings.TrimSpace(authorDate[0])
				if len(authorDate) == 2 {
					last.ApprovedAt = strings.TrimSpace(authorDate[1])
				}
				end++
				continue
			}
		}
		break
	}
	if len(rows) == 0 {
		return nil, start, "", false
	}
	// Collapse one boilerplate note repeated for every direction, but never
	// discard distinct technologist comments.
	if len(rows) > 1 && strings.TrimSpace(rows[0].Comment) != "" {
		allSame := true
		for _, row := range rows[1:] {
			if row.Comment != rows[0].Comment {
				allSame = false
				break
			}
		}
		if allSame {
			commonComment = rows[0].Comment
			for i := range rows {
				rows[i].Comment = "—"
			}
		}
	}
	return rows, end, commonComment, true
}
