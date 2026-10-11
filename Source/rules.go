//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// The seed is generated from the working Excel matrix and method catalog.
// It is embedded so a clean offline installation always starts from the same
// traceable baseline, while the shared JSON can later be enriched with
// corporate documents and verified literature without recompiling the UI.
//
//go:embed rules_seed.json
var embeddedRuleSeed []byte

type RuleSource struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Organization string `json:"organization,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Status       string `json:"status,omitempty"`
	Reference    string `json:"reference,omitempty"`
	URL          string `json:"url,omitempty"`
	Year         int    `json:"year,omitempty"`
}

type MethodCatalogEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SourceID string `json:"source_id"`
	Status   string `json:"status"`
}

type MethodRule struct {
	ID                  string   `json:"id"`
	Complication        string   `json:"complication"`
	Method              string   `json:"method"`
	Purpose             string   `json:"purpose,omitempty"`
	Stage               string   `json:"stage,omitempty"`
	Criteria            []string `json:"criteria,omitempty"`
	RequiredInputs      []string `json:"required_inputs,omitempty"`
	Contraindications   []string `json:"contraindications,omitempty"`
	Control             []string `json:"control,omitempty"`
	Limitations         []string `json:"limitations,omitempty"`
	CriterionStatus     string   `json:"criterion_status,omitempty"`
	EconomicEffectBasis string   `json:"economic_effect_basis,omitempty"`
	PreliminaryStatus   string   `json:"preliminary_status,omitempty"`
	Comment             string   `json:"comment,omitempty"`
	SourceIDs           []string `json:"source_ids,omitempty"`
	Scope               string   `json:"scope,omitempty"`
}

type RuleCatalog struct {
	SchemaVersion int                  `json:"schema_version"`
	Sources       []RuleSource         `json:"sources"`
	MethodCatalog []MethodCatalogEntry `json:"method_catalog"`
	Rules         []MethodRule         `json:"rules"`
}

var ruleCatalog RuleCatalog

func loadRuleCatalog() RuleCatalog {
	var seed RuleCatalog
	if err := json.Unmarshal(embeddedRuleSeed, &seed); err != nil {
		diagLog("RULE seed decode error=%v", err)
	}
	var stored RuleCatalog
	loadJSON(sharedPath("rule_catalog.json"), &stored)
	if stored.SchemaVersion > 0 && len(stored.MethodCatalog) > 0 {
		ruleCatalog = mergeRuleCatalog(seed, stored)
	} else {
		ruleCatalog = seed
	}
	if ruleCatalog.SchemaVersion == 0 {
		ruleCatalog.SchemaVersion = 1
	}
	ruleCatalog = enforceCanonicalRuleCatalog677(ruleCatalog, seed)
	_ = saveJSON(sharedPath("rule_catalog.json"), ruleCatalog)
	return ruleCatalog
}

// enforceCanonicalRuleCatalog677 preserves user-added entries with their own IDs,
// but refreshes every built-in seed ID from the current embedded catalogue. The
// function name is retained for compatibility with the 677 migration tests. Older
// saved rule_catalog.json files must not restore outdated captions or incomplete
// method descriptions over the embedded Russian engineering handbook.
func enforceCanonicalRuleCatalog677(current, seed RuleCatalog) RuleCatalog {
	seedSources := map[string]RuleSource{}
	for _, v := range seed.Sources {
		seedSources[v.ID] = v
	}
	for i := range current.Sources {
		if v, ok := seedSources[current.Sources[i].ID]; ok {
			current.Sources[i] = v
		}
	}
	seedMethods := map[string]MethodCatalogEntry{}
	for _, v := range seed.MethodCatalog {
		seedMethods[v.ID] = v
	}
	for i := range current.MethodCatalog {
		if v, ok := seedMethods[current.MethodCatalog[i].ID]; ok {
			current.MethodCatalog[i] = v
		}
	}
	// RC681: remove the legacy duplicated container entry and collapse exact
	// caption duplicates left by older saved catalogues. Seed entries win, while
	// genuinely user-added methods with unique names are preserved.
	filteredMethods := make([]MethodCatalogEntry, 0, len(current.MethodCatalog))
	seenMethodNames := map[string]bool{}
	for _, method := range current.MethodCatalog {
		if method.ID == "TM-31" {
			continue
		}
		nameKey := normalizeMethodText(method.Name)
		if nameKey == "" || seenMethodNames[nameKey] {
			continue
		}
		seenMethodNames[nameKey] = true
		filteredMethods = append(filteredMethods, method)
	}
	current.MethodCatalog = filteredMethods
	seedRules := map[string]MethodRule{}
	for _, v := range seed.Rules {
		seedRules[v.ID] = v
	}
	for i := range current.Rules {
		if v, ok := seedRules[current.Rules[i].ID]; ok {
			current.Rules[i] = v
		}
	}
	if current.SchemaVersion < seed.SchemaVersion {
		current.SchemaVersion = seed.SchemaVersion
	}
	return current
}

// mergeRuleCatalog preserves the embedded baseline and lets verified additions
// override entries by stable IDs. This is important when a company profile is
// enriched with corporate standards or literature after deployment.
func mergeRuleCatalog(base, overlay RuleCatalog) RuleCatalog {
	out := base
	if overlay.SchemaVersion > out.SchemaVersion {
		out.SchemaVersion = overlay.SchemaVersion
	}
	src := map[string]RuleSource{}
	orderSrc := []string{}
	for _, s := range append(base.Sources, overlay.Sources...) {
		if _, ok := src[s.ID]; !ok {
			orderSrc = append(orderSrc, s.ID)
		}
		src[s.ID] = s
	}
	out.Sources = out.Sources[:0]
	for _, id := range orderSrc {
		out.Sources = append(out.Sources, src[id])
	}

	meth := map[string]MethodCatalogEntry{}
	orderMeth := []string{}
	for _, m := range append(base.MethodCatalog, overlay.MethodCatalog...) {
		if _, ok := meth[m.ID]; !ok {
			orderMeth = append(orderMeth, m.ID)
		}
		meth[m.ID] = m
	}
	out.MethodCatalog = out.MethodCatalog[:0]
	for _, id := range orderMeth {
		out.MethodCatalog = append(out.MethodCatalog, meth[id])
	}

	rules := map[string]MethodRule{}
	orderRules := []string{}
	for _, r := range append(base.Rules, overlay.Rules...) {
		if _, ok := rules[r.ID]; !ok {
			orderRules = append(orderRules, r.ID)
		}
		rules[r.ID] = r
	}
	out.Rules = out.Rules[:0]
	for _, id := range orderRules {
		out.Rules = append(out.Rules, rules[id])
	}
	return out
}

func ruleSource(id string) (RuleSource, bool) {
	for _, s := range ruleCatalog.Sources {
		if s.ID == id {
			return s, true
		}
	}
	return RuleSource{}, false
}

func normalizeMethodText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Avoid constructing a trie/replacer for every record in large cohorts.
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'ё':
			return 'е'
		case '/', '-', '(', ')', '+':
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func ruleMatchesTechnology(r MethodRule, t Technology) bool {
	rn := normalizeMethodText(r.Method)
	tn := normalizeMethodText(t.Name)
	if rn == tn || strings.Contains(rn, tn) || strings.Contains(tn, rn) {
		return true
	}
	// Explicit semantic bridges between the preliminary matrix and the current
	// Technology Module terminology. They are deliberately conservative.
	pairs := [][2]string{
		{"механический скребок", "механическая очистка нкт скребки"},
		{"автоматический скребок", "механическая очистка нкт скребки"},
		{"промывка горячей нефтью", "промывка теплоносителем гв гн пав"},
		{"нкт с покрытием", "защитные покрытия нкт и гно"},
		{"промывка кислотным составом", "удаление солей кислотой комплексоном"},
		{"фильтр сепаратор", "фильтр сепаратор мехпримесей"},
		{"промывка гно водой", "промывка нормализация забоя"},
	}
	for _, p := range pairs {
		if rn == p[0] && tn == p[1] {
			return true
		}
	}
	// Inhibitor/container wording depends on the complication family.
	lc := strings.ToLower(r.Complication)
	if strings.Contains(rn, "закачка ингибитора") || strings.Contains(rn, "ингибирование гидратов") {
		if strings.Contains(lc, "корроз") && strings.Contains(tn, "ингибитор корроз") {
			return true
		}
		if strings.Contains(lc, "соле") && strings.Contains(tn, "ингибитор соле") {
			return true
		}
		if strings.Contains(lc, "гидрат") && strings.Contains(tn, "ингибирование гидрат") {
			return true
		}
	}
	if strings.Contains(rn, "погружной контейнер") && strings.Contains(tn, "погружной контейнер") {
		if strings.Contains(lc, "корроз") && strings.Contains(tn, "трил к") {
			return true
		}
		if strings.Contains(lc, "аспо") && strings.Contains(tn, "трил п см") {
			return true
		}
	}
	return false
}

func rulesForTechnology(t Technology) []MethodRule {
	var out []MethodRule
	for _, r := range ruleCatalog.Rules {
		if ruleMatchesTechnology(r, t) {
			out = append(out, r)
		}
	}
	return out
}

func ruleSummaryForTechnology(t Technology) string {
	rr := rulesForTechnology(t)
	if len(rr) == 0 {
		return "Основание из перенесённой Excel-матрицы пока не сопоставлено. Метод сохранён в действующем справочнике и требует отдельной верификации критериев."
	}
	r := rr[0]
	src := []string{}
	for _, id := range r.SourceIDs {
		if s, ok := ruleSource(id); ok {
			src = append(src, s.Title+" ["+s.Status+"]")
		}
	}
	crit := "—"
	if len(r.Criteria) > 0 {
		crit = strings.Join(r.Criteria, "; ")
	}
	lim := "—"
	if len(r.Limitations) > 0 {
		lim = strings.Join(r.Limitations, "; ")
	}
	srcText := "—"
	if len(src) > 0 {
		srcText = strings.Join(src, "; ")
	}
	return fmt.Sprintf("Осложнение: %s. Назначение: %s.\r\nКритерии проверки: %s.\r\nОграничения: %s.\r\nСтатус основания: %s.\r\nИсточник: %s.", r.Complication, dash(r.Purpose), crit, lim, dash(r.CriterionStatus), srcText)
}

func technologyFromMethodEntry(m MethodCatalogEntry) Technology {
	group := "Требует классификации"
	n := normalizeMethodText(m.Name)
	switch {
	case strings.Contains(n, "корроз") || strings.Contains(n, "скрн") || strings.Contains(n, "трил к"):
		group = "К — коррозия"
	case strings.Contains(n, "соле") || strings.Contains(n, "комплексон") || strings.Contains(n, "рвк"):
		group = "С — соли"
	case strings.Contains(n, "аспо") || strings.Contains(n, "увр") || strings.Contains(n, "скреб") || strings.Contains(n, "теплонос") || strings.Contains(n, "нагреватель") || strings.Contains(n, "магнит") || strings.Contains(n, "трил п см"):
		group = "А — АСПО"
	case strings.Contains(n, "гидрат") || strings.Contains(n, "метанол"):
		group = "GD — гидраты"
	case strings.Contains(n, "газосеп") || strings.Contains(n, "дисперг") || strings.Contains(n, "мультифаз"):
		group = "G — газ"
	case strings.Contains(n, "мехприм") || strings.Contains(n, "износостой") || strings.Contains(n, "нормализация забоя"):
		group = "М — мехпримеси"
	case strings.Contains(n, "термостой") || strings.Contains(n, "термометр") || strings.Contains(n, "кожух пэд"):
		group = "T — температура / условия ГНО"
	}
	t := Technology{
		ID:            m.ID,
		Name:          m.Name,
		Group:         group,
		Applicability: "Предварительный технологический отбор по критериям объекта; окончательная применимость подтверждается действующими НД/МУ и ОЛИ/ОПИ/ОПР.",
		Constraints:   "Критерии и ограничения должны быть подтверждены для конкретного объекта; непроверенные пороги не закрепляются как норматив.",
		Evidence:      "Технологический модуль + карточка доказательной базы РОСА",
		Status:        "К проверке",
	}
	if rr := rulesForTechnology(t); len(rr) > 0 {
		r := rr[0]
		t.Applicability = strings.Join(r.Criteria, "; ")
		t.Constraints = strings.Join(r.Limitations, "; ")
		if strings.TrimSpace(t.Applicability) == "" {
			t.Applicability = "Проверка по фактическим данным объекта"
		}
		if strings.TrimSpace(t.Constraints) == "" {
			t.Constraints = "Ограничения уточняются по действующим документам и фактическим условиям"
		}
		t.Evidence = "Excel-матрица РОСА; требуется последующая привязка к подтверждённым корпоративным/литературным источникам"
	}
	return t
}

func mergeTechnologyCatalog(existing []Technology) []Technology {
	byName := map[string]bool{}
	out := append([]Technology(nil), existing...)
	for _, t := range existing {
		byName[normalizeMethodText(t.Name)] = true
	}
	for _, m := range ruleCatalog.MethodCatalog {
		if !byName[normalizeMethodText(m.Name)] {
			out = append(out, technologyFromMethodEntry(m))
			byName[normalizeMethodText(m.Name)] = true
		}
	}
	return out
}

// methodRulesForName resolves evidence rules for a method name stored in an
// immutable calculation snapshot. It intentionally does not depend on the
// current technology catalogue so historical versions remain explainable even
// after the catalogue is expanded.
func methodRulesForName(name string) []MethodRule {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	probe := Technology{Name: name}
	return rulesForTechnology(probe)
}
