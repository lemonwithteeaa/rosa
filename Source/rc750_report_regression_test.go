//go:build windows

package main

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func zipPartTextRC750(t *testing.T, path, name string) string {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	t.Fatalf("missing %s", name)
	return ""
}

func TestRC750FooterIsOnlyRightAlignedPageNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.docx")
	if err := writeLaboratoryDOCXReport(path, "ОТЧЁТ\r\nОбъект: ТЕСТ"); err != nil {
		t.Fatal(err)
	}
	footer := zipPartTextRC750(t, path, "word/footer1.xml")
	for _, want := range []string{`w:jc w:val="right"`, `w:instr="PAGE"`} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer misses %s: %s", want, footer)
		}
	}
	lower := strings.ToLower(footer)
	if strings.Contains(lower, "роса") || strings.Contains(lower, "страниц") {
		t.Fatalf("branding/page word returned to footer: %s", footer)
	}
}

func TestRC750HistoricalReportCompactsRepeatedLaboratoryNarrative(t *testing.T) {
	section2 := "РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ\r\nДата анализа: 08.10.2026\r\nИсточник / протокол: Лаборатория №7\r\nМодель солеотложений: davies\r\n" +
		"НАПРАВЛЕНИЕ: СОЛЕОТЛОЖЕНИЯ\r\n• Ca²⁺: 520\u00a0мг/л\r\n• CO₃²⁻: 60\u00a0мг/л\r\n• SO₄²⁻: 800\u00a0мг/л\r\n• Cl⁻: 15000\u00a0мг/л\r\n• Mg²⁺: 70\u00a0мг/л\r\n• Na⁺ + K⁺: 12000\u00a0мг/л\r\n" +
		"ВЫВОД ДЛЯ ТЕХНОЛОГА\r\nПроверить риск солей.\r\nИсточник: Лаборатория №7\r\n" +
		"РАСЧЁТЫ, ФОРМУЛЫ, ПОДСТАНОВКА И РЕЗУЛЬТАТЫ\r\n" + strings.Repeat("Длинное пояснение расчёта. ", 40)
	r := FullReportRecord{ID: "R", Sections: map[string]string{"section_2": section2, "section_7": "РАЗДЕЛ 7. ИТОГОВОЕ РЕШЕНИЕ"}}
	report := storedSectionDetailedReport(r, true)
	if strings.Contains(report, "Длинное пояснение расчёта") {
		t.Fatal("verbose historical laboratory calculation was not compacted")
	}
	if strings.Count(report, "Лаборатория №7") != 1 {
		t.Fatalf("laboratory source repeated: %s", report)
	}
	for _, want := range []string{"РАСЧЁТНЫЕ ПОКАЗАТЕЛИ", "Ионная сила I", "Кальцит"} {
		if !strings.Contains(report, want) {
			t.Fatalf("compact calculated summary misses %q: %s", want, report)
		}
	}
}

func TestRC750WorkbookContainsLaboratoryCalculationChain(t *testing.T) {
	pass := LabPassRecord{
		Date: "08.10.2026", Source: "Лаборатория №7", ScaleModel: "davies",
		Groups: map[string]map[string]string{
			"scale":     {"mineralization": "28.0", "ca": "520", "mg": "70", "na_k": "12000", "so4": "800", "co3": "60", "hco3": "300", "cl": "15000", "ba": "4", "sr": "20"},
			"corrosion": {"plate_initial_mass": "20", "plate_final_mass": "19.58", "plate_exposure_days": "30", "plate_area": "100", "plate_material_density": "7.85", "corrosion_rate_without": "0.72", "corrosion_rate_with": "0.16", "wall_thickness": "5.7", "minimum_wall_thickness": "4.8", "pit_depth": "0.6", "pit_period_years": "2"},
			"gases":     {"sample_temp": "40", "hydrate_temp": "23"},
		},
		Units: map[string]map[string]string{
			"scale":     {"mineralization": "г/л", "ca": "мг/л", "mg": "мг/л", "na_k": "мг/л", "so4": "мг/л", "co3": "мг/л", "hco3": "мг/л", "cl": "мг/л", "ba": "мг/л", "sr": "мг/л"},
			"corrosion": {"plate_initial_mass": "г", "plate_final_mass": "г", "plate_exposure_days": "сут", "plate_area": "см²", "plate_material_density": "г/см³", "corrosion_rate_without": "мм/год", "corrosion_rate_with": "мм/год", "wall_thickness": "мм", "minimum_wall_thickness": "мм", "pit_depth": "мм", "pit_period_years": "год"},
			"gases":     {"sample_temp": "°C", "hydrate_temp": "°C"},
		},
	}
	section := "РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ\r\nДата анализа: " + pass.Date + "\r\nИсточник / протокол: " + pass.Source + "\r\nМодель солеотложений: davies\r\n" + reportLaboratoryRawData(pass)
	r := FullReportRecord{ObjectLabel: "Тест", ForecastDays: 1050, NetBenefitK: 12000, Sections: map[string]string{"section_2": section}}
	xml := fullReportCalculationSheetXML(r)
	for _, want := range []string{"Ионная сила I", "Баланс зарядов", "Коэффициент активности γ₂", "Кальцит · Ksp", "Кальцит · IAP", "Кальцит · Кнас", "индекс насыщения SI", "Эффективность ингибиторной защиты", "Средняя скорость коррозии по пластинке", "Запас толщины стенки", "Локальная скорость по язве", "Температурный запас ΔT", "Расчётная жёсткость Ca²⁺+Mg²⁺", "Расхождение TDS и суммы ионов", "Коэффициент Сулина r(Na+K)/rCl", "Тип воды по В. А. Сулину", "LOG10(F"} {
		if !strings.Contains(xml, want) {
			t.Fatalf("calculation sheet misses %q", want)
		}
	}
	if strings.Count(xml, `<f>`) < 15 {
		t.Fatalf("too few actual Excel formulas: %d", strings.Count(xml, `<f>`))
	}
}
func TestRC750SectionOwnershipRemovesSemanticRepeats(t *testing.T) {
	scenario := AlternativeMethodScenario{Name: "Вариант А", Status: "утверждено", SelectionBasis: "основание", ForecastLowDays: 900, ForecastBaseDays: 1050, ForecastHighDays: 1200, NetBenefitK: 10000}
	section7 := reportSection7Final("obj", scenario, "")
	for _, repeated := range []string{"Основание выбора:", "Чистая выгода:", "Прогноз:"} {
		if strings.Contains(section7, repeated) {
			t.Fatalf("section 7 repeats %q: %s", repeated, section7)
		}
	}
	historical := "РАЗДЕЛ 4. МЕТОДЫ И МЕРОПРИЯТИЯ\r\nИсточник прогноза: архив\r\nПредыстория 36 мес.: отказов 4\r\nПрогноз наработки ПОСЛЕ мероприятия: 900 / 1050 / 1200 сут.\r\nПАРАМЕТРЫ РЕАЛИЗАЦИИ\r\nДозирование: по программе"
	compact := compactStoredSectionForExport(4, historical)
	for _, repeated := range []string{"Источник прогноза:", "Предыстория 36 мес.:", "Прогноз наработки ПОСЛЕ мероприятия:"} {
		if strings.Contains(compact, repeated) {
			t.Fatalf("historical methods repeat was not compacted: %q in %s", repeated, compact)
		}
	}
}
