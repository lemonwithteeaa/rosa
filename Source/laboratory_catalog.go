//go:build windows

package main

import (
	"fmt"
	"strings"
)

type LabFieldPassport struct {
	Marker           string
	EvidenceType     string
	Meaning          string
	ExpectedSource   string
	AllowedInference string
	ForbiddenClaim   string
	SampleContext    string
}

func labKeyIn(key string, keys ...string) bool {
	for _, candidate := range keys {
		if key == candidate {
			return true
		}
	}
	return false
}

func laboratoryGroupInference(groupCode string) (string, string) {
	switch groupCode {
	case "corrosion":
		return "использовать как один из факторов предварительной оценки коррозионной активности, защитных мер и необходимости дополнительной проверки", "по одному показателю нельзя подтверждать механизм коррозии, пригодность материала или назначать окончательный класс К"
	case "scale":
		return "использовать для предварительной оценки предпосылок солеотложения совместно с полным составом воды и профилем температуры/давления", "нельзя подтверждать вид и интенсивность солеотложения без проверки состава, условий процесса и специализированного расчёта или исследования"
	case "aspo":
		return "использовать для предварительной оценки склонности к АСПО и вязкостно-температурных ограничений совместно с режимом объекта", "нельзя по одному значению подтверждать образование АСПО, его место или требуемый метод обработки"
	case "solids":
		return "использовать для предварительной оценки количества, размера и возможной природы твёрдой фазы", "нельзя отождествлять мехпримеси с конкретным источником или механизмом без анализа состава и промысловой проверки"
	case "microbiology":
		return "использовать как основание для микробиологической скрининговой гипотезы с учётом метода отбора, хранения и анализа", "нельзя подтверждать микробиологическую коррозию или эффективность биоцида без профильного исследования и динамики"
	case "gases":
		return "использовать для предварительной оценки влияния газовой фазы и условий гидратообразования совместно с фактическими температурой и давлением", "нельзя подтверждать гидратообразование, коррозионный механизм или режимный дефект только по одному газовому показателю"
	default:
		return "использовать только как часть предварительной комплексной оценки", "нельзя использовать как самостоятельный окончательный диагноз"
	}
}

func laboratoryFieldPassport(group LabGroupDef, field LabFieldDef) LabFieldPassport {
	marker := strings.TrimSuffix(laboratoryFieldPrefixForModel(group.Code, field.Key, labScaleModelCode), " · ")
	allowed, forbidden := laboratoryGroupInference(group.Code)
	p := LabFieldPassport{
		Marker: marker, EvidenceType: "прямое измерение или результат испытания по протоколу",
		Meaning:          "Фактическое значение показателя «" + field.Label + "» в указанных единицах.",
		ExpectedSource:   "лабораторный протокол с методом анализа и идентификацией пробы",
		AllowedInference: allowed, ForbiddenClaim: forbidden,
		SampleContext: "объект и точка отбора; дата/время; температура и давление пробы; режим объекта; метод анализа",
	}
	if strings.Contains(marker, "Т") {
		p.EvidenceType = "технологический контекст"
		p.ExpectedSource = "Техрежим; значение автоматически фиксируется в снимке лабораторной диагностики"
		p.Meaning = "Эксплуатационный контекст «" + field.Label + "», а не отдельный лабораторный результат."
	} else if strings.Contains(marker, "О") {
		p.ExpectedSource += "; значение общее для связанных вкладок текущей диагностики"
	}

	if laboratoryIsMixingField(field.Key) {
		p.SampleContext = "точка и дата отбора каждой воды, температура 25 °C, анализ состава и плотности той же водной пробы; доля объёма до смешения"
		p.ForbiddenClaim = "нельзя усреднять pH, подтверждать массу и скорость выпадения осадка или конечный объём и плотность смеси"
		if field.Key == "mix_ph" {
			p.Meaning = "Измеренный pH фактической смеси в выбранном соотношении при 25 °C. Не pH первой или второй воды и не их среднее."
		}
		if field.Key == "mix_percent" {
			p.Meaning = "Доля объёма второй воды до смешения: 25 % означает 25 литров воды 2 и 75 литров воды 1. Для равновесия программа отдельно пересчитает её в долю по массе воды-растворителя."
		}
		if strings.HasPrefix(field.Key, "mix2_") {
			p.ExpectedSource = "отдельный лабораторный протокол второй воды; концентрации общих компонентов и единицы как указано в поле"
		}
	}
	key := field.Key
	switch {
	case labKeyIn(key, "ph", "orp"):
		p.Meaning = "Характеристика кислотно-щелочных или окислительно-восстановительных условий среды; интерпретируется с температурой, составом и методом измерения."
	case labKeyIn(key, "ca", "mg", "na_k", "so4", "co3", "hco3", "cl", "chlorides", "mineralization", "iron_total", "fe2", "fe3", "iron", "sulfide"):
		p.Meaning = "Компонент состава водной фазы; единицы и способ подготовки пробы должны соответствовать протоколу."
	case labKeyIn(key, "sample_temp", "formation_temp", "pump_temp", "wellhead_temp", "profile_temp_min", "profile_temp_max", "pour_point", "wax_onset", "hydrate_temp"):
		p.Meaning = "Температурная характеристика пробы, процесса или фазового перехода; точка измерения обязательна для интерпретации."
	case labKeyIn(key, "sample_pressure", "formation_pressure", "pump_pressure", "wellhead_pressure", "profile_pressure_min", "profile_pressure_max", "bottomhole_pressure", "gas_absolute_pressure", "pco2", "ph2s"):
		p.Meaning = "Характеристика давления процесса или компонента газовой фазы; требуется различать абсолютное, избыточное и парциальное давление."
	case labKeyIn(key, "co2", "h2s", "o2", "co2_gas", "h2s_gas", "methane", "gas_factor", "water_content"):
		p.Meaning = "Показатель растворённой или газовой фазы; форма представления и база пересчёта должны быть указаны в источнике."
	case key == "water":
		p.Meaning = "Содержание воды в лабораторной пробе нефти/АСПО; не подменяется обводнённостью продукции из Техрежима."
		p.ExpectedSource = "лабораторный протокол с описанием пробы и метода определения воды"
	case labKeyIn(key, "paraffins", "resins", "asphaltenes", "viscosity", "density", "water", "mechanical", "solids"):
		p.Meaning = "Физико-химическая характеристика нефти или пробы АСПО, влияющая на фазовое состояние, текучесть и склонность к отложениям."
	case labKeyIn(key, "kvch", "suspended", "sand", "fes", "scale_fraction", "organic_fraction", "size_d50", "size_max", "turbidity", "carbonates", "sulfates", "chloride_salts", "corrosion_products"):
		p.Meaning = "Характеристика количества, размера либо состава твёрдой фазы/отложений; результат зависит от отбора, фильтрации и подготовки пробы."
	case labKeyIn(key, "srb", "apb", "gmb", "atp", "biofilm", "iron_bacteria", "qpcr_srb", "qpcr_apb", "incubation"):
		p.Meaning = "Микробиологический результат, чувствительный к точке отбора, анаэробности, хранению, сроку доставки и методу анализа."
	case labKeyIn(key, "ssc_test", "hic_test", "hydrogen_permeation", "material_zone_conclusion", "material_hardness_base", "material_hardness_weld", "material_yield_strength", "working_stresses", "equipment_material"):
		p.Meaning = "Характеристика материала или результат профильного испытания для оценки поведения в H₂S-содержащей среде."
		p.EvidenceType = "испытание/заключение материаловеда"
		p.ExpectedSource = "протокол испытания материала с указанием стандарта, образца, зоны и условий"
	case labKeyIn(key, "corrosion_rate", "corrosion_rate_without", "corrosion_rate_with", "plate_mass_loss", "plate_exposure_days", "plate_area", "plate_initial_mass", "plate_final_mass", "plate_material_density", "plate_comment"):
		p.Meaning = "Результат коррозионного мониторинга или исходная величина расчёта по контрольной пластинке; условия экспозиции обязательны."
	case labKeyIn(key, "wall_thickness", "minimum_wall_thickness", "corrosion_allowance", "pit_depth", "pit_period_years", "corrosion_planning_years"):
		p.Meaning = "Данные толщинометрии и планирования контроля; точка измерения, метод, погрешность и исходная толщина должны быть известны."
	case labKeyIn(key, "planning_horizon"):
		p.EvidenceType = "плановый параметр"
		p.Meaning = "Горизонт, на котором рассматривается предварительный прогноз; это не лабораторное измерение."
		p.ExpectedSource = "задание на оценку или производственный план"
	case labKeyIn(key, "flow_velocity", "mixture_density"):
		p.EvidenceType = "технологическое измерение или расчётный индикатор"
		p.Meaning = "Параметр потока, используемый как контекст лабораторной интерпретации; источник расчёта или измерения должен быть указан."
	}
	return p
}

func laboratoryFieldPassportText(group LabGroupDef, field LabFieldDef) string {
	p := laboratoryFieldPassport(group, field)
	return fmt.Sprintf("%s · %s\r\nЕдиница: %s\r\nТип доказательства: %s.\r\nФизический смысл: %s\r\nОжидаемый источник: %s.\r\nДопустимый предварительный вывод: %s.\r\nНельзя утверждать: %s.\r\nПаспорт пробы/контекст: %s.", p.Marker, field.Label, field.Unit, p.EvidenceType, p.Meaning, p.ExpectedSource, p.AllowedInference, p.ForbiddenClaim, p.SampleContext)
}

func validateLaboratoryFieldCatalog() []string {
	var issues []string
	seen := map[string]bool{}
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		for _, field := range group.Fields {
			id := group.Code + "/" + field.Key
			if seen[id] {
				issues = append(issues, "дублируется поле "+id)
			}
			seen[id] = true
			p := laboratoryFieldPassport(group, field)
			if strings.TrimSpace(field.Label) == "" || strings.TrimSpace(field.Unit) == "" || strings.TrimSpace(p.Marker) == "" || strings.TrimSpace(p.EvidenceType) == "" || strings.TrimSpace(p.Meaning) == "" || strings.TrimSpace(p.ExpectedSource) == "" || strings.TrimSpace(p.AllowedInference) == "" || strings.TrimSpace(p.ForbiddenClaim) == "" || strings.TrimSpace(p.SampleContext) == "" {
				issues = append(issues, "неполный паспорт поля "+id)
			}
		}
	}
	return issues
}
