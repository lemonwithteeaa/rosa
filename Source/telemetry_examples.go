//go:build windows

package main

import (
	"math"
	"time"
)

const telemetryExampleObject = "ТЕСТ-ТЕЛЕМЕТРИЯ"

type telemetryExampleScenario struct {
	id, name, quality string
	events            []TelemetryEvent
	value             func(channel, point int) float64
}

func telemetryExampleChannels(s telemetryExampleScenario) []TelemetryChannelSummary {
	defs := []struct{ key, name, unit string }{
		{"intake_pressure", "Давление (Р) на приёме", "МПа"}, {"buffer_pressure", "Буферное давление (Рбуф)", "МПа"},
		{"liquid_rate", "Дебит жидкости", "м³/сут"}, {"oil_rate", "Дебит нефти", "т/сут"},
		{"water_cut", "Обводнённость", "%"}, {"current", "Ток", "А"}, {"voltage", "Напряжение", "В"},
		{"temperature", "Температура", "°C"}, {"vibration", "Вибрация", "мм/с"}, {"frequency", "Частота", "Гц"},
	}
	out := make([]TelemetryChannelSummary, 0, len(defs))
	for c, d := range defs {
		stat := runningStat{}
		for i := 0; i < 288; i++ {
			stat.add(s.value(c, i))
		}
		out = append(out, stat.summary(telemetryMap{canonical: d.key, display: d.name, sourceName: "Пример: " + d.name, unit: d.unit}))
	}
	return out
}

func ensureTelemetryExamples() bool {
	have := map[string]bool{}
	for _, item := range telemetryImports {
		have[item.ID] = true
	}
	base := func(channel, point int) float64 {
		levels := []float64{4.8, 1.9, 82, 52, 36, 42, 380, 61, 1.7, 50}
		return levels[channel] * (1 + 0.01*math.Sin(float64(point)/12+float64(channel)))
	}
	scenarios := []telemetryExampleScenario{
		{id: "TEL-EXAMPLE-STABLE", name: "Стабильная работа", quality: "высокое", value: base},
		{id: "TEL-EXAMPLE-RATE-CURRENT", name: "Падение подачи и рост тока", quality: "высокое", events: []TelemetryEvent{{Kind: "trend", Channel: "liquid_rate/current", Description: "Снижение подачи сопровождается ростом тока", Severity: "warning"}}, value: func(c, i int) float64 {
			v := base(c, i)
			if c == 2 || c == 3 {
				return v * (1 - 0.35*float64(i)/287)
			}
			if c == 5 {
				return v * (1 + 0.28*float64(i)/287)
			}
			return v
		}},
		{id: "TEL-EXAMPLE-VIB-TEMP", name: "Рост вибрации и температуры", quality: "высокое", events: []TelemetryEvent{{Kind: "trend", Channel: "vibration/temperature", Description: "Совместный рост вибрации и температуры", Severity: "warning"}}, value: func(c, i int) float64 {
			v := base(c, i)
			if c == 7 || c == 8 {
				return v * (1 + 0.45*float64(i)/287)
			}
			return v
		}},
		{id: "TEL-EXAMPLE-GAPS", name: "Пропуски и выбросы данных", quality: "требует проверки", events: []TelemetryEvent{{Kind: "quality", Description: "В ряду присутствуют пропуски и выбросы; автоматическая интерполяция не выполнялась", Severity: "warning"}}, value: func(c, i int) float64 {
			v := base(c, i)
			if i == 91 || i == 203 {
				return v * 2.8
			}
			return v
		}},
	}
	changed := false
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	for _, scenario := range scenarios {
		if have[scenario.id] {
			continue
		}
		telemetryImports = append(telemetryImports, TelemetryImport{
			ID: scenario.id, ObjectKey: telemetryExampleObject, SourceSystem: "Встроенный пример", SourceFile: scenario.name,
			SourceSHA256: scenario.id, ImportedAt: "01.09.2026 00:00", StartedAt: start.Format("02.01.2006 15:04"),
			EndedAt: start.Add(287 * 5 * time.Minute).Format("02.01.2006 15:04"), Rows: 288, ValidTimeRows: 288,
			MedianIntervalSec: 300, Quality: scenario.quality, Channels: telemetryExampleChannels(scenario), Events: scenario.events,
		})
		changed = true
	}
	return changed
}
