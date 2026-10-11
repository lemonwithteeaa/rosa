//go:build windows

package main

import "testing"

func TestParseEconomicsWorkbookRowsSeparatesCommonAndSchemeInputs(t *testing.T) {
	parameters := [][]string{
		{"РОСА · БЛАНК ЭКОНОМИЧЕСКОЙ ОЦЕНКИ"},
		{"Параметр", "Значение", "Единица", "Пояснение"},
		{"Объект", "Учебная скважина 01"},
		{"Текущий МРП", "800"},
		{"МРП осторожный", "1000"},
		{"МРП центральный", "1200"},
		{"МРП благоприятный", "1500"},
		{"Выбранный сценарий", "Центральный"},
		{"Горизонт", "36"},
		{"Длительность ремонта", "5"},
		{"Стоимость ремонта", "250000"},
		{"OPEX", "100000"},
		{"Инфляция OPEX", "5"},
		{"CAPEX", "300000"},
	}
	variants := [][]string{
		{"№", "Вариант", "Методы (справочно)", "МРП осторожный", "МРП центральный", "МРП благоприятный", "OPEX, руб./год", "CAPEX, руб."},
		{"1", "Схема 1", "Метод А", "950", "1100", "1300", "120000", "350000"},
	}
	got, err := parseEconomicsWorkbookRows(parameters, variants)
	if err != nil {
		t.Fatal(err)
	}
	if got.objectKey != "Учебная скважина 01" || got.uiValues["current_mrp"] != "800" || got.uiValues["horizon"] != "36" {
		t.Fatalf("unexpected common economic inputs: %#v", got)
	}
	if len(got.variants) != 1 || got.variants[0].ordinal != 1 || got.variants[0].mrpBase == nil || *got.variants[0].mrpBase != 1100 || got.variants[0].opex == nil || *got.variants[0].opex != 120000 {
		t.Fatalf("unexpected scheme inputs: %#v", got.variants)
	}
}

func TestParseEconomicsWorkbookRowsRejectsInvalidHorizon(t *testing.T) {
	parameters := [][]string{
		{"Параметр", "Значение"},
		{"Объект", ""}, {"Текущий МРП", "800"}, {"МРП центральный", "1200"},
		{"Выбранный сценарий", "Центральный"}, {"Горизонт", "61"},
		{"Стоимость ремонта", "250000"},
	}
	if _, err := parseEconomicsWorkbookRows(parameters, nil); err == nil {
		t.Fatal("expected invalid horizon to be rejected")
	}
}
