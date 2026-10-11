//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

type laboratoryWaterQC struct {
	ChargeBalancePct    float64
	ChargeBalanceOK     bool
	ChargeBalanceApprox bool
	IonSumMgL           float64
	TDSMgL              float64
	TDSDifferencePct    float64
	TDSComparisonOK     bool
	HardnessMeqL        float64
	HardnessCaCO3MgL    float64
	HardnessOK          bool
}

// laboratoryWaterQualityQC performs transparent screening checks on one water
// analysis. Values are canonical: ions in mg/L and mineralization in g/L.
// This is a data-quality aid, not a substitute for the laboratory protocol.
func laboratoryWaterQualityQC(values map[string]string) laboratoryWaterQC {
	var qc laboratoryWaterQC

	ca, caOK := laboratoryEquivalentMillimoles(values, "ca", 40.078, 2)
	mg, mgOK := laboratoryEquivalentMillimoles(values, "mg", 24.305, 2)
	cl, clOK := laboratoryEquivalentMillimoles(values, "cl", 35.453, 1)
	so4, so4OK := laboratoryEquivalentMillimoles(values, "so4", 96.06, 2)
	hco3, hco3OK := laboratoryEquivalentMillimoles(values, "hco3", 61.0168, 1)
	co3, co3OK := laboratoryEquivalentMillimoles(values, "co3", 60.008, 2)

	naK, naKOK, approximate := laboratorySulinNaKEquivalent(values)
	qc.ChargeBalanceApprox = approximate

	if caOK && mgOK && naKOK && clOK && so4OK && hco3OK {
		cations := ca + mg + naK
		if ba, ok := laboratoryEquivalentMillimoles(values, "ba", 137.327, 2); ok {
			cations += ba
		}
		if sr, ok := laboratoryEquivalentMillimoles(values, "sr", 87.62, 2); ok {
			cations += sr
		}
		anions := cl + so4 + hco3
		if co3OK {
			anions += co3
		}
		den := cations + anions
		if den > 0 {
			qc.ChargeBalancePct = 100 * (cations - anions) / den
			qc.ChargeBalanceOK = true
		}
	}

	if caOK && mgOK {
		qc.HardnessMeqL = ca + mg
		qc.HardnessCaCO3MgL = qc.HardnessMeqL * 50.0
		qc.HardnessOK = true
	}

	keys := []string{"ca", "mg", "cl", "so4", "hco3"}
	ionSum := 0.0
	ionSumOK := true
	for _, key := range keys {
		n, ok := parseLabNumber(values[key])
		if !ok || n < 0 {
			ionSumOK = false
			break
		}
		ionSum += n
	}
	if ionSumOK {
		if co3mg, ok := parseLabNumber(values["co3"]); ok && co3mg >= 0 {
			ionSum += co3mg
		}
		if ba, ok := parseLabNumber(values["ba"]); ok && ba >= 0 {
			ionSum += ba
		}
		if sr, ok := parseLabNumber(values["sr"]); ok && sr >= 0 {
			ionSum += sr
		}
		na, naOK := parseLabNumber(values["na"])
		k, kOK := parseLabNumber(values["k"])
		if naOK && kOK && na >= 0 && k >= 0 {
			ionSum += na + k
		} else if combined, ok := parseLabNumber(values["na_k"]); ok && combined >= 0 {
			ionSum += combined
		} else {
			ionSumOK = false
		}
	}
	if tdsGL, ok := parseLabNumber(values["mineralization"]); ok && tdsGL > 0 && ionSumOK {
		qc.IonSumMgL = ionSum
		qc.TDSMgL = tdsGL * 1000
		qc.TDSDifferencePct = 100 * math.Abs(qc.TDSMgL-qc.IonSumMgL) / qc.TDSMgL
		qc.TDSComparisonOK = true
	}

	return qc
}

func laboratoryWaterQualityQCText(values map[string]string) string {
	qc := laboratoryWaterQualityQC(values)
	lines := []string{"КОНТРОЛЬ КАЧЕСТВА АНАЛИЗА ВОДЫ"}

	if qc.ChargeBalanceOK {
		absBalance := math.Abs(qc.ChargeBalancePct)
		status := "согласование зарядов хорошее"
		switch {
		case absBalance > 10:
			status = "значительное расхождение — сначала проверить единицы, полноту и протокол"
		case absBalance > 5:
			status = "пограничное расхождение — рекомендуется проверить анализ"
		}
		suffix := ""
		if qc.ChargeBalanceApprox {
			suffix = "; Na⁺+K⁺ учтены оценочно как натриевый эквивалент"
		}
		lines = append(lines, fmt.Sprintf("• Ионный баланс: %+.2f %% — %s%s.", qc.ChargeBalancePct, status, suffix))
	} else {
		lines = append(lines, "• Ионный баланс: не рассчитан — нужен полный набор Ca, Mg, Na/K, Cl, SO₄ и HCO₃.")
	}

	if qc.TDSComparisonOK {
		status := "сумма основных ионов согласуется с общей минерализацией"
		switch {
		case qc.TDSDifferencePct > 20:
			status = "существенное расхождение — проверить единицы, пропущенные компоненты и лабораторный протокол"
		case qc.TDSDifferencePct > 10:
			status = "заметное расхождение — требуется сверка состава"
		}
		lines = append(lines, fmt.Sprintf("• TDS-контроль: измерено %.0f мг/л; сумма основных ионов %.0f мг/л; расхождение %.1f %% — %s.", qc.TDSMgL, qc.IonSumMgL, qc.TDSDifferencePct, status))
	} else {
		lines = append(lines, "• TDS-контроль: не рассчитан — нужны общая минерализация и полный основной ионный состав.")
	}

	if qc.HardnessOK {
		lines = append(lines, fmt.Sprintf("• Расчётная жёсткость по Ca²⁺+Mg²⁺: %.2f мг-экв/л (≈ %.0f мг/л как CaCO₃).", qc.HardnessMeqL, qc.HardnessCaCO3MgL))
	} else {
		lines = append(lines, "• Расчётная жёсткость: не рассчитана — нужны Ca²⁺ и Mg²⁺.")
	}

	lines = append(lines, "QC-индикаторы оценивают внутреннюю согласованность данных и не доказывают наличие или отсутствие солеотложения.")
	return strings.Join(lines, "\n")
}

func laboratoryWaterQualityQCShort(values map[string]string) string {
	qc := laboratoryWaterQualityQC(values)
	var parts []string
	if qc.ChargeBalanceOK {
		parts = append(parts, fmt.Sprintf("баланс %+.1f %%", qc.ChargeBalancePct))
	}
	if qc.TDSComparisonOK {
		parts = append(parts, fmt.Sprintf("TDS Δ %.1f %%", qc.TDSDifferencePct))
	}
	if qc.HardnessOK {
		parts = append(parts, fmt.Sprintf("жёсткость %.2f мг-экв/л", qc.HardnessMeqL))
	}
	if len(parts) == 0 {
		return "QC воды: недостаточно данных"
	}
	return "QC воды: " + strings.Join(parts, "; ")
}
