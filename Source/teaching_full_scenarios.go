//go:build windows

package main

import "fmt"

type teachingFullScenario struct {
	Ordinal                                 int
	Name                                    string
	ClassByDirection                        map[string]string
	Methods                                 []string
	ForecastLow, ForecastBase, ForecastHigh float64
}

func teachingFullScenarioForObject(key string) (teachingFullScenario, bool) {
	if _, current := trainingWellForObject(key); current {
		return teachingFullScenario{}, false
	}
	n, ok := teachingWorkflowOrdinal(key)
	if !ok {
		return teachingFullScenario{}, false
	}
	switch n {
	case 1:
		return teachingFullScenario{n, "Солеотложения", map[string]string{"Солеотложения": "С2"}, []string{"Ингибитор солеотложений (дозирование / закачка)", "Промывка от солеотложений", "НКТ с покрытием"}, 1080, 1180, 1270}, true
	case 58:
		return teachingFullScenario{n, "Коррозия", map[string]string{"Коррозия": "К2"}, []string{"Ингибитор коррозии (дозирование / закачка)"}, 970, 1010, 1060}, true
	case 90:
		return teachingFullScenario{n, "АСПО и высокая вязкость", map[string]string{"АСПО": "А2", "Высокая вязкость": "В2"}, []string{"Растворитель АСПО", "Деэмульгатор"}, 970, 1008, 1065}, true
	}
	return teachingFullScenario{}, false
}

func teachingCompleteLabDraft(pass *LabPassRecord) {
	if pass == nil {
		return
	}
	if _, current := trainingWellForObject(pass.ObjectKey); current {
		return
	}
	scenario, ok := teachingFullScenarioForObject(pass.ObjectKey)
	if !ok {
		return
	}
	pass.Source = fmt.Sprintf("УЧЕБНЫЙ ПРОТОКОЛ РОСА-%03d · полный синтетический лабораторный паспорт", scenario.Ordinal)
	pass.ScaleModel = "davies"
	if pass.Groups == nil {
		pass.Groups = map[string]map[string]string{}
	}
	// RC734: every approved teaching report carries a genuinely complete laboratory
	// passport instead of one profile-only group. Values are deterministic synthetic
	// fixtures used only to exercise the full report/export path.
	groups := map[string]map[string]string{
		"scale": {
			"ca": "520", "mg": "138", "na_k": "18400", "so4": "760", "ba": "2.4", "sr": "18", "co3": "24", "hco3": "610", "cl": "29600",
			"alkalinity": "500", "mineralization": "51.4", "ph": "7.3", "sample_temp": "48", "ph_measurement_temp": "25", "sample_pressure": "8.2",
			"solute_mass": "51.4", "brine_density": "1042", "formation_temp": "62", "formation_pressure": "18.5", "pump_temp": "51", "pump_pressure": "9.8",
			"wellhead_temp": "31", "wellhead_pressure": "1.7", "profile_temp_min": "31", "profile_pressure_min": "1.7", "profile_temp_max": "62", "profile_pressure_max": "18.5",
			"hardness_total": "38", "external_saturation_index": "0.7", "deposit_caco3": "42", "deposit_caso4": "8", "deposit_baso4": "2",
			"inhibitor_trial_brand": "Учебный ингибитор S-01", "inhibitor_trial_target": "CaCO3 / карбонатные соли", "inhibitor_trial_dose": "35",
			"inhibitor_trial_temp": "60", "inhibitor_trial_duration_h": "24", "inhibitor_trial_efficiency": "86", "inhibitor_trial_protocol": "УЧ-ЛАБ-SCALE-01",
			"inhibitor_trial_result": "Осадкообразование снижено в учебной пробе; производственная дозировка не назначается автоматически",
		},
		"corrosion": {
			"ph": "6.4", "chlorides": "28400", "co2": "145", "h2s": "6", "o2": "0.12", "corrosion_rate": "0.28", "mineralization": "51.4", "iron_total": "8.6", "orp": "-115",
			"sample_temp": "42", "co2_gas": "3.2", "h2s_gas": "0.018", "pco2": "0.21", "ph2s": "0.0012", "gas_absolute_pressure": "6.6", "bottomhole_pressure": "18.5",
			"mixture_density": "930", "flow_velocity": "1.4", "equipment_material": "НКТ сталь 32Г2, учебная карточка", "material_yield_strength": "620",
			"material_hardness_base": "22 HRC", "material_hardness_weld": "245 HV", "working_stresses": "рабочие; остаточные не подтверждены",
			"ssc_test": "учебный контроль: без разрушения", "hic_test": "учебный контроль: трещины не выявлены", "hydrogen_permeation": "низкая по учебному протоколу",
			"material_zone_conclusion": "допустимо при ингибиторной защите и мониторинге", "fe2": "6.9", "fe3": "1.7", "mercaptans_oil": "4.2", "hydrogen_dissolved": "0.08",
			"fes_water": "7.5", "inhibitor_residual": "18", "wall_thickness": "5.7", "minimum_wall_thickness": "4.8", "corrosion_allowance": "0.9", "pit_depth": "0.6",
			"corrosion_planning_years": "1", "inhibitor_type_water": "плёнкообразующий", "inhibitor_brand_water": "Учебный CI-02",
			"corrosion_rate_without": "0.72", "corrosion_rate_with": "0.16", "plate_mass_loss": "0.42", "plate_exposure_days": "30", "plate_area": "100",
			"inhibitor_trial_brand": "Учебный CI-02", "inhibitor_trial_target": "CO2/H2S-коррозия", "inhibitor_trial_dose": "40", "inhibitor_trial_temp": "50",
			"inhibitor_trial_duration_h": "24", "inhibitor_trial_efficiency": "78", "inhibitor_trial_protocol": "УЧ-ЛАБ-CORR-01",
			"inhibitor_trial_result": "Снижение гравиметрической скорости коррозии в учебной постановке",
		},
		"aspo": {
			"paraffins": "6.8", "resins": "10.5", "asphaltenes": "3.2", "pour_point": "22", "wax_onset": "35", "viscosity": "96", "density": "902", "water": "62",
			"mechanical": "0.18", "solids": "18", "planning_horizon": "90", "oils_fraction": "79.5", "light_fraction": "18", "viscosity_index": "72",
			"kinematic_viscosity_20": "128", "kinematic_viscosity_40": "61", "dynamic_viscosity_40": "55", "flow_type": "эмульсионный, условно ньютоновский в диапазоне испытания",
			"oil_in_water": "115", "water_separation_test": "без деэмульгатора 42%/30 мин; с учебным составом 82%/30 мин",
		},
		"solids": {
			"kvch": "184", "suspended": "96", "sand": "0.34", "fes": "14", "scale_fraction": "31", "organic_fraction": "18", "size_d50": "68", "size_max": "310",
			"iron": "11.2", "turbidity": "165", "carbonates": "28", "sulfates": "9", "chloride_salts": "4", "corrosion_products": "22",
			"suspended_filter": "88", "suspended_membrane": "104", "particle_distribution": "d10=18; d50=68; d90=210 мкм",
			"filter_elemental": "Fe 32%; Ca 18%; Si 14%; S 9%; прочее", "deposit_iron_oxides": "22", "deposit_insoluble_mineral": "31", "deposit_mohs": "5.5",
			"deposit_elemental": "учебный XRF: Fe/Ca/Si/S",
		},
		"microbiology": {
			"srb": "10^2 кл/мл", "apb": "10^2 кл/мл", "gmb": "3.4e3", "atp": "185", "biofilm": "слабая", "iron_bacteria": "следы", "sulfide": "1.8", "ph": "6.8",
			"sample_temp": "34", "incubation": "28 °C / 7 суток", "qpcr_srb": "2.1e2", "qpcr_apb": "1.3e2", "orp": "-105", "srb_activity": "18",
			"iron_bacteria_activity": "12", "other_microorganisms": "не выявлены в значимом титре", "other_microorganisms_activity": "0", "ammonium": "4.8", "nitrate": "1.2",
		},
		"gases": {
			"co2": "145", "h2s": "6", "o2": "0.12", "gas_factor": "68", "water_content": "0.74", "hydrate_temp": "9", "sample_pressure": "6.6", "sample_temp": "34",
			"methane": "89", "co2_gas": "3.2", "h2s_gas": "0.018", "pco2": "0.21", "ph2s": "0.0012", "ph": "6.8", "mineralization": "51.4", "orp": "-105",
			"iron_total": "8.6", "bottomhole_pressure": "18.5", "mixture_density": "930", "formation_temp": "62", "formation_pressure": "18.5", "pump_temp": "51", "pump_pressure": "9.8",
			"wellhead_temp": "31", "wellhead_pressure": "1.7", "profile_temp_min": "31", "profile_pressure_min": "1.7", "profile_temp_max": "62", "profile_pressure_max": "18.5",
			"h2s_oil": "2.2", "h2s_gas_mass": "255", "mercaptans_gas": "34", "hydrogen_dissolved": "0.08",
		},
	}
	// Make the profile complication visibly dominant while keeping all other
	// laboratory directions populated for a realistic multi-section report.
	switch scenario.Ordinal {
	case 1:
		groups["scale"]["external_saturation_index"] = "1.3"
		groups["scale"]["deposit_caco3"] = "68"
		groups["scale"]["inhibitor_trial_efficiency"] = "91"
	case 58:
		groups["corrosion"]["corrosion_rate"] = "0.72"
		groups["corrosion"]["h2s"] = "14"
		groups["corrosion"]["co2"] = "185"
		groups["corrosion"]["inhibitor_residual"] = "9"
		groups["corrosion"]["pit_depth"] = "1.1"
	case 90:
		groups["aspo"]["paraffins"] = "8.4"
		groups["aspo"]["resins"] = "12.1"
		groups["aspo"]["asphaltenes"] = "4.0"
		groups["aspo"]["viscosity"] = "146"
		groups["aspo"]["water"] = "68"
		groups["aspo"]["water_separation_test"] = "без деэмульгатора 31%/30 мин; с учебным составом 88%/30 мин"
	}
	for code, values := range groups {
		if pass.Groups[code] == nil {
			pass.Groups[code] = map[string]string{}
		}
		for key, value := range values {
			pass.Groups[code][key] = value
		}
	}
	if pass.SharedValues == nil {
		pass.SharedValues = map[string]string{}
	}
	liquid, watercut, oil := teachingWorkflowRegimeValues(scenario.Ordinal)
	pass.SharedValues["watercut"] = watercut
	pass.SharedValues["liquid_rate_m3_day"] = liquid
	pass.SharedValues["oil_rate_t_day"] = oil
	pass.SharedValues["sampling_point"] = "устье + контрольная точка до/после насоса (учебная схема)"
	if pass.ExtraValues == nil {
		pass.ExtraValues = map[string]string{}
	}
	pass.ExtraValues["qc_note"] = "Все значения синтетические; предназначены для проверки подробного отчёта, формул и трассируемости."
	pass.ExtraValues["sample_chain"] = "Отбор → маркировка → лаборатория → проверка технологом → утверждение снимка"
	pass.Comment = fmt.Sprintf("Полный синтетический лабораторный паспорт учебного объекта %03d. Профильный риск: %s. Не использовать как производственные данные.", scenario.Ordinal, scenario.Name)
}
