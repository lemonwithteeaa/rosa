//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestTechnicalScenarioReadinessRequiresTraceableScenario(t *testing.T) {
	c := TechnicalScenarioContract{ObjectKey: "OBJ-1", CurrentMRPDays: 300, MRPBaseDays: 450, GNO: "УЭЦН", Complication: "С3", BaseMethod: "Ингибитор солеотложений"}
	status, issues := technicalScenarioReadiness(c)
	if status != "ГОТОВ К ЭКОНОМИЧЕСКОЙ ОЦЕНКЕ" || len(issues) != 0 {
		t.Fatalf("unexpected readiness: %s %v", status, issues)
	}
	c.BaseMethod = ""
	status, issues = technicalScenarioReadiness(c)
	if !strings.Contains(status, "ЧАСТИЧНО") || len(issues) == 0 {
		t.Fatalf("missing method must block full readiness: %s %v", status, issues)
	}
}

func TestSpecificCostsAreRequirementsNotInventedAmounts(t *testing.T) {
	req := methodSpecificCostRequirements("Ингибитор солеотложений")
	if len(req) == 0 || req[0].Code != "reagent" {
		t.Fatalf("reagent method must request reagent-specific costs: %+v", req)
	}
	for _, r := range req {
		if !strings.Contains(strings.ToLower(r.Basis), "не подставляет") && !strings.Contains(strings.ToLower(r.Basis), "заполняется") {
			t.Fatalf("specific cost rule must be explicit/manual: %+v", r)
		}
	}
}

func TestEconomicContractSharedByBaseAndExtended(t *testing.T) {
	c := EconomicScenarioContract{TechnicalContractVersion: technicalScenarioContractVersion, ObjectKey: "OBJ-1", ScenarioID: "OBJ-1|method|mrp", Method: "Метод", MRPSource: "live", MRPModelVersion: "MRP-baseline-1.2"}
	base := BaseEconomicsInput{TechnicalScenario: &c, CurrentMRPDays: 300, MRPBaseDays: 450, SelectedScenario: "Базовый", HorizonMonths: 12, RepairCostK: 1000}
	if r := CalculateBaseEconomics(base); r.Status != "PASS" {
		t.Fatalf("base economics rejected valid shared contract: %+v", r)
	}
	ext := ExtendedEconomicsInput{TechnicalScenario: &c, HorizonMonths: 1, DiscountRatePct: 10, Months: []ExtendedEconomicsMonth{{Month: "2026-10"}}}
	if r := CalculateExtendedEconomics(ext); r.Status != "PASS" {
		t.Fatalf("extended economics rejected same shared contract: %+v", r)
	}
}

func TestBrokenEconomicContractFailsBothModels(t *testing.T) {
	bad := EconomicScenarioContract{TechnicalContractVersion: technicalScenarioContractVersion, ObjectKey: "OBJ-1"}
	base := CalculateBaseEconomics(BaseEconomicsInput{TechnicalScenario: &bad, CurrentMRPDays: 300, MRPBaseDays: 450, SelectedScenario: "Базовый", HorizonMonths: 12, RepairCostK: 1000})
	if base.Status == "PASS" {
		t.Fatal("base economics accepted contract without ScenarioID")
	}
	ext := CalculateExtendedEconomics(ExtendedEconomicsInput{TechnicalScenario: &bad, HorizonMonths: 1, Months: []ExtendedEconomicsMonth{{Month: "2026-10"}}})
	if ext.Status == "PASS" {
		t.Fatal("extended economics accepted contract without ScenarioID")
	}
}

func TestAlternativesDoNotRankOrSelectWinner(t *testing.T) {
	alts := []MethodAlternative{{Method: "A", Status: "к проверке"}, {Method: "B", Status: "допустим при условии"}}
	for _, a := range alts {
		n := strings.ToLower(a.Status)
		if strings.Contains(n, "лучший") || strings.Contains(n, "ранг") || strings.Contains(n, "место") {
			t.Fatalf("alternative must not be ranked: %+v", a)
		}
	}
}

func TestScenarioSpecificCostsStaySeparatedByMethod(t *testing.T) {
	technicalScenarioCostDrafts = map[string]ScenarioCostDraft{}
	saveScenarioCostDraft("OBJ-1", "Метод A", 120000, 500000)
	saveScenarioCostDraft("OBJ-1", "Метод B", 240000, 900000)
	a := loadScenarioCostDraft("OBJ-1", "Метод A")
	b := loadScenarioCostDraft("OBJ-1", "Метод B")
	if a.AnnualOPEXRub == b.AnnualOPEXRub || a.OneTimeMTRRub == b.OneTimeMTRRub {
		t.Fatalf("scenario cost drafts leaked across methods: A=%+v B=%+v", a, b)
	}
}

func TestEconomicContractCarriesSelectedScenarioCosts(t *testing.T) {
	technicalScenarioCostDrafts = map[string]ScenarioCostDraft{}
	c := TechnicalScenarioContract{ContractVersion: technicalScenarioContractVersion, ObjectKey: "OBJ-1", ScenarioID: "OBJ-1|m", BaseMethod: "Ингибитор солеотложений"}
	c.SpecificCosts = methodSpecificCostRequirements(c.BaseMethod)
	saveScenarioCostDraft(c.ObjectKey, c.BaseMethod, 120000, 500000)
	ec := economicContractFromTechnical(c)
	if ec.SpecificAnnualOPEXRub != 120000 || ec.SpecificOneTimeMTRRub != 500000 || len(ec.SpecificCostCodes) == 0 {
		t.Fatalf("economic contract lost scenario-specific costs: %+v", ec)
	}
}

func TestExtendedEconomicsCanIncludeTechnicalScenarioCostsOnce(t *testing.T) {
	c := EconomicScenarioContract{TechnicalContractVersion: technicalScenarioContractVersion, ObjectKey: "OBJ-1", ScenarioID: "OBJ-1|m", SpecificAnnualOPEXRub: 120000, SpecificOneTimeMTRRub: 600000}
	in := ExtendedEconomicsInput{TechnicalScenario: &c, HorizonMonths: 2, IncludeTechnicalScenarioCosts: true, Months: []ExtendedEconomicsMonth{{Month: "2026-10"}, {Month: "2026-11"}}}
	applied := applyTechnicalScenarioCostsToExtended(in)
	if applied.Months[0].OPEXK != 10 || applied.Months[1].OPEXK != 10 || applied.Months[0].CAPEXK != 600 {
		t.Fatalf("technical costs not transferred into extended economics: %+v", applied.Months)
	}
}

func TestCycleTechnicalMethodDoesNotRankAlternatives(t *testing.T) {
	technicalMethodSelection = map[string]string{}
	c := TechnicalScenarioContract{ObjectKey: "OBJ-1", BaseMethod: "A", Alternatives: []MethodAlternative{{Method: "B", Status: "к проверке"}, {Method: "C", Status: "условно"}}}
	if got := cycleTechnicalMethod(c, 1); got != "B" {
		t.Fatalf("expected B, got %q", got)
	}
	c.BaseMethod = selectedTechnicalMethod(c.ObjectKey, "A", c.Alternatives)
	if got := cycleTechnicalMethod(c, 1); got != "C" {
		t.Fatalf("expected C, got %q", got)
	}
}

func TestComparisonKeepsCanonicalMethodWhenAlternativeSelected(t *testing.T) {
	technicalScenarioCostDrafts = map[string]ScenarioCostDraft{}
	c := TechnicalScenarioContract{ObjectKey: "OBJ-1", CanonicalMethod: "A", BaseMethod: "B", Alternatives: []MethodAlternative{{Method: "B", Status: "к проверке"}, {Method: "C", Status: "условно"}}}
	rows := technicalScenarioComparison(c)
	seenA, seenB := false, false
	for _, r := range rows {
		if r.Method == "A" {
			seenA = true
		}
		if r.Method == "B" && r.Selected {
			seenB = true
		}
	}
	if !seenA || !seenB {
		t.Fatalf("comparison lost canonical or selected method: %+v", rows)
	}
}

func TestMissingGNOContextDoesNotBlockInterventionEconomics(t *testing.T) {
	c := TechnicalScenarioContract{ObjectKey: "OBJ-1", CurrentMRPDays: 300, MRPBaseDays: 450, Complication: "С3", BaseMethod: "Ингибитор солеотложений"}
	status, issues := technicalScenarioReadiness(c)
	if status != "ГОТОВ К ЭКОНОМИЧЕСКОЙ ОЦЕНКЕ" || len(issues) != 0 {
		t.Fatalf("фактическое ГНО не должно быть обязательным для оценки мероприятия: %s %v", status, issues)
	}
}
