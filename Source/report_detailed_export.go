//go:build windows

package main

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
)

// A calculation in one laboratory direction must not erase the other
// directions or the raw measurements from an approved report.
func reportCalculationSectionsSafely(pass LabPassRecord) string {
	var b strings.Builder
	for i := range calculationBasisSections {
		index := i
		part := reportSectionSafely(fmt.Sprintf("calculation_%d", i+1), func() string {
			return calculationPassSectionWithModel(pass, index, "")
		})
		if part == "" {
			fmt.Fprintf(&b, "\r\nРасчётный блок %d: автоматический расчёт недоступен; исходные измерения приведены выше.\r\n", i+1)
		} else {
			b.WriteString("\r\n" + part + "\r\n")
		}
	}
	return b.String()
}

func reportSectionSafely(name string, build func() string) (value string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			diagLog("approved detailed section %s PANIC: %v\n%s", name, recovered, debug.Stack())
			value = ""
		}
	}()
	return build()
}

// The fallback reads the bundled teaching fixture directly. It cannot depend
// on Win32 controls or on a calculation that has just failed.
func teachingLaboratoryDetailFallback(objectKey string) string {
	pass := LabPassRecord{ObjectKey: objectKey, Groups: map[string]map[string]string{}, Units: map[string]map[string]string{}}
	teachingCompleteLabDraft(&pass)
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ\r\nПолный набор исходных показателей учебного протокола.\r\n")
	if pass.Source != "" {
		b.WriteString("Протокол: " + pass.Source + "\r\n")
	}
	groups := make([]string, 0, len(pass.Groups))
	for code := range pass.Groups {
		groups = append(groups, code)
	}
	sort.Strings(groups)
	for _, code := range groups {
		b.WriteString("\r\nНАПРАВЛЕНИЕ: " + strings.ToUpper(code) + "\r\n")
		definitions := map[string]LabFieldDef{}
		if group, ok := labGroupByCode(code); ok {
			for _, field := range group.Fields {
				definitions[field.Key] = field
			}
		}
		keys := make([]string, 0, len(pass.Groups[code]))
		for key := range pass.Groups[code] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !reportLaboratoryFieldBelongsToModel(key, pass.ScaleModel) {
				continue
			}
			field, ok := definitions[key]
			label := field.Label
			if !ok {
				label = "Дополнительный показатель (" + key + ")"
			}
			unit := pass.Units[code][key]
			if unit == "" {
				unit = field.Unit
			}
			b.WriteString(reportLaboratoryIndicatorLine(label, pass.Groups[code][key], unit))
		}
	}
	b.WriteString("\r\nРАСЧЁТЫ ПО НАПРАВЛЕНИЯМ\r\n")
	b.WriteString(reportCalculationSectionsSafely(pass))
	return b.String()
}

func teachingMethodsDetailFallback(s AlternativeMethodScenario) string {
	var b strings.Builder
	fmt.Fprintf(&b, "РАЗДЕЛ 4. МЕТОДЫ И МЕРОПРИЯТИЯ\r\nВариант: %s\r\nКлассификация: %s\r\nДоступность: %s\r\nОснование: %s\r\n", s.Name, s.Classification, s.Availability, s.SelectionBasis)
	for i, method := range alternativeScenarioMethodsForCalculation(s) {
		fmt.Fprintf(&b, "\r\nМЕТОД %d: %s\r\n", i+1, method)
		if a, ok := teachingMethodEffect(method); ok {
			b.WriteString(reportMethodAssessmentDetail(a))
		}
	}
	fmt.Fprintf(&b, "\r\nБазовый МРП: %.0f сут.; прогноз: %.0f / %.0f / %.0f сут.; цель: %.0f сут.\r\n", s.BaselineMRPDays, s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays, s.TargetMRPDays)
	return b.String()
}

func buildTeachingApprovedSectionsSafely(r FullReportRecord, scenario AlternativeMethodScenario) map[string]string {
	key := r.ObjectKey
	builders := []func() string{
		func() string { return reportSection1FundAndFailures(key) },
		func() string { return reportSection2Laboratory(key) },
		func() string { return reportSection3Assessment(key) },
		func() string { return reportSection4Methods(key, scenario) },
		func() string { return reportSection5Runlife(key, scenario) },
		func() string { return reportSection6EconomicsDetached(key, scenario) },
		func() string { return reportSection7Final(key, scenario, r.Snapshot) },
	}
	sections := make(map[string]string, 7)
	for i, build := range builders {
		name := fmt.Sprintf("section_%d", i+1)
		sections[name] = reportSectionSafely(name, build)
		if sections[name] != "" {
			continue
		}
		switch i + 1 {
		case 2:
			sections[name] = teachingLaboratoryDetailFallback(key)
		case 4:
			sections[name] = teachingMethodsDetailFallback(scenario)
		case 7:
			sections[name] = "РАЗДЕЛ 7. ИТОГОВЫЙ ОТЧЁТ\r\n" + r.Snapshot
		default:
			sections[name] = fmt.Sprintf("РАЗДЕЛ %d. Сведения из утверждённого снимка\r\n%s", i+1, r.Snapshot)
		}
	}
	return sections
}
