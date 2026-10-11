//go:build windows

package main

import "strings"

type reportChemicalTreatment struct {
	method string
	kind   string
	fields map[string]string
}

func reportChemistryValue(v string) string {
	v = strings.TrimSpace(strings.TrimSuffix(v, "[изменено технологом]"))
	if v == "" || v == "—" || strings.EqualFold(v, "Не выбрано") {
		return "не задано"
	}
	return v
}

func reportChemicalTreatments(section string) []reportChemicalTreatment {
	// Only source actual fields recorded inside this report's approved snapshot.
	// Do not pull a new scheme from the UI or from a later saved method version.
	lines := strings.Split(strings.ReplaceAll(section, "\r\n", "\n"), "\n")
	inParams := false
	var records []reportChemicalTreatment
	current := -1
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "ПАРАМЕТРЫ РЕАЛИЗАЦИИ" {
			inParams = true
			current = -1
			continue
		}
		if !inParams {
			continue
		}
		if line == "ПЛАН ИСПОЛНЕНИЯ И КОНТРОЛЯ" || line == "ИТОГОВОЕ ОСНОВАНИЕ ВЫБОРА ВАРИАНТА" || line == "ВЗАИМОДЕЙСТВИЕ МЕТОДОВ" || line == "ОГРАНИЧЕНИЕ" {
			break
		}
		if strings.Contains(line, " — Дозирование") || strings.Contains(line, " — Промывка / обработка") {
			before, kind, _ := strings.Cut(line, " — ")
			records = append(records, reportChemicalTreatment{method: strings.TrimSpace(before), kind: kind, fields: map[string]string{}})
			current = len(records) - 1
			continue
		}
		if current < 0 || !strings.HasPrefix(line, "• ") {
			continue
		}
		label, value, ok := strings.Cut(strings.TrimPrefix(line, "• "), ": ")
		if ok {
			records[current].fields[strings.ToLower(strings.TrimSpace(label))] = reportChemistryValue(value)
		}
	}
	return records
}

func reportChemicalOverview(section string) string {
	treatments := reportChemicalTreatments(section)
	var out strings.Builder
	for _, t := range treatments {
		reagent := t.fields["реагент"]
		if reagent == "" {
			reagent = t.fields["растворитель / реагент"]
		}
		if reagent == "" {
			continue
		}
		if out.Len() == 0 {
			out.WriteString("СХЕМЫ ПРИМЕНЕНИЯ ХИМРЕАГЕНТОВ\r\n")
		}
		out.WriteString("\r\nМетод: " + t.method + "\r\n")
		reagentLabel := reportChemistryValue(reagent)
		if strings.Contains(strings.ToLower(reagentLabel), "уточнить марку") || strings.Contains(strings.ToLower(reagentLabel), "по карточке метода") {
			reagentLabel = "марка не определена"
		}
		out.WriteString("Химреагент: " + reagentLabel + "\r\n")
		value := t.fields["концентрация / расход"]
		if value == "" {
			value = t.fields["тоннаж / концентрация"]
		}
		if value == "" {
			value = t.fields["объём"]
		}
		rate := reportChemistryValue(value)
		if strings.Contains(strings.ToLower(rate), "по лабораторному") || strings.Contains(strings.ToLower(rate), "рассчитать по") {
			rate = "норма не утверждена"
		}
		if u := reportChemistryValue(t.fields["единица измерения"]); u != "не задано" && rate != "не задано" {
			rate += " " + u
		}
		out.WriteString("Норма подачи / обработки: " + rate + "\r\n")
		basis := reportChemistryValue(t.fields["база дозирования"])
		if basis != "не задано" {
			out.WriteString("База расчёта: " + basis + "\r\n")
		}
		periodicity := reportChemistryValue(t.fields["периодичность"])
		out.WriteString("Частота / режим подачи: " + periodicity + "\r\n")
		if point := reportChemistryValue(t.fields["точка ввода"]); point != "не задано" {
			out.WriteString("Точка ввода: " + point + "\r\n")
		}
	}
	return strings.TrimSpace(out.String())
}

func reportAddChemicalOverview(section string) string {
	if strings.Contains(section, "СХЕМЫ ПРИМЕНЕНИЯ ХИМРЕАГЕНТОВ") {
		return section
	}
	overview := reportChemicalOverview(section)
	if overview == "" {
		return section
	}
	// Place the compact overview ahead of the long method assessments.
	anchor := "УТВЕРЖДЁННОЕ РЕШЕНИЕ ПО МЕТОДАМ"
	if index := strings.Index(section, anchor); index >= 0 {
		return section[:index] + overview + "\r\n\r\n" + section[index:]
	}
	return section + "\r\n\r\n" + overview
}
