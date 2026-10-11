//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	laboratoryScopeObject      = "object"
	laboratoryScopeField       = "field"
	laboratoryScopeDeposit     = "deposit"
	laboratoryScopeObjectPlast = "object_plast"
	laboratoryScopeWorkshop    = "workshop"
	laboratoryScopeSelected    = "selected"
	laboratoryScopeAll         = "all"
)

var laboratoryScopeCodes = []string{
	laboratoryScopeObject,
	laboratoryScopeField,
	laboratoryScopeDeposit,
	laboratoryScopeObjectPlast,
	laboratoryScopeWorkshop,
	laboratoryScopeSelected,
	laboratoryScopeAll,
}

var laboratoryRiskFilterCodes = []string{"all", "corrosion", "scale", "aspo", "solids", "microbiology", "gases"}

var laboratoryRiskFilterNames = []string{
	"Все объекты",
	"Коррозия — повышенные/пограничные",
	"Соли — повышенные/пограничные",
	"АСПО — повышенные/пограничные",
	"Мехпримеси — повышенные/пограничные",
	"Микробиология — повышенные/пограничные",
	"Газы/гидраты — повышенные/пограничные",
}

func laboratoryRiskFilterCodeByIndex(index int) string {
	if index < 0 || index >= len(laboratoryRiskFilterCodes) {
		return "all"
	}
	return laboratoryRiskFilterCodes[index]
}

func laboratoryFilterKeysByDirection(keys []string, passes []LabPassRecord, filterCode string) []string {
	if filterCode == "" || filterCode == "all" {
		return keys
	}
	group, ok := labGroupByCode(filterCode)
	if !ok {
		return keys
	}
	latest := laboratoryLatestSavedPassesForKeys(keys, passes)
	allowed := map[string]bool{}
	for _, pass := range latest {
		g := laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := canonicalizeLaboratoryValues(g, pass.Groups[g.Code], pass.Units[g.Code])
		assessment := laboratoryGroupSummary(g, values, pass.ScaleModel)
		if assessment.HasData && assessment.Level >= 2 {
			allowed[normalizeLaboratoryObjectKey(pass.ObjectKey)] = true
		}
	}
	out := make([]string, 0, len(allowed))
	for _, key := range keys {
		normalized := normalizeLaboratoryObjectKey(key)
		if allowed[normalized] {
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return out
}

var laboratoryScopeNames = []string{
	"Текущий объект",
	"Месторождение выбранного объекта",
	"Залежь выбранного объекта",
	"Объект / пласт выбранного объекта",
	"Цех выбранного объекта",
	"Выбранные объекты",
	"Все объекты с сохранёнными анализами",
}

func laboratorySelectedTechState(objectKey string) (TechObjectState, bool) {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return TechObjectState{}, false
	}
	for _, st := range techCurrent {
		if normalizeLaboratoryObjectKey(st.ObjectKey) == key {
			return st, true
		}
	}
	for _, view := range allObjectViews() {
		if normalizeLaboratoryObjectKey(view.State.ObjectKey) == key {
			return view.State, true
		}
	}
	return TechObjectState{}, false
}

func laboratoryScopeCodeByIndex(index int) string {
	if index < 0 || index >= len(laboratoryScopeCodes) {
		return laboratoryScopeObject
	}
	return laboratoryScopeCodes[index]
}

func laboratoryScopeObjectKeys(scopeCode, selectedObject string, passes []LabPassRecord) []string {
	selected := normalizeLaboratoryObjectKey(selectedObject)
	if selected == "" {
		return nil
	}
	if scopeCode == laboratoryScopeObject {
		return []string{selected}
	}
	if scopeCode == laboratoryScopeSelected {
		return laboratorySelectedObjectKeysText(getText(labScopeObjectsEdit))
	}
	st, ok := laboratorySelectedTechState(selectedObject)
	if !ok && scopeCode != laboratoryScopeAll {
		return []string{selected}
	}
	keys := map[string]bool{}
	if scopeCode == laboratoryScopeAll {
		for _, p := range passes {
			if p.Status == labPassSavedStatus {
				if k := normalizeLaboratoryObjectKey(p.ObjectKey); k != "" {
					keys[k] = true
				}
			}
		}
	} else {
		for _, candidate := range techCurrent {
			include := false
			switch scopeCode {
			case laboratoryScopeField:
				include = strings.EqualFold(strings.TrimSpace(candidate.Field), strings.TrimSpace(st.Field)) && strings.TrimSpace(st.Field) != ""
			case laboratoryScopeDeposit:
				include = strings.EqualFold(strings.TrimSpace(candidate.Deposit), strings.TrimSpace(st.Deposit)) && strings.TrimSpace(st.Deposit) != ""
			case laboratoryScopeObjectPlast:
				include = strings.EqualFold(strings.TrimSpace(candidate.ObjectPlast), strings.TrimSpace(st.ObjectPlast)) && strings.TrimSpace(st.ObjectPlast) != ""
			case laboratoryScopeWorkshop:
				include = strings.EqualFold(strings.TrimSpace(candidate.Workshop), strings.TrimSpace(st.Workshop)) && strings.TrimSpace(st.Workshop) != ""
			}
			if include {
				if k := normalizeLaboratoryObjectKey(candidate.ObjectKey); k != "" {
					keys[k] = true
				}
			}
		}
	}
	if len(keys) == 0 {
		keys[selected] = true
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func laboratoryLatestSavedPassesForKeys(keys []string, passes []LabPassRecord) []LabPassRecord {
	allowed := map[string]bool{}
	for _, key := range keys {
		allowed[normalizeLaboratoryObjectKey(key)] = true
	}
	latest := map[string]LabPassRecord{}
	for _, p := range passes {
		key := normalizeLaboratoryObjectKey(p.ObjectKey)
		if p.Status != labPassSavedStatus || !allowed[key] {
			continue
		}
		old, exists := latest[key]
		if !exists || parseDate(p.Date).After(parseDate(old.Date)) || (p.Date == old.Date && p.UpdatedAt > old.UpdatedAt) {
			latest[key] = p
		}
	}
	out := make([]LabPassRecord, 0, len(latest))
	for _, p := range latest {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.Compare(out[i].ObjectKey, out[j].ObjectKey) < 0 })
	return out
}

func laboratoryPersonalInfluencePass(objectKey string, passes []LabPassRecord, active *LabPassRecord) (LabPassRecord, bool) {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return LabPassRecord{}, false
	}
	if active != nil && normalizeLaboratoryObjectKey(active.ObjectKey) == key {
		has := false
		for groupCode, values := range active.Groups {
			if groupCode == "overall" {
				continue
			}
			for _, value := range values {
				if strings.TrimSpace(value) != "" {
					has = true
					break
				}
			}
			if has {
				break
			}
		}
		if has {
			copy := *active
			copy.Groups = cloneLabGroups(active.Groups)
			copy.Units = cloneLabGroups(active.Units)
			return copy, true
		}
	}
	latest := laboratoryLatestSavedPassesForKeys([]string{key}, passes)
	if len(latest) == 0 {
		return LabPassRecord{}, false
	}
	return latest[0], true
}

func laboratoryAggregateInfluence(scopeCode, selectedObject string, passes []LabPassRecord) ([]laboratoryInfluence, int) {
	keys := laboratoryScopeKeysForCurrentUI(scopeCode, selectedObject, passes)
	latest := laboratoryLatestSavedPassesForKeys(keys, passes)
	result := make([]laboratoryInfluence, 0, 6)
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		riskCount, lowCount, knownCount := 0, 0, 0
		filled, total := 0, 0
		for _, p := range latest {
			g := laboratoryScaleInputGroup(group, p.ScaleModel)
			values := canonicalizeLaboratoryValues(g, p.Groups[g.Code], p.Units[g.Code])
			assessment := laboratoryGroupSummary(g, values, p.ScaleModel)
			if assessment.HasData {
				knownCount++
				filled += assessment.BaseFilled
				total += assessment.BaseTotal
			}
			if assessment.Level >= 2 {
				riskCount++
			}
			if assessment.Level == 1 {
				lowCount++
			}
		}
		state := "НЕТ ДАННЫХ"
		level := 0
		if knownCount > 0 {
			if riskCount > 0 {
				level = 3
				state = fmt.Sprintf("РИСК %d/%d", riskCount, knownCount)
			} else if lowCount > 0 {
				level = 1
				state = fmt.Sprintf("НИЗКОЕ %d/%d", lowCount, knownCount)
			} else {
				state = fmt.Sprintf("ОЦЕНЕНО %d", knownCount)
			}
		}
		result = append(result, laboratoryInfluence{Name: group.Name, Level: level, BaseFilled: filled, BaseTotal: total, State: state})
	}
	return result, len(latest)
}

func laboratoryScopeCaption(scopeCode, selectedObject string, passes []LabPassRecord) string {
	keys := laboratoryScopeKeysForCurrentUI(scopeCode, selectedObject, passes)
	if scopeCode == laboratoryScopeObject {
		return "Персонально по объекту: " + emptyLabText(selectedObject)
	}
	st, _ := laboratorySelectedTechState(selectedObject)
	value := ""
	name := "Группа"
	switch scopeCode {
	case laboratoryScopeField:
		name, value = "Месторождение", st.Field
	case laboratoryScopeDeposit:
		name, value = "Залежь", st.Deposit
	case laboratoryScopeObjectPlast:
		name, value = "Объект / пласт", st.ObjectPlast
	case laboratoryScopeWorkshop:
		name, value = "Цех", st.Workshop
	case laboratoryScopeSelected:
		name, value = "Выбранные объекты", fmt.Sprintf("%d объектов", len(keys))
	case laboratoryScopeAll:
		name, value = "Все объекты", "с сохранёнными лабораторными анализами"
	}
	return fmt.Sprintf("%s: %s · объектов в области: %d", name, emptyLabText(value), len(keys))
}

type laboratoryAttentionSummary struct {
	WithoutAnalysis []string
	Stale           []string
	FutureDate      []string
	Risk            []string
}

func laboratoryAttentionForKeys(keys []string, passes []LabPassRecord, now time.Time) laboratoryAttentionSummary {
	out := laboratoryAttentionSummary{}
	latest := laboratoryLatestSavedPassesForKeys(keys, passes)
	byKey := map[string]LabPassRecord{}
	for _, p := range latest {
		byKey[normalizeLaboratoryObjectKey(p.ObjectKey)] = p
	}
	for _, raw := range keys {
		key := normalizeLaboratoryObjectKey(raw)
		p, ok := byKey[key]
		if !ok {
			out.WithoutAnalysis = append(out.WithoutAnalysis, key)
			continue
		}
		d := parseDate(p.Date)
		if d.IsZero() || now.Sub(d).Hours()/24 > 180 {
			out.Stale = append(out.Stale, key)
		} else if d.After(now) {
			out.FutureDate = append(out.FutureDate, key)
		}
		hasRisk := false
		for _, group := range labGroups {
			if group.Code == "overall" {
				continue
			}
			g := laboratoryScaleInputGroup(group, p.ScaleModel)
			values := canonicalizeLaboratoryValues(g, p.Groups[g.Code], p.Units[g.Code])
			assessment := laboratoryGroupSummary(g, values, p.ScaleModel)
			if assessment.HasData && assessment.Level >= 2 {
				hasRisk = true
				break
			}
		}
		if hasRisk {
			out.Risk = append(out.Risk, key)
		}
	}
	sort.Strings(out.WithoutAnalysis)
	sort.Strings(out.Stale)
	sort.Strings(out.FutureDate)
	sort.Strings(out.Risk)
	return out
}

func laboratoryCompactKeyList(keys []string, limit int) string {
	if len(keys) == 0 {
		return "—"
	}
	if limit <= 0 || limit > len(keys) {
		limit = len(keys)
	}
	parts := make([]string, 0, limit+1)
	for _, key := range keys[:limit] {
		parts = append(parts, laboratoryChartObjectLabel(key))
	}
	if len(keys) > limit {
		parts = append(parts, fmt.Sprintf("+%d", len(keys)-limit))
	}
	return strings.Join(parts, ", ")
}

func laboratoryAttentionCaption(a laboratoryAttentionSummary) string {
	return fmt.Sprintf("Требуют внимания: без анализа — %d; устаревшие — %d; будущая дата — %d; повышенные/пограничные признаки — %d.", len(a.WithoutAnalysis), len(a.Stale), len(a.FutureDate), len(a.Risk))
}

type laboratoryMassAnalytics struct {
	ScopeObjects       int
	ObjectsWithPass    int
	ObjectsWithoutPass int
	Fresh30            int
	Recent180          int
	Stale180           int
	FutureDate         int
	DirectionKnown     map[string]int
	DirectionRisk      map[string]int
}

func laboratoryMassAnalyticsForKeys(keys []string, passes []LabPassRecord, now time.Time) laboratoryMassAnalytics {
	a := laboratoryMassAnalytics{
		ScopeObjects:   len(keys),
		DirectionKnown: map[string]int{},
		DirectionRisk:  map[string]int{},
	}
	latest := laboratoryLatestSavedPassesForKeys(keys, passes)
	a.ObjectsWithPass = len(latest)
	a.ObjectsWithoutPass = maxInt(0, a.ScopeObjects-a.ObjectsWithPass)
	for _, p := range latest {
		d := parseDate(p.Date)
		if d.IsZero() {
			a.Stale180++
		} else {
			days := int(now.Sub(d).Hours() / 24)
			switch {
			case days < 0:
				a.FutureDate++
			case days <= 30:
				a.Fresh30++
			case days <= 180:
				a.Recent180++
			default:
				a.Stale180++
			}
		}
		for _, group := range labGroups {
			if group.Code == "overall" {
				continue
			}
			g := laboratoryScaleInputGroup(group, p.ScaleModel)
			values := canonicalizeLaboratoryValues(g, p.Groups[g.Code], p.Units[g.Code])
			assessment := laboratoryGroupSummary(g, values, p.ScaleModel)
			if assessment.HasData {
				a.DirectionKnown[g.Code]++
			}
			if assessment.Level >= 2 {
				a.DirectionRisk[g.Code]++
			}
		}
	}
	return a
}

func laboratoryMassAnalyticsSummary(a laboratoryMassAnalytics) string {
	return fmt.Sprintf("Охват: %d объектов; есть сохранённый анализ: %d; без анализа: %d. Свежесть последних анализов: ≤30 сут — %d; 31–180 сут — %d; >180 сут/без корректной даты — %d; будущая дата — %d.",
		a.ScopeObjects, a.ObjectsWithPass, a.ObjectsWithoutPass, a.Fresh30, a.Recent180, a.Stale180, a.FutureDate)
}

func laboratoryMassDirectionSummary(a laboratoryMassAnalytics) string {
	labels := []struct{ code, label string }{
		{"corrosion", "Коррозия"}, {"scale", "Соли"}, {"aspo", "АСПО"},
		{"solids", "Мехпримеси"}, {"microbiology", "Микробиология"}, {"gases", "Газы/гидраты"},
	}
	parts := make([]string, 0, len(labels))
	for _, item := range labels {
		known := a.DirectionKnown[item.code]
		risk := a.DirectionRisk[item.code]
		parts = append(parts, fmt.Sprintf("%s %d/%d", item.label, risk, known))
	}
	return "Повышенные/пограничные признаки среди оценённых: " + strings.Join(parts, "; ") + "."
}

type laboratoryKPI struct {
	Label string
	Value string
	Note  string
}

func laboratoryMassKPIs(a laboratoryMassAnalytics) []laboratoryKPI {
	freshNote := "последний анализ ≤30 сут"
	if a.ObjectsWithPass == 0 {
		freshNote = "нет сохранённых анализов"
	}
	stale := a.Stale180 + a.FutureDate
	return []laboratoryKPI{
		{Label: "ВЫБОРКА", Value: fmt.Sprintf("%d", a.ScopeObjects), Note: "объектов после фильтра"},
		{Label: "ЕСТЬ АНАЛИЗ", Value: fmt.Sprintf("%d", a.ObjectsWithPass), Note: "последняя диагностика на объект"},
		{Label: "БЕЗ АНАЛИЗА", Value: fmt.Sprintf("%d", a.ObjectsWithoutPass), Note: "не считать низким риском"},
		{Label: "СВЕЖИЕ", Value: fmt.Sprintf("%d", a.Fresh30), Note: freshNote},
		{Label: "УСТАРЕЛИ", Value: fmt.Sprintf("%d", stale), Note: ">180 сут / ошибка даты"},
	}
}

func laboratoryPersonalKPIs(objectKey string, passes []LabPassRecord) []laboratoryKPI {
	key := normalizeLaboratoryObjectKey(objectKey)
	count := 0
	lastDate := "—"
	last := LabPassRecord{}
	for _, p := range passes {
		if p.Status != labPassSavedStatus || normalizeLaboratoryObjectKey(p.ObjectKey) != key {
			continue
		}
		count++
		if last.PassID == "" || parseDate(p.Date).After(parseDate(last.Date)) || (p.Date == last.Date && p.UpdatedAt > last.UpdatedAt) {
			last = p
			lastDate = emptyLabText(p.Date)
		}
	}
	directions := 0
	if last.PassID != "" {
		for _, group := range labGroups {
			if group.Code == "overall" {
				continue
			}
			g := laboratoryScaleInputGroup(group, last.ScaleModel)
			values := canonicalizeLaboratoryValues(g, last.Groups[g.Code], last.Units[g.Code])
			if laboratoryGroupSummary(g, values, last.ScaleModel).HasData {
				directions++
			}
		}
	}
	return []laboratoryKPI{
		{Label: "АНАЛИЗОВ", Value: fmt.Sprintf("%d", count), Note: "сохранено по объекту"},
		{Label: "ПОСЛЕДНИЙ", Value: lastDate, Note: "дата сохранённого анализа"},
		{Label: "НАПРАВЛЕНИЙ", Value: fmt.Sprintf("%d/6", directions), Note: "есть данные для оценки"},
	}
}
