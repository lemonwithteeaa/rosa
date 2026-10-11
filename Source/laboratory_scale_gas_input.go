//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

// This contract is not connected to the UI until the gas solver is verified.
// Pressure is partial CO2 pressure for a reservoir, total pressure for a
// closed pure-CO2 volume. No ideal-gas estimate of initial moles is made.
type laboratoryCO2GasInput struct {
	Mode             string
	Pressure         float64
	PressureUnit     string
	AbsolutePressure bool
	TemperatureC     float64
	WaterKG          float64
	GasVolumeL       float64
	CO2Percent       float64
	ReservoirMoles   float64
}

type laboratoryCO2GasNormalized struct {
	Mode                                                           string
	PressureATM                                                    float64
	TemperatureC, WaterKG, GasVolumeL, CO2Fraction, ReservoirMoles float64
}

func laboratoryGasPressureATM(value float64, unit string, absolute bool) (float64, error) {
	if !absolute {
		return 0, fmt.Errorf("Укажите абсолютное давление; манометрическое давление без атмосферной поправки не используется")
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0, fmt.Errorf("Давление должно быть конечным числом больше нуля")
	}
	factor := 0.0
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "па", "pa":
		factor = 1.0 / 101325
	case "кпа", "kpa":
		factor = 1000.0 / 101325
	case "мпа", "mpa":
		factor = 1000000.0 / 101325
	case "бар", "bar":
		factor = 100000.0 / 101325
	case "атм", "atm":
		factor = 1
	default:
		return 0, fmt.Errorf("Неизвестная единица давления; используйте Па, кПа, МПа, бар или атм")
	}
	converted := value * factor
	if math.IsNaN(converted) || math.IsInf(converted, 0) || converted <= 0 {
		return 0, fmt.Errorf("Пересчёт давления вышел за числовой диапазон")
	}
	return converted, nil
}

func laboratoryNormalizeCO2GasInput(input laboratoryCO2GasInput) (laboratoryCO2GasNormalized, error) {
	out := laboratoryCO2GasNormalized{}
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	if input.Mode != "external_reservoir" && input.Mode != "closed_volume" {
		return out, fmt.Errorf("Выберите внешний резервуар CO₂ или замкнутый газовый объём")
	}
	if !finite(input.TemperatureC) || math.Abs(input.TemperatureC-25) > .001 {
		return out, fmt.Errorf("Первая газовая постановка ограничена 25 °C")
	}
	if !finite(input.WaterKG) || input.WaterKG <= 0 {
		return out, fmt.Errorf("Масса воды-растворителя должна быть конечным числом больше нуля")
	}
	pressure, err := laboratoryGasPressureATM(input.Pressure, input.PressureUnit, input.AbsolutePressure)
	if err != nil {
		return out, err
	}
	// Conservative proposed first domain, not a limitation of PHREEQC.
	if pressure > 1+1e-12 {
		return out, fmt.Errorf("Первая газовая постановка ограничена давлением не выше 1 атм; высокое давление требует отдельной проверки")
	}
	out.Mode = input.Mode
	out.PressureATM = pressure
	out.TemperatureC = input.TemperatureC
	out.WaterKG = input.WaterKG
	if input.Mode == "external_reservoir" {
		if !finite(input.ReservoirMoles) || input.ReservoirMoles <= 0 {
			return laboratoryCO2GasNormalized{}, fmt.Errorf("Задайте конечный положительный запас CO₂ резервуара в молях; достаточность запаса проверяется расчётом")
		}
		if input.GasVolumeL != 0 || input.CO2Percent != 0 {
			return laboratoryCO2GasNormalized{}, fmt.Errorf("Для резервуара вводится парциальное давление CO₂ и запас; объём замкнутого газа и его состав относятся к другой задаче")
		}
		out.ReservoirMoles = input.ReservoirMoles
	} else {
		if !finite(input.GasVolumeL) || input.GasVolumeL <= 0 {
			return laboratoryCO2GasNormalized{}, fmt.Errorf("Замкнутый объём газа должен быть конечным числом больше нуля")
		}
		if !finite(input.CO2Percent) || math.Abs(input.CO2Percent-100) > 1e-9 {
			return laboratoryCO2GasNormalized{}, fmt.Errorf("Первая замкнутая постановка поддерживает только чистый CO₂, 100 %%; газовая смесь требует отдельной модели")
		}
		if input.ReservoirMoles != 0 {
			return laboratoryCO2GasNormalized{}, fmt.Errorf("Запас внешнего резервуара не задаётся для замкнутого объёма")
		}
		out.GasVolumeL = input.GasVolumeL
		out.CO2Fraction = input.CO2Percent / 100
	}
	return out, nil
}
