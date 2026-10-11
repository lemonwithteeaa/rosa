//go:build windows

package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Reconstruct only measured inputs from the immutable approved section. The
// workbook must never silently use a newer laboratory pass from the UI.
func reportLabSnapshotInputs(section string) LabPassRecord {
	pass := LabPassRecord{Groups: map[string]map[string]string{}, Units: map[string]map[string]string{}}
	current := ""
	for _, line := range strings.Split(strings.ReplaceAll(section, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Модель солеотложений: ") {
			pass.ScaleModel = strings.TrimSpace(strings.TrimPrefix(line, "Модель солеотложений: "))
		}
		if strings.HasPrefix(line, "Дата анализа: ") {
			pass.Date = strings.TrimSpace(strings.TrimPrefix(line, "Дата анализа: "))
		}
		if strings.HasPrefix(line, "Источник / протокол: ") {
			pass.Source = strings.TrimSpace(strings.TrimPrefix(line, "Источник / протокол: "))
		}
		if strings.HasPrefix(line, "НАПРАВЛЕНИЕ: ") {
			current = ""
			for _, group := range labGroups {
				if strings.EqualFold(group.Name, strings.TrimPrefix(line, "НАПРАВЛЕНИЕ: ")) {
					current = group.Code
					break
				}
			}
			continue
		}
		if current == "" || !strings.HasPrefix(line, "• ") {
			continue
		}
		group, ok := labGroupByCode(current)
		if !ok {
			continue
		}
		for _, field := range group.Fields {
			prefix := "• " + field.Label + ": "
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			unit := field.Unit
			if split := strings.Index(value, "\u00a0"); split >= 0 {
				value, unit = value[:split], strings.ReplaceAll(value[split+len("\u00a0"):], "\u00a0", " ")
			} else if unit != "" && unit != "текст" {
				// Historical snapshots used an ordinary space before the unit.
				if suffix := " " + unit; strings.HasSuffix(value, suffix) {
					value = strings.TrimSuffix(value, suffix)
				}
			}
			if _, numeric := parseLabNumber(value); numeric {
				if pass.Groups[current] == nil {
					pass.Groups[current] = map[string]string{}
					pass.Units[current] = map[string]string{}
				}
				pass.Groups[current][field.Key] = value
				pass.Units[current][field.Key] = unit
			}
			break
		}
	}
	return pass
}

func fullReportCalculationSheetXML(r FullReportRecord) string {
	pass := reportLabSnapshotInputs(fullReportSection(r, "section_2"))
	modelCode := strings.TrimSpace(pass.ScaleModel)
	if modelCode == "" {
		modelCode = "davies"
	}
	model := laboratoryScaleModelByCode(modelCode)

	var rows strings.Builder
	row := 1
	add := func(cells ...string) {
		fmt.Fprintf(&rows, `<row r="%d">%s</row>`, row, strings.Join(cells, ""))
		row++
	}
	section := func(title string) {
		if row > 5 {
			row++
		}
		ref := strconv.Itoa(row)
		add(xlsxInlineCell("A"+ref, title, 2))
	}
	formulaRow := func(label string, bCell, cCell, dCell, eCell, formula string, cached float64, explanation string) int {
		ref := strconv.Itoa(row)
		cells := []string{xlsxInlineCell("A"+ref, label, 4)}
		if bCell != "" {
			cells = append(cells, xlsxInlineCell("B"+ref, bCell, 4))
		}
		if cCell != "" {
			cells = append(cells, xlsxInlineCell("C"+ref, cCell, 4))
		}
		if dCell != "" {
			cells = append(cells, xlsxInlineCell("D"+ref, dCell, 4))
		}
		if eCell != "" {
			cells = append(cells, xlsxInlineCell("E"+ref, eCell, 4))
		}
		if formula != "" {
			resultStyle := 4
			if strings.Contains(label, " · IAP") {
				resultStyle = 9
			}
			cells = append(cells, calculationFormulaCell("F"+ref, formula, cached, resultStyle))
		} else {
			cells = append(cells, xlsxNumberCell("F"+ref, cached, 4))
		}
		cells = append(cells, xlsxInlineCell("G"+ref, explanation, 4))
		current := row
		add(cells...)
		return current
	}

	add(xlsxInlineCell("A1", "РАСЧЁТНЫЕ ПОКАЗАТЕЛИ И ПРОВЕРЯЕМЫЕ EXCEL-ФОРМУЛЫ", 2))
	add(xlsxInlineCell("A2", "Объект", 3), xlsxInlineCell("B2", r.ObjectLabel, 0))
	add(xlsxInlineCell("A3", "Лабораторный анализ", 3), xlsxInlineCell("B3", strings.Trim(strings.TrimSpace(pass.Date+" · "+pass.Source), " ·"), 0))
	add(xlsxInlineCell("A4", "Модель солеотложений", 3), xlsxInlineCell("B4", model.Name, 0))
	row = 6
	add(
		xlsxInlineCell("A6", "Расчётный показатель", 2),
		xlsxInlineCell("B6", "Исходное 1", 2),
		xlsxInlineCell("C6", "Исходное 2 / константа", 2),
		xlsxInlineCell("D6", "Исходное 3 / единица", 2),
		xlsxInlineCell("E6", "Промежуточное", 2),
		xlsxInlineCell("F6", "Результат Excel", 2),
		xlsxInlineCell("G6", "Формула / пояснение", 2),
	)

	section("СОЛЕОТЛОЖЕНИЯ · ИОННЫЙ БАЛАНС И НАСЫЩЕНИЕ")
	type ionDef struct {
		key, label   string
		mass, charge float64
	}
	ions := []ionDef{
		{"ca", "Ca²⁺", 40.078, 2}, {"mg", "Mg²⁺", 24.305, 2}, {"ba", "Ba²⁺", 137.327, 2}, {"sr", "Sr²⁺", 87.62, 2},
		{"co3", "CO₃²⁻", 60.01, -2}, {"hco3", "HCO₃⁻", 61.016, -1}, {"so4", "SO₄²⁻", 96.06, -2}, {"cl", "Cl⁻", 35.45, -1},
	}
	if _, okNa := parseLabNumber(pass.Groups["scale"]["na"]); okNa {
		ions = append(ions, ionDef{"na", "Na⁺", 22.990, 1})
		if _, okK := parseLabNumber(pass.Groups["scale"]["k"]); okK {
			ions = append(ions, ionDef{"k", "K⁺", 39.0983, 1})
		}
	} else {
		ions = append(ions, ionDef{"na_k", "Na⁺ + K⁺ (как Na)", 22.990, 1})
	}

	ionRows := map[string]int{}
	ionCharges := map[string]float64{}
	var ionicStrength float64
	firstIonRow := 0
	lastIonRow := 0
	for _, ion := range ions {
		value, ok := parseLabNumber(pass.Groups["scale"][ion.key])
		if !ok {
			continue
		}
		unit := strings.TrimSpace(pass.Units["scale"][ion.key])
		factor := 1000.0
		switch unit {
		case "мг/л", "":
		case "г/л":
			factor = 1
		case "моль/л", "моль/кг":
			factor = 0
		default:
			continue
		}
		ref := strconv.Itoa(row)
		molar := value
		molarFormula := "B" + ref
		if factor != 0 {
			molar = value / factor / ion.mass
			molarFormula = "B" + ref + "/" + strconv.FormatFloat(factor, 'f', 0, 64) + "/C" + ref
		}
		contribution := molar * ion.charge * ion.charge / 2
		contributionFormula := "E" + ref + "*D" + ref + "^2/2"
		add(
			xlsxInlineCell("A"+ref, ion.label+" → c и вклад в I", 4),
			xlsxNumberCell("B"+ref, value, 4),
			xlsxNumberCell("C"+ref, ion.mass, 4),
			xlsxNumberCell("D"+ref, ion.charge, 4),
			calculationFormulaCell("E"+ref, molarFormula, molar, 9),
			calculationFormulaCell("F"+ref, contributionFormula, contribution, 9),
			xlsxInlineCell("G"+ref, "c="+molarFormula+"; Iᵢ="+contributionFormula+"; исходная единица "+firstNonEmpty(unit, "мг/л"), 4),
		)
		ionRows[ion.key] = row - 1
		ionCharges[ion.key] = ion.charge
		ionicStrength += contribution
		if firstIonRow == 0 {
			firstIonRow = row - 1
		}
		lastIonRow = row - 1
	}

	ionicRow := 0
	if firstIonRow > 0 {
		ref := strconv.Itoa(row)
		formula := fmt.Sprintf("SUM(F%d:F%d)", firstIonRow, lastIonRow)
		ionicRow = row
		add(xlsxInlineCell("A"+ref, "Ионная сила I", 4), xlsxInlineCell("D"+ref, "моль/л", 4), calculationFormulaCell("F"+ref, formula, ionicStrength, 4), xlsxInlineCell("G"+ref, "="+formula, 4))
	}

	if balance, ok := laboratoryIonBalance(pass.Groups["scale"]); ok {
		cationTerms := []string{}
		anionTerms := []string{}
		for _, ion := range ions {
			r, exists := ionRows[ion.key]
			if !exists {
				continue
			}
			term := fmt.Sprintf("E%d*%s", r, strconv.FormatFloat(math.Abs(ionCharges[ion.key]), 'f', -1, 64))
			if ionCharges[ion.key] > 0 {
				cationTerms = append(cationTerms, term)
			} else {
				anionTerms = append(anionTerms, term)
			}
		}
		if len(cationTerms) >= 2 && len(anionTerms) >= 2 {
			cat := "(" + strings.Join(cationTerms, "+") + ")"
			an := "(" + strings.Join(anionTerms, "+") + ")"
			formula := "100*(" + cat + "-" + an + ")/(" + cat + "+" + an + ")"
			formulaRow("Баланс зарядов, %", "", "контроль ±5 %", "%", "", formula, balance, "=(Σэкв катионов-Σэкв анионов)/(Σэкв катионов+Σэкв анионов)*100")
		}
	}

	gamma1Row, gamma2Row := 0, 0
	if model.Code == "davies" && ionicRow > 0 {
		refI := strconv.Itoa(ionicRow)
		formula1 := "10^(-0.509*(SQRT(F" + refI + ")/(1+SQRT(F" + refI + "))-0.3*F" + refI + "))"
		gamma1Row = formulaRow("Коэффициент активности γ₁", "z=1", "Дэвис", "безразм.", "", formula1, daviesActivityCoefficient(1, ionicStrength), "формула Дэвиса")
		formula2 := "10^(-0.509*4*(SQRT(F" + refI + ")/(1+SQRT(F" + refI + "))-0.3*F" + refI + "))"
		gamma2Row = formulaRow("Коэффициент активности γ₂", "z=2", "Дэвис", "безразм.", "", formula2, daviesActivityCoefficient(2, ionicStrength), "формула Дэвиса")
		_ = gamma1Row
	}

	results, _, scaleWarnings := laboratorySaturationResults(pass.Groups["scale"], model.Code)
	mineralIons := map[string][2]string{
		"Кальцит": {"ca", "co3"}, "Гипс": {"ca", "so4"}, "Барит": {"ba", "so4"}, "Целестин": {"sr", "so4"},
	}
	for _, result := range results {
		pair := mineralIons[result.Mineral]
		kspRef := strconv.Itoa(row)
		kspFormula := "10^B" + kspRef
		ksp := math.Pow(10, result.LogK)
		kspRow := row
		add(xlsxInlineCell("A"+kspRef, result.Mineral+" · Ksp", 4), xlsxNumberCell("B"+kspRef, result.LogK, 4), xlsxInlineCell("C"+kspRef, "log K", 4), xlsxInlineCell("D"+kspRef, "безразм.", 4), calculationFormulaCell("F"+kspRef, kspFormula, ksp, 9), xlsxInlineCell("G"+kspRef, "Ksp=10^(log K)", 4))

		iap := result.Ratio * ksp
		iapRow := row
		iapFormula := ""
		note := "IAP рассчитан выбранной химической моделью; сохранён как числовой результат"
		catRow, catOK := ionRows[pair[0]]
		anRow, anOK := ionRows[pair[1]]
		if catOK && anOK && (model.Code == "davies" || model.Code == "ideal" || model.Code == "temperature_ideal") {
			catTerm := fmt.Sprintf("E%d", catRow)
			anTerm := fmt.Sprintf("E%d", anRow)
			if model.Code == "davies" && gamma2Row > 0 {
				catTerm += fmt.Sprintf("*F%d", gamma2Row)
				anTerm += fmt.Sprintf("*F%d", gamma2Row)
			}
			iapFormula = catTerm + "*" + anTerm
			note = "IAP=a(катиона)×a(аниона)"
		}
		formulaRow(result.Mineral+" · IAP", "", "", "безразм.", "", iapFormula, iap, note)

		knasFormula := fmt.Sprintf("F%d/F%d", iapRow, kspRow)
		knasRow := formulaRow(result.Mineral+" · Кнас", "", "", "безразм.", "", knasFormula, result.Ratio, "Кнас=IAP/Ksp")
		siFormula := fmt.Sprintf("LOG10(F%d)", knasRow)
		state := "ненасыщено"
		if result.SI > 0.1 {
			state = "пересыщено"
		} else if result.SI >= -0.1 {
			state = "близко к насыщению"
		}
		formulaRow(result.Mineral+" · индекс насыщения SI", "", "", "ед.", "", siFormula, result.SI, "SI=log10(Кнас); "+state)
	}
	if len(scaleWarnings) > 0 {
		ref := strconv.Itoa(row)
		add(xlsxInlineCell("A"+ref, "Ограничение модели", 3), xlsxInlineCell("B"+ref, reportCleanUserText(scaleWarnings[0]), 0))
	}

	// Water-quality QC is calculated in the laboratory UI as well. Keep these
	// coefficients visible in the full-report workbook instead of exporting only
	// their narrative conclusion. All numerical rows below are live Excel formulas
	// built from the immutable laboratory snapshot.
	qc := laboratoryWaterQualityQC(pass.Groups["scale"])
	section("ВОДА · КОНТРОЛЬ КАЧЕСТВА И ТИП ПО СУЛИНУ")
	eqExpr := func(key string) (string, bool) {
		r, ok := ionRows[key]
		if !ok {
			return "", false
		}
		return fmt.Sprintf("E%d*ABS(D%d)*1000", r, r), true
	}
	if qc.HardnessOK {
		caExpr, caOK := eqExpr("ca")
		mgExpr, mgOK := eqExpr("mg")
		if caOK && mgOK {
			formula := caExpr + "+" + mgExpr
			hardnessRow := formulaRow("Расчётная жёсткость Ca²⁺+Mg²⁺", "Ca²⁺", "Mg²⁺", "мг-экв/л", "", formula, qc.HardnessMeqL, "Σ эквивалентов Ca²⁺ и Mg²⁺")
			formulaRow("Жёсткость как CaCO₃", "", "коэффициент 50", "мг/л", "", fmt.Sprintf("F%d*50", hardnessRow), qc.HardnessCaCO3MgL, "жёсткость, мг-экв/л × 50")
		}
	}
	if qc.TDSComparisonOK {
		var terms []string
		for _, ion := range ions {
			r, ok := ionRows[ion.key]
			if !ok {
				continue
			}
			terms = append(terms, fmt.Sprintf("E%d*C%d*1000", r, r))
		}
		if len(terms) > 0 {
			ionSumFormula := strings.Join(terms, "+")
			ionSumRow := formulaRow("Сумма основных ионов", "", "", "мг/л", "", ionSumFormula, qc.IonSumMgL, "Σ(cᵢ·Mᵢ), приведено к мг/л")
			if tds, ok := parseLabNumber(pass.Groups["scale"]["mineralization"]); ok && tds > 0 {
				unit := strings.TrimSpace(pass.Units["scale"]["mineralization"])
				ref := strconv.Itoa(row)
				tdsFormula := "B" + ref
				tdsMg := tds
				if unit == "г/л" || unit == "г/дм³" || unit == "г/дм3" || unit == "" {
					tdsFormula += "*1000"
					tdsMg = tds * 1000
				}
				tdsRow := row
				add(xlsxInlineCell("A"+ref, "Общая минерализация (TDS)", 4), xlsxNumberCell("B"+ref, tds, 4), xlsxInlineCell("C"+ref, firstNonEmpty(unit, "г/л"), 4), xlsxInlineCell("D"+ref, "мг/л", 4), calculationFormulaCell("F"+ref, tdsFormula, tdsMg, 4), xlsxInlineCell("G"+ref, "приведение минерализации к мг/л", 4))
				deltaFormula := fmt.Sprintf("ABS(F%d-F%d)/F%d*100", tdsRow, ionSumRow, tdsRow)
				formulaRow("Расхождение TDS и суммы ионов", "", "контроль данных", "%", "", deltaFormula, qc.TDSDifferencePct, "=|TDS-Σионов|/TDS×100")
			}
		}
	}

	naEqExpr := ""
	if na, ok := eqExpr("na"); ok {
		naEqExpr = na
		if k, okK := eqExpr("k"); okK {
			naEqExpr = "(" + na + "+" + k + ")"
		}
	} else if combined, ok := eqExpr("na_k"); ok {
		naEqExpr = combined
	}
	clEqExpr, clOK := eqExpr("cl")
	if naEqExpr != "" && clOK {
		naEq, naOK, _ := laboratorySulinNaKEquivalent(pass.Groups["scale"])
		clEq, clNumOK := laboratoryEquivalentMillimoles(pass.Groups["scale"], "cl", 35.45, 1)
		if naOK && clNumOK && clEq > 0 {
			ratioFormula := naEqExpr + "/(" + clEqExpr + ")"
			ratioRow := formulaRow("Коэффициент Сулина r(Na+K)/rCl", "", "", "безразм.", "", ratioFormula, naEq/clEq, "отношение эквивалентных концентраций")
			coefficientFormula := ""
			coefficient := 0.0
			if naEq > clEq {
				if so4Expr, ok := eqExpr("so4"); ok {
					if so4Eq, okNum := laboratoryEquivalentMillimoles(pass.Groups["scale"], "so4", 96.06, 2); okNum && so4Eq > 0 {
						coefficientFormula = "(" + naEqExpr + "-(" + clEqExpr + "))/(" + so4Expr + ")"
						coefficient = (naEq - clEq) / so4Eq
					}
				}
			} else if naEq < clEq {
				if mgExpr, ok := eqExpr("mg"); ok {
					if mgEq, okNum := laboratoryEquivalentMillimoles(pass.Groups["scale"], "mg", 24.305, 2); okNum && mgEq > 0 {
						coefficientFormula = "((" + clEqExpr + ")-" + naEqExpr + ")/(" + mgExpr + ")"
						coefficient = (clEq - naEq) / mgEq
					}
				}
			}
			if coefficientFormula != "" {
				formulaRow("Вторичный коэффициент Сулина", "", fmt.Sprintf("по F%d", ratioRow), "безразм.", "", coefficientFormula, coefficient, "при rNa/rCl>1: (rNa-rCl)/rSO₄; иначе (rCl-rNa)/rMg")
			}
			ref := strconv.Itoa(row)
			add(xlsxInlineCell("A"+ref, "Тип воды по В. А. Сулину", 3), xlsxInlineCell("F"+ref, laboratorySulinWaterType(pass.Groups["scale"]), 0), xlsxInlineCell("G"+ref, "классификация по рассчитанным эквивалентным коэффициентам", 0))
		}
	}

	section("КОРРОЗИЯ · РАСЧЁТНЫЕ КОЭФФИЦИЕНТЫ")
	corrosion := pass.Groups["corrosion"]
	loss, lossOK := parseLabNumber(corrosion["plate_mass_loss"])
	initial, initialOK := parseLabNumber(corrosion["plate_initial_mass"])
	final, finalOK := parseLabNumber(corrosion["plate_final_mass"])
	lossRow := 0
	if initialOK && finalOK && initial >= final {
		loss = initial - final
		lossOK = true
		ref := strconv.Itoa(row)
		lossRow = row
		add(xlsxInlineCell("A"+ref, "Потеря массы пластинки", 4), xlsxNumberCell("B"+ref, initial, 4), xlsxNumberCell("C"+ref, final, 4), xlsxInlineCell("D"+ref, "г", 4), calculationFormulaCell("F"+ref, "B"+ref+"-C"+ref, loss, 4), xlsxInlineCell("G"+ref, "=начальная масса-конечная масса", 4))
	} else if lossOK {
		ref := strconv.Itoa(row)
		lossRow = row
		add(xlsxInlineCell("A"+ref, "Потеря массы пластинки", 4), xlsxNumberCell("B"+ref, loss, 4), xlsxInlineCell("D"+ref, "г", 4), calculationFormulaCell("F"+ref, "B"+ref, loss, 4), xlsxInlineCell("G"+ref, "измеренное значение", 4))
	}
	if lossOK {
		days, okDays := parseLabNumber(corrosion["plate_exposure_days"])
		area, okArea := parseLabNumber(corrosion["plate_area"])
		density, okDensity := parseLabNumber(corrosion["plate_material_density"])
		if okDays && okArea && okDensity && days > 0 && area > 0 && density > 0 {
			ref := strconv.Itoa(row)
			lossExpr := "B" + ref
			lossCell := xlsxNumberCell("B"+ref, loss, 4)
			if lossRow > 0 {
				lossExpr = fmt.Sprintf("F%d", lossRow)
				lossCell = calculationFormulaCell("B"+ref, lossExpr, loss, 4)
			}
			formula := "87.6*B" + ref + "*1000/(C" + ref + "*D" + ref + "*E" + ref + "*24)"
			rate := 87.6 * loss * 1000 / (density * area * days * 24)
			add(xlsxInlineCell("A"+ref, "Средняя скорость коррозии по пластинке", 4), lossCell, xlsxNumberCell("C"+ref, density, 4), xlsxNumberCell("D"+ref, area, 4), xlsxNumberCell("E"+ref, days, 4), calculationFormulaCell("F"+ref, formula, rate, 4), xlsxInlineCell("G"+ref, "87.6×Δm×1000/(ρ×S×t×24), мм/год", 4))
		}
	}
	if without, okWithout := parseLabNumber(corrosion["corrosion_rate_without"]); okWithout && without > 0 {
		if with, okWith := parseLabNumber(corrosion["corrosion_rate_with"]); okWith {
			ref := strconv.Itoa(row)
			formula := "(B" + ref + "-C" + ref + ")/B" + ref + "*100"
			add(xlsxInlineCell("A"+ref, "Эффективность ингибиторной защиты", 4), xlsxNumberCell("B"+ref, without, 4), xlsxNumberCell("C"+ref, with, 4), xlsxInlineCell("D"+ref, "%", 4), calculationFormulaCell("F"+ref, formula, (without-with)/without*100, 4), xlsxInlineCell("G"+ref, "=(Vбез-Vс)/Vбез×100", 4))
		}
	}
	if actual, okActual := parseLabNumber(corrosion["wall_thickness"]); okActual {
		if minimum, okMinimum := parseLabNumber(corrosion["minimum_wall_thickness"]); okMinimum {
			ref := strconv.Itoa(row)
			formula := "B" + ref + "-C" + ref
			add(xlsxInlineCell("A"+ref, "Запас толщины стенки", 4), xlsxNumberCell("B"+ref, actual, 4), xlsxNumberCell("C"+ref, minimum, 4), xlsxInlineCell("D"+ref, "мм", 4), calculationFormulaCell("F"+ref, formula, actual-minimum, 4), xlsxInlineCell("G"+ref, "=фактическая-минимально допустимая", 4))
		}
	}
	if depth, okDepth := parseLabNumber(corrosion["pit_depth"]); okDepth {
		if years, okYears := parseLabNumber(corrosion["pit_period_years"]); okYears && years > 0 {
			ref := strconv.Itoa(row)
			formula := "B" + ref + "/C" + ref
			add(xlsxInlineCell("A"+ref, "Локальная скорость по язве", 4), xlsxNumberCell("B"+ref, depth, 4), xlsxNumberCell("C"+ref, years, 4), xlsxInlineCell("D"+ref, "мм/год", 4), calculationFormulaCell("F"+ref, formula, depth/years, 4), xlsxInlineCell("G"+ref, "=глубина язвы/период", 4))
		}
	}

	for _, group := range []struct {
		code, title string
		rules       []calculationThresholdRule
	}{
		{"corrosion", "КОРРОЗИЯ · ПОРОГОВЫЕ ПРОВЕРКИ", []calculationThresholdRule{{"ph", "pH", "<", 6.5}, {"chlorides", "Хлориды", ">", 10000}, {"h2s", "H₂S", ">", 0}, {"corrosion_rate", "Скорость коррозии", ">", 0.1}}},
		{"aspo", "АСПО · ПОРОГОВЫЕ ПРОВЕРКИ", []calculationThresholdRule{{"paraffins", "Парафины", ">", 3}, {"asphaltenes", "Асфальтены", ">", 1}, {"pour_point", "Температура застывания", ">", 15}, {"viscosity", "Вязкость", ">", 50}}},
		{"solids", "МЕХПРИМЕСИ · ПОРОГОВЫЕ ПРОВЕРКИ", []calculationThresholdRule{{"suspended", "Взвешенные вещества", ">", 50}, {"sand", "Песок", ">", 0}, {"turbidity", "Мутность", ">", 0}}},
	} {
		shown := false
		for _, rule := range group.rules {
			value, ok := parseLabNumber(pass.Groups[group.code][rule.Key])
			if !ok {
				continue
			}
			if !shown {
				section(group.title)
				shown = true
			}
			ref := strconv.Itoa(row)
			triggered := (rule.Op == "<" && value < rule.Limit) || (rule.Op == ">" && value > rule.Limit)
			cached := 0.0
			if triggered {
				cached = 1
			}
			formula := "--(B" + ref + rule.Op + "C" + ref + ")"
			add(xlsxInlineCell("A"+ref, rule.Label, 4), xlsxNumberCell("B"+ref, value, 4), xlsxNumberCell("C"+ref, rule.Limit, 4), xlsxInlineCell("D"+ref, pass.Units[group.code][rule.Key], 4), calculationFormulaCell("F"+ref, formula, cached, 4), xlsxInlineCell("G"+ref, "="+formula+"; 1 = признак", 4))
		}
	}

	if actual, ok := parseLabNumber(pass.Groups["gases"]["sample_temp"]); ok {
		if hydrate, ok := parseLabNumber(pass.Groups["gases"]["hydrate_temp"]); ok {
			section("ГАЗЫ · ТЕМПЕРАТУРНЫЙ ЗАПАС")
			ref := strconv.Itoa(row)
			formula := "B" + ref + "-C" + ref
			add(xlsxInlineCell("A"+ref, "Температурный запас ΔT", 4), xlsxNumberCell("B"+ref, actual, 4), xlsxNumberCell("C"+ref, hydrate, 4), xlsxInlineCell("D"+ref, "°C", 4), calculationFormulaCell("F"+ref, formula, actual-hydrate, 4), xlsxInlineCell("G"+ref, "=Tпробы-Tгидрата", 4))
		}
	}

	section("РАСЧЁТНЫЕ ПОКАЗАТЕЛИ ОТЧЁТА")
	ref := strconv.Itoa(row)
	add(xlsxInlineCell("A"+ref, "Прогноз МРП", 3), xlsxNumberCell("B"+ref, r.ForecastDays, 0), xlsxInlineCell("C"+ref, "сут.", 0))
	ref = strconv.Itoa(row)
	add(xlsxInlineCell("A"+ref, "Чистая выгода", 3), xlsxNumberCell("B"+ref, r.NetBenefitK, 0), xlsxInlineCell("C"+ref, "тыс. руб.", 0))
	ref = strconv.Itoa(row)
	prev := strconv.Itoa(row - 1)
	add(xlsxInlineCell("A"+ref, "Чистая выгода, млн руб.", 4), calculationFormulaCell("B"+ref, "B"+prev+"/1000", r.NetBenefitK/1000, 4), xlsxInlineCell("G"+ref, "=B"+prev+"/1000", 4))

	return `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="6" topLeftCell="A7" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="43" customWidth="1"/><col min="2" max="5" width="20" customWidth="1"/><col min="6" max="6" width="23" customWidth="1"/><col min="7" max="7" width="55" customWidth="1"/></cols><sheetData>` + rows.String() + `</sheetData></worksheet>`
}
