//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestCorrosionInfluenceHasFourExplainableBlocks(t *testing.T) {
	a := assessCorrosionInfluence(map[string]string{})
	text := formatCorrosionInfluenceAssessment(a)
	for _, want := range []string{"КИСЛОРОД O₂", "ДИОКСИД УГЛЕРОДА CO₂", "СЕРОВОДОРОД H₂S", "СОВМЕСТНОЕ ВЛИЯНИЕ O₂ + CO₂ + H₂S", "Недостаёт:", "Достоверность:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

func TestH2SCrackingAssessmentIsSeparateAndConservative(t *testing.T) {
	text := formatCorrosionInfluenceAssessment(assessCorrosionInfluence(map[string]string{"h2s": "2", "equipment_material": "НКТ, сталь"}))
	for _, want := range []string{"СКРИНИНГ ВЕРОЯТНОСТИ РАСТРЕСКИВАНИЯ В H₂S-СРЕДЕ", "требуется материаловедческая проверка", "не присваивает автоматически зону пригодности материала", "твёрдость основного металла"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

func TestCorrosionBaselineAndAdditionalMarks(t *testing.T) {
	for _, key := range []string{"ph", "chlorides", "o2", "co2", "h2s", "corrosion_rate"} {
		if !strings.HasPrefix(laboratoryFieldPrefix("corrosion", key), "Б") {
			t.Fatalf("baseline field %s has no Б mark: %q", key, laboratoryFieldPrefix("corrosion", key))
		}
	}
	if got := laboratoryFieldPrefix("corrosion", "watercut"); got != "Т/О · " {
		t.Fatalf("watercut must be marked as tech-regime common context, got %q", got)
	}
	for _, key := range []string{"plate_mass_loss", "plate_area", "flow_velocity", "inhibitor_residual", "corrosion_rate_with"} {
		if !strings.HasPrefix(laboratoryFieldPrefix("corrosion", key), "Д") {
			t.Fatalf("additional field %s has no Д mark: %q", key, laboratoryFieldPrefix("corrosion", key))
		}
	}
}

func TestMineralizationIsBaselineEverywhereItIsShared(t *testing.T) {
	for _, groupCode := range []string{"corrosion", "scale", "gases"} {
		if !laboratoryBaselineKeys(groupCode)["mineralization"] {
			t.Fatalf("mineralization must be baseline in %s", groupCode)
		}
		if got := laboratoryFieldPrefix(groupCode, "mineralization"); got != "Б/О · " {
			t.Fatalf("mineralization must be baseline/shared in %s, got %q", groupCode, got)
		}
	}
}

func TestWatercutIsSeededFromTechRegimeWithoutOverwriting(t *testing.T) {
	groups := map[string]map[string]string{"corrosion": {"ph": "6,5"}, "scale": {"watercut": "41"}}
	if !seedLaboratoryWatercutFromTech(groups, "73,2") {
		t.Fatal("tech-regime watercut was not seeded")
	}
	if groups["corrosion"]["watercut"] != "73,2" || groups["aspo"]["watercut"] != "73,2" {
		t.Fatalf("watercut was not propagated to laboratory groups: %#v", groups)
	}
	if groups["scale"]["watercut"] != "41" {
		t.Fatalf("existing pass value was overwritten: %#v", groups["scale"])
	}
}

func TestCorrosionPagesHaveExplicitSubgroupNames(t *testing.T) {
	wants := []string{"Базовая оценка коррозии", "Газовые и эксплуатационные условия", "Вероятность H₂S-растрескивания", "Коррозионные пластинки и защита", "Толщинометрия и планирование"}
	for page, want := range wants {
		if got := laboratoryFieldsPageTitle("corrosion", page); got != want {
			t.Fatalf("page %d: got %q want %q", page, got, want)
		}
	}
}

func TestCorrosionInfluenceDoesNotDoubleCountGasRepresentations(t *testing.T) {
	a := assessCorrosionInfluence(map[string]string{
		"pco2": "0,3", "co2": "700", "co2_gas": "12", "ph2s": "0,001", "h2s": "5",
	})
	if len(a.CarbonDioxide.Basis) != 3 || !strings.Contains(a.CarbonDioxide.Basis[0], "основной учтённый") {
		t.Fatalf("CO2 representations are not explained: %#v", a.CarbonDioxide.Basis)
	}
	if strings.Count(strings.Join(a.CarbonDioxide.Basis, " "), "основной учтённый") != 1 {
		t.Fatalf("more than one CO2 representation was counted: %#v", a.CarbonDioxide.Basis)
	}
	if a.Combined.rank != 4 {
		t.Fatalf("two active gases must raise only the maximum level by one step, got %d", a.Combined.rank)
	}
}

func TestCorrosionInfluenceUsesCommonContextOnce(t *testing.T) {
	values := map[string]string{"o2": "0,2", "co2": "100", "chlorides": "15000", "watercut": "90", "ph": "6,1", "flow_velocity": "3"}
	text := formatCorrosionInfluenceAssessment(assessCorrosionInfluence(values))
	if strings.Count(text, "хлориды = 15000 мг/л") != 1 || strings.Count(text, "обводнённость = 90 %") != 1 {
		t.Fatalf("common context was duplicated:\n%s", text)
	}
	if !strings.Contains(text, "учтены один раз") {
		t.Fatalf("anti-double-counting rule is not visible:\n%s", text)
	}
}

func TestCorrosionCouponAndInhibitorEvidence(t *testing.T) {
	a := assessCorrosionInfluence(map[string]string{
		"plate_initial_mass": "20", "plate_final_mass": "19,5", "plate_exposure_days": "30",
		"plate_area": "10", "plate_material_density": "7,85",
		"corrosion_rate_without": "0,5", "corrosion_rate_with": "0,1", "corrosion_allowance": "3",
	})
	text := strings.Join(a.ObservedEvidence, " ")
	for _, want := range []string{"потеря массы пластинки = 0.5 г за 30 сут", "средняя скорость по потере массы", "эффект ингибиторной защиты по парному испытанию = 80 %", "припуск на коррозию = 3 мм"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestCorrosionMassValidationRejectsImpossiblePair(t *testing.T) {
	group, _ := labGroupByCode("corrosion")
	clean, _, errors := validateLaboratoryGroup(group, map[string]string{"plate_initial_mass": "10", "plate_final_mass": "11"})
	if len(errors) != 1 || clean["plate_initial_mass"] != "" || clean["plate_final_mass"] != "" {
		t.Fatalf("impossible mass pair was not excluded: clean=%#v errors=%#v", clean, errors)
	}
}

func TestCorrosionEnvironmentCompactRC162(t *testing.T) {
	text := corrosionEnvironmentCompactText(map[string]string{"co2": "100", "ph": "6", "sample_temp": "40"})
	for _, want := range []string{"КОРРОЗИОННАЯ СРЕДА", "Диоксид углерода CO₂", "Общие условия учитываются один раз"} {
		if !strings.Contains(text, want) {
			t.Fatalf("compact corrosion card misses %q: %s", want, text)
		}
	}
}
