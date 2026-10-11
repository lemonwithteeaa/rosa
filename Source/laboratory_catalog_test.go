//go:build windows

package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLaboratoryFieldCatalogIsComplete(t *testing.T) {
	if issues := validateLaboratoryFieldCatalog(); len(issues) != 0 {
		t.Fatalf("неполные паспорта лабораторных полей: %v", issues)
	}
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		for _, field := range group.Fields {
			text := laboratoryFieldPassportText(group, field)
			for _, required := range []string{
				"Единица:", "Тип доказательства:", "Физический смысл:",
				"Ожидаемый источник:", "Допустимый предварительный вывод:",
				"Нельзя утверждать:", "Паспорт пробы/контекст:",
			} {
				if !strings.Contains(text, required) {
					t.Fatalf("%s/%s: в подсказке нет %q: %s", group.Code, field.Key, required, text)
				}
			}
			if strings.Contains(strings.ToLower(text), "базовый минимум") {
				t.Fatalf("%s/%s: сохранена запрещённая формулировка: %s", group.Code, field.Key, text)
			}
		}
	}
}

func TestLaboratoryDisplayLabelsStayCompact(t *testing.T) {
	for _, group := range labGroups {
		for _, field := range group.Fields {
			label := laboratoryFieldDisplayLabel(field)
			if utf8.RuneCountInString(label) > 30 {
				t.Fatalf("%s/%s: экранная подпись слишком длинная (%d): %q", group.Code, field.Key, utf8.RuneCountInString(label), label)
			}
		}
	}
	for _, test := range []struct{ key, want string }{
		{"hydrogen_permeation", "Водородопроницаемость"},
		{"material_zone_conclusion", "Заключение по материалу (H₂S)"},
	} {
		if got := laboratoryFieldDisplayLabel(LabFieldDef{Key: test.key}); got != test.want {
			t.Fatalf("%s: %q вместо %q", test.key, got, test.want)
		}
	}
}

func TestLaboratoryFieldPassportEvidenceTypes(t *testing.T) {
	tests := []struct {
		group, key, evidence string
	}{
		{"corrosion", "ssc_test", "испытание/заключение материаловеда"},
		{"aspo", "planning_horizon", "плановый параметр"},
		{"corrosion", "watercut", "технологический контекст"},
	}
	for _, test := range tests {
		group, ok := labGroupByCode(test.group)
		if !ok {
			t.Fatalf("нет группы %s", test.group)
		}
		found := false
		for _, field := range group.Fields {
			if field.Key == test.key {
				found = true
				if got := laboratoryFieldPassport(group, field).EvidenceType; got != test.evidence {
					t.Fatalf("%s/%s: тип %q, ожидался %q", test.group, test.key, got, test.evidence)
				}
			}
		}
		if !found {
			t.Fatalf("нет поля %s/%s", test.group, test.key)
		}
	}
}

func TestLaboratoryBottomGeometryDoesNotClip(t *testing.T) {
	for _, height := range []int32{768, 900, 984, 1080} {
		for _, expanded := range []bool{false, true} {
			geometry := calculateLaboratoryBottomGeometry(height, expanded)
			if geometry.HistoryY+geometry.HistoryH > height-24 {
				t.Fatalf("h=%d expanded=%t: история выходит за нижнюю границу: %#v", height, expanded, geometry)
			}
			if geometry.HistoryH < 0 {
				t.Fatalf("h=%d expanded=%t: отрицательная высота истории: %#v", height, expanded, geometry)
			}
			if expanded && geometry.ConclusionY+geometry.ConclusionH > geometry.CommentY {
				t.Fatalf("h=%d: заключение перекрывает комментарий: %#v", height, geometry)
			}
		}
	}
}

func TestKZHSpecificAnalysesRemainOptional(t *testing.T) {
	selected := map[string][]string{
		"corrosion":    {"mercaptans_oil", "hydrogen_dissolved", "fes_water", "inhibitor_amine_water"},
		"scale":        {"hardness_total", "silicon", "external_saturation_index", "deposit_baso4"},
		"aspo":         {"oils_fraction", "kinematic_viscosity_20", "water_separation_test"},
		"solids":       {"suspended_membrane", "particle_distribution", "deposit_mohs"},
		"microbiology": {"srb_activity", "other_microorganisms", "ammonium"},
		"gases":        {"h2s_oil", "h2s_gas_mass", "mercaptans_gas"},
	}
	for groupCode, keys := range selected {
		group, ok := labGroupByCode(groupCode)
		if !ok {
			t.Fatalf("нет группы %s", groupCode)
		}
		available := map[string]bool{}
		for _, field := range group.Fields {
			available[field.Key] = true
		}
		for _, key := range keys {
			if !available[key] {
				t.Fatalf("нет поля %s/%s", groupCode, key)
			}
			if laboratoryBaselineKeys(groupCode)[key] {
				t.Fatalf("%s/%s ошибочно стало обязательным", groupCode, key)
			}
			if got := laboratoryFieldMarkerCode(groupCode, key); got != "Д" {
				t.Fatalf("%s/%s: маячок %q, ожидался Д", groupCode, key, got)
			}
		}
	}
}

func TestCorrosionGravimetricMonitoringIsLastSeparatePage(t *testing.T) {
	group, ok := labGroupByCode("corrosion")
	if !ok {
		t.Fatal("нет раздела коррозии")
	}
	want := []string{
		"corrosion_rate_without", "corrosion_rate_with", "corrosion_rate_mass",
		"plate_mass_loss", "plate_exposure_days", "plate_area", "plate_initial_mass",
		"plate_final_mass", "plate_material_density", "plate_comment",
	}
	if len(group.Fields) < len(want) || len(group.Fields)%10 != 0 {
		t.Fatalf("гравиметрическая страница должна быть отдельной полной страницей: всего %d полей", len(group.Fields))
	}
	last := group.Fields[len(group.Fields)-len(want):]
	for i, key := range want {
		if last[i].Key != key {
			t.Fatalf("последняя страница, позиция %d: %s вместо %s", i, last[i].Key, key)
		}
	}
	page := len(group.Fields)/10 - 1
	if got := laboratoryFieldsPageTitle("corrosion", page); got != "Для оценки гравиметрического анализа (коррозионный мониторинг)" {
		t.Fatalf("неверный заголовок последней страницы: %q", got)
	}
}
