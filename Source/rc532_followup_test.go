//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC532ExcelChartCarriesPointCachesAndVisibleTrend(t *testing.T) {
	cache := infographicNumberCacheXML([]float64{45292, 45293.5}, "dd.mm.yyyy")
	for _, want := range []string{`<c:numCache>`, `<c:ptCount val="2"/>`, `<c:pt idx="0"><c:v>45292</c:v></c:pt>`} {
		if !strings.Contains(cache, want) {
			t.Fatalf("chart cache missing %q: %s", want, cache)
		}
	}
	trend := infographicTrendlineXML(true, 0)
	if !strings.Contains(trend, `val="CB1B2B"`) || !strings.Contains(trend, `w="44450"`) || !strings.Contains(trend, `trendlineType val="linear"`) {
		t.Fatalf("trend line is not visibly styled: %s", trend)
	}
}

func TestRC532CalculationTextAndWorkbookUseTransparentStepsAndRealFormulas(t *testing.T) {
	values := map[string]string{"sample_temp": "4", "hydrate_temp": "9"}
	text := calculationBasisExample("gases", values)
	for _, want := range []string{"ШАГ 1", "Формула:", "Подстановка:", "4 − 9 = -5"} {
		if !strings.Contains(text, want) {
			t.Fatalf("transparent calculation missing %q: %s", want, text)
		}
	}
	xml, _ := calculationWorkbookFormulaRows("gases", values, "", 12)
	if !strings.Contains(xml, `<f>B13-C13</f><v>-5</v>`) || !strings.Contains(xml, "Excel-формула / результат") {
		t.Fatalf("workbook lost executable formula: %s", xml)
	}
}

func TestRC532AssessmentReconciliationIsExplicit(t *testing.T) {
	d := assessmentClassDirection{Name: "Коррозия", Prefix: "К"}
	if got := assessmentComparisonStatus("К1", "К2"); !strings.Contains(got, "РАЗНОГЛАСИЕ") {
		t.Fatalf("conflict is not explicit: %q", got)
	}
	advice := assessmentReconciliationAdvice(d, "К1", "К2", "высокая")
	if !strings.Contains(advice, "прямой количественный критерий") {
		t.Fatalf("high-confidence conflict advice is vague: %s", advice)
	}
	plan := assessmentVerificationPlan(d)
	for _, want := range []string{"скорость коррозии", "толщинометрия"} {
		if !strings.Contains(plan, want) {
			t.Fatalf("verification plan missing %q: %s", want, plan)
		}
	}
}

func TestRC532BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.532-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
