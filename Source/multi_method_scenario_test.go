//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestNormalizeMethodListDeduplicates(t *testing.T) {
	got := normalizeMethodList([]string{" A ", "a", "B", ""})
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
func TestCombinedScenarioAggregatesPerMethodCosts(t *testing.T) {
	c := TechnicalScenarioContract{ObjectKey: "OBJ-X", BaseMethod: "Ингибитор солеотложений", MRPModelVersion: "m1", MRPBaseDays: 300, Alternatives: []MethodAlternative{{Method: "НКТ с покрытием", Status: "проверить"}}}
	setCombinedMethods(c.ObjectKey, []string{c.BaseMethod, "НКТ с покрытием"})
	saveScenarioCostDraft(c.ObjectKey, c.BaseMethod, 120000, 0)
	saveScenarioCostDraft(c.ObjectKey, "НКТ с покрытием", 0, 500000)
	x := combinedTechnicalScenario(c)
	if len(x.Methods) != 2 || x.TotalAnnualOPEXRub != 120000 || x.TotalOneTimeMTRRub != 500000 {
		t.Fatalf("bad aggregate: %+v", x)
	}
}
func TestCombinedScenarioIDOrderIndependent(t *testing.T) {
	c := TechnicalScenarioContract{ObjectKey: "OBJ-Y", BaseMethod: "A", MRPModelVersion: "m1", Alternatives: []MethodAlternative{{Method: "B"}}}
	setCombinedMethods(c.ObjectKey, []string{"A", "B"})
	a := combinedTechnicalScenario(c).ScenarioID
	setCombinedMethods(c.ObjectKey, []string{"B", "A"})
	b := combinedTechnicalScenario(c).ScenarioID
	if a != b {
		t.Fatalf("ids differ %s %s", a, b)
	}
}
func TestCombinedEconomicContractKeepsBreakdown(t *testing.T) {
	c := TechnicalScenarioContract{ContractVersion: technicalScenarioContractVersion, ObjectKey: "OBJ-Z", BaseMethod: "Ингибитор", MRPModelVersion: "m1", Alternatives: []MethodAlternative{{Method: "Фильтр"}}}
	setCombinedMethods(c.ObjectKey, []string{"Ингибитор", "Фильтр"})
	saveScenarioCostDraft(c.ObjectKey, "Ингибитор", 100, 20)
	saveScenarioCostDraft(c.ObjectKey, "Фильтр", 30, 40)
	e := economicContractFromCombined(c)
	if len(e.Methods) != 2 || len(e.MethodCosts) != 2 || e.SpecificAnnualOPEXRub != 130 || e.SpecificOneTimeMTRRub != 60 {
		t.Fatalf("bad contract %+v", e)
	}
	if !strings.Contains(e.Method, "+") {
		t.Fatalf("combined label missing: %s", e.Method)
	}
}
func TestRemoveCombinedMethodNeverLeavesEmpty(t *testing.T) {
	c := TechnicalScenarioContract{ObjectKey: "OBJ-R", BaseMethod: "A"}
	setCombinedMethods(c.ObjectKey, []string{"A"})
	if _, ok := removeCombinedMethod(c); ok {
		t.Fatal("must keep anchor")
	}
	if len(selectedCombinedMethods(c)) != 1 {
		t.Fatal("empty")
	}
}
