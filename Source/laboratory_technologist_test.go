//go:build windows

package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestTechnologistHeadingRangesUnicodeAndParagraphs(t *testing.T) {
	text := "😀 Проба\r\n  Что это значит для технолога\r\nОбычный текст\r\nЧТО ЭТО ОЗНАЧАЕТ ДЛЯ ТЕХНОЛОГА\r\n"
	encoded := utf16.Encode([]rune(strings.ReplaceAll(text, "\r\n", "\n")))
	ranges := laboratoryTechnologistRanges(text)
	if len(ranges) != 2 {
		t.Fatalf("want two heading ranges: %v", ranges)
	}
	for _, r := range ranges {
		if !laboratoryTechnologistHeading(string(utf16.Decode(encoded[r.Start:r.End]))) {
			t.Fatalf("wrong UTF16 selection: %v", r)
		}
	}
	if laboratoryTechnologistHeading("Что это значит для технолога: обычное пояснение") {
		t.Fatal("body text must not be accented")
	}
}

func TestPracticalExplanationAllDirections(t *testing.T) {
	for _, code := range []string{"corrosion", "scale", "aspo", "solids", "microbiology", "gases"} {
		text := laboratoryPlainLanguageSummary(LabGroupDef{Code: code}, "")
		for _, want := range []string{"ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА", "Что происходит:", "Чем это грозит:", "Что проверить первым:", "Как принять решение:", "Как понять, помогли ли меры:", "данных пока недостаточно"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", code, want)
			}
		}
	}
}

func TestTechnologistHeadingBoldInOverallAndIndividual(t *testing.T) {
	for _, prefix := range []string{"", "ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ\n"} {
		text := prefix + "ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА\nОбычный текст"
		found := false
		runes := []rune(text)
		for _, r := range laboratoryBoldRanges(text) {
			if laboratoryTechnologistHeading(string(runes[r.Start:r.End])) {
				found = true
			}
		}
		if !found {
			t.Fatal("technologist heading missing from bold selection")
		}
	}
}

func TestPracticalExplanationUsesFourStepTechnologistSchema(t *testing.T) {
	group, _ := labGroupByCode("scale")
	text := laboratoryPracticalExplanation(group, laboratoryResultSummary{Message: "Есть предпосылка по кальциту."})
	for _, want := range []string{
		"Что происходит:",
		"Что означают именно эти данные:",
		"Чего пока нельзя утверждать:",
		"Что проверить следующим действием:",
		"пересыщенность не доказывает наличие фактического осадка",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("technologist schema misses %q:\n%s", want, text)
		}
	}
}

func TestOverallTechnologistSummaryIsIntegratedNotSixRepeatedManuals(t *testing.T) {
	pass := LabPassRecord{Groups: map[string]map[string]string{
		"corrosion": {"ph": "6.1"},
		"aspo":      {"paraffins": "4"},
	}}
	text := laboratoryPassTechnologistSummary(pass)
	for _, want := range []string{"СВОД ПО НАПРАВЛЕНИЯМ", "КОМПЛЕКСНАЯ ИНЖЕНЕРНАЯ ИНТЕРПРЕТАЦИЯ", "Следующее действие:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("overall technologist summary misses %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА") != 1 {
		t.Fatalf("overall summary mechanically duplicated technologist manuals:\n%s", text)
	}
	if strings.Contains(text, "Чем это грозит:") || strings.Contains(text, "Как принять решение:") {
		t.Fatalf("overall summary still duplicates long private manuals:\n%s", text)
	}
}

func TestFourStepTechnologistTextIsDistinctForAllSixDirections(t *testing.T) {
	stockCaveats := []string{
		"Это основание для проверки, а не подтверждённый факт повреждения.",
		"Это не исключает осложнение при другом составе, режиме или по неучтённым причинам.",
		"Это не означает, что осложнения нет.",
	}
	for _, code := range []string{"corrosion", "scale", "aspo", "solids", "microbiology", "gases"} {
		group, ok := labGroupByCode(code)
		if !ok {
			t.Fatalf("group %s not found", code)
		}
		summary := laboratoryResultSummary{Message: "Есть признаки возможного осложнения. Проверочный фактор: 1. Это основание для проверки, а не подтверждённый факт повреждения. Данные или условия оценки требуют уточнения; найденные признаки при этом не отменяются."}
		text := laboratoryPracticalExplanation(group, summary)
		for _, heading := range []string{"Что происходит:", "Что означают именно эти данные:", "Чего пока нельзя утверждать:", "Что проверить следующим действием:"} {
			if strings.Count(text, heading) != 1 {
				t.Fatalf("%s must contain %q exactly once:\n%s", code, heading, text)
			}
		}
		if strings.Contains(text, "Чем это грозит:") || strings.Contains(text, "Как принять решение:") || strings.Contains(text, "Как понять, помогли ли меры:") {
			t.Fatalf("%s contains duplicated legacy manual blocks:\n%s", code, text)
		}
		data := laboratoryPlainSection(text, "Что означают именно эти данные:", "Чего пока нельзя утверждать:")
		for _, caveat := range stockCaveats {
			if strings.Contains(data, caveat) {
				t.Fatalf("%s repeats stock caveat in data paragraph: %q\n%s", code, caveat, text)
			}
		}
		mechanism := laboratoryPlainSection(text, "Что происходит:", "Что означают именно эти данные:")
		if strings.Contains(strings.ToLower(mechanism), "нельзя") || strings.Contains(strings.ToLower(mechanism), "не доказы") {
			t.Fatalf("%s mechanism paragraph duplicates limitations:\n%s", code, text)
		}
	}
}

func TestSavedHistoryUsesCurrentFourStepConclusionInsteadOfStoredLegacyText(t *testing.T) {
	pass := LabPassRecord{
		PassID: "LP-HISTORY", ObjectKey: "OBJ-1", Date: "15.09.2026", ScaleModel: "davies",
		Groups:     map[string]map[string]string{"corrosion": {"h2s": "2"}},
		Units:      map[string]map[string]string{"corrosion": {"h2s": "мг/л"}},
		Conclusion: "СТАРЫЙ ТЕКСТ: требует внимания без пояснения",
	}
	text := laboratoryHistoryConclusion(pass, "corrosion")
	for _, want := range []string{
		"ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА",
		"Что происходит:",
		"Что означают именно эти данные:",
		"Чего пока нельзя утверждать:",
		"Что проверить следующим действием:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("history conclusion misses %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "СТАРЫЙ ТЕКСТ") {
		t.Fatalf("stored legacy conclusion leaked into rebuilt history text:\n%s", text)
	}
}

func TestLegacyRecordPracticalExplanationUsesFourStepSchema(t *testing.T) {
	group, _ := labGroupByCode("corrosion")
	record := LabRecord{GroupCode: "corrosion", GroupName: "Коррозия", Values: map[string]string{"h2s": "2"}}
	text := laboratoryRecordPracticalExplanation(group, record)
	for _, want := range []string{"Что происходит:", "Что означают именно эти данные:", "Чего пока нельзя утверждать:", "Что проверить следующим действием:"} {
		if strings.Count(text, want) != 1 {
			t.Fatalf("legacy history must use current four-step schema, missing/duplicated %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Следующее действие:") || strings.Contains(text, "Ограничение:") {
		t.Fatalf("legacy three-line schema still present:\n%s", text)
	}
}

func TestSavedScaleHistoryUsesSavedModelAndDoesNotRepeatLimitations(t *testing.T) {
	pass := LabPassRecord{
		PassID: "LP-SCALE-HISTORY", ObjectKey: "OBJ-S", Date: "15.09.2026", ScaleModel: "ideal",
		Groups: map[string]map[string]string{"scale": {
			"ca": "40.078", "co3": "60.01", "mg": "0", "na_k": "0", "so4": "0", "hco3": "0", "cl": "0", "mineralization": "1", "ph": "7", "sample_temp": "25",
		}},
		Conclusion: "СТАРАЯ ИНТЕРПРЕТАЦИЯ ПО ДЭВИСУ",
	}
	text := laboratoryHistoryConclusion(pass, "scale")
	if !strings.Contains(text, "Модель: Идеальный раствор") {
		t.Fatalf("saved ScaleModel not used in history reconstruction:\n%s", text)
	}
	if strings.Contains(text, "СТАРАЯ ИНТЕРПРЕТАЦИЯ") {
		t.Fatalf("stored text used instead of current reconstruction:\n%s", text)
	}
	if strings.Count(text, "Чего пока нельзя утверждать:") != 1 {
		t.Fatalf("limitations block must occur once:\n%s", text)
	}
	data := laboratoryPlainSection(text, "Что означают именно эти данные:", "Чего пока нельзя утверждать:")
	if strings.Contains(data, "пересыщенность не доказывает наличие фактического осадка") {
		t.Fatalf("scale limitation duplicated inside data paragraph:\n%s", text)
	}
}

func TestSavedPassHistoryShortConclusionUsesActualSectionResult(t *testing.T) {
	pass := LabPassRecord{
		PassID: "LP-SHORT", ObjectKey: "OBJ-1", Date: "15.09.2026", ScaleModel: "davies",
		Groups: map[string]map[string]string{"corrosion": {"h2s": "2"}},
		Units:  map[string]map[string]string{"corrosion": {"h2s": "мг/л"}},
	}
	got := laboratoryPassGroupShortConclusion(pass, "corrosion")
	if !strings.Contains(got, "Коррозия: Есть признаки возможного осложнения.") {
		t.Fatalf("saved pass short conclusion must reflect calculated section result, got: %s", got)
	}
	if strings.Contains(got, "входят в единую лабораторную диагностику") {
		t.Fatalf("service placeholder leaked into short conclusion: %s", got)
	}
}

func TestSavedPassHistoryOverallShortConclusionSummarizesFilledDirections(t *testing.T) {
	pass := LabPassRecord{
		PassID: "LP-OVERALL-SHORT", ObjectKey: "OBJ-1", Date: "15.09.2026", ScaleModel: "davies",
		Groups: map[string]map[string]string{
			"corrosion": {"h2s": "2"},
			"aspo":      {"paraffins": "4"},
		},
		Units: map[string]map[string]string{},
	}
	got := laboratoryPassOverallShortConclusion(pass)
	for _, want := range []string{"Учтено 2 из 6", "признаки/погранично — 2", "по доступным данным без признаков — 0", "оценить пока нельзя — 0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("overall saved pass short conclusion misses %q: %s", want, got)
		}
	}
}

func TestHistoryColumnsUseCalculatedPassShortConclusion(t *testing.T) {
	previous := labActiveGroup
	defer func() { labActiveGroup = previous }()
	pass := LabPassRecord{
		PassID: "LP-COLUMNS", ObjectKey: "OBJ-1", Date: "15.09.2026", ScaleModel: "davies",
		Groups:    map[string]map[string]string{"corrosion": {"h2s": "2"}},
		Units:     map[string]map[string]string{"corrosion": {"h2s": "мг/л"}},
		Completed: []string{"corrosion"},
	}
	entry := laboratoryHistoryEntry{Kind: "pass", Pass: pass}

	labActiveGroup = "corrosion"
	privateColumns := laboratoryHistoryEntryColumns(entry)
	if len(privateColumns) != 8 || !strings.Contains(privateColumns[7], "Есть признаки возможного осложнения.") {
		t.Fatalf("private history row must contain calculated short result: %#v", privateColumns)
	}
	if strings.Contains(privateColumns[7], "Исходные данные раздела входят") {
		t.Fatalf("private history row still contains service placeholder: %s", privateColumns[7])
	}

	labActiveGroup = "overall"
	overallColumns := laboratoryHistoryEntryColumns(entry)
	if len(overallColumns) != 8 || !strings.Contains(overallColumns[7], "Учтено 1 из 6") {
		t.Fatalf("overall history row must contain compact filled-direction summary: %#v", overallColumns)
	}
}

func TestHistoryCoverageUsesSavedScaleModel(t *testing.T) {
	group, ok := labGroupByCode("scale")
	if !ok {
		t.Fatal("scale group not found")
	}
	values := map[string]string{
		"ca":               "120",
		"mix2_ca_total":    "999",                 // belongs only to the mixing model
		"water_type_sulin": "Хлоридно-кальциевый", // calculated output
		"watercut":         "77",                  // tech-regime context
	}
	filled, total := laboratoryHistoryFieldCoverage(group, values, "davies")
	if filled != 1 {
		t.Fatalf("saved davies history counted inactive/calculated/context fields: filled=%d total=%d", filled, total)
	}
	expectedTotal := 0
	for _, field := range laboratoryScaleInputGroup(group, "davies").Fields {
		if laboratoryCalculatedField(group.Code, field.Key) || laboratoryTechRegimeKey(group.Code, field.Key) {
			continue
		}
		expectedTotal++
	}
	if total != expectedTotal {
		t.Fatalf("saved davies denominator mismatch: got %d want %d", total, expectedTotal)
	}

	mixFilled, mixTotal := laboratoryHistoryFieldCoverage(group, values, "water_mixing")
	if mixFilled != 1 {
		t.Fatalf("saved mixing model must count its own active mix2 field only here: filled=%d total=%d", mixFilled, mixTotal)
	}
	if mixTotal == total {
		t.Fatalf("different saved scale models unexpectedly share the same history denominator: %d", total)
	}
}

func TestOverallHistoryFilledDirectionsIgnoreStaleCompleted(t *testing.T) {
	previous := labActiveGroup
	defer func() { labActiveGroup = previous }()
	labActiveGroup = "overall"
	pass := LabPassRecord{
		PassID: "LP-STALE-COMPLETED", ObjectKey: "OBJ-1", Date: "18.09.2026", ScaleModel: "davies",
		Groups: map[string]map[string]string{
			"corrosion": {"ph": "7.2"},
			"scale":     {"water_type_sulin": "Хлоридно-кальциевый"}, // calculated-only group must not count
		},
		Completed: []string{"corrosion", "scale", "aspo", "solids", "microbiology", "gases"},
	}
	cols := laboratoryHistoryEntryColumns(laboratoryHistoryEntry{Kind: "pass", Pass: pass})
	if len(cols) != 8 {
		t.Fatalf("unexpected history columns: %#v", cols)
	}
	if cols[5] != "1 разд." {
		t.Fatalf("overall filled directions must come from actual group inputs, not stale Completed: %q", cols[5])
	}
	if !strings.Contains(cols[7], "Учтено 1 из 6") {
		t.Fatalf("overall short conclusion must use the same actual group count: %q", cols[7])
	}
}

func TestPassConclusionCoverageUsesSavedScaleModel(t *testing.T) {
	pass := LabPassRecord{
		PassID: "LP-COVERAGE-MODEL", ObjectKey: "OBJ-COVERAGE", Date: "18.09.2026", ScaleModel: "davies",
		Groups: map[string]map[string]string{
			"scale": {
				"ca":               "120",
				"mix2_ca_total":    "999",
				"water_type_sulin": "Хлоридно-кальциевый",
				"watercut":         "77",
			},
		},
	}
	group, _ := labGroupByCode("scale")
	filled, total := laboratoryHistoryFieldCoverage(group, pass.Groups["scale"], pass.ScaleModel)
	text := buildLaboratoryPassConclusion(pass)
	want := fmt.Sprintf("Заполнено показателей: %d из %d.", filled, total)
	if !strings.Contains(text, want) {
		t.Fatalf("pass conclusion does not use saved-model laboratory coverage; want %q in:\n%s", want, text)
	}
	if strings.Contains(text, "999") && filled == 1 {
		// Presence in the raw-source listing is allowed; this guard only documents
		// that the inactive mixing field must not inflate the coverage number.
	}
}
