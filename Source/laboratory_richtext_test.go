//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestNormalizeLaboratoryLineBreaks(t *testing.T) {
	plain := normalizeLaboratoryLineBreaks("  Заголовок\r\nСтрока 2\rСтрока 3\n  ")
	if plain != "Заголовок\nСтрока 2\nСтрока 3" {
		t.Fatalf("unexpected normalized text: %q", plain)
	}
	display := strings.ReplaceAll(plain, "\n", "\r\n")
	if strings.Contains(strings.ReplaceAll(display, "\r\n", ""), "\n") {
		t.Fatalf("display contains a bare LF: %q", display)
	}
}

func TestLaboratoryBoldRangesIncludeCorrosionSections(t *testing.T) {
	display := strings.ReplaceAll("РАСШИРЕННАЯ ОЦЕНКА ВЛИЯНИЯ НА КОРРОЗИЮ\n\nКИСЛОРОД O₂\nУровень влияния: высокий.\n\nДИОКСИД УГЛЕРОДА CO₂\n\nСЕРОВОДОРОД H₂S\n\nСОВМЕСТНОЕ ВЛИЯНИЕ O₂ + CO₂ + H₂S\nНедостаёт: давление процесса.", "\n", "\r\n")
	ranges := laboratoryBoldRanges(display)
	var bold []string
	runes := []rune(display)
	_ = runes
	encoded := []rune(strings.ReplaceAll(display, "\r\n", "\n"))
	for _, r := range ranges {
		// These fixtures contain only BMP symbols, so UTF-16 positions and rune
		// positions are identical. Production code uses UTF-16 lengths explicitly.
		if r.Start >= 0 && r.End <= len(encoded) && r.Start < r.End {
			bold = append(bold, string(encoded[r.Start:r.End]))
		}
	}
	joined := strings.Join(bold, "|")
	for _, want := range []string{"РАСШИРЕННАЯ ОЦЕНКА ВЛИЯНИЯ НА КОРРОЗИЮ", "КИСЛОРОД O₂", "ДИОКСИД УГЛЕРОДА CO₂", "СЕРОВОДОРОД H₂S", "СОВМЕСТНОЕ ВЛИЯНИЕ O₂ + CO₂ + H₂S", "Уровень влияния:", "Недостаёт:"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("bold ranges do not contain %q: %q", want, joined)
		}
	}
}

func TestLaboratoryOverallBoldRangesSelectStructureAndFieldOnly(t *testing.T) {
	display := strings.ReplaceAll("ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ\n\nОбъект: OBJ-1\nРАЗДЕЛ: КОРРОЗИЯ\nИсточник раздела: протокол 17. Дата данных: 01.09.2026.\n\nИСХОДНЫЕ ДАННЫЕ РАЗДЕЛА\n• [Б/О; источник: протокол 17; дата: 01.09.2026] pH: 6.2 ед.\nОбычная поясняющая строка не должна быть жирной.\n\nОБЩИЙ ВЫВОД\nТекст общего вывода.", "\n", "\r\n")
	ranges := laboratoryBoldRanges(display)
	encoded := []rune(strings.ReplaceAll(display, "\r\n", "\n"))
	var bold []string
	for _, r := range ranges {
		if r.Start >= 0 && r.End <= len(encoded) && r.Start < r.End {
			bold = append(bold, string(encoded[r.Start:r.End]))
		}
	}
	joined := strings.Join(bold, "|")
	for _, want := range []string{"ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ", "Объект:", "РАЗДЕЛ: КОРРОЗИЯ", "Источник раздела:", "ИСХОДНЫЕ ДАННЫЕ РАЗДЕЛА", "pH", "ОБЩИЙ ВЫВОД"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("overall bold ranges do not contain %q: %q", want, joined)
		}
	}
	for _, unwanted := range []string{"протокол 17; дата", "Обычная поясняющая строка", "Текст общего вывода"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("overall bold ranges unexpectedly contain %q: %q", unwanted, joined)
		}
	}
}

func TestLaboratoryBoldRangesDoNotBoldHeadingValue(t *testing.T) {
	display := "Предварительное заключение: Соли\r\nОбязательный базовый уровень данных: сформирован (4 из 4 показателей)."
	ranges := laboratoryBoldRanges(display)
	encoded := []rune(strings.ReplaceAll(display, "\r\n", "\n"))
	var bold []string
	for _, r := range ranges {
		if r.Start >= 0 && r.End <= len(encoded) && r.Start < r.End {
			bold = append(bold, string(encoded[r.Start:r.End]))
		}
	}
	joined := strings.Join(bold, "|")
	if strings.Contains(joined, "Соли") || strings.Contains(joined, "сформирован") {
		t.Fatalf("values must remain regular weight: %q", joined)
	}
}
