//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func summaryGroup(t *testing.T, code string) LabGroupDef {
	t.Helper()
	group, ok := labGroupByCode(code)
	if !ok {
		t.Fatal(code)
	}
	return group
}

func TestSummaryDoesNotMakePartialOrInvalidDataGreen(t *testing.T) {
	for _, tc := range []struct {
		code, model string
		values      map[string]string
	}{
		{"corrosion", "", map[string]string{"ph": "7"}},
		{"corrosion", "", map[string]string{"ph": "16"}},
		{"solids", "", map[string]string{"size_d50": "80", "size_max": "20"}},
		{"aspo", "", map[string]string{"paraffins": "40", "resins": "40", "asphaltenes": "40"}},
		{"microbiology", "", map[string]string{"srb": "обнаружено"}},
		{"gases", "", map[string]string{"hydrate_temp": "10"}},
	} {
		summary := laboratoryGroupSummary(summaryGroup(t, tc.code), tc.values, tc.model)
		if summary.Level != 0 || !strings.HasPrefix(summary.Message, "Оценить пока нельзя") {
			t.Fatalf("%s got %+v", tc.code, summary)
		}
	}
}

func TestSummaryKeepsDetectedSignalsWithIncompleteAndInvalidData(t *testing.T) {
	group := summaryGroup(t, "corrosion")
	for _, values := range []map[string]string{{"h2s": "2"}, {"ph": "16", "h2s": "2"}, {"o2": "1"}} {
		summary := laboratoryGroupSummary(group, values, "")
		if summary.Level < 2 || !strings.HasPrefix(summary.Message, "Есть признаки") {
			t.Fatalf("lost detected signal: %+v", summary)
		}
		if _, invalid := values["ph"]; invalid && !strings.Contains(summary.Message, "Ошибочные значения исключены") {
			t.Fatal(summary.Message)
		}
	}
}

func TestSummaryKnownNegativeRequiresCoverage(t *testing.T) {
	group := summaryGroup(t, "aspo")
	complete := map[string]string{"paraffins": "1", "asphaltenes": "0.1", "pour_point": "0", "viscosity": "10"}
	if got := laboratoryGroupSummary(group, complete, ""); got.Level != 1 {
		t.Fatal(got)
	}
	delete(complete, "viscosity")
	if got := laboratoryGroupSummary(group, complete, ""); got.Level != 0 {
		t.Fatal(got)
	}
}

func TestSummaryUsesSaturationInsteadOfIonThresholdWords(t *testing.T) {
	values := map[string]string{"ca": "40.078", "co3": "60.01", "mg": "0", "na_k": "0", "so4": "0", "hco3": "0", "cl": "0", "mineralization": "1", "ph": "7", "sample_temp": "25"}
	got := laboratoryGroupSummary(summaryGroup(t, "scale"), values, "ideal")
	if got.Level < 2 || !strings.Contains(got.Message, "Кальцит") || !strings.Contains(got.Message, "индекс насыщения") {
		t.Fatal(got)
	}
}

func TestSummaryRejectsUnavailableAndFailedScaleRoutes(t *testing.T) {
	group := summaryGroup(t, "scale")
	for _, tc := range []struct {
		model  string
		values map[string]string
	}{
		{"oddo", map[string]string{"ca": "40.078", "co3": "60.01"}},
		{"multicomplex", map[string]string{"sample_temp": "40", "ca_total": "40.078", "tic": "12.0111"}},
		{"multicomplex", map[string]string{"sample_temp": "25", "ph": "8", "ca_total": "40.078", "mg_total": "24.305", "tic": "12.0111", "so4_total": "96.06", "na": "10000", "k": "0", "cl": "10000"}},
	} {
		got := laboratoryGroupSummary(group, tc.values, tc.model)
		if got.Level != 0 {
			t.Fatalf("failed/unavailable route classified as evaluated: %+v", got)
		}
	}
}

func TestSummaryNoLabDataFromTechRegimeOnly(t *testing.T) {
	got := laboratoryGroupSummary(summaryGroup(t, "corrosion"), map[string]string{"watercut": "80"}, "")
	if got.HasData || got.Level != 0 || got.State != "нет данных" {
		t.Fatal(got)
	}
}

func TestCompactOverallUsesOnlyExplicitResults(t *testing.T) {
	text := "РАЗДЕЛ: КОРРОЗИЯ\nИтог для технолога: Есть признаки возможного осложнения.\nРАЗДЕЛ: СОЛИ\nИтог для технолога: Оценить пока нельзя.\nОБЩИЙ ВЫВОД\nЧто проверить: не ищите слова требует внимания\nИтог для технолога: По внесённым данным признаки не выявлены."
	short := laboratoryOverallShortText(text)
	if !strings.Contains(short, "учтено 2 из 6") || !strings.Contains(short, "результат — 1; оценить пока нельзя — 1") {
		t.Fatal(short)
	}
}

func TestOldSummaryWithoutExplicitResultRemainsUnknown(t *testing.T) {
	text := "РАЗДЕЛ: КОРРОЗИЯ\nОбычное старое заключение без статуса."
	if short := laboratoryOverallShortText(text); !strings.Contains(short, "оценить пока нельзя — 1") {
		t.Fatal(short)
	}
}

func TestPracticalExplanationUsesTheSameSummaryAsCard(t *testing.T) {
	group := summaryGroup(t, "aspo")
	summary := laboratoryGroupSummary(group, map[string]string{"paraffins": "1"}, "")
	text := laboratoryPracticalExplanation(group, summary)
	if !strings.Contains(text, summary.Message) || strings.Contains(text, "явных признаков осложнения сейчас не выявлено") {
		t.Fatal(text)
	}
}

func TestScaleSummaryDistinguishesBorderlineNegativeAndWrongTemperature(t *testing.T) {
	group := summaryGroup(t, "scale")
	values := map[string]string{"ca": "1", "co3": "0.000001", "mg": "0", "na_k": "0", "so4": "0", "hco3": "0", "cl": "1.77", "mineralization": "1", "ph": "7", "sample_temp": "25"}
	if got := laboratoryGroupSummary(group, values, "ideal"); got.Level != 1 {
		t.Fatal(got)
	}
	values["sample_temp"] = "75"
	if got := laboratoryGroupSummary(group, values, "ideal"); got.Level != 0 {
		t.Fatalf("wrong-temperature screening made green: %+v", got)
	}
	values["sample_temp"] = "25"
	values["ca"] = "40.078"
	values["cl"] = "70.9"
	values["co3"] = fmt.Sprintf("%.12g", math.Pow(10, -8.48)/.001*60.01*1000)
	if got := laboratoryGroupSummary(group, values, "ideal"); got.Level < 2 || !strings.HasPrefix(got.Message, "Пограничный результат") {
		t.Fatal(got)
	}
}

func TestHistorySummaryUsesActualInputsInsteadOfOldText(t *testing.T) {
	record := LabRecord{GroupCode: "corrosion", GroupName: "Коррозия", Values: map[string]string{"ph": "7"}, Conclusion: "Все прекрасно, выходов за границы не выявлено."}
	if got := laboratoryRecordShortConclusion(record); !strings.Contains(got, "Оценить пока нельзя") {
		t.Fatal(got)
	}
}

func TestFullReportUsesOneSummaryAtTopAndInPracticalExplanation(t *testing.T) {
	group := summaryGroup(t, "corrosion")
	for _, values := range []map[string]string{{"ph": "7"}, {"h2s": "2"}, {"ph": "16"}} {
		summary := laboratoryGroupSummary(group, values, "davies")
		report := buildLaboratoryConclusionWithScaleModel(group, values, "davies")
		if !strings.Contains(report, summary.Text()) {
			t.Fatal("explicit summary missing")
		}
		start := strings.Index(report, "ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА")
		if start < 0 || !strings.Contains(report[start:], summary.Message) {
			t.Fatal("practical explanation differs from summary")
		}
	}
}

func TestGasCombinationKeepsExistingCombinedAssessment(t *testing.T) {
	group := summaryGroup(t, "corrosion")
	values := map[string]string{"o2": "0.01", "co2": "1"}
	existing := assessCorrosionInfluence(values)
	got := laboratoryGroupSummary(group, values, "")
	if existing.Combined.rank < 3 || got.Level < 3 {
		t.Fatalf("lost combined influence: %+v", got)
	}
}

func TestAutoWatercutDoesNotCountAsLaboratoryData(t *testing.T) {
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		if count := laboratoryLabValueCount(group, map[string]string{"watercut": "80"}); count != 0 {
			t.Fatalf("%s counts tech context as lab data: %d", group.Code, count)
		}
	}
}

func TestCalculatedSulinDoesNotCountAsLaboratoryData(t *testing.T) {
	group, ok := labGroupByCode("scale")
	if !ok {
		t.Fatal("scale group not found")
	}
	values := map[string]string{"water_type_sulin": "Хлоридно-кальциевый"}
	if count := laboratoryLabValueCount(group, values); count != 0 {
		t.Fatalf("calculated Sulin water type counted as laboratory input: %d", count)
	}
	result := laboratoryGroupSummary(group, values, "davies")
	if result.HasData {
		t.Fatalf("calculated-only Sulin result must not mark scale group as having laboratory input: %+v", result)
	}
}
