//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC640ImplementationOptions(t *testing.T) {
	field := methodImplementationField{Key: "basis", Suggested: "По воде", Value: "По воде"}
	options := methodImplementationFieldOptions(field)
	if len(options) < 4 || options[0] != "Не выбрано" {
		t.Fatalf("basis options=%v", options)
	}
	seen := map[string]bool{}
	for _, option := range options {
		if seen[option] {
			t.Fatalf("duplicate option %q", option)
		}
		seen[option] = true
	}
}

func TestRC640Version(t *testing.T) {
	if appBuildVersion != "7.3.11.640-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}

func TestRC640ChemicalSchemeDuplicateSignature(t *testing.T) {
	a := []methodImplementationField{{Key: "reagent", Value: " Ингибитор А "}, {Key: "rate", Value: "20 г/т"}}
	b := []methodImplementationField{{Key: "reagent", Value: "ингибитор а"}, {Key: "rate", Value: "20 Г/Т"}}
	c := []methodImplementationField{{Key: "reagent", Value: "ингибитор а"}, {Key: "rate", Value: "25 г/т"}}
	if methodChemicalSchemeSignature(a) != methodChemicalSchemeSignature(b) {
		t.Fatal("case and surrounding spaces must not create a duplicate scheme")
	}
	if methodChemicalSchemeSignature(a) == methodChemicalSchemeSignature(c) {
		t.Fatal("a different rate must remain a distinct chemical scheme")
	}
}

func TestRC640ChemicalSchemeCompatibility(t *testing.T) {
	a := []methodImplementationField{{Key: "reagent"}, {Key: "basis"}, {Key: "unit"}, {Key: "rate"}}
	b := []methodImplementationField{{Key: "reagent"}, {Key: "basis"}, {Key: "unit"}, {Key: "rate"}}
	c := []methodImplementationField{{Key: "reagent"}, {Key: "volume"}, {Key: "rate"}}
	if methodChemicalSchemeFieldKeys(a) != methodChemicalSchemeFieldKeys(b) {
		t.Fatal("equal field sets must be compatible")
	}
	if methodChemicalSchemeFieldKeys(a) == methodChemicalSchemeFieldKeys(c) {
		t.Fatal("different field sets must not be mixed")
	}
}

func TestRC640MeasurementUnitOptions(t *testing.T) {
	field := methodImplementationField{Key: "unit", Suggested: "гр/м³", Value: "гр/м³"}
	options := methodImplementationFieldOptions(field)
	want := map[string]bool{"гр/м³": false, "гр/тн": false, "тн": false, "м³": false}
	for _, option := range options {
		if _, ok := want[option]; ok {
			want[option] = true
		}
	}
	for option, found := range want {
		if !found {
			t.Fatalf("missing measurement unit %q in %v", option, options)
		}
	}
}

func TestRC640MRPUnitNormalization(t *testing.T) {
	for _, input := range []string{"149", "149 сут.", "149 сут..", "149 суток"} {
		if got := formatWholeDaysText(input); got != "149" {
			t.Fatalf("formatWholeDaysText(%q)=%q", input, got)
		}
	}
}

func TestRC640UnitsFollowDosageBasis(t *testing.T) {
	water := methodImplementationUnitOptionsForBasis("По воде")
	oil := methodImplementationUnitOptionsForBasis("По нефти")
	if len(water) != 2 || water[0] != "гр/м³" || water[1] != "м³" {
		t.Fatalf("water units=%v", water)
	}
	if len(oil) != 2 || oil[0] != "гр/тн" || oil[1] != "тн" {
		t.Fatalf("oil units=%v", oil)
	}
}

func TestRC640BasisContributionHasSinglePeriod(t *testing.T) {
	s := methodWorkflowSuggestion{Direction: "Соли", Classification: "К2"}
	a := MethodEffectAssessment{Applicability: 80, Reliability: 70, ReliabilityLevel: "Высокая", EffectQuantified: true, BasePct: 10}
	got := methodsBasisSummaryText(s, a, ReplicationAssessment{}, 1490)
	if strings.Contains(got, "сут..") {
		t.Fatalf("double period remains in basis summary: %q", got)
	}
	if !strings.Contains(got, "≈ +149 сут.") {
		t.Fatalf("expected canonical contribution punctuation, got %q", got)
	}
}
