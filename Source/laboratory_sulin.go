//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

// laboratoryCalculatedField identifies values calculated by ROSA from
// the laboratory composition and must never be requested as manual inputs.
func laboratoryCalculatedField(groupCode, key string) bool {
	return groupCode == "scale" && key == "water_type_sulin"
}

// laboratorySulinWaterType calculates the genetic water type according to
// V. A. Sulin's ionic-equivalent coefficients. Input values are expected in
// canonical mg/L units. Na+K is taken from separate Na and K measurements when
// available; otherwise the conventional combined Na+K field is treated on a
// sodium-equivalent basis and the result is marked as approximate.
func laboratorySulinWaterType(values map[string]string) string {
	naEq, naOK, approximate := laboratorySulinNaKEquivalent(values)
	clEq, clOK := laboratoryEquivalentMillimoles(values, "cl", 35.45, 1)
	if !naOK || !clOK || clEq <= 0 {
		return "Недостаточно данных: нужны Na⁺+K⁺ и Cl⁻"
	}

	ratio := naEq / clEq
	suffix := ""
	if approximate {
		suffix = " (оценочно: Na⁺+K⁺ задано суммарно)"
	}

	// At the exact boundary the Sulin coefficient does not identify one of the
	// four types. Do not force a class from rounding noise.
	if math.Abs(ratio-1) < 1e-6 {
		return "Переходный тип по Сулину: r(Na+K)/rCl ≈ 1" + suffix
	}

	if ratio > 1 {
		so4Eq, ok := laboratoryEquivalentMillimoles(values, "so4", 96.06, 2)
		if !ok {
			return "Недостаточно данных: для типа по Сулину нужен SO₄²⁻" + suffix
		}
		if so4Eq <= 0 {
			return "Гидрокарбонатно-натриевый" + suffix
		}
		coefficient := (naEq - clEq) / so4Eq
		if math.Abs(coefficient-1) < 1e-6 {
			return "Переходный тип по Сулину: (rNa−rCl)/rSO₄ ≈ 1" + suffix
		}
		if coefficient > 1 {
			return "Гидрокарбонатно-натриевый" + suffix
		}
		return "Сульфатно-натриевый" + suffix
	}

	mgEq, ok := laboratoryEquivalentMillimoles(values, "mg", 24.305, 2)
	if !ok {
		return "Недостаточно данных: для типа по Сулину нужен Mg²⁺" + suffix
	}
	if mgEq <= 0 {
		return "Хлоридно-кальциевый" + suffix
	}
	coefficient := (clEq - naEq) / mgEq
	if math.Abs(coefficient-1) < 1e-6 {
		return "Переходный тип по Сулину: (rCl−rNa)/rMg ≈ 1" + suffix
	}
	if coefficient > 1 {
		return "Хлоридно-кальциевый" + suffix
	}
	return "Хлоридно-магниевый" + suffix
}

func laboratoryEquivalentMillimoles(values map[string]string, key string, molarMass float64, charge float64) (float64, bool) {
	raw := strings.TrimSpace(values[key])
	value, ok := parseLabNumber(raw)
	if !ok || value < 0 || molarMass <= 0 || charge <= 0 {
		return 0, false
	}
	// mg/L divided by molar mass gives mmol/L; multiplication by charge gives
	// milliequivalents/L. Ratios are identical to %-equivalent coefficients.
	return value / molarMass * charge, true
}

func laboratorySulinNaKEquivalent(values map[string]string) (float64, bool, bool) {
	na, naOK := laboratoryEquivalentMillimoles(values, "na", 22.989769, 1)
	k, kOK := laboratoryEquivalentMillimoles(values, "k", 39.0983, 1)
	if naOK || kOK {
		return na + k, true, false
	}
	combined, ok := parseLabNumber(strings.TrimSpace(values["na_k"]))
	if !ok || combined < 0 {
		return 0, false, false
	}
	return combined / 22.989769, true, true
}

func laboratoryCalculatedFieldValue(groupCode, key string, canonicalValues map[string]string) string {
	if groupCode == "scale" && key == "water_type_sulin" {
		return laboratorySulinWaterType(canonicalValues)
	}
	return ""
}

func laboratoryCalculatedFieldExplanation(groupCode, key string) string {
	if groupCode == "scale" && key == "water_type_sulin" {
		return "Расчётный параметр. РОСА определяет тип воды по В. А. Сулину из ионного состава в эквивалентной форме; вручную это поле не заполняется."
	}
	return ""
}

func laboratorySulinMethodText(values map[string]string) string {
	result := laboratorySulinWaterType(values)
	if strings.HasPrefix(result, "Недостаточно данных") {
		return "Тип воды по В. А. Сулину: " + result + "."
	}
	return fmt.Sprintf("Тип воды по В. А. Сулину (расчёт РОСА): %s.", result)
}
