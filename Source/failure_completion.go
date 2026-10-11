//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
)

const failureNoSelection = "Не выбрано"
const failureNotApplicable = "Не применяется"

func failureNormalizeEntry(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func failureHasValue(value string) bool {
	v := strings.TrimSpace(value)
	return v != "" && !strings.EqualFold(v, failureNoSelection) && !strings.EqualFold(v, wellChoicePrompt) && !strings.EqualFold(v, failureNotApplicable)
}

func failureIsNoData(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), failureNoSelection)
}

func failureIsNotApplicable(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), failureNotApplicable)
}

func failureExtraValue(e FailureEvent, column string) string {
	if e.Extra == nil {
		return ""
	}
	return strings.TrimSpace(e.Extra[column])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func failureCanonicalOperationMode(value string) string {
	s := strings.ToUpper(failureNormalizeEntry(value))
	s = strings.ReplaceAll(s, "‑", "-")
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "_", " ")
	switch {
	case strings.Contains(s, "ОРЗД") || strings.Contains(s, "ОРД"):
		return "ОРД"
	case strings.Contains(s, "ФОН"):
		return "ФОН"
	case strings.Contains(s, "НПУ"):
		return "НПУ-ВД"
	case strings.Contains(s, "УПЛД"):
		return "УПЛД"
	case strings.Contains(s, "ЭЦН") || strings.Contains(s, "УЭЦН"):
		return "ЭЦН"
	case strings.Contains(s, "ЭВН") || strings.Contains(s, "УЭВН"):
		return "ЭВН"
	case strings.Contains(s, "ШГН") || strings.Contains(s, "УШГН"):
		return "ШГН"
	case strings.Contains(s, "ШВН") || strings.Contains(s, "УШВН"):
		return "ШВН"
	default:
		return ""
	}
}

func failureOperationFamily(e FailureEvent) string {
	s := strings.ToUpper(strings.Join([]string{
		e.OperationMode, e.Equipment, e.EquipmentSize, e.EquipmentSize2, e.EquipmentSize3, e.EquipmentSize4,
	}, " "))
	switch {
	case strings.Contains(s, "ОРД"):
		return "ОРД"
	case strings.Contains(s, "ФОН"):
		return "ФОН"
	case strings.Contains(s, "ЭЦН") || strings.Contains(s, "УЭЦН"):
		return "ЭЦН"
	case strings.Contains(s, "ЭВН") || strings.Contains(s, "УЭВН"):
		return "ЭВН"
	case strings.Contains(s, "ШГН") || strings.Contains(s, "УШГН"):
		return "ШГН"
	case strings.Contains(s, "ШВН") || strings.Contains(s, "УШВН"):
		return "ШВН"
	case strings.Contains(s, "НПУ"):
		return "НПУ-ВД"
	case strings.Contains(s, "УПЛД"):
		return "УПЛД"
	default:
		return strings.TrimSpace(e.OperationMode)
	}
}

type failureCompletionField struct {
	Key   string
	Value string
}

func failureFullFields(e FailureEvent) []failureCompletionField {
	fields := []failureCompletionField{
		{"ТКРС 1", firstNonEmpty(e.TKRS1, failureExtraValue(e, "U"))},
		{"ТКРС 2", firstNonEmpty(e.TKRS2, failureExtraValue(e, "V"))},
		{"Вид повреждения", firstNonEmpty(e.DamageType, failureExtraValue(e, "W"))},
		{"Итоговая причина", firstNonEmpty(e.FinalCause, failureExtraValue(e, "X"))},
		{"Уточнённая категория осложнения", firstNonEmpty(e.RefinedComplication, failureExtraValue(e, "Y"))},
		{"Метод до отказа 1", firstNonEmpty(e.MethodBeforeFailure, failureExtraValue(e, "Z"))},
		{"Метод до отказа 2", firstNonEmpty(e.MethodBeforeFailure2, failureExtraValue(e, "AA"))},
		{"Метод после отказа 1", firstNonEmpty(e.MethodAfterFailure, failureExtraValue(e, "AB"))},
		{"Метод после отказа 2", firstNonEmpty(e.MethodAfterFailure2, failureExtraValue(e, "AC"))},
		{"Подтверждение", firstNonEmpty(e.Confirmation, failureExtraValue(e, "AD"))},
		{"Статус расследования", firstNonEmpty(e.InvestigationStatus, failureExtraValue(e, "AE"))},
		{"Достоверность", firstNonEmpty(e.Confidence, failureExtraValue(e, "AF"))},
		{"Заключение комиссии / акт", firstNonEmpty(e.CommissionConclusion, failureExtraValue(e, "BO"))},
		{"Источник / документ", firstNonEmpty(e.CommissionSource, failureExtraValue(e, "BP"))},
	}

	nkt := []failureCompletionField{
		{"Тип / типоразмер НКТ", failureExtraValue(e, "AG")},
		{"Длина НКТ, м", failureExtraValue(e, "AH")},
		{"Год НКТ", failureExtraValue(e, "AI")},
		{"СПО", failureExtraValue(e, "AJ")},
		{"Состояние НКТ до спуска", failureExtraValue(e, "AK")},
	}
	pump := []failureCompletionField{
		{"Тип насоса", firstNonEmpty(e.FountainPumpType, failureExtraValue(e, "AL"))},
		{"Условный диаметр / размер ШГН, мм", failureExtraValue(e, "AM")},
		{"Производитель насоса", failureExtraValue(e, "AN")},
		{"Глубина спуска насоса, м", firstNonEmpty(e.FountainPumpDepth, failureExtraValue(e, "AO"))},
	}
	motor := []failureCompletionField{
		{"Производитель двигателя", failureExtraValue(e, "AP")},
		{"Типоразмер двигателя, мм", failureExtraValue(e, "AQ")},
		{"Год двигателя", failureExtraValue(e, "AR")},
		{"Состояние двигателя", failureExtraValue(e, "AS")},
	}
	electric := []failureCompletionField{
		{"Типоразмер кабеля, мм²", failureExtraValue(e, "AT")},
		{"Длина кабеля, м", failureExtraValue(e, "AU")},
		{"Состояние кабеля", failureExtraValue(e, "AV")},
		{"Типоразмер удлинителя, мм²", failureExtraValue(e, "AW")},
		{"Длина удлинителя, м", failureExtraValue(e, "AX")},
		{"Состояние удлинителя", failureExtraValue(e, "AY")},
		{"Производитель гидрозащиты", failureExtraValue(e, "AZ")},
		{"Типоразмер гидрозащиты, мм", failureExtraValue(e, "BA")},
		{"Год гидрозащиты", failureExtraValue(e, "BB")},
		{"Состояние гидрозащиты", failureExtraValue(e, "BC")},
		{"Производитель ТМС", failureExtraValue(e, "BD")},
		{"Типоразмер ТМС", failureExtraValue(e, "BE")},
		{"Год ТМС", failureExtraValue(e, "BF")},
		{"Состояние ТМС", failureExtraValue(e, "BG")},
	}
	rods := []failureCompletionField{
		{"Состояние штанг до спуска", failureExtraValue(e, "BH")},
		{"Производитель штанг", failureExtraValue(e, "BI")},
		{"Штанги 19, шт.", failureExtraValue(e, "BJ")},
		{"Штанги 22, шт.", failureExtraValue(e, "BK")},
		{"Штанги 25, шт.", failureExtraValue(e, "BL")},
		{"Год штанг", failureExtraValue(e, "BM")},
		{"СПО штанг", failureExtraValue(e, "BN")},
	}

	switch failureOperationFamily(e) {
	case "ШГН":
		fields = append(fields, pump...)
		fields = append(fields, nkt...)
		fields = append(fields, rods...)
	case "ЭЦН", "ЭВН":
		fields = append(fields, pump...)
		fields = append(fields, nkt...)
		fields = append(fields, motor...)
		fields = append(fields, electric...)
	case "ШВН", "НПУ-ВД", "УПЛД":
		fields = append(fields, pump...)
		fields = append(fields, nkt...)
	case "ФОН":
		// Фонтанный способ задаётся только полем «Способ эксплуатации».
		// Насосный блок для фонтанной добычи не применяется.
		fields = append(fields, nkt...)
	case "ОРД":
		fields = append(fields, pump...)
		fields = append(fields, nkt...)
		equipment := strings.ToUpper(strings.Join([]string{e.Equipment, e.EquipmentSize, e.EquipmentSize2, e.EquipmentSize3, e.EquipmentSize4}, " "))
		if strings.Contains(equipment, "ЭЦН") || strings.Contains(equipment, "ЭВН") || strings.Contains(equipment, "УЭЦН") || strings.Contains(equipment, "УЭВН") {
			fields = append(fields, motor...)
			fields = append(fields, electric...)
		}
	}
	return fields
}

func normalizeFailureEquipmentSchema(e *FailureEvent) {
	if e == nil {
		return
	}
	if canonical := failureCanonicalOperationMode(firstNonEmpty(e.OperationMode, e.Equipment)); canonical != "" {
		e.OperationMode = canonical
		if !failureHasValue(e.Equipment) || failureCanonicalOperationMode(e.Equipment) != "" {
			e.Equipment = canonical
		}
	}
	if e.Extra == nil {
		e.Extra = map[string]string{}
	}
	// RC684 canonical equipment schema: AG:AK = НКТ, AL:AO = насос.
	// Migrate RC683/legacy cards where AG:AJ held pump fields and AK:AO held НКТ.
	oldPumpType := firstNonEmpty(e.FountainPumpType, e.Extra["AG"])
	oldLooksLegacy := failureHasValue(e.Extra["AK"]) && !failureHasValue(e.Extra["AL"])
	if !oldLooksLegacy && failureHasValue(e.Extra["AG"]) && strings.EqualFold(failureNormalizeEntry(e.Extra["AG"]), failureNormalizeEntry(oldPumpType)) {
		oldLooksLegacy = true
	}
	if oldLooksLegacy {
		old := map[string]string{}
		for _, key := range []string{"AG", "AH", "AI", "AJ", "AK", "AL", "AM", "AN", "AO"} {
			old[key] = strings.TrimSpace(e.Extra[key])
		}
		e.Extra["AG"] = old["AK"]
		e.Extra["AH"] = old["AL"]
		e.Extra["AI"] = old["AM"]
		e.Extra["AJ"] = old["AN"]
		e.Extra["AK"] = old["AO"]
		e.Extra["AL"] = firstNonEmpty(e.FountainPumpType, old["AG"])
		e.Extra["AM"] = old["AI"]
		e.Extra["AN"] = old["AJ"]
		e.Extra["AO"] = firstNonEmpty(e.FountainPumpDepth, old["AH"])
	}
	if !failureHasValue(e.FountainPumpType) {
		e.FountainPumpType = strings.TrimSpace(e.Extra["AL"])
	}
	if !failureHasValue(e.FountainPumpDepth) {
		e.FountainPumpDepth = strings.TrimSpace(e.Extra["AO"])
	}
}

func failureFullCompletion(e FailureEvent) (pct, noData, total, filled int) {
	for _, field := range failureFullFields(e) {
		if failureIsNotApplicable(field.Value) {
			continue
		}
		total++
		if failureHasValue(field.Value) {
			filled++
		} else if failureIsNoData(field.Value) {
			noData++
		}
	}
	if total > 0 {
		pct = int(float64(filled)*100/float64(total) + 0.5)
	}
	return
}

func failureOperationalComplete(e FailureEvent) bool {
	required := []string{
		e.Field, e.Well, e.CycleStartDate, e.Date, e.OperationMode,
		e.EquipmentSize, e.EquipmentBrand, e.EventType, e.StopCause,
		e.FailedUnit, firstNonEmpty(e.PreliminaryCause, e.SourceCause, e.PreliminaryConclusion),
		firstNonEmpty(e.PrimaryComplication, e.ComplicationFactor, e.SourceClass, e.UserClass),
	}
	for _, value := range required {
		if !failureHasValue(value) {
			return false
		}
	}
	return true
}

func refreshFailureDerived(e *FailureEvent) {
	if e == nil {
		return
	}
	pct, noData, _, filled := failureFullCompletion(*e)
	e.FullCompletionPct = pct
	e.NoDataCount = noData
	e.HasFullData = filled > 0 || noData > 0
	e.OperationalComplete = failureOperationalComplete(*e)
	e.OperationalOnly = !e.HasFullData
}

func failureDataStatus(e FailureEvent) string {
	refreshFailureDerived(&e)
	switch {
	case e.FullCompletionPct >= 100:
		return "Полная карточка заполнена"
	case e.HasFullData:
		if e.NoDataCount > 0 {
			return fmt.Sprintf("Полная карточка: %d%% · нет данных: %d", e.FullCompletionPct, e.NoDataCount)
		}
		return fmt.Sprintf("Полная карточка: %d%%", e.FullCompletionPct)
	default:
		return "Только оперативный ввод"
	}
}

func failureSourceLabel(e FailureEvent) string {
	if strings.EqualFold(strings.TrimSpace(e.SourceType), "excel") || strings.TrimSpace(e.SourceFile) != "" {
		parts := []string{"Excel"}
		if e.SourceFile != "" {
			parts = append(parts, e.SourceFile)
		}
		if e.SourceSheet != "" {
			parts = append(parts, e.SourceSheet)
		}
		if e.SourceRow > 0 {
			parts = append(parts, "строка "+strconv.Itoa(e.SourceRow))
		}
		return strings.Join(parts, " · ")
	}
	if e.Manual || strings.EqualFold(strings.TrimSpace(e.SourceType), "manual") {
		if e.UpdatedAt != "" {
			return "Ручной ввод · " + e.UpdatedAt
		}
		return "Ручной ввод"
	}
	if strings.TrimSpace(e.Source) != "" {
		return e.Source
	}
	return "—"
}

func failureGNOCompact(e FailureEvent) string {
	values := []string{}
	for _, v := range []string{e.EquipmentSize, e.EquipmentSize2, e.EquipmentSize3, e.EquipmentSize4} {
		if failureHasValue(v) {
			values = append(values, strings.TrimSpace(v))
		}
	}
	base := strings.TrimSpace(e.OperationMode)
	if base == "" {
		base = strings.TrimSpace(e.Equipment)
	}
	if len(values) == 0 {
		return dash(base)
	}
	if len(values) == 1 {
		return strings.Trim(strings.Join([]string{base, values[0]}, " · "), " ·")
	}
	return fmt.Sprintf("%s · %s (+%d)", base, values[0], len(values)-1)
}
