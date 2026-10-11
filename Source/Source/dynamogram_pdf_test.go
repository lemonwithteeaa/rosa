//go:build windows

package main

import "testing"

func TestParseDynamogramPDFReport(t *testing.T) {
	text := `Скважина: 658 Куст: 1014 Цех: ЦДНГ №2 бр.5 Месторождение: Восточно-Анзирское
Дата: 07.08.2024 08:15
Станок: СКДР8-3-Ш
Глубина подвеса насоса(м) 1320
Тип насоса НСВ-38
Обводнённость(%) 4
Уровень динамический (м) 960
Затрубное давление (ат) 0.0
Мин. динам. нагрузка(кгс) 0
Макс. динам. нагрузка(кгс) 1670
Число качаний в мин. 2.2
Режимный ход штока(м) 1.20
Расчетный дебит (м3/сут) 2.59
Коэф.наполнения насоса 1.00
Датчик A315.1052
Блок регистрации Профи №2130`
	data := parseDynamogramPDFReport(text)
	for key, want := range map[string]string{"well": "658", "rod_stroke_m": "1.20", "max_dyn_load_kgf": "1670", "watercut_pct": "4", "sensor": "A315.1052"} {
		if data[key] != want {
			t.Fatalf("%s=%q, want %q", key, data[key], want)
		}
	}
}

func TestNicePDFCalibrationInputs(t *testing.T) {
	data := map[string]string{"rod_stroke_m": "1,20", "min_dyn_load_kgf": "0", "max_dyn_load_kgf": "1670"}
	if x, ok := pdfReportNumber(data, "rod_stroke_m"); !ok || x != 1.2 {
		t.Fatalf("stroke=%v ok=%v", x, ok)
	}
}
