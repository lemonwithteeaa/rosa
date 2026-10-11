//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC752ChemicalOverviewUsesSavedRatesAndFrequency(t *testing.T) {
	stored := "РАЗДЕЛ 4. МЕТОДЫ И МЕРОПРИЯТИЯ\r\nПАРАМЕТРЫ РЕАЛИЗАЦИИ\r\n" +
		"Ингибитор коррозии — Дозирование\r\n• Реагент: Тестовый реагент К-01\r\n• База дозирования: По воде\r\n• Единица измерения: г/м³\r\n• Концентрация / расход: 45\r\n• Периодичность: Непрерывно\r\n• Точка ввода: На приём насоса\r\n" +
		"Деэмульгатор — Дозирование\r\n• Реагент: Тестовый реагент Д-02\r\n• База дозирования: По нефти\r\n• Единица измерения: г/м³\r\n• Концентрация / расход: 18\r\n• Периодичность: Ежесуточно\r\n" +
		"ПЛАН ИСПОЛНЕНИЯ И КОНТРОЛЯ\r\n"
	got := reportAddChemicalOverview(stored)
	for _, str := range []string{"Химреагент: Тестовый реагент К-01", "Норма подачи / обработки: 45 г/м³", "Частота / режим подачи: Непрерывно", "База расчёта: По воде", "Химреагент: Тестовый реагент Д-02", "Норма подачи / обработки: 18 г/м³", "Частота / режим подачи: Ежесуточно"} {
		if !strings.Contains(got, str) {
			t.Fatalf("missing %q in %q", str, got)
		}
	}
	if strings.Count(got, "СХЕМЫ ПРИМЕНЕНИЯ ХИМРЕАГЕНТОВ") != 1 {
		t.Fatal("duplicate chemical title")
	}
	if again := reportAddChemicalOverview(got); again != got {
		t.Fatal("summary is not idempotent")
	}
}

func TestRC752NumericSheetFormulaAndMissingInputs(t *testing.T) {
	r := FullReportRecord{ObjectLabel: "Скважина 42", ForecastDays: 0, NetBenefitK: 0, Sections: map[string]string{
		"section_5": "РАЗДЕЛ 5. НАРАБОТКА И ОТКАЗЫ\r\nБазовая оценка без нового мероприятия: 680 сут.\r\nЦелевой ориентир: 1000 сут.\r\nСдержанный сценарий: 740 сут.\r\nРасчётный сценарий: 960 сут.\r\nБлагоприятный сценарий: 1100 сут.\r\n",
		"section_6": "РАЗДЕЛ 6. ЭКОНОМИЧЕСКАЯ ОЦЕНКА\r\nCAPEX: 1.50 млн руб.; OPEX: 0.25 млн руб./год; затраты первого года: 1.75 млн руб.\r\nЧистая выгода: +2.25 млн руб.; Benefit/Cost: 2.28; окупаемость: 7.5 мес.\r\nРасчётная трассировка: затраты = CAPEX + OPEX = 1.75 млн руб.; валовая выгода = чистая выгода + затраты = 4.00 млн руб.\r\n",
	}}
	mrp := reportScenarioTabXML(r)
	for _, want := range []string{`<f>B7-$B$6</f>`, `<f>IFERROR(B8/$B$6-1,0)</f>`, `<f>B9-$B$11</f>`, `0.0%`} {
		if want == "0.0%" {
			continue
		}
		if !strings.Contains(mrp, want) {
			t.Errorf("MRP formula not found: %s", want)
		}
	}
	econ := reportEconomyTabXML(r)
	for _, want := range []string{`<f>SUM(B6:B7)</f>`, `<f>B10-B8</f>`, `<f>B11-B12</f>`} {
		if !strings.Contains(econ, want) {
			t.Errorf("economics formula not found: %s", want)
		}
	}
	missing := FullReportRecord{Sections: map[string]string{"section_5": "Целевой ориентир: 1000 сут.\r\n", "section_6": "CAPEX: 1 млн руб."}}
	if strings.Contains(reportScenarioTabXML(missing), `<f>B7-$B$6</f>`) {
		t.Fatal("invented MRP delta with no base")
	}
	if strings.Contains(reportEconomyTabXML(missing), `<f>SUM(B6:B7)</f>`) {
		t.Fatal("invented OPEX")
	}
}
