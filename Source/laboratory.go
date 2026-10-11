//go:build windows

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

var laboratoryExportBusy bool
var laboratoryExportResultMu sync.Mutex
var laboratoryExportResultPath string
var laboratoryExportResultErr error

const (
	ID_LAB = 114

	ID_LAB_TAB_CORROSION = 230
	ID_LAB_TAB_SCALE     = 231
	ID_LAB_TAB_ASPO      = 232
	ID_LAB_TAB_SOLIDS    = 233
	ID_LAB_TAB_MICRO     = 234
	ID_LAB_TAB_GASES     = 235
	ID_LAB_TAB_OVERALL   = 292
	ID_LAB_SAVE          = 236
	ID_LAB_USE_SELECTED  = 237
	ID_LAB_FILTER        = 238
	ID_LAB_IMPORT        = 239
	// Keep the main-window command outside the telemetry command range 240..250.
	// A duplicate ID made this button open the telemetry file picker instead of
	// building the laboratory conclusion.
	ID_LAB_ANALYZE            = 286
	ID_LAB_MAP_PREVIEW        = 288
	ID_LAB_MAP_ACCEPT         = 289
	ID_LAB_MAP_CANCEL         = 290
	ID_LAB_EXPORT             = 291
	ID_LAB_TOGGLE             = 293
	ID_LAB_FIELDS_PREV        = 294
	ID_LAB_FIELDS_NEXT        = 295
	ID_LAB_OBJECT_EDIT        = 296
	ID_LAB_CONTINUE           = 297
	ID_LAB_COPY_RESULT        = 5250
	ID_LAB_DELETE_ROW         = 5251
	ID_LAB_CLEAR_ALL          = 5252
	ID_LAB_CLEAR_CURRENT      = 5257
	ID_LAB_DOWNLOAD_TEMPLATE  = 5253
	ID_LAB_IMPORT_CONTINUE    = 5254
	ID_LAB_IMPORT_RETURN      = 5255
	ID_LAB_SOURCE_EDIT        = 5256
	ID_LAB_FIELD_BASE         = 5000
	ID_LAB_UNIT_BASE          = 5100
	ID_LAB_INFLUENCE          = 5200
	ID_LAB_SCALE_MODEL        = 5210
	ID_LAB_LEGEND             = 5220
	ID_LAB_BADGE_BASE         = 5230
	ID_LAB_STIFF_CHART        = 5260
	ID_LAB_MIXING_CHART       = 5261
	ID_LAB_SCOPE_COMBO        = 5262
	ID_LAB_TREND_CHART        = 5263
	ID_LAB_TREND_METRIC       = 5264
	ID_LAB_SCOPE_OBJECTS_EDIT = 5265
	ID_LAB_RISK_FILTER        = 5266
	ID_LAB_GROUP_NAME         = 5267
	ID_LAB_GROUP_COMBO        = 5268
	ID_LAB_GROUP_SAVE         = 5269
	ID_LAB_TREND_SORT         = 5270
	ID_LAB_DETAIL_OBJECT      = 5271
	ID_LAB_OPEN_INFOGRAPHIC   = 5272
	ID_LAB_COMPLEX_ANALYSIS   = 5273
	ID_LAB_CALCULATION_BASIS  = 5274

	TTS_ALWAYSTIP       = 0x01
	TTS_NOPREFIX        = 0x02
	TTF_IDISHWND        = 0x0001
	TTF_SUBCLASS        = 0x0010
	TTM_SETMAXTIPWIDTH  = 0x0400 + 24
	TTM_ADDTOOLW        = 0x0400 + 50
	TTM_UPDATETIPTEXTW  = 0x0400 + 57
	OFN_OVERWRITEPROMPT = 0x00000002
)

type TOOLINFOW struct {
	CbSize     uint32
	UFlags     uint32
	Hwnd       syscall.Handle
	UId        uintptr
	Rect       RECT
	Hinst      syscall.Handle
	LpszText   *uint16
	LParam     uintptr
	LpReserved uintptr
}

type LabFieldDef struct {
	Key   string
	Label string
	Unit  string
}

type LabGroupDef struct {
	ID     int
	Code   string
	Name   string
	Fields []LabFieldDef
}

type LabRecord struct {
	ID         string            `json:"id"`
	ObjectKey  string            `json:"object_key"`
	Date       string            `json:"date"`
	GroupCode  string            `json:"group_code"`
	GroupName  string            `json:"group_name"`
	Source     string            `json:"source,omitempty"`
	Values     map[string]string `json:"values"`
	Units      map[string]string `json:"units,omitempty"`
	ScaleModel string            `json:"scale_model,omitempty"`
	Comment    string            `json:"comment,omitempty"`
	Conclusion string            `json:"conclusion,omitempty"`
	CreatedAt  string            `json:"created_at"`
	ImportedAt string            `json:"imported_at,omitempty"`
	Deleted    bool              `json:"deleted,omitempty"`
}

type laboratoryHistoryEntry struct {
	Kind    string
	Pass    LabPassRecord
	Record  LabRecord
	Version string
	Date    string
	Created string
}

var labGroups = []LabGroupDef{
	{ID: ID_LAB_TAB_CORROSION, Code: "corrosion", Name: "Коррозия", Fields: []LabFieldDef{
		// Подгруппа 1: базовая оценка коррозионной активности.
		{Key: "ph", Label: "Водородный показатель (pH)", Unit: "ед."},
		{Key: "chlorides", Label: "Хлориды Cl⁻", Unit: "мг/л"},
		{Key: "co2", Label: "Растворённый CO₂", Unit: "мг/л"},
		{Key: "h2s", Label: "Растворённый H₂S", Unit: "мг/л"},
		{Key: "o2", Label: "Растворённый O₂", Unit: "мг/л"},
		{Key: "corrosion_rate", Label: "Скорость коррозии", Unit: "мм/год"},
		{Key: "mineralization", Label: "Общая минерализация", Unit: "г/л"},
		{Key: "iron_total", Label: "Железо общее", Unit: "мг/л"},
		{Key: "orp", Label: "Окислительно-восстановительный потенциал (ОВП)", Unit: "мВ"},
		// Подгруппа 2: газовые и эксплуатационные условия.
		{Key: "sample_temp", Label: "Температура пробы", Unit: "°C"},
		{Key: "co2_gas", Label: "CO₂ в газовой фазе", Unit: "% об."},
		{Key: "h2s_gas", Label: "H₂S в газовой фазе", Unit: "% об."},
		{Key: "pco2", Label: "Парциальное давление (Р) CO₂", Unit: "МПа"},
		{Key: "ph2s", Label: "Парциальное давление (Р) H₂S", Unit: "МПа"},
		{Key: "gas_absolute_pressure", Label: "Полное абсолютное давление (Р)", Unit: "МПа abs"},
		{Key: "bottomhole_pressure", Label: "Забойное давление (Р)", Unit: "МПа"},
		{Key: "mixture_density", Label: "Плотность смеси", Unit: "кг/м³"},
		{Key: "flow_velocity", Label: "Скорость потока", Unit: "м/с"},
		{Key: "equipment_material", Label: "Марка и состояние материала", Unit: "текст"},
		// Подгруппа 3: вероятность H₂S-растрескивания.
		{Key: "material_yield_strength", Label: "Предел текучести материала", Unit: "МПа"},
		{Key: "material_hardness_base", Label: "Твёрдость основного металла", Unit: "HRC/HV/HB"},
		{Key: "material_hardness_weld", Label: "Твёрдость сварного шва / зоны термического влияния (ЗТВ)", Unit: "HV/HRC"},
		{Key: "working_stresses", Label: "Рабочие и остаточные напряжения", Unit: "текст"},
		{Key: "ssc_test", Label: "Испытание на сульфидное коррозионное растрескивание под напряжением (SSC)", Unit: "текст"},
		{Key: "hic_test", Label: "Испытание на водородоиндуцированное растрескивание (HIC)", Unit: "текст"},
		{Key: "hydrogen_permeation", Label: "Водородопроницаемость / подвижный водород", Unit: "по протоколу"},
		{Key: "material_zone_conclusion", Label: "Заключение материаловеда по H₂S-среде", Unit: "текст"},
		{Key: "fe2", Label: "Fe²⁺", Unit: "мг/л"},
		{Key: "fe3", Label: "Fe³⁺", Unit: "мг/л"},
		// Подгруппа 4: специальные анализы коррозионной среды и контроль ингибитора.
		{Key: "mercaptans_oil", Label: "Меркаптаны R–SH в нефти", Unit: "мг/л"},
		{Key: "mercaptans_gas", Label: "Меркаптаны R–SH в газе", Unit: "мг/м³"},
		{Key: "hydrogen_dissolved", Label: "Растворённый водород H₂", Unit: "мг/л"},
		{Key: "fes_water", Label: "FeS в водной фазе", Unit: "мг/л"},
		{Key: "fes_oil", Label: "FeS в нефтяной фазе", Unit: "мг/л"},
		{Key: "inhibitor_residual", Label: "Остаточная концентрация ингибитора", Unit: "мг/л"},
		{Key: "inhibitor_amine_water", Label: "Остаток ингибитора по аминной группе в воде", Unit: "мг/л"},
		{Key: "inhibitor_phosphorus_water", Label: "Остаток ингибитора по фосфору в воде", Unit: "мг/л"},
		{Key: "inhibitor_amine_ngvj", Label: "Остаток ингибитора по аминной группе в нефтегазоводяной жидкости (НГВЖ)", Unit: "мг/л"},
		{Key: "inhibitor_phosphorus_ngvj", Label: "Остаток ингибитора по фосфору в нефтегазоводяной жидкости (НГВЖ)", Unit: "мг/л"},
		// Подгруппа 5: толщинометрия, планирование и паспорт применённого ингибитора.
		{Key: "wall_thickness", Label: "Фактическая толщина стенки", Unit: "мм"},
		{Key: "minimum_wall_thickness", Label: "Минимально допустимая толщина", Unit: "мм"},
		{Key: "corrosion_allowance", Label: "Припуск на коррозию", Unit: "мм"},
		{Key: "pit_depth", Label: "Глубина максимальной язвы", Unit: "мм"},
		{Key: "pit_period_years", Label: "Период развития язвы", Unit: "лет"},
		{Key: "corrosion_planning_years", Label: "Плановый горизонт контроля", Unit: "лет"},
		{Key: "inhibitor_type_water", Label: "Тип ингибитора в водной пробе", Unit: "текст"},
		{Key: "inhibitor_brand_water", Label: "Марка ингибитора в водной пробе", Unit: "текст"},
		{Key: "inhibitor_type_ngvj", Label: "Тип ингибитора в пробе нефтегазоводяной жидкости (НГВЖ)", Unit: "текст"},
		{Key: "inhibitor_brand_ngvj", Label: "Марка ингибитора в пробе нефтегазоводяной жидкости (НГВЖ)", Unit: "текст"},
		// Подгруппа 6: только гравиметрический анализ и контрольные пластины.
		{Key: "corrosion_rate_without", Label: "Скорость коррозии без ингибитора", Unit: "мм/год"},
		{Key: "corrosion_rate_with", Label: "Скорость коррозии с ингибитором", Unit: "мм/год"},
		{Key: "corrosion_rate_mass", Label: "Скорость коррозии гравиметрическая", Unit: "г/(м²·ч)"},
		{Key: "plate_mass_loss", Label: "Потеря массы контрольной пластинки", Unit: "г"},
		{Key: "plate_exposure_days", Label: "Срок экспозиции контрольной пластинки", Unit: "сут"},
		{Key: "plate_area", Label: "Площадь рабочей поверхности пластинки", Unit: "см²"},
		{Key: "plate_initial_mass", Label: "Начальная масса пластинки", Unit: "г"},
		{Key: "plate_final_mass", Label: "Конечная масса пластинки", Unit: "г"},
		{Key: "plate_material_density", Label: "Плотность материала пластинки", Unit: "г/см³"},
		{Key: "plate_comment", Label: "Условия и очистка пластинки", Unit: "текст"},
		// Подгруппа 7: фактическое лабораторное испытание ингибитора. Поля
		// сохраняют измеренные точки; РОСА не интерполирует «оптимальную» дозу.
		{Key: "inhibitor_trial_brand", Label: "Марка ингибитора в испытании", Unit: "текст"},
		{Key: "inhibitor_trial_target", Label: "Цель / механизм испытания ингибитора", Unit: "текст"},
		{Key: "inhibitor_trial_dose", Label: "Доза ингибитора в испытании", Unit: "мг/л"},
		{Key: "inhibitor_trial_temp", Label: "Температура испытания ингибитора", Unit: "°C"},
		{Key: "inhibitor_trial_duration_h", Label: "Продолжительность испытания ингибитора", Unit: "ч"},
		{Key: "inhibitor_trial_efficiency", Label: "Фактическая эффективность ингибитора", Unit: "%"},
		{Key: "inhibitor_trial_protocol", Label: "Протокол испытания ингибитора", Unit: "текст"},
		{Key: "inhibitor_trial_result", Label: "Фактический результат испытания ингибитора", Unit: "текст"},
	}},
	{ID: ID_LAB_TAB_SCALE, Code: "scale", Name: "Солеотложения", Fields: []LabFieldDef{
		{Key: "ca", Label: "Ca²⁺", Unit: "мг/л"},
		{Key: "mg", Label: "Mg²⁺", Unit: "мг/л"},
		{Key: "na_k", Label: "Na⁺ + K⁺", Unit: "мг/л"},
		{Key: "so4", Label: "SO₄²⁻", Unit: "мг/л"},
		{Key: "ba", Label: "Ba²⁺ (для барита)", Unit: "мг/л"},
		{Key: "sr", Label: "Sr²⁺ (для целестина)", Unit: "мг/л"},
		{Key: "co3", Label: "CO₃²⁻", Unit: "мг/л"},
		{Key: "hco3", Label: "HCO₃⁻", Unit: "мг/л"},
		{Key: "cl", Label: "Cl⁻", Unit: "мг/л"},
		{Key: "alkalinity", Label: "Щёлочность общая (как CaCO₃)", Unit: "мг/л"},
		{Key: "mg_total", Label: "Общий магний (как Mg)", Unit: "мг/л"},
		{Key: "so4_total", Label: "Общий сульфат (как SO₄)", Unit: "мг/л"},
		{Key: "ca_total", Label: "Общий кальций (как Ca)", Unit: "мг/л"},
		{Key: "tic", Label: "Неорганический углерод (как C)", Unit: "мг/л"},
		{Key: "na", Label: "Na⁺ (отдельный анализ)", Unit: "мг/л"},
		{Key: "k", Label: "K⁺ (отдельный анализ)", Unit: "мг/л"},
		{Key: "mineralization", Label: "Общая минерализация", Unit: "г/л"},
		{Key: "ph", Label: "Водородный показатель (pH)", Unit: "ед."},
		{Key: "sample_temp", Label: "Температура пробы", Unit: "°C"},
		{Key: "ph_measurement_temp", Label: "Температура измерения pH", Unit: "°C"},
		{Key: "sample_pressure", Label: "Давление пробы при измерении", Unit: "МПа"},
		{Key: "solute_mass", Label: "Полная масса растворённых веществ", Unit: "г/л"},
		{Key: "brine_density", Label: "Плотность водной фазы", Unit: "кг/м³"},
		{Key: "bottomhole_pressure", Label: "Забойное давление (Р)", Unit: "МПа"},
		{Key: "mixture_density", Label: "Плотность смеси", Unit: "кг/м³"},
		{Key: "formation_temp", Label: "Температура пласта", Unit: "°C"},
		{Key: "formation_pressure", Label: "Давление (Р) пласта", Unit: "МПа"},
		{Key: "pump_temp", Label: "Температура на приёме насоса", Unit: "°C"},
		{Key: "pump_pressure", Label: "Давление (Р) на приёме насоса", Unit: "МПа"},
		{Key: "wellhead_temp", Label: "Температура на устье", Unit: "°C"},
		{Key: "wellhead_pressure", Label: "Давление (Р) на устье", Unit: "МПа"},
		{Key: "profile_temp_min", Label: "Минимальная температура профиля", Unit: "°C"},
		{Key: "profile_pressure_min", Label: "Давление (Р) при минимальной температуре", Unit: "МПа"},
		{Key: "profile_temp_max", Label: "Максимальная температура профиля", Unit: "°C"},
		{Key: "profile_pressure_max", Label: "Давление (Р) при максимальной температуре", Unit: "МПа"},
		{Key: "hardness_total", Label: "Общая жёсткость воды", Unit: "мг-экв/л"},
		{Key: "silicon", Label: "Кремний Si", Unit: "мг/л"},
		{Key: "aluminosilicates", Label: "Алюмосиликаты AlSiO₄", Unit: "мг/л"},
		{Key: "water_type_sulin", Label: "Тип воды по В. А. Сулину (расчёт)", Unit: "расчёт"},
		{Key: "external_saturation_index", Label: "Индекс насыщения воды по протоколу", Unit: "ед."},
		{Key: "external_stability_index", Label: "Индекс стабильности воды по протоколу", Unit: "ед."},
		{Key: "deposit_caco3", Label: "CaCO₃ в отложениях", Unit: "% масс."},
		{Key: "deposit_caso4", Label: "CaSO₄ / гипс в отложениях", Unit: "% масс."},
		{Key: "deposit_baso4", Label: "BaSO₄ в отложениях", Unit: "% масс."},
		{Key: "mix2_ca_total", Label: "Общий Ca воды 2", Unit: "мг/л"},
		{Key: "mix2_mg_total", Label: "Общий Mg воды 2", Unit: "мг/л"},
		{Key: "mix2_tic", Label: "Углерод воды 2 (как C)", Unit: "мг/л"},
		{Key: "mix2_so4_total", Label: "Сульфат воды 2 (как SO₄)", Unit: "мг/л"},
		{Key: "mix2_na", Label: "Na воды 2", Unit: "мг/л"},
		{Key: "mix2_k", Label: "K воды 2", Unit: "мг/л"},
		{Key: "mix2_cl", Label: "Cl воды 2", Unit: "мг/л"},
		{Key: "mix2_ba", Label: "Ba воды 2", Unit: "мг/л"},
		{Key: "mix2_sr", Label: "Sr воды 2", Unit: "мг/л"},
		{Key: "mix2_solute_mass", Label: "Растворённые вещества воды 2", Unit: "г/л"},
		{Key: "mix2_brine_density", Label: "Плотность воды 2", Unit: "кг/м³"},
		{Key: "mix_percent", Label: "Доля воды 2 по объёму", Unit: "%"},
		{Key: "mix_ph", Label: "Измеренный pH смеси", Unit: "ед."},
		{Key: "mix_temp", Label: "Температура смеси", Unit: "°C"},
		{Key: "mix_ph_temp", Label: "Температура измерения pH смеси", Unit: "°C"},
		{Key: "mix2_sample_temp", Label: "Температура воды 2", Unit: "°C"},
		{Key: "mix1_source", Label: "Источник анализа воды 1", Unit: "текст"},
		{Key: "mix2_source", Label: "Источник анализа воды 2", Unit: "текст"},
		// Фактические испытания ингибитора солеотложений. Это журнал точек
		// dose-response, а не расчёт рекомендуемой дозировки.
		{Key: "inhibitor_trial_brand", Label: "Марка ингибитора в испытании", Unit: "текст"},
		{Key: "inhibitor_trial_target", Label: "Минерал / цель испытания ингибитора", Unit: "текст"},
		{Key: "inhibitor_trial_dose", Label: "Доза ингибитора в испытании", Unit: "мг/л"},
		{Key: "inhibitor_trial_temp", Label: "Температура испытания ингибитора", Unit: "°C"},
		{Key: "inhibitor_trial_duration_h", Label: "Продолжительность испытания ингибитора", Unit: "ч"},
		{Key: "inhibitor_trial_efficiency", Label: "Фактическая эффективность ингибитора", Unit: "%"},
		{Key: "inhibitor_trial_protocol", Label: "Протокол испытания ингибитора", Unit: "текст"},
		{Key: "inhibitor_trial_result", Label: "Фактический результат испытания ингибитора", Unit: "текст"},
		// Проверенный учебный режим CO₂ ↔ вода. Эти поля показываются только
		// для модели co2_gas_water и не смешиваются с обычной солевой формой.
		{Key: "co2_eq_mode", Label: "Постановка CO₂: резервуар / замкнутый", Unit: "текст"},
		{Key: "co2_eq_pressure_atm", Label: "Начальное / заданное давление CO₂", Unit: "атм abs"},
		{Key: "co2_eq_water_kg", Label: "Масса воды-растворителя", Unit: "кг"},
		{Key: "co2_eq_na_molal", Label: "Фоновый Na", Unit: "моль/кг воды"},
		{Key: "co2_eq_cl_molal", Label: "Фоновый Cl", Unit: "моль/кг воды"},
		{Key: "co2_eq_reservoir_mol", Label: "Запас CO₂ резервуара", Unit: "моль"},
		{Key: "co2_eq_gas_volume_l", Label: "Замкнутый объём газа", Unit: "л"},
		{Key: "co2_eq_gas_percent", Label: "CO₂ в замкнутом газе", Unit: "% об."},
		{Key: "co2_eq_temp", Label: "Температура CO₂-равновесия", Unit: "°C"},
	}},
	{ID: ID_LAB_TAB_ASPO, Code: "aspo", Name: "АСПО", Fields: []LabFieldDef{
		{Key: "paraffins", Label: "Парафины", Unit: "% масс."},
		{Key: "resins", Label: "Смолы", Unit: "% масс."},
		{Key: "asphaltenes", Label: "Асфальтены", Unit: "% масс."},
		{Key: "pour_point", Label: "Температура застывания", Unit: "°C"},
		{Key: "wax_onset", Label: "Начало кристаллизации", Unit: "°C"},
		{Key: "viscosity", Label: "Динамическая вязкость", Unit: "мПа·с"},
		{Key: "density", Label: "Плотность нефти", Unit: "кг/м³"},
		{Key: "water", Label: "Вода в лабораторной пробе", Unit: "%"},
		{Key: "mechanical", Label: "Мехпримеси в нефти", Unit: "%"},
		{Key: "solids", Label: "Твёрдая фаза АСПО", Unit: "%"},
		{Key: "planning_horizon", Label: "Горизонт планирования", Unit: "сут"},
		{Key: "oils_fraction", Label: "Масла в групповом составе", Unit: "% масс."},
		{Key: "light_fraction", Label: "Лёгкие фракции", Unit: "% масс."},
		{Key: "viscosity_index", Label: "Индекс вязкости", Unit: "ед."},
		{Key: "kinematic_viscosity_20", Label: "Кинематическая вязкость при 20 °C", Unit: "мм²/с"},
		{Key: "kinematic_viscosity_40", Label: "Кинематическая вязкость при 40 °C", Unit: "мм²/с"},
		{Key: "dynamic_viscosity_40", Label: "Динамическая вязкость при 40 °C", Unit: "мПа·с"},
		{Key: "flow_type", Label: "Характер течения нефти / эмульсии", Unit: "текст"},
		{Key: "oil_in_water", Label: "Нефтепродукты в воде", Unit: "мг/л"},
		{Key: "water_separation_test", Label: "Водоотделение с/без деэмульгатора", Unit: "текст"},
	}},
	{ID: ID_LAB_TAB_SOLIDS, Code: "solids", Name: "Механические примеси", Fields: []LabFieldDef{
		{Key: "kvch", Label: "Количество взвешенных частиц (КВЧ)", Unit: "мг"},
		{Key: "suspended", Label: "Взвешенные вещества", Unit: "мг/л"},
		{Key: "sand", Label: "Песок", Unit: "%"},
		{Key: "fes", Label: "Сульфид железа (FeS)", Unit: "%"},
		{Key: "scale_fraction", Label: "Минеральные отложения", Unit: "%"},
		{Key: "organic_fraction", Label: "Органическая часть", Unit: "%"},
		{Key: "size_d50", Label: "Медианный размер d50", Unit: "мкм"},
		{Key: "size_max", Label: "Макс. размер частиц", Unit: "мкм"},
		{Key: "iron", Label: "Железо", Unit: "мг/л"},
		{Key: "turbidity", Label: "Мутность", Unit: "ед."},
		{Key: "carbonates", Label: "Карбонаты в отложениях", Unit: "%"},
		{Key: "sulfates", Label: "Сульфаты в отложениях", Unit: "%"},
		{Key: "chloride_salts", Label: "Хлоридные соли в отложениях", Unit: "%"},
		{Key: "corrosion_products", Label: "Продукты коррозии", Unit: "%"},
		{Key: "suspended_filter", Label: "Взвешенные вещества, фильтр 3–10 мкм", Unit: "мг/л"},
		{Key: "suspended_membrane", Label: "Взвешенные вещества, мембрана 0,45 мкм", Unit: "мг/л"},
		{Key: "particle_distribution", Label: "Распределение частиц по размерам", Unit: "текст"},
		{Key: "filter_elemental", Label: "Элементный состав осадка на фильтрах", Unit: "текст"},
		{Key: "deposit_iron_oxides", Label: "Оксиды железа в отложениях", Unit: "% масс."},
		{Key: "deposit_insoluble_mineral", Label: "Нерастворимые минеральные мехпримеси", Unit: "% масс."},
		{Key: "deposit_mohs", Label: "Твёрдость минеральной части по Моосу", Unit: "ед."},
		{Key: "deposit_elemental", Label: "Элементный состав отложений", Unit: "текст"},
	}},
	{ID: ID_LAB_TAB_MICRO, Code: "microbiology", Name: "Микробиология", Fields: []LabFieldDef{
		{Key: "srb", Label: "Сульфатвосстанавливающие бактерии (СВБ)", Unit: "результат"},
		{Key: "apb", Label: "Кислотообразующие бактерии", Unit: "результат"},
		{Key: "gmb", Label: "Общее микробное число", Unit: "КОЕ/мл"},
		{Key: "atp", Label: "Аденозинтрифосфат (АТФ)", Unit: "отн. ед."},
		{Key: "biofilm", Label: "Биоплёнка", Unit: "результат"},
		{Key: "iron_bacteria", Label: "Железобактерии", Unit: "результат"},
		{Key: "sulfide", Label: "Сульфиды", Unit: "мг/л"},
		{Key: "ph", Label: "Водородный показатель (pH)", Unit: "ед."},
		{Key: "sample_temp", Label: "Температура пробы", Unit: "°C"},
		{Key: "incubation", Label: "Условия / срок анализа", Unit: "текст"},
		{Key: "qpcr_srb", Label: "СВБ — количественная полимеразная цепная реакция (qPCR)", Unit: "копий/мл"},
		{Key: "qpcr_apb", Label: "Кислотообразующие бактерии — количественная полимеразная цепная реакция (qPCR)", Unit: "копий/мл"},
		{Key: "orp", Label: "Окислительно-восстановительный потенциал (ОВП)", Unit: "мВ"},
		{Key: "srb_activity", Label: "Активность сульфатвосстанавливающих бактерий (СВБ)", Unit: "%"},
		{Key: "iron_bacteria_activity", Label: "Активность железобактерий", Unit: "%"},
		{Key: "other_microorganisms", Label: "Другие микроорганизмы", Unit: "результат"},
		{Key: "other_microorganisms_activity", Label: "Активность других микроорганизмов", Unit: "%"},
		{Key: "ammonium", Label: "Аммоний NH₄⁺", Unit: "мг/л"},
		{Key: "nitrate", Label: "Нитраты NO₃⁻", Unit: "мг/л"},
	}},
	{ID: ID_LAB_TAB_GASES, Code: "gases", Name: "Газы и гидраты", Fields: []LabFieldDef{
		{Key: "co2", Label: "CO₂", Unit: "мг/л"},
		{Key: "h2s", Label: "H₂S", Unit: "мг/л"},
		{Key: "o2", Label: "O₂", Unit: "мг/л"},
		{Key: "gas_factor", Label: "Газовый фактор", Unit: "м³/т"},
		{Key: "water_content", Label: "Влагосодержание газа", Unit: "г/м³"},
		{Key: "hydrate_temp", Label: "Равновесная T гидратов", Unit: "°C"},
		{Key: "sample_pressure", Label: "Давление (Р) пробы", Unit: "МПа"},
		{Key: "sample_temp", Label: "Температура пробы", Unit: "°C"},
		{Key: "methane", Label: "CH₄", Unit: "% об."},
		{Key: "co2_gas", Label: "CO₂ в газе", Unit: "% об."},
		{Key: "h2s_gas", Label: "H₂S в газе", Unit: "% об."},
		{Key: "pco2", Label: "Парциальное давление (Р) CO₂", Unit: "МПа"},
		{Key: "ph2s", Label: "Парциальное давление (Р) H₂S", Unit: "МПа"},
		{Key: "ph", Label: "Водородный показатель (pH)", Unit: "ед."},
		{Key: "mineralization", Label: "Общая минерализация", Unit: "г/л"},
		{Key: "orp", Label: "Окислительно-восстановительный потенциал (ОВП)", Unit: "мВ"},
		{Key: "iron_total", Label: "Железо общее", Unit: "мг/л"},
		{Key: "bottomhole_pressure", Label: "Забойное давление (Р)", Unit: "МПа"},
		{Key: "mixture_density", Label: "Плотность смеси", Unit: "кг/м³"},
		{Key: "formation_temp", Label: "Температура пласта", Unit: "°C"},
		{Key: "formation_pressure", Label: "Давление (Р) пласта", Unit: "МПа"},
		{Key: "pump_temp", Label: "Температура на приёме насоса", Unit: "°C"},
		{Key: "pump_pressure", Label: "Давление (Р) на приёме насоса", Unit: "МПа"},
		{Key: "wellhead_temp", Label: "Температура на устье", Unit: "°C"},
		{Key: "wellhead_pressure", Label: "Давление (Р) на устье", Unit: "МПа"},
		{Key: "profile_temp_min", Label: "Минимальная температура профиля", Unit: "°C"},
		{Key: "profile_pressure_min", Label: "Давление (Р) при минимальной температуре", Unit: "МПа"},
		{Key: "profile_temp_max", Label: "Максимальная температура профиля", Unit: "°C"},
		{Key: "profile_pressure_max", Label: "Давление (Р) при максимальной температуре", Unit: "МПа"},
		{Key: "h2s_oil", Label: "H₂S в нефти", Unit: "мг/л"},
		{Key: "h2s_gas_mass", Label: "H₂S в газе, массовая концентрация", Unit: "мг/м³"},
		{Key: "mercaptans_gas", Label: "Меркаптаны R–SH в газе", Unit: "мг/м³"},
		{Key: "hydrogen_dissolved", Label: "Растворённый водород H₂", Unit: "мг/л"},
	}},
	{ID: ID_LAB_TAB_OVERALL, Code: "overall", Name: "Общая оценка"},
}

var (
	labRecords                []LabRecord
	labRecordsLoaded          bool
	labActiveGroup            = "corrosion"
	labTabButtons             = map[int]syscall.Handle{}
	labObjectEdit             syscall.Handle
	labObjectCaption          syscall.Handle
	labPinnedDraftID          string
	labDateEdit               syscall.Handle
	labSourceEdit             syscall.Handle
	labFilterEdit             syscall.Handle
	labCommentEdit            syscall.Handle
	labCommentLabel           syscall.Handle
	labConclusionEdit         syscall.Handle
	labConclusionTitle        syscall.Handle
	labShortConclusion        syscall.Handle
	labFieldBadges            [10]syscall.Handle
	labFieldLabels            [10]syscall.Handle
	labFieldEdits             [10]syscall.Handle
	labUnitLabels             [10]syscall.Handle
	labInfluenceChart         syscall.Handle
	labInfographicButton      syscall.Handle
	labComplexAnalysisButton  syscall.Handle
	labCalculationBasisButton syscall.Handle
	labStiffChart             syscall.Handle
	labMixingChart            syscall.Handle
	labTrendChart             syscall.Handle
	labTrendMetricCombo       syscall.Handle
	labScopeObjectsEdit       syscall.Handle
	labRiskFilterCombo        syscall.Handle
	labGroupNameEdit          syscall.Handle
	labSavedGroupCombo        syscall.Handle
	labSaveGroupButton        syscall.Handle
	labTrendSortCombo         syscall.Handle
	labDetailObjectCombo      syscall.Handle
	labScopeLabel             syscall.Handle
	labScopeCombo             syscall.Handle
	labOverviewScopeCode      = laboratoryScopeObject
	labOverviewRiskFilterCode = "all"
	labScaleModelLabel        syscall.Handle
	labScaleModelCombo        syscall.Handle
	labScaleModelInfo         syscall.Handle
	labScaleModelCode         = "davies"
	labFieldsPrev             syscall.Handle
	labFieldsNext             syscall.Handle
	labFieldsPageLabel        syscall.Handle
	labTitle                  syscall.Handle
	labLegendLabel            syscall.Handle
	labObjectLabel            syscall.Handle
	labDateLabel              syscall.Handle
	labSourceLabel            syscall.Handle
	labFilterLabel            syscall.Handle
	labFieldsPage             int
	labPageSwitching          bool
	labGroupTransitionBusy    bool
	labAtomicUpdateDepth      int
	labHistoryList            syscall.Handle
	labHistoryTitle           syscall.Handle
	labDeleteButton           syscall.Handle
	labClearButton            syscall.Handle
	labClearCurrentButton     syscall.Handle
	labHistoryEntries         []laboratoryHistoryEntry
	labInfo                   syscall.Handle
	labReadinessPanel         syscall.Handle
	labPassportPanel          syscall.Handle
	labAnalyzeButton          syscall.Handle
	labSaveButton             syscall.Handle
	labImportButton           syscall.Handle
	labTemplateButton         syscall.Handle
	labExportButton           syscall.Handle
	labUseObjectButton        syscall.Handle
	labContinueButton         syscall.Handle
	labCopyButton             syscall.Handle
	labTooltip                syscall.Handle
	labTooltipText            []uint16
	labFieldTipText           [32][]uint16
	labFieldTipAdded          [32]bool
	labLastConclusion         string
	labConclusionOpen         bool
	labDrafts                 = map[string]map[string]string{} // active-pass compatibility view
	labUnitDrafts             = map[string]map[string]string{}
	labPassDrafts             = map[string]*LabPassRecord{}
	labPasses                 []LabPassRecord
	labPassesLoaded           bool
	labActivePassID           string
	labSharedSyncing          bool
	labCalculatedSyncing      bool
	labObjectSwitching        bool
	labActiveObjectKey        string
	selectedObjectKey         string

	labMappingDialogDone     bool
	labMappingDialogAccepted bool
	labMappingDialogRows     [][]string
	labMappingDialogPath     string
	labMappingDialogValue    LabColumnMapping
	labMappingDialogCombos   [8]syscall.Handle
	labMappingDialogPreview  syscall.Handle
)

func labPath() string { return filepath.Join(dataDir, "laboratory_records.jsonl") }

func loadLabRecords() []LabRecord {
	labRecordsLoaded = true
	f, err := os.Open(labPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	latest := map[string]LabRecord{}
	order := []string{}
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line, decErr := decryptPersonalLine(sc.Bytes())
		var r LabRecord
		if decErr != nil || json.Unmarshal(line, &r) != nil || r.ID == "" {
			continue
		}
		if !seen[r.ID] {
			seen[r.ID] = true
			order = append(order, r.ID)
		}
		latest[r.ID] = r
	}
	out := make([]LabRecord, 0, len(latest))
	for _, id := range order {
		r := latest[id]
		if !r.Deleted {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return parseDate(out[i].Date).After(parseDate(out[j].Date)) })
	return out
}

func appendLabRecord(r LabRecord) error {
	if r.ID == "" {
		return fmt.Errorf("пустой идентификатор лабораторной записи")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if personalPath(labPath()) {
		b, err = encryptPersonalLine(b)
		if err != nil {
			return err
		}
	}
	f, err := os.OpenFile(labPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func currentLabGroup() LabGroupDef {
	for _, g := range labGroups {
		if g.Code == labActiveGroup {
			return laboratoryScaleInputGroup(g, labScaleModelCode)
		}
	}
	return labGroups[0]
}

func laboratoryBoolFlag(value bool) uintptr {
	if value {
		return 1
	}
	return 0
}

func labGroupByID(id int) (LabGroupDef, bool) {
	for _, g := range labGroups {
		if g.ID == id {
			return g, true
		}
	}
	return LabGroupDef{}, false
}

func labGroupByCode(code string) (LabGroupDef, bool) {
	for _, g := range labGroups {
		if g.Code == code {
			return g, true
		}
	}
	return LabGroupDef{}, false
}

func buildLaboratoryUI() {
	labTitle = pageTitle(ID_LAB, "Лабораторные данные", 242, 94, 370, 34)
	labLegendLabel = create("STATIC", "", WS_CHILD|SS_OWNERDRAW, 242, py(128), 1010, 26, hMain, ID_LAB_LEGEND)
	addPageControl(ID_LAB, labLegendLabel)
	x := int32(242)
	widths := []int32{112, 150, 220, 176, 160, 150, 174}
	for i, g := range labGroups {
		caption := g.Name
		// Deliberate two-line captions retain the full terminology while keeping
		// both words centered inside the tab on narrower/high-DPI windows.
		switch g.Code {
		case "solids":
			caption = "Механические\nпримеси"
		case "gases":
			caption = "Газы и\nгидраты"
		}
		b := button(ID_LAB, caption, x, 158, widths[i], 38, g.ID)
		labTabButtons[g.ID] = b
		x += widths[i] + 8
	}
	labShortConclusion = create("STATIC", "Краткий вывод: анализ ещё не выполнен. Нажмите, чтобы раскрыть полный текст.", WS_CHILD|SS_LEFT|SS_NOTIFY|SS_NOPREFIX|WS_BORDER, 242, py(272), 1010, 46, hMain, ID_LAB_TOGGLE)
	addPageControl(ID_LAB, labShortConclusion)
	pSendMessageW.Call(uintptr(labShortConclusion), WM_SETFONT, uintptr(hFont), 1)
	// Import is a service action, not a seventh laboratory section. Keep it in
	// the title row so it cannot overlap the six section tabs on narrower windows.
	labImportButton = button(ID_LAB, "Загрузить анализы", 956, 94, 296, 40, ID_LAB_IMPORT)
	labTemplateButton = button(ID_LAB, "Скачать бланк для заполнения", 640, 94, 304, 40, ID_LAB_DOWNLOAD_TEMPLATE)
	labObjectLabel = label(ID_LAB, "Ключ объекта", 242, 205, 105, 24)
	labObjectEdit = edit(ID_LAB, "", 352, 200, 345, 30, false, false, ID_LAB_OBJECT_EDIT)
	labObjectCaption = label(ID_LAB, "Выберите объект на Главной", 352, 200, 345, 30)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(labObjectCaption), WM_SETFONT, uintptr(hFontBold), 1)
	}
	labUseObjectButton = button(ID_LAB, "Выбрать объект", 709, 198, 260, 38, ID_LAB_USE_SELECTED)
	labDateLabel = label(ID_LAB, "Дата анализа", 986, 205, 95, 24)
	labDateEdit = edit(ID_LAB, time.Now().Format("02.01.2006"), 1085, 200, 167, 30, false, false, 0)
	labSourceLabel = label(ID_LAB, "Источник / протокол", 242, 241, 145, 24)
	labSourceEdit = edit(ID_LAB, "", 392, 236, 430, 30, false, false, ID_LAB_SOURCE_EDIT)
	labFilterLabel = label(ID_LAB, "Поиск", 842, 241, 72, 24)
	labFilterEdit = edit(ID_LAB, "", 968, 236, 284, 30, false, false, ID_LAB_FILTER)
	for _, heading := range []syscall.Handle{labObjectLabel, labDateLabel, labSourceLabel, labFilterLabel} {
		if heading != 0 && hFontBold != 0 {
			pSendMessageW.Call(uintptr(heading), WM_SETFONT, uintptr(hFontBold), 1)
		}
	}

	for i := 0; i < 10; i++ {
		col := i / 5
		row := i % 5
		baseX := int32(242)
		if col == 1 {
			baseX = 752
		}
		y := int32(330 + row*34)
		labFieldBadges[i] = create("STATIC", "", WS_CHILD|SS_OWNERDRAW, baseX, py(y), 20, 24, hMain, ID_LAB_BADGE_BASE+i)
		addPageControl(ID_LAB, labFieldBadges[i])
		labFieldLabels[i] = label(ID_LAB, "", baseX+26, y, 234, 24)
		if hFontBold != 0 {
			pSendMessageW.Call(uintptr(labFieldLabels[i]), WM_SETFONT, uintptr(hFontBold), 1)
		}
		labFieldEdits[i] = edit(ID_LAB, "", baseX+265, y-5, 100, 29, false, false, ID_LAB_FIELD_BASE+i)
		labUnitLabels[i] = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, baseX+370, py(y-5), 125, 220, hMain, ID_LAB_UNIT_BASE+i)
		addPageControl(ID_LAB, labUnitLabels[i])
	}
	labFieldsPrev = button(ID_LAB, "‹ Предыдущая", 242, 320, 220, 42, ID_LAB_FIELDS_PREV)
	labFieldsPageLabel = create("STATIC", "", WS_CHILD|WS_BORDER|SS_OWNERDRAW, 470, py(320), 340, 42, hMain, 0)
	addPageControl(ID_LAB, labFieldsPageLabel)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(labFieldsPageLabel), WM_SETFONT, uintptr(hFontBold), 1)
	}
	labFieldsNext = button(ID_LAB, "Следующая ›", 818, 320, 220, 42, ID_LAB_FIELDS_NEXT)
	labConclusionTitle = label(ID_LAB, "Предварительный вывод", 242, 480, 250, 22)
	labConclusionEdit = laboratoryInfoCard()
	labInfluenceChart = create("STATIC", "", WS_CHILD|WS_BORDER|SS_OWNERDRAW, 242, py(320), 790, 176, hMain, ID_LAB_INFLUENCE)
	addPageControl(ID_LAB, labInfluenceChart)
	labStiffChart = create("STATIC", "", WS_CHILD|WS_BORDER|SS_OWNERDRAW, 242, py(320), 790, 176, hMain, ID_LAB_STIFF_CHART)
	addPageControl(ID_LAB, labStiffChart)
	labMixingChart = create("STATIC", "", WS_CHILD|WS_BORDER|SS_OWNERDRAW, 242, py(320), 790, 176, hMain, ID_LAB_MIXING_CHART)
	addPageControl(ID_LAB, labMixingChart)
	labScopeLabel = label(ID_LAB, "Инфографика осложнений", 0, 0, 0, 0)
	labInfographicButton = button(ID_LAB, "ОТКРЫТЬ ИНФОГРАФИКУ ОСЛОЖНЕНИЙ", 0, 0, 0, 44, ID_LAB_OPEN_INFOGRAPHIC)
	labComplexAnalysisButton = button(ID_LAB, "КОМПЛЕКСНАЯ ДИАГНОСТИКА", 0, 0, 0, 44, ID_LAB_COMPLEX_ANALYSIS)
	labCalculationBasisButton = button(ID_LAB, "РАСЧЁТЫ И ФОРМУЛЫ", 0, 0, 0, 44, ID_LAB_CALCULATION_BASIS)
	labScopeCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 0, 0, 0, 240, hMain, ID_LAB_SCOPE_COMBO)
	addPageControl(ID_LAB, labScopeCombo)
	for _, scopeName := range laboratoryScopeNames {
		pSendMessageW.Call(uintptr(labScopeCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(scopeName))))
	}
	pSendMessageW.Call(uintptr(labScopeCombo), CB_SETCURSEL, 0, 0)
	labTrendChart = create("STATIC", "", WS_CHILD|WS_BORDER|SS_OWNERDRAW, 0, 0, 0, 0, hMain, ID_LAB_TREND_CHART)
	addPageControl(ID_LAB, labTrendChart)
	labTrendMetricCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 0, 0, 0, 220, hMain, ID_LAB_TREND_METRIC)
	addPageControl(ID_LAB, labTrendMetricCombo)
	for _, metric := range laboratoryTrendMetrics {
		pSendMessageW.Call(uintptr(labTrendMetricCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(metric.Label))))
	}
	pSendMessageW.Call(uintptr(labTrendMetricCombo), CB_SETCURSEL, 0, 0)
	labScopeObjectsEdit = edit(ID_LAB, "", 0, 0, 0, 28, false, false, ID_LAB_SCOPE_OBJECTS_EDIT)
	labRiskFilterCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 0, 0, 0, 220, hMain, ID_LAB_RISK_FILTER)
	addPageControl(ID_LAB, labRiskFilterCombo)
	for _, name := range laboratoryRiskFilterNames {
		pSendMessageW.Call(uintptr(labRiskFilterCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(name))))
	}
	pSendMessageW.Call(uintptr(labRiskFilterCombo), CB_SETCURSEL, 0, 0)
	labGroupNameEdit = edit(ID_LAB, "", 0, 0, 0, 28, false, false, ID_LAB_GROUP_NAME)
	labSavedGroupCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 0, 0, 0, 220, hMain, ID_LAB_GROUP_COMBO)
	addPageControl(ID_LAB, labSavedGroupCombo)
	labSaveGroupButton = button(ID_LAB, "Сохранить группу", 0, 0, 0, 28, ID_LAB_GROUP_SAVE)
	labTrendSortCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 0, 0, 0, 220, hMain, ID_LAB_TREND_SORT)
	addPageControl(ID_LAB, labTrendSortCombo)
	for _, name := range laboratoryTrendSortNames {
		pSendMessageW.Call(uintptr(labTrendSortCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(name))))
	}
	pSendMessageW.Call(uintptr(labTrendSortCombo), CB_SETCURSEL, 0, 0)
	labDetailObjectCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 0, 0, 0, 260, hMain, ID_LAB_DETAIL_OBJECT)
	addPageControl(ID_LAB, labDetailObjectCombo)
	laboratorySavedGroups = loadLaboratorySavedGroups()
	for _, g := range laboratorySavedGroups {
		pSendMessageW.Call(uintptr(labSavedGroupCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(g.Name))))
	}
	labScaleModelLabel = label(ID_LAB, "МОДЕЛЬ СОЛЕОБРАЗОВАНИЯ", 0, 0, 0, 0)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(labScaleModelLabel), WM_SETFONT, uintptr(hFontBold), 1)
	}
	labScaleModelCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 0, 0, 0, 240, hMain, ID_LAB_SCALE_MODEL)
	addPageControl(ID_LAB, labScaleModelCombo)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(labScaleModelCombo), WM_SETFONT, uintptr(hFontBold), 1)
	}
	labScaleModelInfo = laboratoryInfoCard()
	initializeLaboratoryScaleModels()
	labAnalyzeButton = button(ID_LAB, "Произвести анализ", 1046, 504, 206, 32, ID_LAB_ANALYZE)
	labExportButton = button(ID_LAB, "Выгрузить отчёт", 1046, 542, 206, 32, ID_LAB_EXPORT)
	labCommentLabel = label(ID_LAB, "Комментарий (необязательно)", 242, 586, 205, 24)
	labCommentEdit = edit(ID_LAB, "", 452, 580, 580, 38, true, false, 0)
	labSaveButton = button(ID_LAB, "Сохранить изменения", 1046, 580, 206, 38, ID_LAB_SAVE)
	labContinueButton = button(ID_LAB, "Продолжить редактирование", 1046, 626, 206, 38, ID_LAB_CONTINUE)
	labCopyButton = button(ID_LAB, "Копировать заключение", 1046, 672, 206, 38, ID_LAB_COPY_RESULT)
	labInfo = create("STATIC", "", WS_CHILD|SS_NOPREFIX|SS_ENDELLIPSIS|0x000C, 242, py(626), 1010, 24, hMain, 0)
	addPageControl(ID_LAB, labInfo)
	labHistoryTitle = label(ID_LAB, "Предыдущие анализы", 242, 636, 1010, 24)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(labHistoryTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	labDeleteButton = button(ID_LAB, "Удалить", 0, 0, 126, 30, ID_LAB_DELETE_ROW)
	labClearButton = button(ID_LAB, "Очистить всё", 0, 0, 160, 30, ID_LAB_CLEAR_ALL)
	labClearCurrentButton = button(ID_LAB, "Очистить введённое", 0, 0, 206, 30, ID_LAB_CLEAR_CURRENT)
	labHistoryList = newList(ID_LAB, 242, 662, 1010, 150, []struct {
		n string
		w int32
	}{
		{"Версия", 190}, {"Дата", 105}, {"Объект", 220}, {"Раздел", 145}, {"Источник", 190}, {"Заполнено", 95}, {"Статус внесения", 300}, {"Комментарий", 210}, {"Краткий вывод", 480},
	})
	setListHeaderFont(labHistoryList, hFontHeaderSmall)
	centerListHeaders(labHistoryList, 9)
	pSendMessageW.Call(uintptr(labHistoryList), LVM_SETEXTENDEDLISTVIEWSTYLE, 0x00000400, 0x00000400) // LVS_EX_INFOTIP for long document names
	pSendMessageW.Call(uintptr(labHistoryList), LVM_SETBKCOLOR, 0, rgb(246, 248, 251))
	pSendMessageW.Call(uintptr(labHistoryList), LVM_SETTEXTBKCOLOR, 0, rgb(246, 248, 251))
	pSendMessageW.Call(uintptr(labHistoryList), LVM_SETTEXTCOLOR, 0, rgb(43, 46, 50))
	labReadinessPanel = laboratoryInfoCard()
	labPassportPanel = create("STATIC", "", WS_CHILD|WS_BORDER|SS_LEFT|SS_NOPREFIX, 0, 0, 0, 0, hMain, 0)
	addPageControl(ID_LAB, labPassportPanel)
	pSendMessageW.Call(uintptr(labPassportPanel), WM_SETFONT, uintptr(hFont), 1)
	initLaboratoryTooltip()
	switchLabGroup("corrosion", false)
}

func laboratoryTooltipInfo() TOOLINFOW {
	var text *uint16
	if len(labTooltipText) > 0 {
		text = &labTooltipText[0]
	}
	return TOOLINFOW{
		CbSize:   uint32(unsafe.Sizeof(TOOLINFOW{})),
		UFlags:   TTF_IDISHWND | TTF_SUBCLASS,
		Hwnd:     hMain,
		UId:      uintptr(labShortConclusion),
		LpszText: text,
	}
}

func initLaboratoryTooltip() {
	if labShortConclusion == 0 || labTooltip != 0 {
		return
	}
	labTooltip = create("tooltips_class32", "", 0x80000000|TTS_ALWAYSTIP|TTS_NOPREFIX, 0, 0, 0, 0, hMain, 0)
	if labTooltip == 0 {
		return
	}
	labTooltipText, _ = syscall.UTF16FromString(getText(labConclusionEdit))
	ti := laboratoryTooltipInfo()
	pSendMessageW.Call(uintptr(labTooltip), TTM_SETMAXTIPWIDTH, 0, 720)
	pSendMessageW.Call(uintptr(labTooltip), TTM_ADDTOOLW, 0, uintptr(unsafe.Pointer(&ti)))
	updateLaboratoryFieldTooltip(20, labScaleModelCombo, "Выберите математический метод оценки насыщения. Ионное произведение активности / произведение растворимости (IAP/Ksp) даёт коэффициент насыщения; индекс насыщения SI — его десятичный логарифм. Карточка ниже показывает область применимости и ограничения.")
	updateLaboratoryFieldTooltip(21, labScaleModelInfo, "Справка по выбранному методу: какие исходные данные используются, где модель применима и почему термодинамическое пересыщение ещё не доказывает фактическое выпадение соли.")
}

func updateLaboratoryFieldTooltip(slot int, control syscall.Handle, value string) {
	if labTooltip == 0 || control == 0 || slot < 0 || slot >= len(labFieldTipText) || slot >= len(labFieldTipAdded) {
		return
	}
	labFieldTipText[slot], _ = syscall.UTF16FromString(value)
	ti := TOOLINFOW{
		CbSize:   uint32(unsafe.Sizeof(TOOLINFOW{})),
		UFlags:   TTF_IDISHWND | TTF_SUBCLASS,
		Hwnd:     hMain,
		UId:      uintptr(control),
		LpszText: &labFieldTipText[slot][0],
	}
	message := uintptr(TTM_UPDATETIPTEXTW)
	if !labFieldTipAdded[slot] {
		message = TTM_ADDTOOLW
		labFieldTipAdded[slot] = true
	}
	pSendMessageW.Call(uintptr(labTooltip), message, 0, uintptr(unsafe.Pointer(&ti)))
}

func setLaboratoryConclusion(text string) {
	plain := normalizeLaboratoryLineBreaks(text)
	if plain == "" {
		plain = "Заполните доступные показатели и нажмите «Произвести анализ»."
	}
	labLastConclusion = plain
	display := strings.ReplaceAll(plain, "\n", "\r\n")
	setText(labConclusionEdit, display)
	formatLaboratoryConclusionRichText(display)
	setText(labShortConclusion, laboratoryShortConclusion(plain))
	if labTooltip != 0 {
		labTooltipText, _ = syscall.UTF16FromString(display)
		ti := laboratoryTooltipInfo()
		pSendMessageW.Call(uintptr(labTooltip), TTM_UPDATETIPTEXTW, 0, uintptr(unsafe.Pointer(&ti)))
	}
}

type laboratoryTextRange struct {
	Start int
	End   int
}

func normalizeLaboratoryLineBreaks(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func laboratoryUTF16Len(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func laboratoryBoldRanges(display string) []laboratoryTextRange {
	// RichEdit20 stores a paragraph break as one character even when
	// SetWindowText receives CRLF. Calculate selection positions against that
	// internal representation; otherwise every following bold range drifts by
	// one character per line.
	display = strings.ReplaceAll(display, "\r\n", "\n")
	display = strings.ReplaceAll(display, "\r", "\n")
	if strings.Contains(strings.ToUpper(display), "ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ") {
		return laboratoryOverallBoldRanges(display)
	}
	headings := []string{
		"ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ",
		"ПРЕДВАРИТЕЛЬНОЕ ЗАКЛЮЧЕНИЕ",
		"РАСШИРЕННАЯ ОЦЕНКА ВЛИЯНИЯ НА КОРРОЗИЮ",
		"КИСЛОРОД O₂", "ДИОКСИД УГЛЕРОДА CO₂", "СЕРОВОДОРОД H₂S",
		"ОЦЕНКА ВЕРОЯТНОСТИ H₂S-РАСТРЕСКИВАНИЯ",
		"СОВМЕСТНОЕ ВЛИЯНИЕ O₂ + CO₂ + H₂S",
		"ОЦЕНКА КОРРОЗИОННЫХ ПЛАСТИНОК И ИНГИБИТОРНОЙ ЗАЩИТЫ",
		"МЕТОД КОЭФФИЦИЕНТА НАСЫЩЕНИЯ",
		"ТЕХНОЛОГИЧЕСКИЙ ИТОГ",
		"ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО СОЛЕОТЛОЖЕНИЯМ",
		"ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО CO₂-РАВНОВЕСИЮ",
		"ОБЩИЙ ВЫВОД", "ИСХОДНЫЕ ДАННЫЕ РАЗДЕЛА",
		"ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА", "ЧТО ЭТО ОЗНАЧАЕТ ДЛЯ ТЕХНОЛОГА",
	}
	labels := []string{
		"Итог для технолога:", "Объект:", "Идентификатор диагностики:", "Дата анализа:", "Источник / протокол:",
		"Заполнено направлений:", "Обязательный базовый уровень данных:",
		"Базовый минимум данных:", "Дополнительные исследования:", "Дополнительные параметры:",
		"Контекст техрежима:", "Общие условия:", "Уровень влияния:", "Достоверность:",
		"Учтённые данные:", "Механизм:", "Недостаёт:", "Вывод:", "Ограничение:",
		"Приоритет дополнительной проверки:", "Одновременно требуют проверки направления:",
		"Что происходит:", "Что означают именно эти данные:", "Чего пока нельзя утверждать:", "Что проверить следующим действием:",
		"Следующее действие:",
	}
	var ranges []laboratoryTextRange
	offset := 0
	for _, part := range strings.SplitAfter(display, "\n") {
		line := strings.TrimSuffix(part, "\n")
		trimmed := strings.TrimSpace(line)
		lineStart := offset + laboratoryUTF16Len(line[:len(line)-len(strings.TrimLeft(line, " \t"))])
		matched := false
		for _, heading := range headings {
			if strings.HasPrefix(strings.ToUpper(trimmed), heading) {
				// If the heading is followed by a value, keep only the heading bold.
				end := len(heading)
				if end > len(trimmed) {
					end = len(trimmed)
				}
				ranges = append(ranges, laboratoryTextRange{lineStart, lineStart + laboratoryUTF16Len(trimmed[:end])})
				matched = true
				break
			}
		}
		if !matched {
			for _, label := range labels {
				if strings.HasPrefix(trimmed, label) {
					ranges = append(ranges, laboratoryTextRange{lineStart, lineStart + laboratoryUTF16Len(label)})
					matched = true
					break
				}
			}
			if !matched && strings.HasPrefix(trimmed, "• ") {
				if dash := strings.Index(trimmed, " —"); dash > 2 {
					ranges = append(ranges, laboratoryTextRange{lineStart, lineStart + laboratoryUTF16Len(trimmed[:dash])})
				}
			}
		}
		offset += laboratoryUTF16Len(part)
	}
	return ranges
}

func laboratoryOverallBoldRanges(display string) []laboratoryTextRange {
	display = strings.ReplaceAll(display, "\r\n", "\n")
	display = strings.ReplaceAll(display, "\r", "\n")
	structuralHeadings := []string{
		"ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ",
		"РАЗДЕЛ:",
		"ИСХОДНЫЕ ДАННЫЕ РАЗДЕЛА",
		"ПРЕДВАРИТЕЛЬНОЕ ЗАКЛЮЧЕНИЕ:",
		"РАСШИРЕННАЯ ОЦЕНКА ВЛИЯНИЯ НА КОРРОЗИЮ",
		"КИСЛОРОД O₂", "ДИОКСИД УГЛЕРОДА CO₂", "СЕРОВОДОРОД H₂S",
		"ОЦЕНКА ВЕРОЯТНОСТИ H₂S-РАСТРЕСКИВАНИЯ",
		"СОВМЕСТНОЕ ВЛИЯНИЕ O₂ + CO₂ + H₂S",
		"ОЦЕНКА КОРРОЗИОННЫХ ПЛАСТИНОК И ИНГИБИТОРНОЙ ЗАЩИТЫ",
		"ТЕХНОЛОГИЧЕСКИЙ ИТОГ",
		"ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО СОЛЕОТЛОЖЕНИЯМ",
		"ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО CO₂-РАВНОВЕСИЮ",
		"ВИЗУАЛЬНАЯ СВОДКА ПО НАПРАВЛЕНИЯМ",
		"СВОД ОБЩИХ ПОЛЕЙ",
		"КОНТРОЛЬ ИСТОЧНИКОВ И АКТУАЛЬНОСТИ",
		"ОБЩИЙ ВЫВОД",
		"СВОД ПО НАПРАВЛЕНИЯМ", "КОМПЛЕКСНАЯ ИНЖЕНЕРНАЯ ИНТЕРПРЕТАЦИЯ",
		"ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА", "ЧТО ЭТО ОЗНАЧАЕТ ДЛЯ ТЕХНОЛОГА",
	}
	lineLabels := []string{
		"Итог для технолога:", "Объект:", "Идентификатор диагностики:", "Дата анализа:", "Источник / протокол:",
		"Автор:", "Заполнено направлений:", "Заполнено показателей:", "Источник раздела:",
		"Обязательный базовый уровень данных:", "Базовый минимум данных:",
		"Дополнительные исследования:", "Дополнительные параметры:",
		"Контекст техрежима:", "Общие условия:", "Уровень влияния:",
		"Достоверность:", "Учтённые данные:", "Механизм:", "Недостаёт:",
		"Вывод:", "Ограничение:", "Приоритет дополнительной проверки:",
		"Одновременно требуют проверки направления:",
		"Что происходит:", "Что означают именно эти данные:", "Чего пока нельзя утверждать:", "Что проверить следующим действием:",
		"Следующее действие:",
	}
	var ranges []laboratoryTextRange
	offset := 0
	for _, part := range strings.SplitAfter(display, "\n") {
		line := strings.TrimSuffix(part, "\n")
		trimmed := strings.TrimSpace(line)
		leading := len(line) - len(strings.TrimLeft(line, " \t"))
		lineStart := offset + laboratoryUTF16Len(line[:leading])
		upper := strings.ToUpper(trimmed)
		matched := false
		for _, heading := range structuralHeadings {
			if strings.HasPrefix(upper, heading) {
				end := len(heading)
				if heading == "РАЗДЕЛ:" {
					end = len(trimmed)
				}
				if end > len(trimmed) {
					end = len(trimmed)
				}
				ranges = append(ranges, laboratoryTextRange{lineStart, lineStart + laboratoryUTF16Len(trimmed[:end])})
				matched = true
				break
			}
		}
		if !matched {
			for _, label := range lineLabels {
				if strings.HasPrefix(trimmed, label) {
					ranges = append(ranges, laboratoryTextRange{lineStart, lineStart + laboratoryUTF16Len(label)})
					matched = true
					break
				}
			}
		}
		if !matched && strings.HasPrefix(trimmed, "• [") {
			closeMarker := strings.Index(trimmed, "]")
			if closeMarker > 0 {
				fieldStart := closeMarker + 1
				for fieldStart < len(trimmed) && (trimmed[fieldStart] == ' ' || trimmed[fieldStart] == '\t') {
					fieldStart++
				}
				if relativeColon := strings.Index(trimmed[fieldStart:], ":"); relativeColon > 0 {
					colon := fieldStart + relativeColon
					ranges = append(ranges, laboratoryTextRange{lineStart + laboratoryUTF16Len(trimmed[:fieldStart]), lineStart + laboratoryUTF16Len(trimmed[:colon])})
				}
			}
		} else if !matched && strings.HasPrefix(trimmed, "• ") {
			if dash := strings.Index(trimmed, " —"); dash > 2 {
				ranges = append(ranges, laboratoryTextRange{lineStart, lineStart + laboratoryUTF16Len(trimmed[:dash])})
			}
		}
		offset += laboratoryUTF16Len(part)
	}
	return ranges
}

func formatLaboratoryConclusionRichText(display string) {
	if labConclusionEdit == 0 {
		return
	}
	pSendMessageW.Call(uintptr(labConclusionEdit), WM_SETFONT, uintptr(hFontPanel), 1)
	plainFormat := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR | 0x20000000, CrTextColor: uint32(rgb(43, 46, 50))}
	copy(plainFormat.SzFaceName[:], utf16.Encode([]rune("Segoe UI")))
	pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&plainFormat)))
	boldFormat := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82))}
	for _, textRange := range laboratoryBoldRanges(display) {
		pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETSEL, uintptr(textRange.Start), uintptr(textRange.End))
		pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&boldFormat)))
	}
	offset := 0
	for _, line := range strings.SplitAfter(normalizeLaboratoryLineBreaks(display), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Проверить:") || strings.HasPrefix(trimmed, "ОШИБКА") || strings.Contains(trimmed, "требует внимания") {
			color := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_COLOR, CrTextColor: uint32(rgb(203, 27, 43))}
			pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETSEL, uintptr(offset), uintptr(offset+laboratoryUTF16Len(strings.TrimSuffix(line, "\n"))))
			pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&color)))
		}
		offset += laboratoryUTF16Len(line)
	}
	accent := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(180, 20, 38))}
	for _, textRange := range laboratoryTechnologistRanges(display) {
		pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETSEL, uintptr(textRange.Start), uintptr(textRange.End))
		pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&accent)))
	}
	pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETSEL, 0, 0)
	pSendMessageW.Call(uintptr(labConclusionEdit), 0x00B7, 0, 0) // EM_SCROLLCARET
}

func laboratoryOverallSummaryCounts(text string) (int, int) {
	known := map[string]string{}
	for _, group := range labGroups {
		if group.Code != "overall" {
			known[strings.ToUpper(group.Name)] = group.Code
		}
	}
	present, attention := map[string]bool{}, map[string]bool{}
	active := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "РАЗДЕЛ: ") {
			active = known[strings.TrimSpace(strings.TrimPrefix(line, "РАЗДЕЛ: "))]
			if active != "" {
				present[active] = true
			}
		} else if active != "" && strings.Contains(strings.ToLower(line), "требует внимания") {
			attention[active] = true
		}
	}
	return len(present), len(attention)
}

func laboratoryShortConclusion(text string) string {
	name := currentLabGroup().Name
	if strings.Contains(text, "анализ ещё не выполнен") || strings.Contains(text, "Заполните доступные показатели") {
		return "Краткий вывод: " + name + " — анализ ещё не выполнен. Нажмите, чтобы раскрыть полный текст."
	}
	if currentLabGroup().Code == "overall" {
		return laboratoryOverallShortText(text)
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, laboratorySummaryPrefix) {
			return "Краткий вывод: " + name + " — " + laboratorySummaryShortMessage(strings.TrimPrefix(line, laboratorySummaryPrefix)) + " Нажмите для полного текста."
		}
	}
	return "Краткий вывод: " + name + " — оценка требует уточнения. Нажмите для полного текста."
}

func laboratoryAttentionPhrase(n int, one, few, many string) string {
	word := many
	lastTwo := n % 100
	last := n % 10
	if lastTwo < 11 || lastTwo > 14 {
		if last == 1 {
			word = one
		} else if last >= 2 && last <= 4 {
			word = few
		}
	}
	verb := "требуют внимания"
	if n == 1 {
		verb = "требует внимания"
	}
	return fmt.Sprintf("%s %d %s", verb, n, word)
}

func laboratoryRecordShortConclusion(r LabRecord) string {
	group, ok := labGroupByCode(r.GroupCode)
	if !ok {
		return r.GroupName + ": оценка требует уточнения"
	}
	values := canonicalizeLaboratoryValues(group, r.Values, r.Units)
	assessment := laboratoryGroupSummary(group, values, r.ScaleModel)
	return group.Name + ": " + laboratorySummaryShortMessage(assessment.Message)
}

func toggleLaboratoryConclusion() {
	atomic := beginLaboratoryAtomicUpdate()
	defer endLaboratoryAtomicUpdate(atomic)
	labConclusionOpen = !labConclusionOpen
	var r RECT
	pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&r)))
	layoutLaboratory(r.Right-r.Left, r.Bottom-r.Top)
}

func normalizeLabHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer("ё", "е", "²", "2", "³", "3", "⁺", "+", "⁻", "-", "_", " ", "-", " ", "/", " ", "(", " ", ")", " ", ",", " ", ".", " ")
	return strings.Join(strings.Fields(r.Replace(s)), " ")
}

func labHeaderIndex(headers []string, aliases ...string) int {
	for i, h := range headers {
		n := normalizeLabHeader(h)
		for _, a := range aliases {
			if n == normalizeLabHeader(a) {
				return i
			}
		}
	}
	return -1
}

func labCell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func normalizeLabDate(s string) string {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64); err == nil && n > 20000 && n < 100000 {
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.Local).AddDate(0, 0, int(n)).Format("02.01.2006")
	}
	for _, layout := range []string{"2006-01-02", "02.01.2006", "02/01/2006", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("02.01.2006")
		}
	}
	if s == "" {
		return time.Now().Format("02.01.2006")
	}
	return s
}

func resolveLabObjectKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return strings.TrimSpace(selectedObjectKey)
	}
	for _, st := range techCurrent {
		if strings.EqualFold(st.ObjectKey, raw) {
			return st.ObjectKey
		}
	}
	var match string
	for _, st := range techCurrent {
		if strings.EqualFold(strings.TrimSpace(st.Well), raw) {
			if match != "" && match != st.ObjectKey {
				return ""
			}
			match = st.ObjectKey
		}
	}
	if match != "" {
		return match
	}
	// Импорт лабораторного бланка не должен зависеть от того, загружен ли
	// сейчас оперативный справочник техрежима. Сохранённый в бланке ключ —
	// самостоятельный идентификатор диагностики; справочник используется для
	// подсказок и проверки дублей, но не блокирует чтение заполненного файла.
	return raw
}

func labGroupFromText(s string) string {
	n := normalizeLabHeader(s)
	for _, g := range labGroups {
		if n == normalizeLabHeader(g.Name) || n == normalizeLabHeader(g.Code) {
			return g.Code
		}
	}
	aliases := map[string]string{"коррозионные": "corrosion", "солеотложения": "scale", "соли": "scale", "аспо": "aspo", "асфальтосмолопарафиновые отложения аспо": "aspo", "асфальтосмолопарафиновые отложения": "aspo", "мехпримеси": "solids", "механические примеси": "solids", "микробиология": "microbiology", "свб": "microbiology", "газы": "gases", "газы и гидраты": "gases", "гидраты": "gases"}
	return aliases[n]
}

func labFieldLookup() map[string]struct{ group, key, unit string } {
	out := map[string]struct{ group, key, unit string }{}
	for _, g := range labGroups {
		for _, f := range g.Fields {
			if laboratoryCalculatedField(g.Code, f.Key) {
				continue
			}
			v := struct{ group, key, unit string }{g.Code, f.Key, f.Unit}
			out[normalizeLabHeader(f.Key)] = v
			out[normalizeLabHeader(f.Label)] = v
			if g.Code == "scale" {
				aliases := map[string][]string{"mg_total": {"Общий магний", "Mg общий"}, "so4_total": {"Общий сульфат", "SO4 общий"}, "ca_total": {"Общий кальций", "Ca общий"}, "na": {"Na⁺", "Натрий"}, "k": {"K⁺", "Калий"}, "tic": {"Общий неорганический углерод", "Общий неорганический углерод (как C)", "TIC"}}
				for _, alias := range aliases[f.Key] {
					out[normalizeLabHeader(alias)] = v
				}
			}
			if g.Code == "aspo" && f.Key == "water" {
				out[normalizeLabHeader("Вода в пробе")] = v
			}
		}
	}
	// A saved demo or exported pass may contain a snapshot of Tech Regime
	// context. It is deliberately not a manual laboratory field, but the import
	// must still preserve it so the infographic can reproduce the saved state.
	watercut := struct{ group, key, unit string }{"corrosion", "watercut", "%"}
	for _, alias := range []string{"watercut", "обводнённость", "обводненность", "обводнённость %"} {
		out[normalizeLabHeader(alias)] = watercut
	}
	return out
}

func chooseLaboratoryFile() string {
	filter := utf16.Encode([]rune("Лабораторные данные (*.xlsx;*.csv;*.txt;*.docx;*.doc)\x00*.xlsx;*.csv;*.txt;*.docx;*.doc\x00Excel (*.xlsx)\x00*.xlsx\x00CSV/TXT (*.csv;*.txt)\x00*.csv;*.txt\x00Word (*.docx;*.doc)\x00*.docx;*.doc\x00Все файлы\x00*.*\x00\x00"))
	buf := make([]uint16, 32768)
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: p("Выберите файл лабораторных анализов"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func decodeLaboratoryUTF16(data []byte, order binary.ByteOrder) string {
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[i*2 : i*2+2])
	}
	return string(utf16.Decode(units))
}

func readLaboratoryDelimited(path string) ([][]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 64*1024*1024 {
		return nil, fmt.Errorf("лабораторный CSV/TXT больше 64 МБ; разделите файл перед импортом")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := ""
	switch {
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe:
		text = decodeLaboratoryUTF16(data[2:], binary.LittleEndian)
	case len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff:
		text = decodeLaboratoryUTF16(data[2:], binary.BigEndian)
	case utf8.Valid(data):
		text = strings.TrimPrefix(string(data), "\ufeff")
	default:
		text = decodeCP1251(data)
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var rows [][]string
	delim := rune(0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if delim == 0 {
			delim = detectDelimiter(line)
		}
		reader := csv.NewReader(strings.NewReader(line))
		reader.Comma = delim
		reader.FieldsPerRecord = -1
		reader.LazyQuotes = true
		record, readErr := reader.Read()
		if readErr != nil {
			return nil, fmt.Errorf("строка %d: %v", lineNo, readErr)
		}
		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}
		rows = append(rows, record)
		if len(rows) > 100000 {
			return nil, fmt.Errorf("более 100 тыс. строк лабораторных данных; разделите файл")
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func readLaboratoryRows(path string) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx":
		return readTelemetryXLSX(path)
	case ".csv", ".txt":
		return readLaboratoryDelimited(path)
	case ".docx":
		return readLaboratoryDOCX(path)
	case ".doc":
		return readLaboratoryDOC(path)
	default:
		return nil, fmt.Errorf("неподдерживаемый формат лабораторного файла; используйте XLSX, CSV, TXT, DOCX или DOC")
	}
}

type LabImportIssue struct {
	Row    int
	Reason string
	Detail string
}

var lastLaboratoryImportLoaded []string
var lastLaboratoryImportSourceFile string

// Object key of the first pass created by the current import.  It is used to
// immediately show the imported pass in the laboratory screen when the user
// has not already selected another object.
var lastLaboratoryImportObjectKey string
var lastLaboratoryImportPassID string

type LabColumnMapping struct {
	HeaderRow int
	Object    int
	Date      int
	Group     int
	Parameter int
	Value     int
	Unit      int
	Source    int
	Comment   int
}

func detectLaboratoryHeaderRow(rows [][]string) int {
	headerRow := 0
	best := -1
	for i := 0; i < len(rows) && i < 20; i++ {
		score := 0
		for _, h := range rows[i] {
			n := normalizeLabHeader(h)
			if n != "" {
				score++
			}
			if strings.Contains(n, "объект") || strings.Contains(n, "скваж") || strings.Contains(n, "показател") || strings.Contains(n, "значен") {
				score += 3
			}
		}
		if score > best {
			best, headerRow = score, i
		}
	}
	return headerRow
}

func detectLaboratoryColumnMapping(rows [][]string) LabColumnMapping {
	m := LabColumnMapping{HeaderRow: detectLaboratoryHeaderRow(rows), Object: -1, Date: -1, Group: -1, Parameter: -1, Value: -1, Unit: -1, Source: -1, Comment: -1}
	if m.HeaderRow < 0 || m.HeaderRow >= len(rows) {
		return m
	}
	head := rows[m.HeaderRow]
	m.Object = labHeaderIndex(head, "ключ объекта", "шифр объекта", "код объекта", "объект", "object key", "object_key", "скважина", "номер скважины", "№ скв")
	m.Date = labHeaderIndex(head, "дата", "дата анализа", "дата исследования", "дата отбора", "date")
	m.Group = labHeaderIndex(head, "раздел", "направление анализа", "тип анализа", "группа", "осложнение")
	m.Parameter = labHeaderIndex(head, "показатель", "показатель исследования", "параметр", "наименование показателя", "наименование параметра")
	m.Value = labHeaderIndex(head, "значение", "полученное значение", "результат", "результат анализа", "value")
	m.Unit = labHeaderIndex(head, "единица", "ед. изм.", "единица измерения", "размерность")
	m.Source = labHeaderIndex(head, "источник", "источник данных", "протокол", "номер протокола", "лаборатория")
	m.Comment = labHeaderIndex(head, "комментарий", "комментарий лаборатории", "примечание", "примечания")
	return m
}

func applyLaboratoryColumnMapping(rows [][]string, m LabColumnMapping) [][]string {
	mapped := make([][]string, len(rows))
	for i := range rows {
		mapped[i] = append([]string(nil), rows[i]...)
	}
	if m.HeaderRow < 0 || m.HeaderRow >= len(mapped) {
		return mapped
	}
	head := mapped[m.HeaderRow]
	canonical := []struct {
		column int
		name   string
	}{
		{m.Object, "Ключ объекта"}, {m.Date, "Дата"}, {m.Group, "Раздел"},
		{m.Parameter, "Показатель"}, {m.Value, "Значение"}, {m.Unit, "Единица"},
		{m.Source, "Источник"}, {m.Comment, "Комментарий"},
	}
	for _, item := range canonical {
		if item.column >= 0 && item.column < len(head) {
			head[item.column] = item.name
		}
	}
	return mapped
}

func laboratoryMappingPreview(path string, rows [][]string, m LabColumnMapping) string {
	mapped := applyLaboratoryColumnMapping(rows, m)
	if m.HeaderRow < 0 || m.HeaderRow >= len(mapped) {
		return "Не удалось определить строку заголовков. Запись ещё не выполнена."
	}
	head := mapped[m.HeaderRow]
	vertical := labHeaderIndex(head, "Показатель") >= 0 && labHeaderIndex(head, "Значение") >= 0
	structure := "широкая таблица"
	if vertical {
		structure = "строки «показатель — значение»"
	}
	dataRows := len(mapped) - m.HeaderRow - 1
	if dataRows < 0 {
		dataRows = 0
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Файл: %s\r\n", filepath.Base(path))
	fmt.Fprintf(&b, "Строка заголовков: %d. Структура: %s. Строк данных: %d.\r\n", m.HeaderRow+1, structure, dataRows)
	b.WriteString("\r\nСОПОСТАВЛЕНИЕ\r\n")
	labels := []struct {
		name string
		col  int
	}{
		{"Ключ объекта", m.Object}, {"Дата", m.Date}, {"Раздел", m.Group},
		{"Показатель", m.Parameter}, {"Значение", m.Value}, {"Единица", m.Unit},
		{"Источник", m.Source}, {"Комментарий", m.Comment},
	}
	for _, item := range labels {
		value := "не сопоставлено"
		if item.col >= 0 && item.col < len(rows[m.HeaderRow]) {
			value = fmt.Sprintf("колонка %d «%s»", item.col+1, rows[m.HeaderRow][item.col])
		}
		fmt.Fprintf(&b, "%s: %s.\r\n", item.name, value)
	}
	b.WriteString("\r\nПЕРВЫЕ СТРОКИ\r\n")
	limit := m.HeaderRow + 6
	if limit > len(rows) {
		limit = len(rows)
	}
	for i := m.HeaderRow + 1; i < limit; i++ {
		fmt.Fprintf(&b, "Строка %d: %s\r\n", i+1, labImportRowDetail(rows[i]))
	}
	b.WriteString("\r\nЗапись ещё не выполнена. Проверьте сопоставление и нажмите «Импортировать». ")
	b.WriteString("Нераспознанные строки будут перечислены в итоговом отчёте.")
	return b.String()
}

func labImportRowDetail(row []string) string {
	var values []string
	for _, value := range row {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
		if len(values) == 4 {
			break
		}
	}
	detail := strings.Join(values, " | ")
	if len([]rune(detail)) > 160 {
		detail = string([]rune(detail)[:157]) + "..."
	}
	return detail
}

func laboratoryImportReport(added, skipped, duplicates int, issues []LabImportIssue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Добавлено новых анализов: %d.\n", added)
	fmt.Fprintf(&b, "Повторных анализов распознано, но не продублировано: %d.\n", duplicates)
	fmt.Fprintf(&b, "Пропущено строк: %d.\n", skipped)
	if len(lastLaboratoryImportLoaded) > 0 {
		b.WriteString("\nЗАГРУЖЕНО:\n")
		limit := len(lastLaboratoryImportLoaded)
		for _, line := range lastLaboratoryImportLoaded[:limit] {
			b.WriteString("• " + line + "\n")
		}
		if len(lastLaboratoryImportLoaded) > limit {
			fmt.Fprintf(&b, "Ещё загруженных строк вне окна отчёта: %d.\n", len(lastLaboratoryImportLoaded)-limit)
		}
	}
	if len(issues) > 0 {
		b.WriteString("\nНЕ ЗАГРУЖЕНО / ТРЕБУЕТ ИСПРАВЛЕНИЯ\n")
		limit := len(issues)
		for _, issue := range issues[:limit] {
			fmt.Fprintf(&b, "Строка %d: %s", issue.Row, issue.Reason)
			if issue.Detail != "" {
				fmt.Fprintf(&b, " — %s", issue.Detail)
			}
			b.WriteString(".\n")
		}
		if len(issues) > limit {
			fmt.Fprintf(&b, "Ещё строк вне окна отчёта: %d.\n", len(issues)-limit)
		}
	}
	b.WriteString("\nПредыдущие лабораторные записи сохранены.")
	return b.String()
}

func laboratoryImportUnitAllowed(groupCode, key, unit string) bool {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return true
	}
	for _, group := range labGroups {
		if group.Code != groupCode {
			continue
		}
		for _, field := range group.Fields {
			if field.Key != key {
				continue
			}
			for _, candidate := range laboratoryUnitOptions(field) {
				if strings.EqualFold(strings.TrimSpace(candidate), unit) {
					return true
				}
			}
			return false
		}
	}
	return true
}

func laboratoryImportValueShouldBeNumeric(unit string) bool {
	n := normalizeLabHeader(unit)
	return n != "текст" && n != "результат" && n != "по протоколу"
}

func importLaboratoryRowsDetailed(rows [][]string, forcedHeaderRow int) (int, int, int, []LabImportIssue, error) {
	lastLaboratoryImportLoaded = nil
	lastLaboratoryImportObjectKey = ""
	lastLaboratoryImportPassID = ""
	if len(rows) < 2 {
		return 0, 0, 0, nil, fmt.Errorf("в файле нет строк лабораторных данных")
	}
	headerRow := forcedHeaderRow
	if headerRow < 0 || headerRow >= len(rows) {
		headerRow = detectLaboratoryHeaderRow(rows)
	}
	head := rows[headerRow]
	objectCol := labHeaderIndex(head, "ключ объекта", "шифр объекта", "код объекта", "объект", "object key", "object_key", "скважина", "номер скважины", "№ скв")
	dateCol := labHeaderIndex(head, "дата", "дата анализа", "дата исследования", "дата отбора", "date")
	groupCol := labHeaderIndex(head, "раздел", "направление анализа", "тип анализа", "группа", "осложнение")
	paramCol := labHeaderIndex(head, "показатель", "показатель исследования", "параметр", "наименование показателя", "наименование параметра")
	valueCol := labHeaderIndex(head, "значение", "полученное значение", "результат", "результат анализа", "value")
	unitCol := labHeaderIndex(head, "единица", "ед. изм.", "единица измерения", "размерность")
	sourceCol := labHeaderIndex(head, "источник", "источник данных", "протокол", "номер протокола", "лаборатория")
	commentCol := labHeaderIndex(head, "комментарий", "комментарий лаборатории", "примечание", "примечания")
	fields := labFieldLookup()
	type aggregate struct {
		key, date, group, source, comment string
		values, units                     map[string]string
		extra                             map[string]string
	}
	agg := map[string]*aggregate{}
	skipped := 0
	var issues []LabImportIssue
	for offset, row := range rows[headerRow+1:] {
		rowNo := headerRow + offset + 2
		objectRaw := labCell(row, objectCol)
		objectKey := resolveLabObjectKey(objectRaw)
		if objectKey == "" {
			skipped++
			issues = append(issues, LabImportIssue{Row: rowNo, Reason: "не распознан или неоднозначен ключ объекта", Detail: labImportRowDetail(row)})
			continue
		}
		dateRaw := strings.TrimSpace(labCell(row, dateCol))
		date := normalizeLabDate(dateRaw)
		source, comment := labCell(row, sourceCol), labCell(row, commentCol)
		// The visible source identifies the imported file. Preserve a protocol
		// written in the sheet in the comment, so neither provenance is lost.
		if strings.TrimSpace(lastLaboratoryImportSourceFile) != "" {
			filename := filepath.Base(lastLaboratoryImportSourceFile)
			if strings.TrimSpace(source) != "" && !strings.EqualFold(strings.TrimSpace(source), filename) {
				comment = strings.TrimSpace("Протокол в бланке: " + strings.TrimSpace(source) + ". " + comment)
			}
			source = filename
		}
		add := func(group, key, value, unit string) bool {
			if group == "" || key == "" || strings.TrimSpace(value) == "" {
				return false
			}
			// Source and comment are metadata of the same object/date/group analysis.
			// They are often filled only once in a vertical export and therefore must
			// not split its indicator rows into separate records.
			base := objectKey + "|" + date + "|" + group
			a := agg[base]
			if a == nil {
				a = &aggregate{objectKey, date, group, source, comment, map[string]string{}, map[string]string{}, map[string]string{}}
				agg[base] = a
			}
			// In vertical laboratory exports the comment or source may be filled
			// only on one of several indicator rows. Preserve the first non-empty
			// value instead of losing it when the first row is blank.
			if a.source == "" && source != "" {
				a.source = source
			}
			if a.comment == "" && comment != "" {
				a.comment = comment
			}
			a.values[key] = strings.TrimSpace(value)
			a.units[key] = strings.TrimSpace(unit)
			return true
		}
		if paramCol >= 0 && valueCol >= 0 {
			parameter := labCell(row, paramCol)
			f, ok := fields[normalizeLabHeader(parameter)]
			if !ok {
				// Частые лабораторные варианты написания одного показателя.
				parameterAliases := map[string]string{
					"диоксид углерода co2": "Растворённый CO₂",
					"сероводород h2s":      "Растворённый H₂S",
					"кислород o2":          "Растворённый O₂",
					"твердая фаза асфальтосмолопарафиновых отложений аспо": "Твёрдая фаза АСПО",
				}
				if canonical, exists := parameterAliases[normalizeLabHeader(parameter)]; exists {
					parameter = canonical
					f, ok = fields[normalizeLabHeader(parameter)]
				}
			}
			if !ok {
				// Common source names use a chemical name instead of the
				// canonical short label. Try all equivalent labels first.
				parameterAliases := map[string]string{
					"диоксид углерода co2": "CO₂", "сероводород h2s": "H₂S",
					"кислород o2": "O₂", "метан ch4": "CH₄",
					"твердая фаза асфальтосмолопарафиновых отложений аспо": "Твёрдая фаза АСПО",
				}
				if canonical, exists := parameterAliases[normalizeLabHeader(parameter)]; exists {
					parameter = canonical
					f, ok = fields[normalizeLabHeader(parameter)]
				}
			}
			groupRaw := labCell(row, groupCol)
			group := labGroupFromText(groupRaw)
			if groupRaw != "" && group == "" {
				skipped++
				issues = append(issues, LabImportIssue{Row: rowNo, Reason: "не распознан раздел «" + groupRaw + "»", Detail: labImportRowDetail(row)})
				continue
			}
			if group == "" {
				if ok {
					group = f.group
				} else {
					group = "imported_extra"
				}
			}
			unit := labCell(row, unitCol)
			if unit == "" && ok {
				unit = f.unit
			}
			value := strings.TrimSpace(labCell(row, valueCol))
			if dateRaw == "" {
				skipped++
				issues = append(issues, LabImportIssue{Row: rowNo, Reason: "не указана дата анализа", Detail: labImportRowDetail(row)})
				continue
			}
			if ok && !laboratoryImportUnitAllowed(group, f.key, unit) {
				skipped++
				issues = append(issues, LabImportIssue{Row: rowNo, Reason: "единица «" + unit + "» не подходит показателю «" + parameter + "»", Detail: labImportRowDetail(row)})
				continue
			}
			if value != "" && ok && laboratoryImportValueShouldBeNumeric(unit) {
				if _, ok := parseLabNumber(value); !ok {
					skipped++
					issues = append(issues, LabImportIssue{Row: rowNo, Reason: "значение «" + value + "» не удалось распознать как число", Detail: labImportRowDetail(row)})
					continue
				}
			}
			if !ok {
				if value == "" {
					skipped++
					issues = append(issues, LabImportIssue{Row: rowNo, Reason: "не заполнено значение дополнительного показателя «" + parameter + "»", Detail: labImportRowDetail(row)})
					continue
				}
				base := objectKey + "|" + date + "|" + group
				a := agg[base]
				if a == nil {
					a = &aggregate{objectKey, date, group, source, comment, map[string]string{}, map[string]string{}, map[string]string{}}
					agg[base] = a
				}
				if a.extra == nil {
					a.extra = map[string]string{}
				}
				a.extra[strings.TrimSpace(parameter)] = strings.TrimSpace(value + func() string {
					if unit != "" {
						return " " + unit
					}
					return ""
				}())
				lastLaboratoryImportLoaded = append(lastLaboratoryImportLoaded, fmt.Sprintf("%s · %s = %s %s (дополнительный показатель сохранён без классификации)", objectKey, parameter, value, unit))
				continue
			}
			if !add(group, f.key, value, unit) {
				skipped++
				issues = append(issues, LabImportIssue{Row: rowNo, Reason: "не заполнено значение показателя «" + parameter + "»", Detail: labImportRowDetail(row)})
			}
			if value != "" {
				lastLaboratoryImportLoaded = append(lastLaboratoryImportLoaded, fmt.Sprintf("%s · %s = %s %s", objectKey, parameter, value, unit))
			}
			continue
		}
		rowAdded := false
		for col, h := range head {
			f, ok := fields[normalizeLabHeader(h)]
			if ok {
				rowAdded = add(f.group, f.key, labCell(row, col), f.unit) || rowAdded
			}
		}
		if !rowAdded {
			skipped++
			issues = append(issues, LabImportIssue{Row: rowNo, Reason: "нет заполненных распознаваемых показателей", Detail: labImportRowDetail(row)})
		}
	}
	// RC153: new imports are aggregated into one laboratory pass per
	// object/date/source instead of creating one independent history row per tab.
	// Legacy LabRecord files remain readable but are no longer the target format.
	type importedPassBucket struct {
		pass LabPassRecord
	}
	buckets := map[string]*importedPassBucket{}
	for _, a := range agg {
		bucketKey := normalizeLaboratoryObjectKey(a.key) + "|" + a.date + "|" + strings.TrimSpace(a.source)
		b := buckets[bucketKey]
		if b == nil {
			b = &importedPassBucket{pass: LabPassRecord{
				ObjectKey: a.key, Date: a.date, Source: strings.TrimSpace(a.source), Comment: strings.TrimSpace(a.comment),
				Status: labPassSavedStatus, Groups: map[string]map[string]string{}, Units: map[string]map[string]string{}, ScaleModel: "davies", ExtraValues: map[string]string{},
			}}
			buckets[bucketKey] = b
		}
		if b.pass.Groups[a.group] == nil {
			b.pass.Groups[a.group] = map[string]string{}
		}
		if b.pass.Units[a.group] == nil {
			b.pass.Units[a.group] = map[string]string{}
		}
		for key, value := range a.values {
			b.pass.Groups[a.group][key] = value
		}
		for key, unit := range a.units {
			b.pass.Units[a.group][key] = unit
		}
		for key, value := range a.extra {
			b.pass.ExtraValues[key] = value
		}
		if b.pass.Comment == "" && strings.TrimSpace(a.comment) != "" {
			b.pass.Comment = strings.TrimSpace(a.comment)
		}
	}
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	havePass := map[string]bool{}
	for _, p := range labPasses {
		havePass[p.PassID] = true
	}
	added := 0
	duplicates := 0
	now := time.Now()
	for _, bucket := range buckets {
		pass := bucket.pass
		pass.ImportedAt = now.Format(time.RFC3339)
		if strings.TrimSpace(lastLaboratoryImportSourceFile) != "" {
			pass.ImportFileName = filepath.Base(lastLaboratoryImportSourceFile)
		}
		pass.SharedValues = collectLaboratorySharedValues(pass.Groups)
		if watercut := strings.TrimSpace(pass.SharedValues["watercut"]); watercut != "" {
			seedLaboratoryWatercutFromTech(pass.Groups, watercut)
			pass.SharedValues = collectLaboratorySharedValues(pass.Groups)
		}
		pass.Completed = completedLabGroupsForModel(pass.Groups, pass.ScaleModel)
		conflicts := laboratorySharedValueConflictsWithUnits(pass.Groups, pass.Units)
		if len(conflicts) > 0 {
			issues = append(issues, LabImportIssue{Reason: "расхождение общих показателей между вкладками", Detail: strings.Join(conflicts, "; ")})
			pass.Comment = strings.TrimSpace(strings.TrimSpace(pass.Comment) + " | Требуется сверка общих показателей: " + strings.Join(conflicts, "; "))
		}
		pass.PassID = laboratoryImportedPassID(pass)
		if havePass[pass.PassID] {
			duplicates++
			// Preserve the original values while recording which file was imported
			// again. Older saved passes gain provenance without a duplicate row.
			for i := range labPasses {
				if labPasses[i].PassID == pass.PassID && pass.ImportFileName != "" {
					labPasses[i].ImportFileName = pass.ImportFileName
					labPasses[i].ImportedAt = pass.ImportedAt
					labPasses[i].UpdatedAt = now.UTC().Format(time.RFC3339)
					if err := appendLabPass(labPasses[i]); err != nil {
						return added, skipped, duplicates, issues, err
					}
					break
				}
			}
			// A repeated upload must still be transferable to the visible fields.
			// The pass already exists, so reuse it without creating another record.
			lastLaboratoryImportPassID = pass.PassID
			if lastLaboratoryImportObjectKey == "" {
				lastLaboratoryImportObjectKey = pass.ObjectKey
			}
			continue
		}
		pass.Conclusion = buildLaboratoryPassConclusion(pass)
		if err := pass.prepare(now); err != nil {
			return added, skipped, duplicates, issues, err
		}
		if err := appendLabPass(pass); err != nil {
			return added, skipped, duplicates, issues, err
		}
		labPasses = append(labPasses, pass)
		lastLaboratoryImportPassID = pass.PassID
		if lastLaboratoryImportObjectKey == "" {
			lastLaboratoryImportObjectKey = pass.ObjectKey
		}
		havePass[pass.PassID] = true
		added++
	}
	if added == 0 && skipped == 0 && duplicates == 0 {
		return 0, 0, 0, issues, fmt.Errorf("не найдены распознаваемые лабораторные показатели")
	}
	if added > 0 {
		invalidateMethodsBackground()
	}
	return added, skipped, duplicates, issues, nil
}

func importLaboratoryFileDetailed(path string) (int, int, int, []LabImportIssue, error) {
	rows, err := readLaboratoryRows(path)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	return importLaboratoryRowsDetailed(rows, -1)
}

func importLaboratoryFileWithMapping(path string, mapping LabColumnMapping) (int, int, int, []LabImportIssue, error) {
	rows, err := readLaboratoryRows(path)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	return importLaboratoryRowsDetailed(applyLaboratoryColumnMapping(rows, mapping), mapping.HeaderRow)
}

func importLaboratoryFile(path string) (int, int, error) {
	added, skipped, _, _, err := importLaboratoryFileDetailed(path)
	return added, skipped, err
}

const (
	labCBAddString  = 0x0143
	labCBGetCurSel  = 0x0147
	labCBSetCurSel  = 0x014E
	labCBSDropList  = 0x0003
	labMappingClose = 0x0010
)

var (
	labIncompleteDialogDone     bool
	labIncompleteDialogContinue bool
	labIncompleteDialogText     string
)

var labMappingFieldLabels = []string{
	"Ключ объекта", "Дата", "Раздел", "Показатель",
	"Значение", "Единица", "Источник", "Комментарий",
}

func laboratoryMappingColumns(m *LabColumnMapping) []*int {
	return []*int{&m.Object, &m.Date, &m.Group, &m.Parameter, &m.Value, &m.Unit, &m.Source, &m.Comment}
}

func populateLaboratoryMappingCombo(combo syscall.Handle, headers []string, selected int) {
	pSendMessageW.Call(uintptr(combo), labCBAddString, 0, uintptr(unsafe.Pointer(p("(не сопоставлено)"))))
	for i, header := range headers {
		caption := fmt.Sprintf("%d — %s", i+1, strings.TrimSpace(header))
		if strings.TrimSpace(header) == "" {
			caption = fmt.Sprintf("%d — (пустой заголовок)", i+1)
		}
		pSendMessageW.Call(uintptr(combo), labCBAddString, 0, uintptr(unsafe.Pointer(p(caption))))
	}
	selection := selected + 1
	if selected < 0 || selected >= len(headers) {
		selection = 0
	}
	pSendMessageW.Call(uintptr(combo), labCBSetCurSel, uintptr(selection), 0)
}

func readLaboratoryMappingDialog() LabColumnMapping {
	m := labMappingDialogValue
	for i, column := range laboratoryMappingColumns(&m) {
		selected, _, _ := pSendMessageW.Call(uintptr(labMappingDialogCombos[i]), labCBGetCurSel, 0, 0)
		*column = int(int32(selected)) - 1
	}
	return m
}

func validateLaboratoryMapping(rows [][]string, m LabColumnMapping) error {
	if m.HeaderRow < 0 || m.HeaderRow >= len(rows) {
		return fmt.Errorf("не определена строка заголовков")
	}
	used := map[int]string{}
	for i, column := range laboratoryMappingColumns(&m) {
		if *column < 0 {
			continue
		}
		if previous, exists := used[*column]; exists {
			return fmt.Errorf("одна колонка не может одновременно означать «%s» и «%s»", previous, labMappingFieldLabels[i])
		}
		used[*column] = labMappingFieldLabels[i]
	}
	if (m.Parameter >= 0) != (m.Value >= 0) {
		return fmt.Errorf("для вертикальной формы сопоставьте одновременно колонки «Показатель» и «Значение»")
	}
	if m.Parameter < 0 {
		recognized := 0
		fields := labFieldLookup()
		for _, header := range rows[m.HeaderRow] {
			if _, ok := fields[normalizeLabHeader(header)]; ok {
				recognized++
			}
		}
		if recognized == 0 {
			return fmt.Errorf("не найдены колонки показателей: сопоставьте «Показатель» и «Значение» либо используйте узнаваемые названия показателей в широкой таблице")
		}
	}
	if m.Object < 0 && strings.TrimSpace(selectedObjectKey) == "" {
		return fmt.Errorf("сопоставьте колонку «Ключ объекта» или сначала выберите текущий объект")
	}
	return nil
}

func updateLaboratoryMappingPreview() {
	labMappingDialogValue = readLaboratoryMappingDialog()
	text := laboratoryMappingPreview(labMappingDialogPath, labMappingDialogRows, labMappingDialogValue)
	if err := validateLaboratoryMapping(labMappingDialogRows, labMappingDialogValue); err != nil {
		text += "\r\n\r\nНУЖНО ИСПРАВИТЬ\r\n" + err.Error()
	}
	setText(labMappingDialogPreview, text)
}

func laboratoryMappingWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		modalCreate("STATIC", "Сопоставьте поля нестандартной формы. Автоматически найденные варианты уже выбраны.", WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 16, 970, 24, syscall.Handle(hwnd), 0)
		modalCreate("STATIC", "Поле формы", WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 50, 155, 22, syscall.Handle(hwnd), 0)
		modalCreate("STATIC", "Колонка файла", WS_CHILD|WS_VISIBLE|SS_LEFT, 184, 50, 286, 22, syscall.Handle(hwnd), 0)
		modalCreate("STATIC", "Предварительный просмотр — данные ещё не записаны", WS_CHILD|WS_VISIBLE|SS_LEFT, 500, 50, 500, 22, syscall.Handle(hwnd), 0)
		headers := labMappingDialogRows[labMappingDialogValue.HeaderRow]
		columns := laboratoryMappingColumns(&labMappingDialogValue)
		for i, labelText := range labMappingFieldLabels {
			y := int32(78 + i*50)
			modalCreate("STATIC", labelText, WS_CHILD|WS_VISIBLE|SS_LEFT, 22, y+6, 155, 24, syscall.Handle(hwnd), 0)
			labMappingDialogCombos[i] = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|labCBSDropList, 184, y, 286, 220, syscall.Handle(hwnd), 260+i)
			populateLaboratoryMappingCombo(labMappingDialogCombos[i], headers, *columns[i])
		}
		labMappingDialogPreview = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL|ES_READONLY, 500, 78, 500, 400, syscall.Handle(hwnd), 0)
		modalCreate("BUTTON", "Обновить просмотр", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 500, 500, 170, 38, syscall.Handle(hwnd), ID_LAB_MAP_PREVIEW)
		modalCreate("BUTTON", "Перенести в программу", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 678, 500, 190, 38, syscall.Handle(hwnd), ID_LAB_MAP_ACCEPT)
		modalCreate("BUTTON", "Отмена", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 878, 500, 122, 38, syscall.Handle(hwnd), ID_LAB_MAP_CANCEL)
		updateLaboratoryMappingPreview()
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case ID_LAB_MAP_PREVIEW:
			updateLaboratoryMappingPreview()
			return 0
		case ID_LAB_MAP_ACCEPT:
			labMappingDialogValue = readLaboratoryMappingDialog()
			if err := validateLaboratoryMapping(labMappingDialogRows, labMappingDialogValue); err != nil {
				pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(p(err.Error()))), uintptr(unsafe.Pointer(p("Сопоставление колонок"))), MB_OK|MB_ICONWARNING)
				updateLaboratoryMappingPreview()
				return 0
			}
			labMappingDialogAccepted = true
			labMappingDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case ID_LAB_MAP_CANCEL:
			labMappingDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		labMappingDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func chooseLaboratoryColumnMapping(path string, rows [][]string) (LabColumnMapping, bool) {
	if len(rows) < 2 {
		return LabColumnMapping{}, false
	}
	labMappingDialogRows = rows
	labMappingDialogPath = path
	labMappingDialogValue = detectLaboratoryColumnMapping(rows)
	labMappingDialogDone = false
	labMappingDialogAccepted = false
	cls := p("ROSALaboratoryMappingDialogV1")
	hInstRaw, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(laboratoryMappingWndProc), HInstance: syscall.Handle(hInstRaw), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — просмотр и сопоставление лабораторного импорта"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 1040, 610, uintptr(hMain), 0, hInstRaw, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return LabColumnMapping{}, false
	}
	var message MSG
	for !labMappingDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return labMappingDialogValue, labMappingDialogAccepted
}

func chooseLaboratoryTemplatePath(defaultName string) string {
	filter := utf16.Encode([]rune("Excel (*.xlsx)\x00*.xlsx\x00Все файлы (*.*)\x00*.*\x00\x00"))
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(defaultName)))
	of := OPENFILENAMEW{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain,
		LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)),
		LpstrTitle: p("Сохранить штатный бланк лабораторного анализа"),
		Flags:      OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT, LpstrDefExt: p("xlsx"),
	}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	return path
}

func downloadLaboratoryTemplate() {
	path := chooseLaboratoryTemplatePath("РОСА_БЛАНК_ЛАБОРАТОРНОГО_АНАЛИЗА.xlsx")
	if path == "" {
		return
	}
	if len(laboratoryBlankTemplateXLSX) == 0 {
		msg("Штатный бланк", "Встроенный бланк недоступен.", MB_OK|MB_ICONERROR)
		return
	}
	if err := os.WriteFile(path, laboratoryBlankTemplateXLSX, 0o600); err != nil {
		msg("Штатный бланк", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Штатный бланк", "Бланк сохранён. Заполните значения, не меняя названия показателей и столбцов, затем загрузите файл кнопкой «Загрузить анализы».", MB_OK|MB_ICONINFORMATION)
}

func isOfficialLaboratoryTemplate(rows [][]string, m LabColumnMapping) bool {
	if m.HeaderRow < 0 || m.HeaderRow >= len(rows) {
		return false
	}
	head := rows[m.HeaderRow]
	// RC97+ blanks begin with the human-readable status column and may include
	// a subsection column. Import is still keyed by the canonical seven fields.
	statusOK := len(head) > 0 && normalizeLabHeader(head[0]) == "статус показателя"
	// Older distributed blanks have the same eight canonical columns without
	// the status column. Their empty indicator rows need the same treatment.
	canonical := m.Date >= 0 && m.Group >= 0 && m.Parameter >= 0 && m.Value >= 0 && m.Unit >= 0 && m.Source >= 0 && m.Comment >= 0
	if !statusOK {
		canonical = canonical && m.Object >= 0
	}
	return canonical && (statusOK || normalizeLabHeader(head[0]) == "ключ объекта")
}

func officialLaboratoryBaselineCoverage(rows [][]string, m LabColumnMapping) (filled, total int) {
	if !isOfficialLaboratoryTemplate(rows, m) || m.HeaderRow < 0 || m.HeaderRow >= len(rows) {
		return 0, 0
	}
	head := rows[m.HeaderRow]
	statusCol := labHeaderIndex(head, "статус показателя")
	if statusCol < 0 || m.Value < 0 {
		return 0, 0
	}
	for _, row := range rows[m.HeaderRow+1:] {
		status := strings.ToLower(strings.TrimSpace(labCell(row, statusCol)))
		if !strings.HasPrefix(status, "базовый") {
			continue
		}
		total++
		if strings.TrimSpace(labCell(row, m.Value)) != "" {
			filled++
		}
	}
	return filled, total
}

func laboratoryIncompleteImportWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		modalCreate("STATIC", "Неполные базовые данные", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 20, 520, 30, syscall.Handle(hwnd), 0)
		info := modalCreate("STATIC", labIncompleteDialogText, WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 60, 520, 86, syscall.Handle(hwnd), 0)
		if info != 0 && hFont != 0 {
			pSendMessageW.Call(uintptr(info), WM_SETFONT, uintptr(hFont), 1)
		}
		modalCreate("BUTTON", "Продолжить загрузку", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 24, 164, 238, 42, syscall.Handle(hwnd), ID_LAB_IMPORT_CONTINUE)
		modalCreate("BUTTON", "Вернуться к заполнению", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 280, 164, 264, 42, syscall.Handle(hwnd), ID_LAB_IMPORT_RETURN)
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case ID_LAB_IMPORT_CONTINUE:
			labIncompleteDialogContinue = true
			labIncompleteDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case ID_LAB_IMPORT_RETURN:
			labIncompleteDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		labIncompleteDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func confirmIncompleteLaboratoryImport(filled, total int) bool {
	if total <= 0 || filled >= total {
		return true
	}
	labIncompleteDialogText = fmt.Sprintf("Заполнены не все базовые показатели: %d из %d. Анализ можно загрузить, но результаты будут неполными и могут потребовать уточнения. Продолжить загрузку?", filled, total)
	labIncompleteDialogDone = false
	labIncompleteDialogContinue = false
	cls := p("ROSALaboratoryIncompleteImportDialogV1")
	hInstRaw, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(laboratoryIncompleteImportWndProc), HInstance: syscall.Handle(hInstRaw), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — проверка полноты анализа"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 590, 270, uintptr(hMain), 0, hInstRaw, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return false
	}
	var message MSG
	for !labIncompleteDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return labIncompleteDialogContinue
}

func normalizeOfficialLaboratoryTemplateRows(rows [][]string, m LabColumnMapping) [][]string {
	if !isOfficialLaboratoryTemplate(rows, m) {
		return rows
	}
	// The blank contains every declared laboratory indicator. Empty rows are
	// intentionally ignored, not reported as import errors. One date/source may
	// be entered once and is propagated to all filled rows of the same file.
	defaultDate := ""
	defaultSource := ""
	for _, row := range rows[m.HeaderRow+1:] {
		if defaultDate == "" {
			defaultDate = labCell(row, m.Date)
		}
		if defaultSource == "" {
			defaultSource = labCell(row, m.Source)
		}
	}
	if strings.TrimSpace(defaultDate) == "" {
		defaultDate = strings.TrimSpace(getText(labDateEdit))
		if defaultDate == "" {
			defaultDate = time.Now().Format("02.01.2006")
		}
	}
	if strings.TrimSpace(defaultSource) == "" {
		defaultSource = strings.TrimSpace(getText(labSourceEdit))
		if defaultSource == "" {
			defaultSource = "Импортированный лабораторный бланк"
		}
	}
	out := append([][]string(nil), rows[:m.HeaderRow+1]...)
	for _, original := range rows[m.HeaderRow+1:] {
		if strings.TrimSpace(labCell(original, m.Value)) == "" {
			continue
		}
		row := append([]string(nil), original...)
		for len(row) <= m.Comment {
			row = append(row, "")
		}
		if m.Date >= 0 && strings.TrimSpace(labCell(row, m.Date)) == "" {
			row[m.Date] = defaultDate
		}
		if m.Source >= 0 && strings.TrimSpace(labCell(row, m.Source)) == "" {
			row[m.Source] = defaultSource
		}
		out = append(out, row)
	}
	return out
}

func importLaboratoryUI() {
	path := chooseLaboratoryFile()
	if path == "" {
		return
	}
	rows, err := readLaboratoryRows(path)
	if err != nil {
		msg("Импорт лабораторных данных", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	lastLaboratoryImportSourceFile = path
	defer func() { lastLaboratoryImportSourceFile = "" }()
	setText(labSourceEdit, filepath.Base(path))
	mapping := detectLaboratoryColumnMapping(rows)
	var mappedRows [][]string
	headerRow := mapping.HeaderRow
	if isOfficialLaboratoryTemplate(rows, mapping) {
		if mapping.Object < 0 && strings.TrimSpace(selectedObjectKey) == "" {
			msg("Импорт лабораторных данных", "Сначала выберите и закрепите объект на Главной. Штатный бланк не содержит ключ объекта и относится к выбранному объекту.", MB_OK|MB_ICONWARNING)
			return
		}
		// Empty indicators are absence of data, not an import error. If the baseline
		// set is incomplete, ask whether the technologist wants to continue with a
		// deliberately incomplete analysis instead of rejecting the workbook.
		baseFilled, baseTotal := officialLaboratoryBaselineCoverage(rows, mapping)
		if !confirmIncompleteLaboratoryImport(baseFilled, baseTotal) {
			return
		}
		// Штатный бланк имеет фиксированную схему: ручное сопоставление не требуется.
		mappedRows = normalizeOfficialLaboratoryTemplateRows(rows, mapping)
		if mapping.Object < 0 {
			mappedRows[headerRow] = append(mappedRows[headerRow], "Ключ объекта")
			for i := headerRow + 1; i < len(mappedRows); i++ {
				mappedRows[i] = append(mappedRows[i], selectedObjectKey)
			}
		}
	} else {
		var accepted bool
		mapping, accepted = chooseLaboratoryColumnMapping(path, rows)
		if !accepted {
			return
		}
		mappedRows = applyLaboratoryColumnMapping(rows, mapping)
		headerRow = mapping.HeaderRow
	}
	added, skipped, duplicates, issues, err := importLaboratoryRowsDetailed(mappedRows, headerRow)
	if err != nil {
		msg("Импорт лабораторных данных", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	// If no object was selected before the import, select the object just
	// created so that the newly transferred pass is visible immediately.
	if strings.TrimSpace(selectedObjectKey) == "" && strings.TrimSpace(lastLaboratoryImportObjectKey) != "" {
		selectedObjectKey = lastLaboratoryImportObjectKey
	}
	refreshLaboratory()
	openLaboratoryImportReportDialog("Отчёт импорта лабораторных данных", laboratoryImportReport(added, skipped, duplicates, issues))
}

func applyLaboratorySourceToPass(pass *LabPassRecord, source string) {
	if pass == nil {
		return
	}
	source = strings.TrimSpace(source)
	// A stray bracket/caret from malformed spreadsheet exports is not a source
	// value and must not appear in the passport line.
	if strings.Trim(strings.TrimSpace(source), "[](){}") == "" {
		source = ""
	}
	pass.Source = source
}

// handleLaboratorySourceChange keeps the pass metadata in sync while the user
// types. It deliberately updates only the in-memory draft; the append-only store
// is written by the normal draft/save path, not on every keystroke.
func handleLaboratorySourceChange() {
	if labSourceEdit == 0 || labObjectSwitching {
		return
	}
	pass := ensureActiveLabPass()
	applyLaboratorySourceToPass(pass, getText(labSourceEdit))
	updateLaboratoryReadinessPanel()
}

func saveLabDraft() {
	g := currentLabGroup()
	m := labDrafts[g.Code]
	if m == nil {
		m = map[string]string{}
	}
	start := labFieldsPage * 10
	for i := 0; i < 10 && start+i < len(g.Fields); i++ {
		m[g.Fields[start+i].Key] = getText(labFieldEdits[i])
	}
	labDrafts[g.Code] = m
	labUnitDrafts[g.Code] = currentLaboratoryUnits()
	pass := ensureActiveLabPass()
	pass.Groups = cloneLabGroups(labDrafts)
	pass.Units = cloneLabGroups(labUnitDrafts)
	pass.ScaleModel = labScaleModelCode
	applyLaboratorySourceToPass(pass, getText(labSourceEdit))
	pass.Comment = strings.TrimSpace(getText(labCommentEdit))
	pass.Date = strings.TrimSpace(getText(labDateEdit))
	pass.SharedValues = collectLaboratorySharedValues(labDrafts)
	_ = pass.prepare(time.Now())
	if strings.TrimSpace(pass.ObjectKey) != "" {
		_ = appendLabPass(*pass)
	}
}

func ensureActiveLabPass() *LabPassRecord {
	objectKey := strings.TrimSpace(getText(labObjectEdit))
	key := normalizeLaboratoryObjectKey(objectKey)
	if labActivePassID != "" {
		if pass := labPassDrafts[key+"|"+labActivePassID]; pass != nil {
			return pass
		}
		// The object changed. Never seed its pass with values from the previous
		// object; restore an existing draft for this object or start empty.
		for draftKey, pass := range labPassDrafts {
			if strings.HasPrefix(draftKey, key+"|") {
				labActivePassID = pass.PassID
				labDrafts = cloneLabGroups(pass.Groups)
				labUnitDrafts = cloneLabGroups(pass.Units)
				setLaboratoryScaleModel(pass.ScaleModel)
				return pass
			}
		}
		if !labPassesLoaded {
			labPasses = loadLabPasses()
			labPassesLoaded = true
		}
		for i := range labPasses {
			pass := labPasses[i]
			if normalizeLaboratoryObjectKey(pass.ObjectKey) == key && pass.Status == labPassDraftStatus {
				labActivePassID = pass.PassID
				labDrafts = cloneLabGroups(pass.Groups)
				labUnitDrafts = cloneLabGroups(pass.Units)
				setLaboratoryScaleModel(pass.ScaleModel)
				labPassDrafts[key+"|"+pass.PassID] = &pass
				return &pass
			}
		}
		labActivePassID = ""
		labDrafts = map[string]map[string]string{}
		labUnitDrafts = map[string]map[string]string{}
		setLaboratoryScaleModel("davies")
	}
	if key != "" {
		if !labPassesLoaded {
			labPasses = loadLabPasses()
			labPassesLoaded = true
		}
		for i := range labPasses {
			pass := labPasses[i]
			if normalizeLaboratoryObjectKey(pass.ObjectKey) == key && pass.Status == labPassDraftStatus {
				labActivePassID = pass.PassID
				labDrafts = cloneLabGroups(pass.Groups)
				labUnitDrafts = cloneLabGroups(pass.Units)
				setLaboratoryScaleModel(pass.ScaleModel)
				labPassDrafts[key+"|"+pass.PassID] = &pass
				return &pass
			}
		}
	}
	now := time.Now()
	pass := &LabPassRecord{ObjectKey: objectKey, Date: strings.TrimSpace(getText(labDateEdit)), Source: strings.TrimSpace(getText(labSourceEdit)), Status: labPassDraftStatus, Groups: cloneLabGroups(labDrafts), Units: cloneLabGroups(labUnitDrafts), ScaleModel: labScaleModelCode}
	_ = pass.prepare(now)
	labActivePassID = pass.PassID
	labPassDrafts[key+"|"+pass.PassID] = pass
	return pass
}

// Laboratory modes update captions, positions and logical visibility together.
// Page activation already owns a transition, so this batch only runs for a
// visible laboratory page.
func beginLaboratoryAtomicUpdate() bool {
	if hMain == 0 || currentPage != ID_LAB || pageSwitching {
		return false
	}
	labAtomicUpdateDepth++
	if labAtomicUpdateDepth == 1 {
		beginPageVisualUpdate(ID_LAB)
	}
	return true
}

func endLaboratoryAtomicUpdate(started bool) {
	if !started || labAtomicUpdateDepth <= 0 {
		return
	}
	labAtomicUpdateDepth--
	if labAtomicUpdateDepth != 0 {
		return
	}
	endPageVisualUpdate(ID_LAB)
}

func laboratoryRedraw(flags uintptr) {
	if labAtomicUpdateDepth > 0 || hMain == 0 || currentPage != ID_LAB {
		return
	}
	pRedrawWindow.Call(uintptr(hMain), 0, 0, flags)
}

func switchLabFieldsPage(delta int) {
	atomic := beginLaboratoryAtomicUpdate()
	defer endLaboratoryAtomicUpdate(atomic)
	saveLabDraft()
	g := currentLabGroup()
	pages := (len(g.Fields) + 9) / 10
	if pages < 1 {
		pages = 1
	}
	labFieldsPage += delta
	if labFieldsPage < 0 {
		labFieldsPage = 0
	}
	if labFieldsPage >= pages {
		labFieldsPage = pages - 1
	}
	labPageSwitching = true
	defer func() { labPageSwitching = false }()
	switchLabGroup(g.Code, false)
}

func switchLabGroup(code string, refresh bool) {
	started := time.Now()
	defer reportSlowFrame("lab_group", ID_LAB, started, 150*time.Millisecond)
	atomic := beginLaboratoryAtomicUpdate()
	defer endLaboratoryAtomicUpdate(atomic)
	previousGroup := labActiveGroup
	if previousGroup != code {
		labGroupGeneration++
	}
	if labObjectEdit != 0 && !labObjectSwitching && !labPageSwitching {
		saveLabDraft()
	}
	labActiveGroup = code
	// A complication tab always opens at its first indicator page.  Carrying a
	// page index from another complication was both confusing and a source of
	// abrupt row-height changes during the same click transaction.
	if !labPageSwitching && previousGroup != code {
		labFieldsPage = 0
	}
	g := currentLabGroup()
	if g.Code != "overall" {
		pages := (len(g.Fields) + 9) / 10
		if labFieldsPage >= pages {
			labFieldsPage = 0
		}
	}
	labConclusionOpen = false
	if g.Code == "overall" {
		setText(labConclusionTitle, "Общая оценка по скважине")
		setLaboratoryConclusion(buildCurrentLaboratoryPassConclusion())
		setText(labAnalyzeButton, "Произвести общий анализ")
		// Кнопка не должна обрезать подпись в узкой колонке.
		setText(labSaveButton, "Сохранить общие данные")
		show(labCommentLabel, false)
		show(labCommentEdit, false)
		show(labSaveButton, true)
		show(labContinueButton, true)
	} else {
		setText(labConclusionTitle, "Предварительный вывод")
		setLaboratoryConclusion("Заполните доступные показатели раздела «" + g.Name + "» и нажмите «Произвести анализ».")
		setText(labAnalyzeButton, "Произвести анализ")
		show(labCommentLabel, false)
		show(labCommentEdit, false)
		show(labSaveButton, true)
		show(labContinueButton, false)
	}
	if labDrafts[g.Code] == nil {
		labDrafts[g.Code] = map[string]string{}
	}
	draft := labDrafts[g.Code]
	start := labFieldsPage * 10
	for i := 0; i < 10; i++ {
		if start+i < len(g.Fields) {
			f := g.Fields[start+i]
			calculated := laboratoryCalculatedField(g.Code, f.Key)
			setText(labFieldBadges[i], laboratoryFieldMarkerCode(g.Code, f.Key))
			caption := laboratoryWrapFieldCaption(laboratoryFieldDisplayLabel(f))
			setText(labFieldLabels[i], caption)
			// Reused controls must reapply their alignment and font on every page.
			// Long laboratory headings use the compact bold face and centered lines;
			// the full name remains available in its field passport and tooltip.
			// Captions stay anchored immediately after their color marker. Use the
			// same bold UI font everywhere; wrap long names into two lines. Only
			// captions that genuinely need a third line receive the compact face.
			style, _, _ := pGetWindowLongPtrW.Call(uintptr(labFieldLabels[i]), ^uintptr(15)) // GWL_STYLE = -16
			style &^= 0x3                                                                    // SS_TYPEMASK
			style |= SS_LEFT
			pSetWindowLongPtrW.Call(uintptr(labFieldLabels[i]), ^uintptr(15), style)
			labelHeight := laboratoryFieldLabelHeightForWidth(22, caption, 200)
			if labelHeight > 42 && hFontHeaderSmall != 0 {
				pSendMessageW.Call(uintptr(labFieldLabels[i]), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
			} else if hFontBold != 0 {
				pSendMessageW.Call(uintptr(labFieldLabels[i]), WM_SETFONT, uintptr(hFontBold), 1)
			}
			selectedUnit := labUnitDrafts[g.Code][f.Key]
			if selectedUnit == "" {
				selectedUnit = f.Unit
			}
			setLaboratoryUnitCombo(labUnitLabels[i], laboratoryUnitOptions(f), selectedUnit)
			if calculated {
				canonical := canonicalizeLaboratoryValues(g, draft, labUnitDrafts[g.Code])
				draft[f.Key] = laboratoryCalculatedFieldValue(g.Code, f.Key, canonical)
			}
			labCalculatedSyncing = true
			setText(labFieldEdits[i], laboratoryVisibleDecimal(draft[f.Key]))
			labCalculatedSyncing = false
			pSendMessageW.Call(uintptr(labFieldEdits[i]), 0x00CF, laboratoryBoolFlag(calculated), 0) // EM_SETREADONLY
			pEnableWindow.Call(uintptr(labUnitLabels[i]), laboratoryBoolFlag(!calculated))
			passport := laboratoryFieldMarkerCaption(laboratoryFieldMarkerCode(g.Code, f.Key)) + ".\r\nЧисла можно вводить с запятой или точкой; в форме они показываются с точкой.\r\n\r\n" + laboratoryFieldPassportText(g, f)
			if explanation := laboratoryCalculatedFieldExplanation(g.Code, f.Key); explanation != "" {
				passport = explanation + "\r\n\r\n" + passport
			}
			updateLaboratoryFieldTooltip(i, labFieldLabels[i], passport)
			updateLaboratoryFieldTooltip(22+i, labFieldBadges[i], passport)
			updateLaboratoryFieldTooltip(10+i, labFieldEdits[i], passport)
			show(labFieldBadges[i], true)
			show(labFieldLabels[i], true)
			show(labFieldEdits[i], true)
			show(labUnitLabels[i], true)
		} else {
			show(labFieldBadges[i], false)
			show(labFieldLabels[i], false)
			show(labFieldEdits[i], false)
			show(labUnitLabels[i], false)
		}
	}
	updateLaboratoryFieldPassport(0)
	updateLaboratoryReadinessPanel()
	updateLaboratoryScaleModelInfo()
	pages := (len(g.Fields) + 9) / 10
	show(labFieldsPrev, g.Code != "overall" && pages > 1)
	show(labFieldsNext, g.Code != "overall" && pages > 1)
	show(labFieldsPageLabel, g.Code != "overall" && pages > 1)
	if pages > 1 {
		prevEnabled, nextEnabled := uintptr(0), uintptr(0)
		if labFieldsPage > 0 {
			prevEnabled = 1
		}
		if labFieldsPage < pages-1 {
			nextEnabled = 1
		}
		pEnableWindow.Call(uintptr(labFieldsPrev), prevEnabled)
		pEnableWindow.Call(uintptr(labFieldsNext), nextEnabled)
		setText(labFieldsPageLabel, fmt.Sprintf("%s · %d из %d", laboratoryFieldsPageTitle(g.Code, labFieldsPage), labFieldsPage+1, pages))
	}
	validateCurrentLaboratoryPage()
	for id, b := range labTabButtons {
		_ = id
		pInvalidateRect.Call(uintptr(b), 0, 0)
	}
	if refresh {
		refreshLaboratory()
	}
	var r RECT
	pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&r)))
	layoutLaboratory(r.Right-r.Left, r.Bottom-r.Top)
	laboratoryRedraw(RDW_INVALIDATE | RDW_ALLCHILDREN)
}

func switchLabGroupByID(id int) (handled bool) {
	g, ok := labGroupByID(id)
	if !ok {
		return false
	}
	if labGroupTransitionBusy {
		// Native control notifications can be re-entered while Windows processes
		// synchronous layout and redraw messages. Ignore the nested click; the
		// active transition will commit one complete frame.
		diagLog("LAB complication switch ignored during transition target=%s active=%s", g.Code, labActiveGroup)
		return true
	}
	previousGroup := labActiveGroup
	labGroupTransitionBusy = true
	defer func() {
		labGroupTransitionBusy = false
		if recovered := recover(); recovered != nil {
			diagLog("LAB complication switch PANIC target=%s panic=%v", g.Code, recovered)
			// switchLabGroup's atomic-update defer restores redraw even on panic.
			// Rebuild the previous group from its saved draft before returning to
			// the message pump, so a failed transition cannot leave mixed controls.
			func() {
				defer func() {
					if restorePanic := recover(); restorePanic != nil {
						diagLog("LAB complication rollback PANIC group=%s panic=%v", previousGroup, restorePanic)
					}
				}()
				labActiveGroup = previousGroup
				switchLabGroup(previousGroup, false)
				setText(labInfo, "Переход не завершён. Предыдущая вкладка восстановлена; повторите действие.")
			}()
			handled = true
		}
	}()
	// A repeated click on the already active complication must be a no-op.  It
	// used to rebuild the complete ListView, readiness cards and geometry again,
	// which amplified flicker when users clicked tabs quickly.
	if g.Code == labActiveGroup && g.Code != "overall" {
		return true
	}
	if g.Code == "overall" {
		// The summary tab is a pure read-only aggregation of the current pass.
		// Do not rebuild/select history here: a pending ListView click used to open
		// an older record and unexpectedly switch the user back to another group.
		switchLabGroup(g.Code, false)
		analyzeLaboratoryInputs()
		setText(labInfo, "Общее заключение сформировано из текущих значений и заключений всех лабораторных вкладок. Переход к старым записям не выполнялся.")
		return true
	}
	switchLabGroup(g.Code, true)
	return true
}

func saveLaboratoryRecord() {
	if currentLabGroup().Code == "overall" {
		saveLaboratoryPass()
		return
	}
	key := strings.TrimSpace(getText(labObjectEdit))
	if key == "" {
		msg("Лабораторные данные", "Укажите ключ объекта. Можно выбрать объект в разделе «Объекты и паспорт» и нажать «Подставить выбранный объект».", MB_OK|MB_ICONWARNING)
		return
	}
	date := strings.TrimSpace(getText(labDateEdit))
	if date == "" {
		date = time.Now().Format("02.01.2006")
	}
	g := currentLabGroup()
	values := currentLabValues()
	units := currentLaboratoryUnits()
	filled := 0
	for _, f := range g.Fields {
		if laboratoryCalculatedField(g.Code, f.Key) {
			continue
		}
		v := strings.TrimSpace(values[f.Key])
		if v != "" {
			filled++
		}
	}
	if filled == 0 {
		msg("Лабораторные данные", "Для сохранения новой лабораторной записи заполните хотя бы один доступный показатель выбранного раздела. Это ограничение сохранения пустой записи, а не блокировка предварительной оценки.", MB_OK|MB_ICONWARNING)
		return
	}
	canonicalValues := canonicalizeLaboratoryValues(g, values, units)
	for _, f := range g.Fields {
		if laboratoryCalculatedField(g.Code, f.Key) {
			values[f.Key] = canonicalValues[f.Key]
		}
	}
	_, validationWarnings, validationErrors := validateLaboratoryGroup(g, canonicalValues)
	if len(validationErrors) > 0 {
		msg("Проверка лабораторных данных", "Сохранение остановлено: исправьте неадекватные значения.\n\n"+strings.Join(validationErrors, "\n"), MB_OK|MB_ICONWARNING)
		return
	}
	if len(validationWarnings) > 0 && strings.TrimSpace(getText(labCommentEdit)) == "" {
		msg("Проверка лабораторных данных", "Для сохранения пограничного значения добавьте комментарий с подтверждением источника и единицы.\n\n"+strings.Join(validationWarnings, "\n"), MB_OK|MB_ICONWARNING)
		return
	}
	base := key + "|" + date + "|" + g.Code + "|" + time.Now().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(base))
	conclusion := buildLaboratoryConclusion(g, canonicalValues)
	setLaboratoryConclusion(conclusion)
	r := LabRecord{ID: "LAB-" + hex.EncodeToString(sum[:8]), ObjectKey: key, Date: date, GroupCode: g.Code, GroupName: g.Name, Source: strings.TrimSpace(getText(labSourceEdit)), Values: values, Units: units, ScaleModel: labScaleModelCode, Comment: strings.TrimSpace(getText(labCommentEdit)), Conclusion: conclusion, CreatedAt: time.Now().Format(time.RFC3339)}
	if err := appendLabRecord(r); err != nil {
		msg("Лабораторные данные", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	labRecords = append([]LabRecord{r}, labRecords...)
	msg("Лабораторные данные", "Результат сохранён как новая запись. Предыдущие анализы объекта не изменены.", MB_OK|MB_ICONINFORMATION)
	refreshLaboratory()
}

func buildCurrentLaboratoryPassConclusion() string {
	pass := ensureActiveLabPass()
	pass.Groups = cloneLabGroups(labDrafts)
	pass.Units = cloneLabGroups(labUnitDrafts)
	pass.ScaleModel = labScaleModelCode
	pass.SharedValues = collectLaboratorySharedValues(pass.Groups)
	pass.ObjectKey = strings.TrimSpace(getText(labObjectEdit))
	pass.Date = strings.TrimSpace(getText(labDateEdit))
	applyLaboratorySourceToPass(pass, getText(labSourceEdit))
	pass.Comment = strings.TrimSpace(getText(labCommentEdit))
	return buildLaboratoryPassConclusion(*pass)
}

func saveLaboratoryPass() {
	pass := ensureActiveLabPass()
	pass.ObjectKey = strings.TrimSpace(getText(labObjectEdit))
	if pass.ObjectKey == "" {
		msg("Сохранение лабораторных данных", "Укажите ключ объекта.", MB_OK|MB_ICONWARNING)
		return
	}
	pass.Date = strings.TrimSpace(getText(labDateEdit))
	applyLaboratorySourceToPass(pass, getText(labSourceEdit))
	pass.Comment = strings.TrimSpace(getText(labCommentEdit))
	pass.Author = currentUserLabel()
	pass.Groups = cloneLabGroups(labDrafts)
	pass.Units = cloneLabGroups(labUnitDrafts)
	pass.ScaleModel = labScaleModelCode
	pass.SharedValues = collectLaboratorySharedValues(pass.Groups)
	pass.Completed = completedLabGroupsForModel(pass.Groups, pass.ScaleModel)
	if len(pass.Completed) == 0 {
		msg("Сохранение лабораторных данных", "Заполните хотя бы один лабораторный раздел.", MB_OK|MB_ICONWARNING)
		return
	}
	validationWarnings, validationErrors := validateLaboratoryPassWithUnits(pass)
	if len(validationErrors) > 0 {
		msg("Проверка лабораторных данных", "Сохранение остановлено: исправьте неадекватные значения. Ошибочные показатели уже исключены из предварительного анализа.\n\n"+strings.Join(validationErrors, "\n"), MB_OK|MB_ICONWARNING)
		return
	}
	if len(validationWarnings) > 0 && strings.TrimSpace(pass.Comment) == "" {
		msg("Проверка лабораторных данных", "Для сохранения пограничных, но физически возможных значений добавьте общий комментарий с подтверждением источника и единиц.\n\n"+strings.Join(validationWarnings, "\n"), MB_OK|MB_ICONWARNING)
		return
	}
	pass.Status = labPassSavedStatus
	pass.Conclusion = buildLaboratoryPassConclusion(*pass)
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	pass.Conclusion += "\n\n" + laboratoryPassComparisonText(*pass, labPasses)
	if err := pass.prepare(time.Now()); err != nil {
		msg("Сохранение лабораторных данных", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	if err := appendLabPass(*pass); err != nil {
		msg("Сохранение лабораторных данных", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	labPassesLoaded = true
	replaced := false
	for i := range labPasses {
		if labPasses[i].PassID == pass.PassID {
			labPasses[i] = *pass
			replaced = true
			break
		}
	}
	if !replaced {
		labPasses = append([]LabPassRecord{*pass}, labPasses...)
	}
	invalidateMethodsBackground()
	setLaboratoryConclusion(pass.Conclusion)
	msg("Лабораторные данные сохранены", "Данные сохранены одной записью истории. Отдельные строки по вкладкам не создавались.", MB_OK|MB_ICONINFORMATION)
	refreshLaboratory()
}

func continueLaboratoryPassEditing() {
	for _, group := range labGroups {
		if group.Code != "overall" && len(labDrafts[group.Code]) > 0 {
			labFieldsPage = 0
			switchLabGroup(group.Code, true)
			return
		}
	}
	switchLabGroup("corrosion", true)
}

func currentLabValues() map[string]string {
	g := currentLabGroup()
	values := make(map[string]string, len(g.Fields))
	for key, value := range labDrafts[g.Code] {
		values[key] = strings.TrimSpace(value)
	}
	start := labFieldsPage * 10
	for i := 0; i < 10 && start+i < len(g.Fields); i++ {
		values[g.Fields[start+i].Key] = strings.TrimSpace(getText(labFieldEdits[i]))
	}
	return values
}

func laboratorySharedKey(group, key string) string {
	direct := map[string]string{
		"mineralization": "mineralization", "ph": "ph", "watercut": "watercut",
		"o2": "dissolved_o2", "co2": "dissolved_co2", "h2s": "dissolved_h2s",
		"sample_temp": "sample_temp", "orp": "orp", "iron_total": "iron_total",
		"co2_gas": "co2_gas", "h2s_gas": "h2s_gas", "pco2": "pco2", "ph2s": "ph2s",
		"bottomhole_pressure": "bottomhole_pressure", "mixture_density": "mixture_density",
		"formation_temp": "formation_temp", "formation_pressure": "formation_pressure",
		"pump_temp": "pump_temp", "pump_pressure": "pump_pressure",
		"wellhead_temp": "wellhead_temp", "wellhead_pressure": "wellhead_pressure",
		"profile_temp_min": "profile_temp_min", "profile_pressure_min": "profile_pressure_min",
		"profile_temp_max": "profile_temp_max", "profile_pressure_max": "profile_pressure_max",
	}
	if group == "corrosion" && key == "chlorides" {
		return "chlorides"
	}
	if group == "scale" && key == "cl" {
		return "chlorides"
	}
	return direct[key]
}

func collectLaboratorySharedValues(groups map[string]map[string]string) map[string]string {
	out := map[string]string{}
	for group, values := range groups {
		for key, value := range values {
			shared := laboratorySharedKey(group, key)
			if shared != "" && strings.TrimSpace(value) != "" && out[shared] == "" {
				out[shared] = value
			}
		}
	}
	return out
}

func synchronizeLaboratorySharedValue(group, key, value string) {
	shared := laboratorySharedKey(group, key)
	if shared == "" || labSharedSyncing {
		return
	}
	var sourceField LabFieldDef
	foundSource := false
	for _, g := range labGroups {
		if g.Code != group {
			continue
		}
		for _, field := range g.Fields {
			if field.Key == key {
				sourceField = field
				foundSource = true
				break
			}
		}
	}
	if !foundSource {
		return
	}
	sourceUnit := sourceField.Unit
	if u := strings.TrimSpace(labUnitDrafts[group][key]); u != "" {
		sourceUnit = u
	}
	canonical := laboratoryCanonicalValue(sourceField, value, sourceUnit)
	labSharedSyncing = true
	defer func() { labSharedSyncing = false }()
	for _, candidate := range labGroups {
		if candidate.Code == "overall" {
			continue
		}
		if labDrafts[candidate.Code] == nil {
			labDrafts[candidate.Code] = map[string]string{}
		}
		for _, field := range candidate.Fields {
			if laboratorySharedKey(candidate.Code, field.Key) != shared {
				continue
			}
			// Shared fields are synchronized only when their canonical units match.
			// This prevents accidental propagation between physically different fields.
			if !strings.EqualFold(strings.TrimSpace(field.Unit), strings.TrimSpace(sourceField.Unit)) {
				continue
			}
			targetUnit := field.Unit
			if u := strings.TrimSpace(labUnitDrafts[candidate.Code][field.Key]); u != "" {
				targetUnit = u
			}
			labDrafts[candidate.Code][field.Key] = laboratoryDisplayValue(field, canonical, targetUnit)
		}
	}
	start := labFieldsPage * 10
	g := currentLabGroup()
	for i := 0; i < 10 && start+i < len(g.Fields); i++ {
		field := g.Fields[start+i]
		if laboratorySharedKey(g.Code, field.Key) != shared {
			continue
		}
		display := labDrafts[g.Code][field.Key]
		if getText(labFieldEdits[i]) != display {
			setText(labFieldEdits[i], display)
		}
	}
	pass := ensureActiveLabPass()
	pass.Groups = cloneLabGroups(labDrafts)
	pass.Units = cloneLabGroups(labUnitDrafts)
	pass.SharedValues = collectLaboratorySharedValues(labDrafts)
}

func handleLaboratoryFieldChange(id int) bool {
	index := id - ID_LAB_FIELD_BASE
	if index < 0 || index >= 10 || labActiveGroup == "overall" || labSharedSyncing || labCalculatedSyncing {
		return false
	}
	g := currentLabGroup()
	fieldIndex := labFieldsPage*10 + index
	if fieldIndex >= len(g.Fields) {
		return false
	}
	if labDrafts[g.Code] == nil {
		labDrafts[g.Code] = map[string]string{}
	}
	field := g.Fields[fieldIndex]
	if laboratoryCalculatedField(g.Code, field.Key) {
		return true
	}
	value := getText(labFieldEdits[index])
	labDrafts[g.Code][field.Key] = value
	synchronizeLaboratorySharedValue(g.Code, field.Key, value)
	if g.Code == "scale" {
		canonical := canonicalizeLaboratoryValues(g, labDrafts[g.Code], currentLaboratoryUnits())
		labDrafts[g.Code]["water_type_sulin"] = canonical["water_type_sulin"]
	}
	warnings, errors := validateCurrentLaboratoryPage()
	if len(errors) > 0 {
		setText(labInfo, "Ошибка ввода: "+errors[0]+". Значение не будет использовано в расчёте.")
	} else if len(warnings) > 0 {
		setText(labInfo, "Предупреждение: "+warnings[0]+".")
	}
	updateLaboratoryFieldPassport(index)
	updateLaboratoryReadinessPanel()
	updateLaboratoryScaleModelInfo()
	return true
}

func handleLaboratoryFieldFocus(id int) bool {
	index := id - ID_LAB_FIELD_BASE
	if index < 0 || index >= 10 || labActiveGroup == "overall" {
		return false
	}
	updateLaboratoryFieldPassport(index)
	return true
}
func laboratoryVisibleDecimal(s string) string {
	trimmed := strings.TrimSpace(s)
	normalized := strings.ReplaceAll(trimmed, ",", ".")
	if _, ok := parseLabNumber(normalized); ok {
		return normalized
	}
	return s
}
func normalizeLaboratoryFieldOnBlur(id int) bool {
	index := id - ID_LAB_FIELD_BASE
	if index < 0 || index >= len(labFieldEdits) || labActiveGroup == "overall" {
		return false
	}
	before := getText(labFieldEdits[index])
	after := laboratoryVisibleDecimal(before)
	if before != after {
		setText(labFieldEdits[index], after)
		handleLaboratoryFieldChange(id)
	}
	return true
}

func updateLaboratoryFieldPassport(index int) {
	if labPassportPanel == 0 || labActiveGroup == "overall" {
		return
	}
	g := currentLabGroup()
	fieldIndex := labFieldsPage*10 + index
	if fieldIndex < 0 || fieldIndex >= len(g.Fields) {
		setText(labPassportPanel, "")
		return
	}
	field := g.Fields[fieldIndex]
	passport := laboratoryFieldPassport(g, field)
	unit := labUnitDrafts[g.Code][field.Key]
	if unit == "" {
		unit = field.Unit
	}
	setText(labPassportPanel, fmt.Sprintf("ПАСПОРТ ПОЛЯ · %s · %s · единица: %s\r\nСмысл: %s\r\nДопустимо: %s. Нельзя: %s.", passport.Marker, field.Label, unit, passport.Meaning, passport.AllowedInference, passport.ForbiddenClaim))
}

func laboratoryReadinessMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "!"
}

func updateLaboratoryReadinessPanel() {
	if labReadinessPanel == 0 || labActiveGroup == "overall" {
		return
	}
	g := currentLabGroup()
	values := currentLabValues()
	baseFilled, baseTotal, optionalFilled, optionalTotal := laboratoryDataCoverageForModel(g, values, labScaleModelCode)
	warnings, errors := validateCurrentLaboratoryPage()
	objectReady := strings.TrimSpace(getText(labObjectEdit)) != ""
	dateReady := strings.TrimSpace(getText(labDateEdit)) != ""
	sourceReady := strings.TrimSpace(getText(labSourceEdit)) != ""
	status := "Ошибок ввода нет"
	if len(errors) > 0 {
		status = fmt.Sprintf("Ошибки ввода: %d", len(errors))
	} else if len(warnings) > 0 {
		status = fmt.Sprintf("Предупреждения: %d", len(warnings))
	}
	qualityText := laboratoryDataQualityCardText(g, values, labScaleModelCode, getText(labDateEdit), getText(labSourceEdit), time.Now(), len(warnings), len(errors))
	historyPasses := repositoryLabPassesForObject(getText(labObjectEdit))
	dynamicsText := laboratoryCurrentGroupComparisonShort(g, values, labUnitDrafts[g.Code], getText(labObjectEdit), getText(labDateEdit), labScaleModelCode, historyPasses)
	extraContext := ""
	if g.Code == "corrosion" {
		extraContext = "\r\n" + corrosionEnvironmentCompactText(canonicalizeLaboratoryValues(g, values, labUnitDrafts[g.Code]))
	}
	trendText := laboratoryGroupTrendShort(g, getText(labObjectEdit), getText(labDateEdit), historyPasses)
	inhibitorText := ""
	if g.Code == "corrosion" || g.Code == "scale" {
		inhibitorText = "\r\n" + laboratoryInhibitorHistoryShort(getText(labObjectEdit), historyPasses)
	}
	panelText := fmt.Sprintf("ГОТОВНОСТЬ АНАЛИЗА\r\nЧисла: 7,2 или 7.2; на экране — 7.2.\r\n%s Объект: %s\r\n%s Дата анализа: %s\r\n%s Источник / протокол: %s\r\nОБЯЗАТЕЛЬНЫЙ БАЗОВЫЙ УРОВЕНЬ\r\nЗаполнено: %d из %d\r\nДополнительные: %d из %d\r\n%s\r\n\r\n%s\r\n%s\r\n%s%s\r\nПаспорт пробы: проверьте точку и время отбора, T/P, режим, хранение и метод анализа.",
		laboratoryReadinessMark(objectReady), laboratoryAttentionValue(objectReady, getText(labObjectEdit)),
		laboratoryReadinessMark(dateReady), laboratoryAttentionValue(dateReady, getText(labDateEdit)),
		laboratoryReadinessMark(sourceReady), laboratoryAttentionValue(sourceReady, getText(labSourceEdit)),
		baseFilled, baseTotal, optionalFilled, optionalTotal, status, qualityText, dynamicsText, trendText+inhibitorText, extraContext)
	setText(labReadinessPanel, panelText)
	formatLaboratoryReadinessPanel(panelText)
}

// Both right-hand cards share the same typography and text rectangle.
func laboratoryInfoCard() syscall.Handle {
	panel := richEdit(ID_LAB, "", 0, 0, 1, 1, true, 0)
	pSendMessageW.Call(uintptr(panel), WM_SETFONT, uintptr(hFontPanel), 1)
	return panel
}

func layoutLaboratoryInfoCard(panel syscall.Handle, x, y, w, h int32) {
	setPos(panel, x, y, w, h)
	rect := RECT{Left: 8, Top: 6, Right: max32(9, w-10), Bottom: max32(7, h-8)}
	pSendMessageW.Call(uintptr(panel), 0x00B3, 0, uintptr(unsafe.Pointer(&rect))) // EM_SETRECT
}

func formatLaboratoryReadinessPanel(text string) {
	formatLaboratoryInfoPanel(labReadinessPanel, text)
}

func formatLaboratoryInfoPanel(panel syscall.Handle, text string) {
	pSendMessageW.Call(uintptr(panel), WM_SETFONT, uintptr(hFontPanel), 1)
	plain := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR | 0x20000000, CrTextColor: uint32(rgb(43, 46, 50))}
	copy(plain.SzFaceName[:], utf16.Encode([]rune("Segoe UI")))
	pSendMessageW.Call(uintptr(panel), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(panel), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&plain)))
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	offset := 0
	for _, part := range strings.SplitAfter(normalized, "\n") {
		line := strings.TrimSuffix(part, "\n")
		trimmed := strings.TrimSpace(line)
		bold := trimmed == "МЕТОД" || trimmed == "СТАТУС" || trimmed == "УСЛОВИЯ РАСЧЁТА" || trimmed == "ДОСТУПНЫЙ РЕЗУЛЬТАТ" || trimmed == "ОГРАНИЧЕНИЕ" || strings.HasPrefix(trimmed, "ЧАСТИЧНЫЙ РАСЧЁТ") || trimmed == "ГОТОВНОСТЬ АНАЛИЗА" || trimmed == "ОБЯЗАТЕЛЬНЫЙ БАЗОВЫЙ УРОВЕНЬ" || trimmed == "КАЧЕСТВО ИСХОДНЫХ ДАННЫХ" || trimmed == "КОРРОЗИОННАЯ СРЕДА" || trimmed == "ТРЕНД ИСТОРИИ" || strings.HasPrefix(trimmed, "Паспорт пробы:")
		color := rgb(15, 48, 82)
		if strings.HasPrefix(trimmed, "!") || strings.HasPrefix(trimmed, "Ошибки") || strings.HasPrefix(trimmed, "ЧАСТИЧНЫЙ РАСЧЁТ") || strings.HasPrefix(trimmed, "ОГРАНИЧЕННЫЙ РАСЧЁТ") || trimmed == "ОГРАНИЧЕНИЕ" {
			bold, color = true, rgb(203, 27, 43)
		} else if strings.HasPrefix(trimmed, "Предупреждения") {
			bold, color = true, rgb(180, 105, 20)
		}
		if bold {
			format := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, DwEffects: CFE_BOLD, CrTextColor: uint32(color)}
			pSendMessageW.Call(uintptr(panel), EM_SETSEL, uintptr(offset), uintptr(offset+laboratoryUTF16Len(line)))
			pSendMessageW.Call(uintptr(panel), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&format)))
		}
		offset += laboratoryUTF16Len(part)
	}
	pSendMessageW.Call(uintptr(panel), EM_SETSEL, 0, 0)
	pSendMessageW.Call(uintptr(panel), 0x00B7, 0, 0)
}

func laboratoryAttentionValue(ok bool, value string) string {
	if !ok {
		return "не указано"
	}
	value = strings.TrimSpace(value)
	return value
}

func handleLaboratoryObjectChange() {
	if labObjectSwitching {
		return
	}
	key := normalizeLaboratoryObjectKey(getText(labObjectEdit))
	if key == labActiveObjectKey {
		return
	}
	atomic := beginLaboratoryAtomicUpdate()
	defer endLaboratoryAtomicUpdate(atomic)
	labObjectSwitching = true
	defer func() { labObjectSwitching = false }()
	labActiveObjectKey = key
	labActivePassID = ""
	labDrafts = map[string]map[string]string{}
	labUnitDrafts = map[string]map[string]string{}
	setLaboratoryScaleModel("davies")
	ensureActiveLabPass()
	applyTechRegimeContextToLaboratory(getText(labObjectEdit))
	labFieldsPage = 0
	switchLabGroup(labActiveGroup, false)
	pInvalidateRect.Call(uintptr(labInfluenceChart), 0, 0)
	pInvalidateRect.Call(uintptr(labTrendChart), 0, 0)
}

func techStateForLaboratoryObject(objectKey string) (TechObjectState, bool) {
	key := normalizeLaboratoryObjectKey(objectKey)
	for _, state := range techCurrent {
		if normalizeLaboratoryObjectKey(state.ObjectKey) == key {
			return state, true
		}
	}
	return TechObjectState{}, false
}

func applyTechRegimeContextToLaboratory(objectKey string) bool {
	state, ok := techStateForLaboratoryObject(objectKey)
	if !ok || strings.TrimSpace(state.WaterCut) == "" {
		return false
	}
	changed := seedLaboratoryWatercutFromTech(labDrafts, state.WaterCut)
	if changed {
		pass := ensureActiveLabPass()
		pass.Groups = cloneLabGroups(labDrafts)
		pass.SharedValues = collectLaboratorySharedValues(labDrafts)
	}
	return changed
}

func seedLaboratoryWatercutFromTech(groups map[string]map[string]string, watercut string) bool {
	watercut = strings.TrimSpace(watercut)
	if watercut == "" {
		return false
	}
	changed := false
	// Обводнённость является контекстом Техрежима: она участвует в расчётах,
	// но больше нигде не показывается как ручное лабораторное поле.
	for _, code := range []string{"corrosion", "scale", "aspo", "solids", "microbiology", "gases"} {
		if groups[code] == nil {
			groups[code] = map[string]string{}
		}
		if strings.TrimSpace(groups[code]["watercut"]) != watercut {
			groups[code]["watercut"] = watercut
			changed = true
		}
	}
	return changed
}

func parseLabNumber(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}

func laboratoryBaselineKeys(groupCode string) map[string]bool {
	keys := map[string][]string{
		"corrosion":    {"ph", "chlorides", "o2", "co2", "h2s", "corrosion_rate", "mineralization"},
		"scale":        {"ca", "so4", "hco3", "ph", "mineralization"},
		"aspo":         {"paraffins", "asphaltenes", "pour_point", "viscosity"},
		"solids":       {"suspended", "sand", "size_d50", "turbidity"},
		"microbiology": {"srb", "gmb", "sulfide"},
		"gases":        {"h2s", "co2", "water_content", "hydrate_temp", "mineralization"},
	}
	out := map[string]bool{}
	for _, key := range keys[groupCode] {
		out[key] = true
	}
	return out
}

func laboratoryTechRegimeKey(groupCode, key string) bool {
	return key == "watercut"
}

func laboratoryFieldsPageTitle(groupCode string, page int) string {
	if groupCode == "corrosion" {
		titles := []string{
			"Коррозионная активность среды — базовая оценка",
			"Газовая среда и условия эксплуатации",
			"Материал и вероятность H₂S-растрескивания",
			"Специальные анализы среды и контроль ингибитора",
			"Техническое состояние и паспорт ингибитора",
			"Для оценки гравиметрического анализа (коррозионный мониторинг)",
		}
		if page >= 0 && page < len(titles) {
			return titles[page]
		}
	}
	if groupCode == "scale" && labScaleModelCode == "co2_gas_water" {
		return "CO₂ ↔ вода: постановка, газовый запас и фон NaCl"
	}
	if groupCode == "scale" && labScaleModelCode == "water_mixing" {
		titles := []string{"Вода 1: общий состав и растворённые вещества", "Вода 2: общий состав и растворённые вещества", "Смешение: доля, плотности и измеренный pH"}
		if page >= 0 && page < len(titles) {
			return titles[page]
		}
	}
	if groupCode == "scale" {
		titles := []string{
			"Ионный состав и контроль баланса",
			"Условия пробы и технологический контекст",
			"Температурно-барический профиль",
			"Специальные анализы воды и отложений",
			"Дополнительное подтверждение состава солей",
		}
		if page >= 0 && page < len(titles) {
			return titles[page]
		}
	}
	return "Показатели"
}

// laboratoryFieldDisplayLabel keeps the compact two-column form readable.
// Field.Label remains the full wording used in passports, tooltips and reports.
func laboratoryFieldDisplayLabel(field LabFieldDef) string {
	labels := map[string]string{
		"orp":                           "Окислительно-\nвосстановительный\nпотенциал (ОВП)",
		"working_stresses":              "Рабочие / остат. напряжения",
		"hydrogen_permeation":           "Водородопроницаемость",
		"material_zone_conclusion":      "Заключение по материалу (H₂S)",
		"material_hardness_weld":        "Твёрдость шва / ЗТВ",
		"ssc_test":                      "Испытание SSC (H₂S-растрескивание)",
		"hic_test":                      "Испытание HIC (водородное растрескивание)",
		"inhibitor_residual":            "Остаток ингибитора",
		"inhibitor_amine_water":         "Ингибитор (амины), вода",
		"inhibitor_phosphorus_water":    "Ингибитор (фосфор), вода",
		"inhibitor_amine_ngvj":          "Ингибитор (амины), НГВЖ",
		"inhibitor_phosphorus_ngvj":     "Ингибитор (фосфор), НГВЖ",
		"inhibitor_brand_water":         "Марка ингибитора — вода",
		"inhibitor_type_ngvj":           "Тип ингибитора — НГВЖ",
		"inhibitor_brand_ngvj":          "Марка ингибитора — НГВЖ",
		"corrosion_rate_without":        "Коррозия без ингибитора",
		"corrosion_rate_with":           "Коррозия с ингибитором",
		"corrosion_rate_mass":           "Гравиметрическая скорость",
		"plate_mass_loss":               "Потеря массы пластинки",
		"plate_exposure_days":           "Экспозиция пластинки",
		"plate_area":                    "Рабочая площадь пластинки",
		"solute_mass":                   "Масса растворённых веществ",
		"profile_temp_min":              "Минимальная T профиля",
		"profile_pressure_min":          "Давление при Tmin профиля",
		"profile_temp_max":              "Максимальная T профиля",
		"profile_pressure_max":          "Давление при Tmax профиля",
		"external_saturation_index":     "Индекс насыщения (протокол)",
		"external_stability_index":      "Индекс стабильности (протокол)",
		"kinematic_viscosity_20":        "Кинем. вязкость, 20 °C",
		"kinematic_viscosity_40":        "Кинем. вязкость, 40 °C",
		"dynamic_viscosity_40":          "Динам. вязкость, 40 °C",
		"flow_type":                     "Течение нефти / эмульсии",
		"water_separation_test":         "Водоотделение с/без реагента",
		"suspended_filter":              "Взвесь, фильтр 3–10 мкм",
		"suspended_membrane":            "Взвесь, мембрана 0,45 мкм",
		"particle_distribution":         "Размерный состав частиц",
		"filter_elemental":              "Элементный состав на фильтрах",
		"deposit_insoluble_mineral":     "Минеральные мехпримеси",
		"deposit_mohs":                  "Твёрдость минералов по Моосу",
		"qpcr_apb":                      "Количественная ПЦР (qPCR) —\nкислотообразующие\nбактерии",
		"qpcr_srb":                      "Количественная ПЦР (qPCR) — СВБ",
		"other_microorganisms_activity": "Активность прочих микробов",
		"h2s_gas_mass":                  "H₂S в газе, мг/м³",
		"inhibitor_trial_brand":         "Марка ингибитора — испытание",
		"inhibitor_trial_target":        "Цель испытания ингибитора",
		"inhibitor_trial_dose":          "Доза ингибитора — испытание",
		"inhibitor_trial_temp":          "Температура испытания",
		"inhibitor_trial_duration_h":    "Продолжительность испытания",
		"inhibitor_trial_efficiency":    "Эффективность — испытание",
		"inhibitor_trial_protocol":      "Протокол испытания",
		"inhibitor_trial_result":        "Результат испытания",
		"solids":                        "Твёрдая фаза\nасфальтосмоло-\nпарафиновых отложений\n(АСПО)",
	}
	if label := labels[field.Key]; label != "" {
		return label
	}
	return field.Label
}

// laboratoryWrapFieldCaption makes the visible caption obey the real label
// envelope. STATIC controls do not word-wrap by themselves, so a long name
// must be split before it is assigned to the control. The split is performed
// at word boundaries and keeps existing deliberate line breaks. Two-line
// captions keep the normal bold font; the layout height/compact-font rule
// handles a genuine third line.
func laboratoryCaptionRunesPerLine(width int32) int {
	// Russian bold UI text is visibly wider than a 7 px ASCII approximation.
	// Use a conservative width budget so labels remain fully visible at 125/150%
	// DPI.  Very wide windows intentionally stop at 30 runes to keep the form
	// visually consistent rather than stretching captions into long sentences.
	if width <= 0 {
		return 27
	}
	chars := int((width - 10) / 8)
	if chars < 12 {
		chars = 12
	}
	if chars > 30 {
		chars = 30
	}
	return chars
}

func laboratoryCaptionTokenChunks(token string, limit int) []string {
	if limit < 4 {
		limit = 4
	}
	runes := []rune(token)
	if len(runes) <= limit {
		return []string{token}
	}
	out := make([]string, 0, (len(runes)+limit-1)/limit)
	for len(runes) > limit {
		cut := limit
		// Prefer a natural break after a hyphen/slash near the end of the line.
		for i := limit - 1; i >= limit/2; i-- {
			if runes[i] == '-' || runes[i] == '/' || runes[i] == '–' || runes[i] == '—' {
				cut = i + 1
				break
			}
		}
		out = append(out, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

func laboratoryWrapFieldCaptionForWidth(text string, width int32) string {
	maxRunesPerLine := laboratoryCaptionRunesPerLine(width)
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return ""
	}
	var out []string
	for _, explicit := range strings.Split(text, "\n") {
		words := strings.Fields(explicit)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, word := range words {
			for _, chunk := range laboratoryCaptionTokenChunks(word, maxRunesPerLine) {
				candidate := chunk
				if line != "" {
					candidate = line + " " + chunk
				}
				if utf8.RuneCountInString(candidate) <= maxRunesPerLine || line == "" {
					line = candidate
					continue
				}
				out = append(out, line)
				line = chunk
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\r\n")
}

func laboratoryWrapFieldCaption(text string) string {
	return laboratoryWrapFieldCaptionForWidth(text, 226)
}

func laboratoryFieldPrefix(groupCode, key string) string {
	if laboratoryCalculatedField(groupCode, key) {
		return "Р · "
	}
	baseline := laboratoryBaselineKeys(groupCode)[key]
	shared := laboratorySharedKey(groupCode, key) != ""
	if laboratoryTechRegimeKey(groupCode, key) {
		if shared {
			return "Т/О · "
		}
		return "Т · "
	}
	if baseline && shared {
		return "Б/О · "
	}
	if baseline {
		return "Б · "
	}
	if shared {
		return "Д/О · "
	}
	return "Д · "
}

func laboratoryFieldMarkerCode(groupCode, key string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(laboratoryFieldPrefixForModel(groupCode, key, labScaleModelCode)), "·"))
}

func laboratoryDataCoverage(group LabGroupDef, values map[string]string) (int, int, int, int) {
	baseline := laboratoryBaselineKeys(group.Code)
	baseFilled, baseTotal, optionalFilled, optionalTotal := 0, 0, 0, 0
	for _, field := range group.Fields {
		filled := strings.TrimSpace(values[field.Key]) != ""
		if !laboratoryInputFieldEligible(group.Code, field.Key) {
			continue
		}
		if baseline[field.Key] {
			baseTotal++
			if filled {
				baseFilled++
			}
			continue
		}
		optionalTotal++
		if filled {
			optionalFilled++
		}
	}
	return baseFilled, baseTotal, optionalFilled, optionalTotal
}

func laboratoryDataCoverageText(group LabGroupDef, values map[string]string) string {
	baseFilled, baseTotal, optionalFilled, optionalTotal := laboratoryDataCoverage(group, values)
	baseState := "сформирован"
	availability := "доступен по полностью заполненному обязательному базовому уровню"
	if baseFilled == 0 {
		baseState = "не сформирован"
		availability = "возможен только как ограниченный скрининг; заполните доступные базовые показатели"
	} else if baseFilled < baseTotal {
		baseState = "сформирован частично"
		availability = "доступен по заполненной части обязательного базового уровня с явным указанием пробелов"
	}
	techContext := "обводнённость из техрежима не найдена"
	if strings.TrimSpace(values["watercut"]) != "" {
		techContext = "обводнённость автоматически получена из техрежима: " + strings.TrimSpace(values["watercut"]) + " %"
	}
	return fmt.Sprintf("Обязательный базовый уровень данных: %s (%d из %d показателей). Предварительный вывод %s.\nКонтекст техрежима: %s.\nДополнительные параметры: заполнено %d из %d; их отсутствие не блокирует предварительную оценку, но ограничивает её точность.", baseState, baseFilled, baseTotal, availability, techContext, optionalFilled, optionalTotal)
}

func buildLaboratoryConclusion(group LabGroupDef, values map[string]string) string {
	return buildLaboratoryConclusionWithScaleModel(group, values, labScaleModelCode)
}

func copyLaboratoryConclusion() {
	if labConclusionEdit == 0 || strings.TrimSpace(getText(labConclusionEdit)) == "" {
		msg("Копирование заключения", "Сначала сформируйте лабораторный вывод.", MB_OK|MB_ICONWARNING)
		return
	}
	pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(labConclusionEdit), WM_COPY, 0, 0)
	pSendMessageW.Call(uintptr(labConclusionEdit), EM_SETSEL, 0, 0)
	msg("Копирование заключения", "Полный текст заключения скопирован в буфер обмена.", MB_OK|MB_ICONINFORMATION)
}

func laboratoryAttentionExplanation(groupCode, key string) string {
	explanations := map[string]string{
		"corrosion/ph":             "кислая среда может ускорять электрохимическую коррозию; сопоставить с CO₂/H₂S, минерализацией, материалом и фактической скоростью коррозии",
		"corrosion/chlorides":      "высокая концентрация хлоридов повышает проводимость и риск локальной коррозии; сама по себе не определяет механизм и скорость",
		"corrosion/h2s":            "наличие H₂S поддерживает риск сульфидной коррозии и водородного повреждения; проверить парциальное давление, материал, твёрдость и условия эксплуатации",
		"corrosion/corrosion_rate": "измеренная скорость выше рабочего ориентира и является прямым основанием для проверки защиты, точки контроля и динамики утонения",
		"scale/ca":                 "высокий Ca²⁺ создаёт предпосылку для кальцита или CaSO₄ только при наличии соответствующего аниона и пересыщения; один кальций не доказывает выпадение соли",
		"scale/so4":                "высокий SO₄²⁻ поддерживает риск сульфатов Ca/Ba/Sr при наличии соответствующих катионов; нужен расчёт насыщения и проверка смешения вод",
		"scale/hco3":               "высокий HCO₃⁻ поддерживает карбонатный механизм, но вероятность осадка определяется pH, Ca²⁺, температурой, давлением и выделением CO₂",
		"scale/ph":                 "щелочная реакция смещает карбонатное равновесие в сторону CO₃²⁻ и может повысить склонность к CaCO₃ при наличии кальция",
		"aspo/paraffins":           "повышенное содержание парафинов увеличивает потенциальную массу твёрдой фазы при охлаждении ниже температуры начала кристаллизации",
		"aspo/asphaltenes":         "высокая доля асфальтенов указывает на повышенную склонность к их дестабилизации; подтвердить совместимостью флюидов, изменением давления и лабораторным тестом осаждения",
		"aspo/pour_point":          "высокая температура застывания означает риск потери текучести при охлаждении потока до этого диапазона; сопоставить с температурным профилем скважины и линии",
		"aspo/viscosity":           "высокая вязкость ухудшает транспорт и может усиливать удержание АСПО/эмульсии; проверить вязкость при фактической температуре и обводнённости",
		"solids/suspended":         "повышенная концентрация взвеси увеличивает вероятность засорения фильтров, клапанов и проточных каналов; уточнить состав и источник частиц",
		"solids/sand":              "содержание песка поддерживает риск абразивного износа и накопления осадка; проверить вынос, гранулометрию и скорость потока",
		"solids/size_d50":          "крупная фракция повышает риск осаждения и механического заклинивания; сопоставить с проходными сечениями и скоростью потока",
		"solids/turbidity":         "высокая мутность подтверждает присутствие дисперсной фазы, но не определяет её состав; нужен анализ твёрдого остатка",
		"microbiology/gmb":         "повышенная численность микроорганизмов поддерживает биологический фактор; подтвердить профильными группами, биоплёнкой и условиями отбора",
		"microbiology/sulfide":     "сульфиды совместимы с активностью СВБ, но могут иметь и иной источник; сопоставить с H₂S, посевами и производственной историей",
		"gases/h2s":                "H₂S повышает коррозионную и токсикологическую опасность; оценить фазовое распределение и фактические T/P",
		"gases/co2":                "CO₂ может поддерживать углекислотную коррозию и изменение карбонатного равновесия; нужны pH, вода и фактические T/P",
		"gases/water_content":      "наличие водной фазы создаёт необходимое условие для гидратообразования, но не подтверждает его без состава газа и сопоставления фактических T/P с равновесной кривой",
	}
	if text := explanations[groupCode+"/"+key]; text != "" {
		return text
	}
	return "показатель вышел за рабочий скрининговый ориентир; требуется сопоставление с другими факторами механизма и производственными данными"
}

func laboratoryMechanismDecisionText(group LabGroupDef, values map[string]string) string {
	checks := laboratoryScreeningChecks()
	names := map[string]string{"corrosion": "коррозионного воздействия", "aspo": "образования и удержания АСПО", "solids": "осаждения/накопления мехпримесей", "microbiology": "микробиологического фактора", "gases": "газового или гидратного осложнения"}
	actions := map[string]string{
		"corrosion":    "сопоставить среду, материал, фактическую скорость коррозии и историю отказов; проверить действующую защиту",
		"aspo":         "сопоставить температуры начала кристаллизации/застывания с профилем скважины, выполнить тест стабильности и исследовать фактические отложения",
		"solids":       "определить минеральный и гранулометрический состав твёрдой фазы, источник частиц и условия их осаждения",
		"microbiology": "подтвердить профильными посевами/ПЦР, биоплёнкой, корректным отбором и динамикой H₂S/сульфидов",
		"gases":        "сопоставить состав газа и воды с фактической траекторией температуры и давления; для гидратов нужна равновесная кривая",
	}
	filled, risky := 0, []string{}
	for _, item := range checks[group.Code] {
		if n, ok := parseLabNumber(values[item.key]); ok {
			filled++
			if item.risky(n) {
				risky = append(risky, item.label)
			}
		}
	}
	baseFilled, baseTotal, _, _ := laboratoryDataCoverage(group, values)
	confidence := "ограниченная"
	if baseTotal > 0 && baseFilled == baseTotal {
		confidence = "рабочая для предварительного скрининга"
	}
	lines := []string{"ТЕХНОЛОГИЧЕСКИЙ ИТОГ"}
	if filled == 0 {
		lines = append(lines, "Вероятность механизма: НЕ ОПРЕДЕЛЕНА — нет корректных числовых показателей для скрининга.")
	} else if len(risky) == 0 {
		lines = append(lines, "Вероятность механизма: выраженные скрининговые признаки "+names[group.Code]+" по заполненным показателям НЕ ВЫЯВЛЕНЫ. Это не исключает осложнение по отсутствующим данным или в других режимных условиях.")
	} else {
		lines = append(lines, "Вероятность механизма: ВЫЯВЛЕНЫ ПРИЗНАКИ, поддерживающие возможность "+names[group.Code]+".", "Основания: "+strings.Join(risky, ", ")+". Отдельные признаки не являются доказательством фактического осложнения.")
	}
	lines = append(lines, "Уверенность: "+confidence+fmt.Sprintf("; базовых показателей %d из %d.", baseFilled, baseTotal), "Действие технолога: "+actions[group.Code]+".")
	return strings.Join(lines, "\n")
}

func laboratoryPlainLanguageSummary(group LabGroupDef, decision string) string {
	lower := strings.ToLower(decision)
	state := "По имеющимся данным однозначный вывод сделать нельзя. Это не означает, что осложнения нет: данных пока недостаточно."
	if strings.Contains(lower, "предпосылка выявлена") || strings.Contains(lower, "выявлены признаки") {
		state = "Осложнение возможно: заполненные показатели дают основания считать этот риск реальным. Расчёт показывает условия для его возникновения, но сам по себе ещё не подтверждает, что осложнение уже произошло на объекте."
	} else if strings.Contains(lower, "предпосылка не выявлена") || strings.Contains(lower, "признаки") && strings.Contains(lower, "не выявлены") {
		state = "По заполненным показателям явных признаков осложнения сейчас не выявлено. Однако такой результат действует только для внесённых данных и не исключает риск при другом режиме работы или по отсутствующим анализам."
	} else if strings.Contains(lower, "пограничная") {
		state = "Результат находится около границы: небольшое изменение состава или режима может перевести объект в зону риска. Поэтому считать ситуацию устойчиво безопасной пока нельзя."
	}
	actions := map[string]string{
		"corrosion":    "Что происходит: вода и растворённые в ней вещества могут разрушать металл труб и насоса. Кислород, углекислый газ и сероводород действуют по-разному, поэтому одного показателя недостаточно, чтобы назвать причину повреждения. При сероводороде отдельно проверяют пригодность материала: металл может треснуть даже без большой потери толщины.\n\nЧем это грозит: стенка трубы становится тоньше, могут появляться глубокие отдельные язвы, течи и отказы оборудования. Небольшая средняя скорость коррозии ещё не исключает локального повреждения.\n\nЧто проверить первым: сравните анализ среды с толщинометрией, видом повреждений и результатами контрольных пластин. Пластина показывает потерю металла за время выдержки в конкретной точке; она не описывает автоматически всю скважину. Уточните, где и когда отобрана проба, какая была обводнённость и поступал ли воздух. Затем проверьте подачу ингибитора, то есть реагента для защиты металла, и его остаточное содержание.\n\nКак принять решение: при подтверждённом разрушении разберите причину и проверьте схему защиты и материал оборудования. По одному анализу среды нельзя рассчитать срок до отказа или назначить дозировку реагента.\n\nКак понять, помогли ли меры: сопоставьте скорость потери металла и глубину язв до и после изменения защиты при сравнимых условиях. Одного наличия реагента в пробе недостаточно: нужна проверка фактического состояния металла.",
		"scale":        "Что происходит: вещества, растворённые в воде, при изменении температуры, давления или смешении вод могут превратиться в твёрдую соль. Если расчёт показывает пересыщение, это означает, что вода способна выделять такую соль. Это ещё не означает, что отложения уже появились или что они будут быстро расти.\n\nЧем это грозит: соль может накапливаться на трубах, в насосе и узких проходах. Проход уменьшается, подача может снижаться, нагрузка на оборудование — расти. Название минерала в расчёте указывает предполагаемый состав соли, но состав реального осадка нужно проверить анализом.\n\nЧто проверить первым: посмотрите, какая соль отмечена в расчёте, и проверьте исходный состав воды и температуру пробы. Сравните условия расчёта с условиями у насоса, по стволу и на поверхности. Уточните, смешиваются ли разные воды. Если pH смеси не измерен, нельзя просто усреднить pH исходных вод. Для метода, который не учитывает давление или весь температурный профиль, результат нельзя автоматически распространить на всю скважину.\n\nКак принять решение: при наличии реальных отложений сначала определите их состав, затем проверяйте подходящий способ удаления и предупреждения. Если есть только расчётная предпосылка, организуйте контроль и испытание защиты. Дозировку ингибитора солеотложений, частоту обработки и массу будущего осадка этот вывод сам по себе не определяет.\n\nКак понять, помогли ли меры: сравните количество и состав отложений, стабильность подачи и интервалы между очистками при сравнимом режиме. Отсутствие предпосылки в одной пробе не исключает её появления после изменения режима или состава воды.",
		"aspo":         "Что происходит: при охлаждении нефти парафин может кристаллизоваться, а асфальтены и смолы — терять устойчивость и накапливаться вместе с ним. АСПО — это асфальтосмолопарафиновые отложения. Само содержание парафина или асфальтенов ещё не показывает, где и насколько быстро образуется пробка.\n\nЧем это грозит: отложения сужают проход труб, затрудняют движение жидкости и могут ухудшать работу насоса. Высокая вязкость дополнительно затрудняет течение, но не доказывает наличие АСПО.\n\nЧто проверить первым: сравните температуру начала кристаллизации парафина с фактической температурой жидкости по пути от пласта до поверхности. Температура застывания — другой показатель: она описывает потерю текучести в условиях испытания и не заменяет температуру начала кристаллизации. Исследуйте снятые отложения и проверьте, совпадает ли их состав с предполагаемой причиной.\n\nКак принять решение: выбирайте растворитель, прогрев, очистку или предупреждающую обработку по составу отложений и результатам испытания. По одному проценту парафина нельзя назначить подходящий растворитель и безопасную частоту прогрева.\n\nКак понять, помогли ли меры: сравните количество отложений, нагрузку, подачу и интервал между очистками при близких температуре и режиме работы.",
		"solids":       "Что происходит: в жидкости есть твёрдые частицы. Они могут приходить из пласта, образовываться при коррозии или выпадении солей, а также оставаться после работ на скважине. Анализ количества частиц не определяет их источник.\n\nЧем это грозит: частицы могут истирать рабочие детали, забивать фильтры и узкие проходы или оседать там, где поток замедляется. Даже небольшое количество крупных и твёрдых частиц может быть важнее большого количества мелких мягких частиц.\n\nЧто проверить первым: определите состав и распределение частиц по размерам. Показатель d50 — размер, относительно которого половина распределения лежит ниже, половина выше; он не означает, что все частицы такого размера. Сопоставьте частицы с проходами и фильтрами установленного оборудования, подачей и изменениями после ремонта.\n\nКак принять решение: сначала установите источник. Для песка из пласта, продуктов коррозии и солевого осадка нужны разные меры. Фильтр подбирают по размерам частиц и условиям потока; нельзя выбирать его только по мутности пробы.\n\nКак понять, помогли ли меры: контролируйте количество и размер частиц, перепад давления на фильтре, подачу и износ деталей. Уменьшение мутности не всегда означает прекращение поступления опасных крупных частиц.",
		"microbiology": "Что происходит: результаты могут указывать на присутствие микроорганизмов, способных участвовать в образовании сульфидов, кислот и биоплёнки. Биоплёнка — слой микроорганизмов и продуктов их деятельности на поверхности. Под ним условия для металла могут отличаться от условий в общей пробе жидкости.\n\nЧем это грозит: возможны локальное разрушение металла и накопление загрязнений. Но обнаружение бактерий само по себе не доказывает, что именно они вызвали повреждение оборудования.\n\nЧто проверить первым: уточните способ отбора и хранения пробы. СВБ — сульфатвосстанавливающие бактерии. Количественная ПЦР, обозначаемая также qPCR, считает копии определённой ДНК: положительный результат не доказывает, что клетки живые и сейчас активны. Сопоставьте ПЦР с профильными посевами, исследованием биоплёнки, сульфидами, сероводородом и видом повреждений металла.\n\nКак принять решение: при согласующихся результатах проверьте биоцидную защиту и её совместимость с другими реагентами. Биоцид — реагент для подавления микроорганизмов. Его марку и дозировку выбирают по испытаниям, а не только по числу копий ДНК.\n\nКак понять, помогли ли меры: сравните повторные анализы по одной методике, признаки активности, биоплёнку и состояние металла. После обработки ДНК погибших клеток может сохраняться, поэтому одна ПЦР не показывает весь эффект.",
		"gases":        "Что происходит: растворённый или выделяющийся газ может влиять на работу насоса и свойства среды. Сероводород и углекислый газ также важны для оценки коррозии. Гидраты — твёрдые соединения газа с водой, которые могут образовываться при определённом сочетании состава, температуры и давления. Наличие газа или воды ещё не доказывает образование гидрата.\n\nЧем это грозит: свободный газ может нарушать устойчивую подачу насоса, а гидратная пробка — перекрывать проход. Газовое влияние на насос, коррозия и гидраты — разные причины; их нельзя объединять в один подтверждённый диагноз.\n\nЧто проверить первым: сопоставьте состав газа и воды с фактическими температурой и давлением в нужной точке. Для гидратов сравните рабочие условия с границей образования гидратов, рассчитанной для этого состава газа. Одна температура без соответствующего давления не даёт ответа. Для газового влияния на насос проверьте условия на приёме и фактическую устойчивость подачи.\n\nКак принять решение: выбирайте меру по подтверждённому механизму. Данные о H₂S и CO₂ одновременно передайте в оценку коррозии и пригодности материала. По одному анализу газа нельзя назначить дозу метанола или другого ингибитора гидратов.\n\nКак понять, помогли ли меры: сравните устойчивость подачи, давление, температуру и повторяемость нарушений при одинаковых условиях. Для гидратов контролируйте условия возможного образования и фактическую работу системы предупреждения.",
	}
	return "ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА\n" + state + "\n\n" + actions[group.Code]
}

func buildLaboratoryConclusionWithScaleModel(group LabGroupDef, values map[string]string, modelCode string) string {
	group = laboratoryScaleInputGroup(group, modelCode)
	if group.Code == "scale" && modelCode == "co2_gas_water" {
		validated, validationWarnings, validationErrors := validateLaboratoryGroup(group, values)
		validationText := laboratoryValidationText(validationWarnings, validationErrors)
		result := "Предварительное заключение: CO₂ ↔ вода\n\n" + laboratoryCO2EquilibriumCoverageText(group, validated) + "\n\n" + laboratoryCO2EquilibriumDecisionText(validated) + "\n\n" + strings.ReplaceAll(laboratoryCO2EquilibriumResultText(validated), "\r\n", "\n") + "\n\nОбласть: 25 °C, чистый CO₂, P≤1 атм, электронейтральный NaCl≤0,1 моль/кг воды. Режим не определяет насыщение минералов, осаждение, коррозию или МРП."
		if validationText != "" {
			result = validationText + "\n\n" + result
		}
		return result
	}
	summary := laboratoryGroupSummary(group, values, modelCode)
	validated, validationWarnings, validationErrors := validateLaboratoryGroup(group, values)
	values = validated
	validationText := laboratoryValidationText(validationWarnings, validationErrors)
	if group.Code == "scale" && modelCode == "water_mixing" {
		result := "Предварительное заключение: Смешение двух вод\n\n" + summary.Text() + "\n\n" + laboratoryDataCoverageTextForModel(group, values, modelCode) + "\n\n" + laboratoryScaleDecisionText(values, modelCode) + "\n\n" + laboratorySaturationTextForModel(values, modelCode) + "\n\n" + laboratoryPracticalExplanation(group, summary)
		if validationText != "" {
			result = validationText + "\n\n" + result
		}
		return result
	}
	var facts []string
	addThreshold := func(key, label, unit, threshold string, risky func(float64) bool) {
		n, ok := parseLabNumber(values[key])
		if !ok {
			return
		}
		result := "выход за рабочую границу не выявлен"
		if risky(n) {
			result = "требует внимания — " + laboratoryAttentionExplanation(group.Code, key)
		}
		facts = append(facts, fmt.Sprintf("• %s — фактическое значение: %g %s; рабочий порог: %s; результат: %s.", label, n, unit, threshold, result))
	}
	addContextValue := func(key, label, unit, context, result string) {
		n, ok := parseLabNumber(values[key])
		if !ok {
			return
		}
		facts = append(facts, fmt.Sprintf("• %s — фактическое значение: %g %s; числовой рабочий порог: не применяется (%s); результат: %s.", label, n, unit, context, result))
	}
	addQualitative := func(key, label string) {
		value := strings.TrimSpace(values[key])
		if value == "" {
			return
		}
		facts = append(facts, fmt.Sprintf("• %s — фактический результат: %s; единица / тип: качественный результат; числовой рабочий порог: не применяется; результат: требуется экспертная интерпретация.", label, value))
	}

	switch group.Code {
	case "corrosion":
		addThreshold("ph", "pH", "ед.", "ниже 6,5 ед.", func(n float64) bool { return n < 6.5 })
		addThreshold("chlorides", "Хлориды", "мг/л", "выше 10 000 мг/л", func(n float64) bool { return n > 10000 })
		addThreshold("h2s", "Растворённый H₂S", "мг/л", "выше 0 мг/л", func(n float64) bool { return n > 0 })
		addThreshold("corrosion_rate", "Скорость коррозии", "мм/год", "выше 0,1 мм/год", func(n float64) bool { return n > 0.1 })
	case "scale":
		if modelCode == "water_mixing" {
			break
		}
		addThreshold("ca", "Ca²⁺", "мг/л", "выше 2 000 мг/л", func(n float64) bool { return n > 2000 })
		addThreshold("so4", "SO₄²⁻", "мг/л", "выше 500 мг/л", func(n float64) bool { return n > 500 })
		addThreshold("hco3", "HCO₃⁻", "мг/л", "выше 500 мг/л", func(n float64) bool { return n > 500 })
		addThreshold("ph", "pH", "ед.", "выше 7,5 ед.", func(n float64) bool { return n > 7.5 })
	case "aspo":
		addThreshold("paraffins", "Парафины", "% масс.", "выше 3 % масс.", func(n float64) bool { return n > 3 })
		addThreshold("asphaltenes", "Асфальтены", "% масс.", "выше 1 % масс.", func(n float64) bool { return n > 1 })
		addThreshold("pour_point", "Температура застывания", "°C", "выше 15 °C", func(n float64) bool { return n > 15 })
		addThreshold("viscosity", "Динамическая вязкость", "мПа·с", "выше 50 мПа·с", func(n float64) bool { return n > 50 })
	case "solids":
		addThreshold("suspended", "Взвешенные вещества", "мг/л", "выше 100 мг/л", func(n float64) bool { return n > 100 })
		addThreshold("sand", "Песок", "%", "выше 0,1 %", func(n float64) bool { return n > 0.1 })
		addThreshold("size_d50", "Медианный размер d50", "мкм", "выше 50 мкм", func(n float64) bool { return n > 50 })
		addThreshold("turbidity", "Мутность", "ед.", "выше 50 ед.", func(n float64) bool { return n > 50 })
	case "microbiology":
		addThreshold("gmb", "Общее микробное число", "КОЕ/мл", "выше 10³ КОЕ/мл", func(n float64) bool { return n > 1000 })
		addThreshold("sulfide", "Сульфиды", "мг/л", "выше 0 мг/л", func(n float64) bool { return n > 0 })
		addContextValue("qpcr_srb", "СВБ, количественная ПЦР (qPCR)", "копий/мл", "метод определяет число копий ДНК целевой группы", "подтверждает наличие генетического материала СВБ, но отдельно не доказывает жизнеспособность и активность клеток")
		addContextValue("qpcr_apb", "Кислотообразующие бактерии, количественная ПЦР (qPCR)", "копий/мл", "метод определяет число копий ДНК целевой группы", "подтверждает наличие генетического материала кислотообразующих бактерий, но отдельно не доказывает жизнеспособность и активность клеток")
		addQualitative("srb", "СВБ")
		addQualitative("apb", "Кислотообразующие бактерии")
		addQualitative("biofilm", "Биоплёнка")
		addQualitative("iron_bacteria", "Железобактерии")
	case "gases":
		addThreshold("h2s", "H₂S", "мг/л", "выше 0 мг/л", func(n float64) bool { return n > 0 })
		addThreshold("co2", "CO₂", "мг/л", "выше 50 мг/л", func(n float64) bool { return n > 50 })
		addThreshold("water_content", "Влагосодержание газа", "г/м³", "выше 0,1 г/м³", func(n float64) bool { return n > 0.1 })
		addContextValue("hydrate_temp", "Равновесная температура гидратов", "°C", "нужны температура и давление процесса", "требуется сопоставление с фактической температурой и давлением процесса")
	}

	if len(facts) == 0 {
		result := fmt.Sprintf("%s\n\nРаздел «%s»: заполненных корректных числовых показателей для скрининга не найдено. Анализ не блокируется: внесите доступные результаты либо сохраните качественные данные в комментарии.\n\nЗаключение предварительное и не заменяет лабораторную интерпретацию, ОЛИ/ОПИ и производственную проверку.", laboratoryDataCoverageTextForModel(group, values, modelCode), group.Name)
		if group.Code == "corrosion" {
			result += "\n\n" + formatCorrosionInfluenceAssessment(assessCorrosionInfluence(values))
		}
		if group.Code == "scale" {
			result += "\n\n" + laboratorySulinMethodText(values)
			result += "\n\n" + strings.ReplaceAll(laboratorySaturationTextForModel(values, modelCode), "\r\n", "\n")
		}
		if validationText != "" {
			result = validationText + "\n\n" + result
		}
		result += "\n\n" + summary.Text()
		result += "\n\n" + laboratoryPracticalExplanation(group, summary)
		return result
	}
	decision := laboratoryMechanismDecisionText(group, values)
	if group.Code == "scale" {
		decision = laboratoryScaleDecisionText(values, modelCode)
	}
	conclusionTitle := "Предварительное заключение"
	if group.Code == "scale" && laboratoryScaleSixComponentReady(values) {
		conclusionTitle = "Итоговое осторожное заключение (требует уточнения)"
	}
	result := fmt.Sprintf("%s: %s\n\n%s\n\n%s\n\n%s\n\n%s\n\nГраницы являются рабочими скрининговыми ориентирами, а не нормативным диагнозом. Итог требует проверки лабораторией, ОЛИ/ОПИ и производственными данными.", conclusionTitle, group.Name, summary.Text(), decision, laboratoryDataCoverageTextForModel(group, values, modelCode), strings.Join(facts, "\n"))
	if group.Code == "corrosion" {
		result += "\n\n" + formatCorrosionInfluenceAssessment(assessCorrosionInfluence(values))
	}
	if group.Code == "scale" {
		result += "\n\n" + laboratorySulinMethodText(values)
		result += "\n\n" + strings.ReplaceAll(laboratorySaturationTextForModel(values, modelCode), "\r\n", "\n")
	}
	if validationText != "" {
		result = validationText + "\n\n" + result
	}
	result += "\n\n" + laboratoryPracticalExplanation(group, summary)
	return result
}

func latestLaboratoryRecordsForObject(objectKey string) map[string]LabRecord {
	if !labRecordsLoaded {
		labRecords = loadLabRecords()
	}
	objectKey = strings.TrimSpace(objectKey)
	out := map[string]LabRecord{}
	for _, r := range labRecords {
		if objectKey == "" || !strings.EqualFold(strings.TrimSpace(r.ObjectKey), objectKey) || r.GroupCode == "overall" {
			continue
		}
		old, exists := out[r.GroupCode]
		if !exists || r.CreatedAt > old.CreatedAt || (r.CreatedAt == old.CreatedAt && parseDate(r.Date).After(parseDate(old.Date))) {
			out[r.GroupCode] = r
		}
	}
	return out
}

func buildOverallLaboratoryConclusion(objectKey string) string {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return "Общая оценка не сформирована: укажите ключ объекта или подставьте выбранный объект. После этого будут сопоставлены последние сохранённые записи всех лабораторных разделов."
	}
	latest := latestLaboratoryRecordsForObject(objectKey)
	var lines, attention, uncertain []string
	for _, g := range labGroups {
		if g.Code == "overall" {
			continue
		}
		r, ok := latest[g.Code]
		if !ok {
			continue
		}
		g = laboratoryScaleInputGroup(g, r.ScaleModel)
		values := canonicalizeLaboratoryValues(g, r.Values, r.Units)
		assessment := laboratoryGroupSummary(g, values, r.ScaleModel)
		if assessment.Level >= 2 {
			attention = append(attention, g.Name)
		}
		if assessment.Level == 0 {
			uncertain = append(uncertain, g.Name)
		}
		lines = append(lines, fmt.Sprintf("РАЗДЕЛ: %s\nПоследняя запись: %s; заполнено %d из %d.\n%s", strings.ToUpper(g.Name), r.Date, labRecordFilled(r), len(g.Fields), assessment.Text()))
	}
	if len(lines) == 0 {
		return fmt.Sprintf("Общая оценка по объекту %s пока не сформирована: нет сохранённых лабораторных записей. Заполните нужные разделы и сохраните каждый анализ как новую запись.", objectKey)
	}
	priority := "По учтённым направлениям явных выходов за рабочие скрининговые границы не выявлено."
	if len(attention) == 1 {
		priority = "Приоритет дополнительной проверки: " + attention[0] + "."
	} else if len(attention) > 1 {
		priority = "Одновременно требуют проверки направления: " + strings.Join(attention, ", ") + ". Их совместное влияние следует оценивать комплексно по техрежиму, истории ремонтов и ОЛИ/ОПИ."
	}
	plain := "По направлениям, которые удалось оценить, признаков осложнения не выявлено. Это относится только к внесённым анализам; остальные причины и другие режимы работы не исключены."
	if len(attention) == 1 {
		plain = "В первую очередь разберите направление «" + attention[0] + "»: по нему есть показатели, которые могут быть связаны с осложнением. Проверьте их повторным анализом и сопоставьте с режимом работы и фактическим состоянием оборудования."
	} else if len(attention) > 1 {
		plain = "Признаки риска есть сразу по нескольким направлениям. Их нельзя рассматривать изолированно: сначала сопоставьте общий режим объекта, историю отказов и ремонтов, затем подтвердите ключевые показатели повторными анализами."
	}
	if len(uncertain) > 0 {
		if len(uncertain) == len(lines) {
			priority = "Общий риск пока не определён: представленные направления требуют уточнения."
		}
		priority += " Оценить пока нельзя: " + strings.Join(uncertain, ", ") + "."
		if len(attention) == 0 {
			plain = "Объект нельзя считать благополучным по этому выводу: часть направлений не удалось оценить. Сначала уточните данные по направлениям: " + strings.Join(uncertain, ", ") + "."
		}
	}
	var practical []string
	for _, g := range labGroups {
		if record, ok := latest[g.Code]; ok && g.Code != "overall" {
			practical = append(practical, laboratoryRecordPracticalExplanation(g, record))
		}
	}
	plain += "\n\n" + strings.Join(practical, "\n\n")
	return fmt.Sprintf("Общее предварительное заключение по объекту %s. Учтено лабораторных направлений: %d из 6. Для каждого направления взята последняя сохранённая запись; предыдущие записи истории не перезаписываются.\n\n%s\n\n%s\n\nКомплексная оценка является скрининговой и требует проверки специалистом, лабораторией и производственными данными.\n\nЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА\n%s", objectKey, len(lines), strings.Join(lines, "\n"), priority, plain)
}

func analyzeLaboratoryInputs() {
	atomic := beginLaboratoryAtomicUpdate()
	defer endLaboratoryAtomicUpdate(atomic)
	g := currentLabGroup()
	if g.Code == "overall" {
		setLaboratoryConclusion(buildCurrentLaboratoryPassConclusion())
		setText(labInfo, "Единое заключение сформировано по всем заполненным вкладкам текущей записи.")
	} else {
		setLaboratoryConclusion(buildLaboratoryConclusion(g, canonicalizeLaboratoryValues(g, currentLabValues(), currentLaboratoryUnits())))
		setText(labInfo, "Предварительный вывод сформирован и раскрыт ниже. Его также можно выгрузить в отчёт.")
	}
	// Analysis is an explicit action: show its result immediately instead of
	// leaving the conclusion edit hidden behind the collapsed short summary.
	labConclusionOpen = true
	var client RECT
	pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&client)))
	layoutLaboratory(client.Right-client.Left, client.Bottom-client.Top)
	pInvalidateRect.Call(uintptr(labShortConclusion), 0, 0)
	pInvalidateRect.Call(uintptr(labConclusionEdit), 0, 0)
	pInvalidateRect.Call(uintptr(labInfluenceChart), 0, 0)
}

func chooseLaboratoryReportPath(defaultName string) (string, string) {
	buf := make([]uint16, 32768)
	name, _ := syscall.UTF16FromString(defaultName)
	copy(buf, name)
	filter := utf16.Encode([]rune("PDF (*.pdf)\x00*.pdf\x00DOCX (*.docx)\x00*.docx\x00Excel (*.xlsx)\x00*.xlsx\x00Инфографика PNG (*.png)\x00*.png\x00\x00"))
	of := OPENFILENAMEW{
		LStructSize:  uint32(unsafe.Sizeof(OPENFILENAMEW{})),
		HwndOwner:    hMain,
		LpstrFilter:  &filter[0],
		NFilterIndex: 1,
		LpstrFile:    &buf[0],
		NMaxFile:     uint32(len(buf)),
		LpstrDefExt:  p("pdf"),
		LpstrTitle:   p("Выберите формат: PDF, DOCX, Excel или инфографика PNG"),
		Flags:        OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT,
	}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return "", ""
	}
	path := syscall.UTF16ToString(buf)
	format := "pdf"
	switch of.NFilterIndex {
	case 2:
		format = "docx"
	case 3:
		format = "xlsx"
	case 4:
		format = "png"
	}
	wanted := "." + format
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".pdf" && ext != ".docx" && ext != ".xlsx" && ext != ".png" {
		path += wanted
	} else if ext != wanted {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + wanted
	}
	return path, format
}

func laboratoryReportText() string {
	// Экспорт всегда формирует единый свод по всем лабораторным вкладкам текущей диагностики,
	// независимо от того, какая вкладка открыта в момент нажатия кнопки.
	pass := ensureActiveLabPass()
	pass.ObjectKey = strings.TrimSpace(getText(labObjectEdit))
	pass.Date = strings.TrimSpace(getText(labDateEdit))
	applyLaboratorySourceToPass(pass, getText(labSourceEdit))
	pass.Comment = strings.TrimSpace(getText(labCommentEdit))
	groups := cloneLabGroups(labDrafts)
	units := cloneLabGroups(labUnitDrafts)
	if g := currentLabGroup(); g.Code != "overall" {
		groups[g.Code] = currentLabValues()
		units[g.Code] = currentLaboratoryUnits()
	}
	pass.Groups = groups
	pass.Units = units
	pass.SharedValues = collectLaboratorySharedValues(pass.Groups)
	pass.ScaleModel = labScaleModelCode
	setLaboratoryConclusion(buildLaboratoryPassConclusion(*pass))
	calculations := ""
	if labInfoOptionsInclude[0] {
		calculations = "\r\n\r\n" + laboratoryCalculationAppendix(*pass)
	}
	brief := laboratoryReportBrief(*pass)
	sources := laboratoryReportSourceRows(*pass)
	source := strings.TrimSpace(pass.Source)
	if source == "" {
		source = "не указан"
	}
	return strings.ReplaceAll(fmt.Sprintf("ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ\nОбъект: %s\nДата анализа: %s\nИсточник / протокол: %s\n\n%s\n\n%s%s", emptyLabText(pass.ObjectKey), emptyLabText(pass.Date), source, brief, sources, calculations), "\n", "\r\n")
}

func exportLaboratoryReport() {
	if laboratoryExportBusy {
		return
	}
	if !chooseLaboratoryInfographicOptions() {
		return
	}
	key := safeFilePart(strings.TrimSpace(getText(labObjectEdit)))
	if key == "" {
		key = "объект"
	}
	path, format := chooseLaboratoryReportPath("РОСА_сводный_лабораторный_отчёт_" + key + "_" + time.Now().Format("20060102") + ".pdf")
	if path == "" {
		return
	}
	report := laboratoryReportText()
	laboratoryExportBusy = true
	setText(labExportButton, "Формирование отчёта…")
	pEnableWindow.Call(uintptr(labExportButton), 0)
	if format == "png" {
		laboratoryReportExportFinished(path, renderLaboratoryInfographicPNG(path, 100))
		return
	}
	go func() {
		var err error
		switch format {
		case "pdf":
			err = writeLaboratoryPDFReport(path, report)
		case "docx":
			err = writeLaboratoryDOCXReport(path, report)
		default:
			err = writeLaboratoryXLSXReport(path, report)
		}
		laboratoryExportResultMu.Lock()
		laboratoryExportResultPath, laboratoryExportResultErr = path, err
		laboratoryExportResultMu.Unlock()
		pPostMessageW.Call(uintptr(hMain), WM_APP_LAB_REPORT_DONE, 0, 0)
	}()
}

func laboratoryReportExportFinished(path string, err error) {
	laboratoryExportBusy = false
	pRedrawWindow.Call(uintptr(hMain), 0, 0, 0x0001|0x0004|0x0080)
	setText(labExportButton, "Выгрузить отчёт")
	pEnableWindow.Call(uintptr(labExportButton), 1)
	if err != nil {
		if fallback, ok := err.(*laboratoryPDFFallbackError); ok {
			msg("Выгрузка лабораторного отчёта", fallback.Error(), MB_OK|MB_ICONWARNING)
			return
		}
		msg("Выгрузка лабораторного отчёта", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Выгрузка лабораторного отчёта", "Отчёт сохранён:\n"+path, MB_OK|MB_ICONINFORMATION)
}

func laboratoryReportExportDone() {
	laboratoryExportResultMu.Lock()
	path, err := laboratoryExportResultPath, laboratoryExportResultErr
	laboratoryExportResultMu.Unlock()
	laboratoryReportExportFinished(path, err)
}

func laboratoryHistoryFieldCoverage(group LabGroupDef, values map[string]string, modelCode string) (filled, total int) {
	group = laboratoryScaleInputGroup(group, modelCode)
	for _, field := range group.Fields {
		// History must reflect only real laboratory inputs that belonged to the
		// model saved with this version. Calculated outputs (for example Sulin)
		// and values inherited from the current Tехрежим are context, not filled
		// laboratory cells, so neither may inflate the numerator or denominator.
		if !laboratoryInputFieldEligible(group.Code, field.Key) {
			continue
		}
		total++
		if strings.TrimSpace(values[field.Key]) != "" {
			filled++
		}
	}
	return filled, total
}

func labRecordFilled(r LabRecord) int {
	group, ok := labGroupByCode(r.GroupCode)
	if !ok {
		return 0
	}
	filled, _ := laboratoryHistoryFieldCoverage(group, r.Values, r.ScaleModel)
	return filled
}

func labSummaryForObject(objectKey string) string {
	if strings.TrimSpace(objectKey) == "" {
		return "—"
	}
	if !labRecordsLoaded {
		labRecords = loadLabRecords()
	}
	groups := map[string]bool{}
	latest := ""
	latestTime := time.Time{}
	for _, r := range labRecords {
		if r.ObjectKey != objectKey {
			continue
		}
		groups[r.GroupCode] = true
		t := parseDate(r.Date)
		if latest == "" || t.After(latestTime) {
			latest = r.Date
			latestTime = t
		}
	}
	if len(groups) == 0 {
		return "нет данных"
	}
	if latest == "" {
		return fmt.Sprintf("%d разд.", len(groups))
	}
	return fmt.Sprintf("%d разд.; %s", len(groups), latest)
}

func laboratoryCount() int {
	if !labRecordsLoaded {
		labRecords = loadLabRecords()
	}
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	savedPasses := 0
	for _, pass := range labPasses {
		if pass.Status == labPassSavedStatus {
			savedPasses++
		}
	}
	return len(labRecords) + savedPasses
}

func filteredLabRecords() []LabRecord {
	q := strings.ToLower(strings.TrimSpace(getText(labFilterEdit)))
	var out []LabRecord
	for _, r := range labRecords {
		if r.Deleted || isSupersededTrainingLabRecord(r) {
			continue
		}
		// История должна показывать весь архив тестовых проходок, а не только
		// текущий объект/раздел. Фильтрация выполняется только поисковой строкой.
		s := strings.ToLower(r.ObjectKey + " " + r.Source + " " + r.Comment + " " + r.Date)
		if q == "" || strings.Contains(s, q) {
			out = append(out, r)
		}
	}
	return out
}

func filteredSavedLabPasses() []LabPassRecord {
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	q := strings.ToLower(strings.TrimSpace(getText(labFilterEdit)))
	var out []LabPassRecord
	canonicalIDs := canonicalTrainingPassIDs()
	for _, pass := range labPasses {
		if pass.Status != labPassSavedStatus {
			continue
		}
		if isSupersededTrainingPass(pass) {
			if _, current := canonicalIDs[strings.ToUpper(strings.TrimSpace(pass.PassID))]; !current {
				continue
			}
		}
		// Не ограничиваем архив выбранным объектом: все проходки должны быть
		// видны в «Предыдущих анализах» одновременно.
		haystack := strings.ToLower(pass.ObjectKey + " " + pass.Source + " " + pass.Comment + " " + pass.Date + " " + pass.Author)
		if q == "" || strings.Contains(haystack, q) {
			out = append(out, pass)
		}
	}
	return out
}

func labPassFilled(pass LabPassRecord) int {
	count := 0
	for group, values := range pass.Groups {
		if group == "overall" {
			continue
		}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				count++
			}
		}
	}
	return count
}

func laboratoryPassHasGroupData(pass LabPassRecord, groupCode string) bool {
	group, ok := labGroupByCode(groupCode)
	if !ok || group.Code == "overall" {
		return false
	}
	filled, _ := laboratoryHistoryFieldCoverage(group, pass.Groups[group.Code], pass.ScaleModel)
	return filled > 0
}

func laboratoryPassFilledGroupCount(pass LabPassRecord) int {
	count := 0
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		if laboratoryPassHasGroupData(pass, group.Code) {
			count++
		}
	}
	return count
}

func buildVisibleLaboratoryHistory() []laboratoryHistoryEntry {
	var entries []laboratoryHistoryEntry
	for _, pass := range filteredSavedLabPasses() {
		entries = append(entries, laboratoryHistoryEntry{Kind: "pass", Pass: pass, Version: pass.PassID, Date: pass.Date, Created: pass.UpdatedAt})
	}
	for _, record := range filteredLabRecords() {
		entries = append(entries, laboratoryHistoryEntry{Kind: "legacy", Record: record, Version: record.ID, Date: record.Date, Created: record.CreatedAt})
	}
	if len(entries) < 4 {
		entries = append(entries, teachingLaboratoryHistoryEntries(selectedObjectKey, time.Now())...)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := parseDate(entries[i].Date), parseDate(entries[j].Date)
		if !left.Equal(right) {
			return left.After(right)
		}
		return entries[i].Created > entries[j].Created
	})
	return entries
}

func laboratoryPassGroupShortConclusion(pass LabPassRecord, groupCode string) string {
	group, ok := labGroupByCode(groupCode)
	if !ok || group.Code == "overall" {
		return "Оценить пока нельзя; требуется уточнение данных."
	}
	if !laboratoryPassHasGroupData(pass, group.Code) {
		return group.Name + ": данные не заполнены."
	}
	inputGroup := laboratoryScaleInputGroup(group, pass.ScaleModel)
	values := canonicalizeLaboratoryValues(inputGroup, pass.Groups[group.Code], pass.Units[group.Code])
	assessment := laboratoryGroupSummary(inputGroup, values, pass.ScaleModel)
	return group.Name + ": " + laboratorySummaryShortMessage(assessment.Message)
}

func laboratoryPassOverallShortConclusion(pass LabPassRecord) string {
	filled, attention, low, unknown := 0, 0, 0, 0
	for _, group := range labGroups {
		if group.Code == "overall" || !laboratoryPassHasGroupData(pass, group.Code) {
			continue
		}
		inputGroup := laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := canonicalizeLaboratoryValues(inputGroup, pass.Groups[group.Code], pass.Units[group.Code])
		assessment := laboratoryGroupSummary(inputGroup, values, pass.ScaleModel)
		filled++
		switch {
		case assessment.Level >= 2:
			attention++
		case assessment.Level == 1:
			low++
		default:
			unknown++
		}
	}
	if filled == 0 {
		return "Общая диагностика: заполненных лабораторных направлений нет."
	}
	return fmt.Sprintf("Учтено %d из 6: признаки/погранично — %d; по доступным данным без признаков — %d; оценить пока нельзя — %d.", filled, attention, low, unknown)
}

func laboratoryHistoryEntryColumns(entry laboratoryHistoryEntry) []string {
	return laboratoryHistoryEntryColumnsWithCaptions(entry, readableObjectKeyIndex())
}

func laboratoryHistoryEntryColumnsWithCaptions(entry laboratoryHistoryEntry, captions map[string]string) []string {
	if entry.Kind == "pass" {
		pass := entry.Pass
		section := "Общая диагностика"
		filled := fmt.Sprintf("%d разд.", laboratoryPassFilledGroupCount(pass))
		summary := laboratoryPassOverallShortConclusion(pass)
		if labActiveGroup != "overall" {
			if group, ok := labGroupByCode(labActiveGroup); ok {
				section = group.Name + " · единая версия"
				count, total := laboratoryHistoryFieldCoverage(group, pass.Groups[group.Code], pass.ScaleModel)
				filled = fmt.Sprintf("%d/%d", count, total)
				summary = laboratoryPassGroupShortConclusion(pass, group.Code)
			}
		}
		return []string{pass.PassID, pass.Date, readableObjectKeyFromIndex(pass.ObjectKey, captions), section, pass.Source, filled, laboratoryEntryStatus(pass.ImportedAt, pass.ImportFileName, pass.CreatedAt), pass.Comment, summary}
	}
	record := entry.Record
	sectionSuffix := " · старая запись"
	status := laboratoryEntryStatus(record.ImportedAt, "", record.CreatedAt)
	if entry.Kind == "teaching" {
		sectionSuffix = " · учебная история"
		status = "УЧЕБНЫЙ ПРИМЕР · синтетическая запись"
	}
	return []string{record.ID, record.Date, readableObjectKeyFromIndex(record.ObjectKey, captions), record.GroupName + sectionSuffix, record.Source, fmt.Sprintf("%d/%d", labRecordFilled(record), labRecordFieldTotal(record)), status, record.Comment, laboratoryRecordShortConclusion(record)}
}
func laboratoryEntryStatus(importedAt, filename, createdAt string) string {
	stamp := createdAt
	if importedAt != "" {
		stamp = importedAt
	}
	date := stamp
	if parsed, err := time.Parse(time.RFC3339, stamp); err == nil {
		date = parsed.Local().Format("02.01.2006 15:04")
	}
	if date == "" {
		date = "дата не сохранена"
	}
	if importedAt != "" {
		if filename == "" {
			filename = "имя файла ранее не сохранено"
		}
		return "Загружено: " + filename + " · " + date
	}
	return "Ручной ввод · " + date
}

// Cached visual fingerprint excludes layout, which changes independently on DPI
// and tab transitions. Actual native item count is still checked after every
// reload to detect a ListView reset or a failed row insertion.
var labHistoryRenderSignature uint64

func refreshLaboratory() {
	if labHistoryList == 0 {
		return
	}
	if !labRecordsLoaded {
		labRecords = loadLabRecords()
	}
	if strings.TrimSpace(getText(labObjectEdit)) == "" && selectedObjectKey != "" {
		setText(labObjectEdit, selectedObjectKey)
		handleLaboratoryObjectChange()
	} else if applyTechRegimeContextToLaboratory(getText(labObjectEdit)) {
		// Do not save the still-empty visible control over the value that has just
		// been seeded from the current Tехрежим.
		labObjectSwitching = true
		switchLabGroup(labActiveGroup, false)
		labObjectSwitching = false
	}
	updateLaboratoryObjectCaption()
	x := filteredLabRecords()
	passes := filteredSavedLabPasses()
	labHistoryEntries = buildVisibleLaboratoryHistory()
	setText(labHistoryTitle, fmt.Sprintf("Предыдущие анализы · %d записей", len(labHistoryEntries)))
	limit := len(labHistoryEntries)
	if limit > 500 {
		limit = 500
	}
	// Preparing row text is much cheaper than clearing/reinserting hundreds of
	// native ListView items for every complication tab click. Store nothing if
	// any actual caption changes; preserve the existing scroll position if not.
	captions := readableObjectKeyIndex()
	rows := make([][]string, 0, limit)
	hash := fnv.New64a()
	for i := 0; i < limit; i++ {
		row := laboratoryHistoryEntryColumnsWithCaptions(labHistoryEntries[i], captions)
		rows = append(rows, row)
		for _, cell := range row {
			_, _ = hash.Write([]byte(cell))
			_, _ = hash.Write([]byte{0})
		}
	}
	signature := hash.Sum64()
	const lvmGetItemCount = 0x1004
	count, _, _ := pSendMessageW.Call(uintptr(labHistoryList), lvmGetItemCount, 0, 0)
	if signature != labHistoryRenderSignature || int(count) != limit {
		listBeginUpdate(labHistoryList)
		listClear(labHistoryList)
		for i, row := range rows {
			listAdd(labHistoryList, i, row)
		}
		listEndUpdate(labHistoryList)
		count, _, _ = pSendMessageW.Call(uintptr(labHistoryList), lvmGetItemCount, 0, 0)
		if int(count) == limit {
			labHistoryRenderSignature = signature
		} else {
			labHistoryRenderSignature = 0
			diagLog("LAB history incomplete expected=%d got=%d", limit, count)
		}
	}
	g := currentLabGroup()
	if g.Code == "overall" {
		setLaboratoryConclusion(buildCurrentLaboratoryPassConclusion())
		setText(labInfo, fmt.Sprintf("Общая оценка: сохранённых единых диагностик %d; старых раздельных записей %d.", len(passes), len(x)))
	} else {
		setText(labInfo, fmt.Sprintf("Раздел «%s»: сохранённых версий %d. Обязательные и дополнительные показатели обозначены в легенде сверху.", g.Name, len(labHistoryEntries)))
	}
	refreshSectionNotice()
}

func selectLabHistory() {
	i := selectedRow(labHistoryList)
	if i < 0 || i >= len(labHistoryEntries) {
		return
	}
	entry := labHistoryEntries[i]
	if entry.Kind == "teaching" {
		msg("Учебная история", "Это синтетическая учебная запись. Она показывает связанный сценарий для проверки интерфейса и не заменяет текущий лабораторный анализ.", MB_OK|MB_ICONINFORMATION)
		return
	}
	section := "Общая диагностика"
	objectKey, date := entry.Pass.ObjectKey, entry.Pass.Date
	if entry.Kind == "legacy" {
		section = entry.Record.GroupName + " (старая раздельная запись)"
		objectKey, date = entry.Record.ObjectKey, entry.Record.Date
	} else if labActiveGroup != "overall" {
		if group, ok := labGroupByCode(labActiveGroup); ok {
			section = group.Name + " (единая диагностика)"
		}
	}
	if !confirmLaboratoryHistoryRestore(entry.Version, objectKey, date, section) {
		return
	}
	preferredGroup := labActiveGroup
	pass := entry.Pass
	if entry.Kind == "legacy" {
		pass = legacyLabPassView(entry.Record)
		preferredGroup = entry.Record.GroupCode
	}
	openLaboratoryPass(pass, preferredGroup)
	setText(labInfo, "Загружена рабочая копия версии "+entry.Version+". Вывод заново сформирован по сохранённым исходным данным в актуальной структуре; исходная запись не изменяется, сохранение создаст новую версию.")
}

func laboratoryPassHasAnyData(pass LabPassRecord) bool {
	for group, values := range pass.Groups {
		if group == "overall" {
			continue
		}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return true
			}
		}
	}
	return false
}

func tombstoneLegacyLabRecord(record LabRecord) error {
	record.Deleted = true
	record.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return appendLabRecord(record)
}

func deleteLaboratoryHistoryEntry(entry laboratoryHistoryEntry, groupCode string) error {
	if entry.Kind == "legacy" {
		return tombstoneLegacyLabRecord(entry.Record)
	}
	pass := entry.Pass
	if groupCode == "overall" {
		pass.Status = labPassDiscardedStatus
	} else {
		delete(pass.Groups, groupCode)
		delete(pass.Units, groupCode)
		if !laboratoryPassHasAnyData(pass) {
			pass.Status = labPassDiscardedStatus
		} else {
			pass.Status = labPassSavedStatus
		}
	}
	pass.UpdatedAt = ""
	return appendLabPass(pass)
}

func restoreLaboratoryHistoryEntries(entries []laboratoryHistoryEntry) error {
	for _, entry := range entries {
		if entry.Kind == "legacy" {
			record := entry.Record
			record.Deleted = false
			record.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := appendLabRecord(record); err != nil {
				return err
			}
			continue
		}
		pass := entry.Pass
		pass.Status = labPassSavedStatus
		pass.UpdatedAt = ""
		if err := appendLabPass(pass); err != nil {
			return err
		}
	}
	labRecords = loadLabRecords()
	labRecordsLoaded = true
	labPasses = loadLabPasses()
	labPassesLoaded = true
	invalidateMethodsBackground()
	refreshLaboratory()
	return nil
}

func deleteSelectedLaboratoryHistory() {
	index := selectedRow(labHistoryList)
	if index < 0 || index >= len(labHistoryEntries) {
		msg("Лабораторные данные", "Сначала выберите строку в списке сохранённых расчётов.", MB_OK|MB_ICONWARNING)
		return
	}
	entry := labHistoryEntries[index]
	scope := "всю выбранную лабораторную диагностику"
	if labActiveGroup != "overall" {
		if group, ok := labGroupByCode(labActiveGroup); ok {
			scope = "только данные раздела «" + group.Name + "» из выбранной версии; остальные лабораторные разделы этой версии останутся сохранены"
		}
	}
	text := fmt.Sprintf("Удалить выбранную строку?\n\nВерсия: %s\nДата: %s\n\nБудет удалено: %s.\n\nПродолжить?", entry.Version, entry.Date, scope)
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Удаление лабораторной записи"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	if err := deleteLaboratoryHistoryEntry(entry, labActiveGroup); err != nil {
		msg("Лабораторные данные", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	deletedGroup := labActiveGroup
	addSessionTrashItem("Лабораторная диагностика · "+entry.Version, "Объект: "+readableObjectKey(entry.Pass.ObjectKey)+"\r\nДата: "+entry.Date+"\r\nУдалённый раздел: "+deletedGroup, func() error {
		return restoreLaboratoryHistoryEntries([]laboratoryHistoryEntry{entry})
	}, nil)
	// Любое ручное удаление фиксирует запрет повторной демонстрационной
	// автозагрузки в текущем профиле.
	_ = os.WriteFile(filepath.Join(dataDir, "demo_seed_complete"), []byte(time.Now().Format(time.RFC3339)), 0600)
	labRecords = loadLabRecords()
	labRecordsLoaded = true
	labPasses = loadLabPasses()
	labPassesLoaded = true
	refreshLaboratory()
	setText(labInfo, "Выбранная строка удалена. Другие сохранённые лабораторные данные не изменены.")
	invalidateMethodsBackground()
}

func clearLaboratoryCurrentInput() {
	g, ok := labGroupByCode(labActiveGroup)
	if !ok || g.Code == "overall" {
		msg("Лабораторные данные", "На странице общей оценки очистка выполняется через редактирование разделов.", MB_OK|MB_ICONINFORMATION)
		return
	}
	text := fmt.Sprintf("Очистить введённые значения текущей страницы раздела «%s»?\n\nСохранённые диагностики и другие разделы не изменятся.", g.Name)
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Очистить введённое"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	if labDrafts[g.Code] == nil {
		labDrafts[g.Code] = map[string]string{}
	}
	start := labFieldsPage * 10
	for i := start; i < len(g.Fields) && i < start+10; i++ {
		delete(labDrafts[g.Code], g.Fields[i].Key)
		if labUnitDrafts[g.Code] != nil {
			delete(labUnitDrafts[g.Code], g.Fields[i].Key)
		}
	}
	switchLabGroup(g.Code, false)
	setText(labInfo, "Введённые значения текущей страницы очищены. Сохранённые диагностики не изменены.")
}

func clearLaboratoryHistory() {
	objectKey := strings.TrimSpace(getText(labObjectEdit))
	if objectKey == "" {
		msg("Лабораторные данные", "Для полной очистки сначала укажите или выберите объект. Очистка без ключа объекта запрещена.", MB_OK|MB_ICONWARNING)
		return
	}
	groupCode := labActiveGroup
	label := "все лабораторные диагностики и старые записи объекта"
	if groupCode != "overall" {
		if group, ok := labGroupByCode(groupCode); ok {
			label = "все сохранённые строки раздела «" + group.Name + "» для объекта; данные остальных лабораторных разделов останутся сохранены"
		}
	}
	text := fmt.Sprintf("Очистить лабораторные данные?\n\nОбъект: %s\n\nБудет удалено: %s.\n\nЭто отдельная операция полной очистки. Продолжить?", objectKey, label)
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Очистка лабораторных данных"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	normalized := normalizeLaboratoryObjectKey(objectKey)
	var firstErr error
	var deletedEntries []laboratoryHistoryEntry
	for _, pass := range loadLabPasses() {
		if pass.Status != labPassSavedStatus || normalizeLaboratoryObjectKey(pass.ObjectKey) != normalized {
			continue
		}
		entry := laboratoryHistoryEntry{Kind: "pass", Pass: pass, Version: pass.PassID, Date: pass.Date}
		if groupCode != "overall" && !laboratoryPassHasGroupData(pass, groupCode) {
			continue
		}
		if err := deleteLaboratoryHistoryEntry(entry, groupCode); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			deletedEntries = append(deletedEntries, entry)
		}
	}
	for _, record := range loadLabRecords() {
		if normalizeLaboratoryObjectKey(record.ObjectKey) != normalized {
			continue
		}
		if groupCode != "overall" && record.GroupCode != groupCode {
			continue
		}
		if err := tombstoneLegacyLabRecord(record); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			deletedEntries = append(deletedEntries, laboratoryHistoryEntry{Kind: "legacy", Record: record, Version: record.ID, Date: record.Date})
		}
	}
	labRecords = loadLabRecords()
	labRecordsLoaded = true
	labPasses = loadLabPasses()
	labPassesLoaded = true
	refreshLaboratory()
	if len(deletedEntries) > 0 {
		invalidateMethodsBackground()
		entries := append([]laboratoryHistoryEntry(nil), deletedEntries...)
		addSessionTrashItem(fmt.Sprintf("Лаборатория · %d записей", len(entries)), "Объект: "+readableObjectKey(objectKey)+"\r\nОбласть очистки: "+label, func() error {
			return restoreLaboratoryHistoryEntries(entries)
		}, nil)
	}
	if firstErr != nil {
		msg("Лабораторные данные", "Часть строк не удалось удалить: "+firstErr.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	// После очистки демонстрационный набор не должен появляться снова при
	// следующем запуске приложения.
	_ = os.WriteFile(filepath.Join(dataDir, "demo_seed_complete"), []byte(time.Now().Format(time.RFC3339)), 0600)
	setText(labInfo, "Очистка выполнена для выбранного объекта и текущего лабораторного раздела.")
}

func confirmLaboratoryHistoryRestore(version, objectKey, date, section string) bool {
	warning := ""
	if selectedObjectKey != "" && !laboratoryObjectKeysEquivalent(objectKey, selectedObjectKey) {
		warning = fmt.Sprintf("\n\nВНИМАНИЕ: в лаборатории будет открыт другой объект.\nЗакреплён на Главной: %s.\nОткрывается временно: %s.", readableObjectKey(selectedObjectKey), readableObjectKey(objectKey))
	}
	text := fmt.Sprintf("Открыть сохранённую запись временно?\n\nШифр: %s\nОбъект: %s\nДата анализа: %s\nРаздел: %s%s\n\nТекущие введённые данные будут сохранены как черновик. Закреплённый на Главной объект не изменится. Кнопка возврата восстановит его черновик. Продолжить?", version, readableObjectKey(objectKey), date, section, warning)
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Замена текущих лабораторных данных"))), 0x00000004|MB_ICONWARNING)
	if r == 6 && strings.TrimSpace(getText(labObjectEdit)) != "" {
		saveLabDraft()
		if strings.EqualFold(strings.TrimSpace(getText(labObjectEdit)), strings.TrimSpace(selectedObjectKey)) {
			labPinnedDraftID = labActivePassID
		}
	}
	return r == 6
}

func laboratoryOverallHistorySelection(index int, passes []LabPassRecord, records []LabRecord) (LabPassRecord, *LabRecord, string) {
	if index < 0 {
		return LabPassRecord{}, nil, ""
	}
	if index < len(passes) {
		return passes[index], nil, "pass"
	}
	recordIndex := index - len(passes)
	if recordIndex < 0 || recordIndex >= len(records) {
		return LabPassRecord{}, nil, ""
	}
	return LabPassRecord{}, &records[recordIndex], "legacy"
}

func discardActiveLaboratoryDraft() {
	key := labActiveObjectKey + "|" + labActivePassID
	if pass := labPassDrafts[key]; pass != nil {
		if pass.Status == labPassDraftStatus {
			pass.Status = labPassDiscardedStatus
			_ = appendLabPass(*pass)
		}
		delete(labPassDrafts, key)
	}
	labActivePassID = ""
	labDrafts = map[string]map[string]string{}
	labUnitDrafts = map[string]map[string]string{}
}

func firstLaboratoryGroupWithData(pass LabPassRecord) string {
	for _, group := range labGroups {
		if group.Code != "overall" && laboratoryPassHasGroupData(pass, group.Code) {
			return group.Code
		}
	}
	return "corrosion"
}

func openLaboratoryPass(source LabPassRecord, preferredGroup string) {
	pass, err := laboratoryDraftFromSavedPass(source, time.Now())
	if err != nil {
		msg("Открытие сохранённого анализа", "Новый черновик не создан: "+err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	teachingCompleteLabDraft(&pass)
	if scenario, full := teachingFullScenarioForObject(pass.ObjectKey); full {
		switch scenario.Ordinal {
		case 1:
			preferredGroup = "scale"
		case 58:
			preferredGroup = "corrosion"
		case 90:
			preferredGroup = "aspo"
		}
	}
	// Opening history starts another working copy while preserving the prior draft.
	if preferredGroup == "" || preferredGroup == "overall" || !laboratoryPassHasGroupData(pass, preferredGroup) {
		preferredGroup = firstLaboratoryGroupWithData(pass)
	}
	labObjectSwitching = true
	setText(labObjectEdit, pass.ObjectKey)
	setText(labDateEdit, pass.Date)
	setText(labSourceEdit, pass.Source)
	setText(labCommentEdit, pass.Comment)
	labActiveObjectKey = normalizeLaboratoryObjectKey(pass.ObjectKey)
	labActivePassID = pass.PassID
	labDrafts = cloneLabGroups(pass.Groups)
	labUnitDrafts = cloneLabGroups(pass.Units)
	setLaboratoryScaleModel(pass.ScaleModel)
	copyPass := pass
	labPassDrafts[labActiveObjectKey+"|"+pass.PassID] = &copyPass
	labFieldsPage = 0
	switchLabGroup(preferredGroup, false)
	labObjectSwitching = false
	updateLaboratoryObjectCaption()
	// Historical text may have been written by an older version with a different
	// conclusion structure. Rebuild only the visible interpretation from the
	// immutable saved inputs; the saved record itself remains unchanged.
	if scenario, full := teachingFullScenarioForObject(pass.ObjectKey); full {
		setLaboratoryConclusion("УЧЕБНЫЙ СЦЕНАРИЙ · " + scenario.Name + ". Данные синтетические; показатели заполнены для проверки маршрута РОСА.")
	} else {
		setLaboratoryConclusion(laboratoryHistoryConclusion(source, preferredGroup))
	}
	// A selected saved or teaching example must expose its freshly rebuilt
	// conclusion immediately instead of looking like an empty calculation.
	_, teaching := teachingFullScenarioForObject(pass.ObjectKey)
	labConclusionOpen = !teaching
	if currentPage == ID_LAB {
		var client RECT
		pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&client)))
		layoutLaboratory(client.Right-client.Left, client.Bottom-client.Top)
		refreshLaboratory()
	}
}

func approveOpenLaboratoryObject() bool {
	openedKey := strings.TrimSpace(getText(labObjectEdit))
	if openedKey == "" {
		return false
	}
	if selectedObjectKey != "" && laboratoryObjectKeysEquivalent(openedKey, selectedObjectKey) {
		return true
	}
	text := laboratoryOffMainRepinWarning(openedKey)
	if !askYesNo("Перезакрепить объект вне Главной?", text) {
		return false
	}
	if !applyGlobalObjectSelection(openedKey, true) {
		msg("Перезакрепление объекта", "Объект не изменён. Текущие данные и глобальный ключ сохранены без изменений.", MB_OK|MB_ICONWARNING)
		return false
	}
	updateLaboratoryObjectCaption()
	if !laboratoryObjectKeysEquivalent(strings.TrimSpace(selectedObjectKey), openedKey) {
		msg("Перезакрепление объекта", "Не удалось подтвердить новый глобальный ключ объекта. Изменение отменено для дальнейших расчётов.", MB_OK|MB_ICONERROR)
		return false
	}
	return true
}

// laboratoryOffMainRepinWarning makes the cross-page consequence explicit.
// Re-pinning from a saved laboratory/teaching record changes the global object,
// but it cannot guarantee that every downstream section already has current
// data for that object. The warning appears before the global key is changed.
func laboratoryOffMainRepinWarning(openedKey string) string {
	return fmt.Sprintf("ПЕРЕЗАКРЕПИТЬ ОБЪЕКТ НЕ С ГЛАВНОЙ?\n\n%s\n\nОбъект станет рабочим для всех вкладок РОСА. После перезакрепления необходимо последовательно открыть все вкладки и проверить, дополнить или обновить данные по этому объекту: лабораторию, классы осложнений, методы и мероприятия, МРП и отказы, экономику и итоговое заключение.\n\nРанее сохранённые сведения будут восстановлены, но их актуальность нужно подтвердить. Продолжить?", readableObjectKey(openedKey))
}

func useSelectedObjectForLab() {
	openedKey := strings.TrimSpace(getText(labObjectEdit))
	// When a saved or teaching example is open, the primary action must approve
	// that visible object globally.  The old behaviour silently restored the
	// previous Main-page object, leaving the assessment page disconnected from
	// the fully populated laboratory record.
	if openedKey != "" && (selectedObjectKey == "" || !laboratoryObjectKeysEquivalent(openedKey, selectedObjectKey)) {
		if !approveOpenLaboratoryObject() {
			return
		}
		refreshAssessments()
		msg("Объект утверждён", "Для всех вкладок закреплён объект:\n"+readableObjectKey(openedKey)+"\n\nКлассификация осложнений рассчитана по открытой лабораторной записи. Проверьте и при необходимости дополните или обновите данные на остальных вкладках.", MB_OK|MB_ICONINFORMATION)
		return
	}
	if selectedObjectKey == "" {
		msg("Лабораторные данные", "Сначала откройте лабораторную запись либо выберите объект на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	if normalizeLaboratoryObjectKey(getText(labObjectEdit)) != normalizeLaboratoryObjectKey(selectedObjectKey) || labPinnedDraftID != "" && labActivePassID != labPinnedDraftID {
		saveLabDraft()
	}
	if labPinnedDraftID != "" && labActivePassID != labPinnedDraftID {
		if draft := labPassDrafts[normalizeLaboratoryObjectKey(selectedObjectKey)+"|"+labPinnedDraftID]; draft != nil {
			labObjectSwitching = true
			setText(labObjectEdit, selectedObjectKey)
			setText(labDateEdit, draft.Date)
			setText(labSourceEdit, draft.Source)
			setText(labCommentEdit, draft.Comment)
			labActiveObjectKey = normalizeLaboratoryObjectKey(selectedObjectKey)
			labActivePassID = draft.PassID
			labDrafts = cloneLabGroups(draft.Groups)
			labUnitDrafts = cloneLabGroups(draft.Units)
			setLaboratoryScaleModel(draft.ScaleModel)
			labObjectSwitching = false
			labPinnedDraftID = ""
			labFieldsPage = 0
			switchLabGroup(labActiveGroup, false)
			refreshLaboratory()
			return
		}
	}
	setText(labObjectEdit, selectedObjectKey)
	handleLaboratoryObjectChange()
	applyTechRegimeContextToLaboratory(selectedObjectKey)
	switchLabGroup(labActiveGroup, false)
	refreshLaboratory()
}

func updateLaboratoryObjectCaption() {
	if labObjectCaption == 0 {
		return
	}
	key := strings.TrimSpace(getText(labObjectEdit))
	caption := "Выберите объект на Главной"
	if key != "" {
		caption = readableObjectKey(key)
		if selectedObjectKey != "" && !laboratoryObjectKeysEquivalent(key, selectedObjectKey) {
			caption = "⚠ ДРУГОЙ ОБЪЕКТ: " + caption
		}
	}
	setText(labObjectCaption, caption)
	if labUseObjectButton != 0 {
		buttonCaption := "Объект с Главной"
		if key != "" && (selectedObjectKey == "" || !laboratoryObjectKeysEquivalent(key, selectedObjectKey)) {
			buttonCaption = "ЗАКРЕПИТЬ ЭТОТ ОБЪЕКТ"
		} else if key != "" && selectedObjectKey != "" {
			buttonCaption = "ОБЪЕКТ ЗАКРЕПЛЁН"
		}
		setText(labUseObjectButton, buttonCaption)
	}
}

const laboratoryBottomSafeMargin int32 = 32

type laboratoryBottomGeometry struct {
	ConclusionTitleY int32
	ConclusionY      int32
	ConclusionH      int32
	PanelTop         int32
	CommentY         int32
	InfoY            int32
	HistoryY         int32
	HistoryH         int32
}

func calculateLaboratoryBottomGeometry(h int32, expanded bool) laboratoryBottomGeometry {
	bottom := h - laboratoryBottomSafeMargin
	if expanded {
		conclusionY := py(344)
		available := max32(256, bottom-conclusionY)
		historyH := min32(160, max32(112, available/4))
		historyY := bottom - historyH
		infoY := historyY - 34
		commentY := infoY
		return laboratoryBottomGeometry{
			ConclusionTitleY: py(320),
			ConclusionY:      conclusionY,
			ConclusionH:      max32(80, infoY-conclusionY-12),
			PanelTop:         conclusionY,
			CommentY:         commentY,
			InfoY:            infoY,
			HistoryY:         historyY,
			HistoryH:         max32(0, bottom-historyY),
		}
	}
	// Five rows end at design Y=630. Keep a dedicated gap below them so the
	// explanatory line and history header can never cover the last inputs.
	panelTop := py(648)
	historyY := min32(bottom, panelTop+32)
	return laboratoryBottomGeometry{
		PanelTop: panelTop,
		CommentY: panelTop,
		InfoY:    panelTop,
		HistoryY: historyY,
		HistoryH: max32(0, bottom-historyY),
	}
}

func setLaboratoryFieldsVisible(visible bool) {
	g := currentLabGroup()
	start := labFieldsPage * 10
	for i := 0; i < 10; i++ {
		fieldVisible := visible && g.Code != "overall" && start+i < len(g.Fields)
		show(labFieldBadges[i], fieldVisible)
		show(labFieldLabels[i], fieldVisible)
		show(labFieldEdits[i], fieldVisible)
		show(labUnitLabels[i], fieldVisible)
	}
	pages := (len(g.Fields) + 9) / 10
	pagerVisible := visible && g.Code != "overall" && pages > 1
	show(labFieldsPrev, pagerVisible)
	show(labFieldsNext, pagerVisible)
	show(labFieldsPageLabel, pagerVisible)
}

const (
	laboratoryConceptGap      = int32(18)
	laboratoryFieldLabelH     = int32(24)
	laboratoryFieldEditGap    = int32(4)
	laboratoryFieldEditH      = int32(30)
	laboratoryFieldRowGap     = int32(4)
	laboratoryFieldEditOffset = laboratoryFieldLabelH + laboratoryFieldEditGap
	laboratoryFieldRowStep    = laboratoryFieldEditOffset + laboratoryFieldEditH + laboratoryFieldRowGap
)

type laboratoryVerticalLayout struct {
	TitleY, LegendY, TabsY, TabsSecondY                int32
	ObjectY, SourceY, SummaryY, OverviewGap            int32
	TopActionH, LegendH, TabH, ObjectButtonH, SummaryH int32
}

func laboratoryTabFirstRowCount(contentW int32) int32 {
	// RC162: on narrower/high-DPI clients the visually longest captions must not
	// be forced into a 4-column 26/30 px strip. Keep three tabs on the first row
	// so wrapping has real horizontal space; wider desktops retain four.
	if contentW < 1050 {
		return 3
	}
	return 4
}

func laboratoryVerticalLayoutForHeight(h int32) laboratoryVerticalLayout {
	if h < 900 {
		// RC132: never move the laboratory title/actions into the global warning band.
		// Compact only the rows below the title; preserve a readable, non-overlapping
		// metadata stack even on high-DPI/short clients.
		return laboratoryVerticalLayout{
			TitleY: 94, LegendY: 140, TabsY: 190, TabsSecondY: 236,
			ObjectY: 282, SourceY: 314, SummaryY: 320, OverviewGap: 6,
			TopActionH: 42, LegendH: 44, TabH: 40, ObjectButtonH: 26, SummaryH: 28,
		}
	}
	if h < 1150 {
		// RC132: 1080p-class layout keeps the enlarged history area, but restores
		// proper vertical bands: title/actions, legend, two tab rows, metadata,
		// short conclusion, inputs. Nothing is allowed to intrude into the warning.
		return laboratoryVerticalLayout{
			TitleY: 94, LegendY: 138, TabsY: 190, TabsSecondY: 238,
			ObjectY: 286, SourceY: 322, SummaryY: 324, OverviewGap: 8,
			TopActionH: 42, LegendH: 46, TabH: 42, ObjectButtonH: 30, SummaryH: 30,
		}
	}
	return laboratoryVerticalLayout{
		TitleY: 94, LegendY: 150, TabsY: 204, TabsSecondY: 256,
		ObjectY: 310, SourceY: 352, SummaryY: 398, OverviewGap: 14,
		TopActionH: 44, LegendH: 48, TabH: 42, ObjectButtonH: 34, SummaryH: 44,
	}
}

// laboratorySummaryVisualY guarantees a real gap after the last metadata row.
// RC131 subtracted a width-dependent verticalShift only from the summary/input
// band, which could pull the short conclusion back over the object/source row.
func laboratorySummaryVisualY(v laboratoryVerticalLayout, compactMetadata bool) int32 {
	metadataBottom := v.ObjectY + 30
	if !compactMetadata {
		metadataBottom = max32(metadataBottom, v.SourceY+28)
	}
	return max32(v.SummaryY, metadataBottom+8)
}

type laboratoryFieldLayoutMetrics struct {
	LabelH, EditGap, EditH, RowGap       int32
	EditOffset, RowStep, FieldBaseOffset int32
	PagerOffset, PagerH, HistoryGap      int32
}

func laboratoryFieldLabelHeightForText(base int32, text string) int32 {
	return laboratoryFieldLabelHeightForWidth(base, text, 0)
}

func laboratoryFieldLabelHeightForWidth(base int32, text string, width int32) int32 {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return base
	}
	// RC180: reserve height from the REAL caption width, not only from total
	// rune count. This catches the Windows/DPI case where a medium-length label
	// becomes 3-4 visual lines because its input column is narrow.
	explicit := strings.Split(trimmed, "\n")
	visualLines := int32(0)
	charsPerLine := 0
	if width > 0 {
		charsPerLine = int(max32(12, (width-8)/7))
	}
	for _, line := range explicit {
		runes := utf8.RuneCountInString(strings.TrimSpace(line))
		if charsPerLine <= 0 || runes <= charsPerLine {
			visualLines++
			continue
		}
		wrapped := int32((runes + charsPerLine - 1) / charsPerLine)
		visualLines += max32(1, wrapped)
	}
	if visualLines < 1 {
		visualLines = 1
	}
	allRunes := utf8.RuneCountInString(strings.ReplaceAll(trimmed, "\n", ""))
	if allRunes > 68 && visualLines < 4 {
		visualLines = 4
	} else if allRunes > 48 && visualLines < 3 {
		visualLines = 3
	} else if (allRunes > 30 || strings.Contains(trimmed, "Окислительно-восстановительный")) && visualLines < 2 {
		visualLines = 2
	}
	switch {
	case visualLines >= 5:
		return max32(base, 96)
	case visualLines == 4:
		return max32(base, 78)
	case visualLines == 3:
		return max32(base, 60)
	case visualLines == 2:
		return max32(base, 42)
	default:
		return base
	}
}

func laboratoryFieldLayoutForHeight(h int32) laboratoryFieldLayoutMetrics {
	if h < 900 {
		// RC139: the user still observed clipped edit borders. On compact clients keep
		// a real themed control envelope: slightly taller edits and a dedicated gap
		// above/below the frame so the border is never visually cut off.
		labelH, editGap, editH, rowGap := int32(22), int32(2), int32(26), int32(2)
		editOffset := labelH + editGap
		// RC145: preserve the new horizontal proportions, but move the whole
		// input/side-card envelope lower relative to the pager so the first
		// caption and edit frame are not visually clipped.
		return laboratoryFieldLayoutMetrics{
			LabelH: labelH, EditGap: editGap, EditH: editH, RowGap: rowGap,
			EditOffset: editOffset, RowStep: editOffset + editH + rowGap,
			FieldBaseOffset: 70, PagerOffset: 30, PagerH: 24, HistoryGap: 8,
		}
	}
	if h < 1150 {
		// RC139: restore a safer edit-box frame on normal 1080p/high-DPI Windows.
		// The input border needs a little more vertical room than RC135/138 provided.
		labelH, editGap, editH, rowGap := int32(22), int32(3), int32(28), int32(3)
		editOffset := labelH + editGap
		return laboratoryFieldLayoutMetrics{
			LabelH: labelH, EditGap: editGap, EditH: editH, RowGap: rowGap,
			EditOffset: editOffset, RowStep: editOffset + editH + rowGap,
			FieldBaseOffset: 74, PagerOffset: 32, PagerH: 26, HistoryGap: 10,
		}
	}
	labelH, editGap, editH, rowGap := int32(22), int32(3), int32(28), int32(4)
	editOffset := labelH + editGap
	return laboratoryFieldLayoutMetrics{
		LabelH: labelH, EditGap: editGap, EditH: editH, RowGap: rowGap,
		EditOffset: editOffset, RowStep: editOffset + editH + rowGap,
		FieldBaseOffset: 110, PagerOffset: 48, PagerH: 36, HistoryGap: 10,
	}
}

type laboratorySideActionsGeometry struct {
	InfoH      int32
	PrimaryY   int32
	SecondaryY int32
	ButtonH    int32
}

// laboratorySideActions packs the three normal-view actions into the same
// right-hand concept column as readiness/model help. The primary action gets
// a full row and the two secondary actions share the row below it. This keeps
// the history header free of unrelated red buttons and makes the page read as
// three clear bands: inputs, actions, history.
func laboratorySideActions(panelTop, panelBottom int32) laboratorySideActionsGeometry {
	panelH := max32(0, panelBottom-panelTop)
	buttonH, rowGap, infoToAct, minInfoH := int32(40), int32(8), int32(10), int32(104)
	if panelH < 230 {
		// RC129: keep the action stack proportional to the compact five-row input
		// envelope. The history table gets the released height; button font stays
		// unchanged and labels still fit in the 34 px control envelope.
		buttonH, rowGap, infoToAct, minInfoH = 34, 6, 6, 70
	}
	actionsH := buttonH*2 + rowGap
	infoH := panelH - actionsH - infoToAct
	if infoH < minInfoH {
		infoH = minInfoH
	}
	primaryY := panelTop + infoH + infoToAct
	secondaryY := primaryY + buttonH + rowGap
	// If a very short client area is forced by Windows, keep every action inside
	// the field envelope and sacrifice help-card height before overlapping history.
	if secondaryY+buttonH > panelBottom {
		secondaryY = max32(panelTop+buttonH+rowGap, panelBottom-buttonH)
		primaryY = max32(panelTop, secondaryY-buttonH-rowGap)
		infoH = max32(0, primaryY-panelTop-infoToAct)
	}
	return laboratorySideActionsGeometry{InfoH: infoH, PrimaryY: primaryY, SecondaryY: secondaryY, ButtonH: buttonH}
}

func laboratoryConceptWidths(contentW int32, scalePanel bool) (fieldsW, readinessW int32) {
	fieldsW = contentW
	wideConcept := contentW >= 980
	if wideConcept {
		// RC125: the user explicitly asked to narrow the left input matrix and
		// expand the right explanatory/action panel. RC146: make the size change visible, not just positional. Target roughly a 50/50 split
		// on wide desktops, while preserving at least 560 px for two compact input columns.
		maxPanel := max32(340, contentW-560-laboratoryConceptGap)
		targetPanel := max32(440, contentW*50/100)
		readinessW = min32(820, min32(targetPanel, maxPanel))
		fieldsW = contentW - readinessW - laboratoryConceptGap
		return
	}
	if scalePanel {
		readinessW = min32(330, max32(270, contentW/3))
		fieldsW = contentW - readinessW - laboratoryConceptGap
	}
	return
}

func laboratoryMetadataWidths(contentW int32) (sourceW, searchW int32) {
	const fixed = int32(194 + 18 + 162)
	available := max32(320, contentW-fixed)
	sourceW = min32(max32(180, contentW*40/100), available-180)
	searchW = available - sourceW
	return
}

func laboratoryFieldWidths(columnW int32) (editW, unitW int32) {
	// RC146: numeric laboratory values use visibly shorter controls. Keep the
	// unit selector immediately beside the compact value field and leave deliberate
	// whitespace at the end of each column. On genuinely narrow windows we still
	// consume the available width so neither control clips.
	unitW = min32(116, max32(100, columnW/4))
	available := max32(100, columnW-unitW-8)
	if columnW < 360 {
		editW = available
	} else {
		editW = min32(180, available)
	}
	return
}

const laboratorySourceLabelW int32 = 176

func laboratoryWideMetadataWidths(contentW int32) (objectW, sourceW int32) {
	const (
		objectEditX = int32(376)
		minSourceW  = int32(180)
	)
	metadataRight := int32(242) + contentW - 576
	// Reserve the full bold Russian caption «Источник / протокол» plus a real
	// source edit before choosing the object-field width. This prevents the label
	// from being clipped into the neighbouring controls on wide Windows layouts.
	maxObjectW := metadataRight - objectEditX - 14 - laboratorySourceLabelW - minSourceW
	objectW = max32(340, min32(420, maxObjectW))
	sourceEditX := objectEditX + objectW + 14 + laboratorySourceLabelW
	sourceW = max32(minSourceW, metadataRight-sourceEditX)
	return
}

type laboratoryOverallVisualMetrics struct {
	TrendH      int32
	ShowTrend   bool
	ConclusionH int32
	HistoryY    int32
	HistoryH    int32
	InfoY       int32
}

func laboratoryOverallInfographicHeight(h, natural int32) int32 {
	// Инфографика по шести направлениям обязательна: на низком окне
	// сжимаем её до минимальной высоты трёх читаемых строк, но не скрываем.
	capH := int32(174)
	if h < 900 {
		capH = 148
	} else if h >= 1080 {
		capH = 194
	}
	if natural < 148 {
		natural = 148
	}
	return min32(natural, capH)
}

func laboratoryOverallVisualMetricsForHeight(h, trendY int32, mass bool) laboratoryOverallVisualMetrics {
	bottom := h - laboratoryBottomSafeMargin
	// RC365 / этап 1: текстовая общая оценка и предыдущие анализы являются
	// обязательным содержимым экрана. График тренда вспомогательный и полностью
	// сворачивается на низком клиенте вместо того, чтобы оставлять бесполезную
	// полоску высотой 40–60 px и обрезать заключение.
	// RC370: the user asked to shorten the general conclusion and dedicate much
	// more of the lower half to previous analyses. Keep the conclusion readable,
	// but reserve a visibly useful history drawer first.
	minConclusion := int32(108)
	minHistory := int32(172)
	if h >= 900 {
		minConclusion = 126
		minHistory = 205
	}
	if h >= 1080 {
		minConclusion = 155
		minHistory = 245
	}
	const (
		titleGap       = int32(34)
		historyGap     = int32(10)
		minUsefulTrend = int32(82)
	)
	desiredTrend := int32(165)
	if mass {
		desiredTrend = 175
	}
	if h < 960 {
		desiredTrend = 105
	}
	availableTrend := bottom - trendY - titleGap - minConclusion - historyGap - minHistory
	showTrend := h >= 820 && availableTrend >= minUsefulTrend
	trendH := int32(0)
	if showTrend {
		trendH = min32(desiredTrend, availableTrend)
	}
	conclusionTitleY := trendY
	if showTrend {
		conclusionTitleY += trendH + 8
	}
	conclusionY := conclusionTitleY + 28
	historyY := max32(conclusionY+84+historyGap, bottom-minHistory)
	if historyY > bottom {
		historyY = bottom
	}
	historyH := max32(0, bottom-historyY)
	maxConclusionBottom := max32(conclusionY+84, historyY-historyGap)
	conclusionH := max32(84, maxConclusionBottom-conclusionY)
	if conclusionH >= minConclusion {
		conclusionH = max32(minConclusion, conclusionH)
	}
	infoY := conclusionY + conclusionH + 6
	return laboratoryOverallVisualMetrics{TrendH: trendH, ShowTrend: showTrend, ConclusionH: conclusionH, HistoryY: historyY, HistoryH: historyH, InfoY: infoY}
}

func layoutLaboratory(w, h int32) {
	// An off-page data refresh must not move a hidden laboratory workspace or
	// invalidate the currently visible section. Activation lays it out afresh.
	if labHistoryList == 0 || currentPage != ID_LAB {
		return
	}
	// The internal complication selector shares one parent with more than a
	// hundred controls. Apply their final rectangles together so intermediate
	// areas never paint their construction coordinates.
	beginLayoutBatch()
	defer endLayoutBatch()
	contentW := max32(720, w-270)
	right := int32(242) + contentW
	// RC125 keeps the normal airy layout, but switches to a height-aware
	// geometry profile on short logical client areas (typically Windows
	// 125/150% scaling). Font sizes stay unchanged; only unused vertical gaps
	// are reduced so the previous-analysis history never vanishes.
	vertical := laboratoryVerticalLayoutForHeight(h)
	fieldLayout := laboratoryFieldLayoutForHeight(h)
	laboratoryTitleY := vertical.TitleY
	laboratoryLegendY := vertical.LegendY
	laboratoryTabsY := vertical.TabsY
	laboratoryTabsSecondY := vertical.TabsSecondY
	laboratoryObjectY := vertical.ObjectY
	laboratorySourceY := vertical.SourceY
	actionTopY := laboratoryTitleY
	// RC132: page title is now an explicit layout participant. RC131 left the
	// title at its construction coordinate while pulling the action buttons upward,
	// which hid the heading and intruded into the global warning strip.
	setPos(labTitle, 242, py(laboratoryTitleY), 370, 42)
	// Import and template actions are peers: same width, height and font.
	const actionTopW int32 = 300
	const actionTopGap int32 = 14
	setPos(labImportButton, right-actionTopW, py(actionTopY), actionTopW, vertical.TopActionH)
	setPos(labTemplateButton, right-2*actionTopW-actionTopGap, py(actionTopY), actionTopW, vertical.TopActionH)
	setPos(labLegendLabel, 242, py(laboratoryLegendY), contentW, vertical.LegendH)
	// Two deliberate tab rows avoid squeezing seven long Russian captions into
	// one strip. This is more stable than shrinking fonts or allowing 3-line clips.
	tabGap := int32(12)
	firstRow := min32(laboratoryTabFirstRowCount(contentW), int32(len(labGroups)))
	firstW := (contentW - tabGap*(firstRow-1)) / firstRow
	for i := int32(0); i < firstRow; i++ {
		g := labGroups[i]
		x := int32(242) + i*(firstW+tabGap)
		wTab := firstW
		if i == firstRow-1 {
			wTab = int32(242) + contentW - x
		}
		setPos(labTabButtons[g.ID], x, py(laboratoryTabsY), wTab, vertical.TabH)
	}
	remaining := int32(len(labGroups)) - firstRow
	if remaining > 0 {
		secondW := (contentW - tabGap*(remaining-1)) / remaining
		for i := int32(0); i < remaining; i++ {
			g := labGroups[firstRow+i]
			x := int32(242) + i*(secondW+tabGap)
			wTab := secondW
			if i == remaining-1 {
				wTab = int32(242) + contentW - x
			}
			setPos(labTabButtons[g.ID], x, py(laboratoryTabsSecondY), wTab, vertical.TabH)
		}
	}
	// RC132: the working-page short conclusion is intentionally hidden. The full
	// conclusion appears after calculation, while the history table keeps its own
	// «Краткий вывод» column. The former summary band is retained as breathing room
	// for the upper controls; it is NOT donated to the history area.
	dateX := right - 300
	// The selected object's full readable key and the source must have separate
	// rows, including on wide displays. A one-row layout clipped long keys.
	compactMetadata := false
	// Summary is anchored after metadata, not shifted upward independently. This
	// is the key RC132 fix for the row-on-row overlap visible in the Windows shot.
	summaryVisualY := laboratorySummaryVisualY(vertical, compactMetadata)
	buttonX := dateX - 262
	objectLabelX := int32(242)
	objectEditX := objectLabelX + 134
	objectW := max32(160, contentW-712)
	if compactMetadata {
		objectW, sourceW := laboratoryWideMetadataWidths(contentW)
		sourceLabelX := objectEditX + objectW + 14
		sourceEditX := sourceLabelX + laboratorySourceLabelW
		setPos(labSourceLabel, sourceLabelX, py(laboratoryObjectY+4), laboratorySourceLabelW, 24)
		setPos(labSourceEdit, sourceEditX, py(laboratoryObjectY), sourceW, 30)
		pEnableWindow.Call(uintptr(labSourceEdit), 1)
		pSendMessageW.Call(uintptr(labSourceEdit), 0x00CF, 0, 0)          // EM_SETREADONLY=false
		pSetWindowPos.Call(uintptr(labSourceEdit), 0, 0, 0, 0, 0, 0x0013) // HWND_TOP, no move/size/activate
	} else {
		sourceW, searchW := laboratoryMetadataWidths(contentW)
		_ = searchW
		setPos(labSourceLabel, 242, py(laboratorySourceY+4), 186, 24)
		setPos(labSourceEdit, 436, py(laboratorySourceY), sourceW, 28)
		pEnableWindow.Call(uintptr(labSourceEdit), 1)
		pSendMessageW.Call(uintptr(labSourceEdit), 0x00CF, 0, 0)          // EM_SETREADONLY=false
		pSetWindowPos.Call(uintptr(labSourceEdit), 0, 0, 0, 0, 0, 0x0013) // HWND_TOP, no move/size/activate
		buttonX = objectEditX + objectW + 12
	}
	setPos(labObjectLabel, objectLabelX, py(laboratoryObjectY), 126, 30)
	setPos(labObjectEdit, -10000, -10000, 1, 1)
	setPos(labObjectCaption, objectEditX, py(laboratoryObjectY), objectW, 30)
	// Keep the label and its value on the same optical baseline.  SS_LEFT alone
	// aligns the top text metrics, which makes the bold caption look a few
	// pixels higher than the selected object on some Windows font/DPI profiles.
	for _, handle := range []syscall.Handle{labObjectLabel, labObjectCaption} {
		style, _, _ := pGetWindowLongPtrW.Call(uintptr(handle), ^uintptr(15))
		style |= SS_LEFT | SS_CENTERIMAGE | SS_NOPREFIX
		pSetWindowLongPtrW.Call(uintptr(handle), ^uintptr(15), style)
	}
	setPos(labUseObjectButton, buttonX, py(laboratoryObjectY-2), 250, vertical.ObjectButtonH)
	setPos(labDateLabel, dateX, py(laboratoryObjectY), 128, 30)
	setPos(labDateEdit, dateX+132, py(laboratoryObjectY), 168, 30)
	setPos(labShortConclusion, 242, py(summaryVisualY), contentW, vertical.SummaryH)
	show(labShortConclusion, false)

	// At desktop width the input matrix and readiness panel follow the approved
	// concept. At smaller widths the panel is hidden and the fields reclaim the
	// full width instead of shrinking or becoming clipped.
	expanded := labConclusionOpen || labActiveGroup == "overall"
	scalePanel := labActiveGroup == "scale" && !expanded
	wideConcept := contentW >= 980
	fieldsW, readinessW := laboratoryConceptWidths(contentW, scalePanel)
	// A little more air between the two input columns prevents long Russian
	// captions and unit selectors from visually merging into one dense block.
	gap := int32(24)
	colW := (fieldsW - gap) / 2
	leftX := int32(242)
	rightX := leftX + colW + gap
	// RC148: field rows must never intrude into the pager/navigation strip.
	// Use the actual pager bottom as a hard lower bound instead of relying only
	// on profile offsets; this keeps the first row clear at Windows DPI scaling.
	pagerY := summaryVisualY + fieldLayout.PagerOffset
	pagerBottom := pagerY + fieldLayout.PagerH
	fieldBaseY := max32(summaryVisualY+fieldLayout.FieldBaseOffset, pagerBottom+18)
	group := currentLabGroup()
	rowLabelH := [5]int32{}
	rowEditOffset := [5]int32{}
	rowY := [5]int32{}
	fieldLabelH := [10]int32{}
	fieldCaptions := [10]string{}
	labelWidth := max32(120, colW-34)
	cursorY := fieldBaseY
	for row := int32(0); row < 5; row++ {
		labelH := fieldLayout.LabelH
		for col := int32(0); col < 2; col++ {
			idx := int(row + col*5)
			fieldIndex := labFieldsPage*10 + idx
			if fieldIndex >= 0 && fieldIndex < len(group.Fields) {
				caption := laboratoryWrapFieldCaptionForWidth(laboratoryFieldDisplayLabel(group.Fields[fieldIndex]), labelWidth)
				fieldCaptions[idx] = caption
				ownH := laboratoryFieldLabelHeightForWidth(fieldLayout.LabelH, caption, labelWidth)
				fieldLabelH[idx] = ownH
				labelH = max32(labelH, ownH)
			}
		}
		rowY[row] = cursorY
		rowLabelH[row] = labelH
		rowEditOffset[row] = labelH + fieldLayout.EditGap
		cursorY += rowEditOffset[row] + fieldLayout.EditH + fieldLayout.RowGap
	}
	for i := 0; i < 10; i++ {
		columnX := leftX
		if i >= 5 {
			columnX = rightX
		}
		row := int32(i % 5)
		y := rowY[row]
		labelH := rowLabelH[row]
		editOffset := rowEditOffset[row]
		ownH := fieldLabelH[i]
		if ownH <= 0 {
			ownH = fieldLayout.LabelH
		}
		// A short caption next to a three-line caption is centered in the
		// shared row envelope. This keeps labels such as «Растворённый H₂S»
		// aligned with the neighbouring long indicator instead of floating at
		// the top of the row.
		labelY := y + max32(0, (labelH-ownH)/2)
		fieldIndex := labFieldsPage*10 + i
		if fieldIndex >= 0 && fieldIndex < len(group.Fields) {
			setText(labFieldLabels[i], fieldCaptions[i])
			if ownH > 42 && hFontHeaderSmall != 0 {
				pSendMessageW.Call(uintptr(labFieldLabels[i]), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
			} else if hFontBold != 0 {
				pSendMessageW.Call(uintptr(labFieldLabels[i]), WM_SETFONT, uintptr(hFontBold), 1)
			}
		}
		setPos(labFieldBadges[i], columnX, py(labelY), 20, max32(20, ownH))
		setPos(labFieldLabels[i], columnX+26, py(labelY), labelWidth, ownH)
		editW, unitW := laboratoryFieldWidths(colW)
		setPos(labFieldEdits[i], columnX, py(y+editOffset), editW, fieldLayout.EditH)
		setPos(labUnitLabels[i], columnX+editW+8, py(y+editOffset), unitW, 220)
	}
	// The pager spans the whole page, independently of the compact input matrix.
	pageButtonW := int32(160)
	pagerH := fieldLayout.PagerH
	setPos(labFieldsPrev, leftX, py(pagerY), pageButtonW, pagerH)
	setPos(labFieldsPageLabel, leftX+pageButtonW+8, py(pagerY), contentW-2*pageButtonW-16, pagerH)
	setPos(labFieldsNext, right-pageButtonW, py(pagerY), pageButtonW, pagerH)

	actionW := min32(300, max32(280, contentW/4))
	actionX := right - actionW
	outputW := max32(420, contentW-actionW-12)
	setLaboratoryFieldsVisible(!expanded)
	// The detailed passport remains available in hover help; the permanent box
	// consumed space and was clipped under common Windows DPI settings.
	show(labPassportPanel, false)
	show(labReadinessPanel, !expanded && labActiveGroup != "overall" && wideConcept && !scalePanel)
	show(labScaleModelLabel, scalePanel)
	show(labScaleModelCombo, scalePanel)
	show(labScaleModelInfo, scalePanel)
	// Keep every side card inside the exact vertical envelope of the five input
	// rows. The row pitch now includes explicit gaps above and below each edit,
	// so the side card follows the same real envelope instead of covering history.
	inputPanelTop := py(fieldBaseY)
	inputPanelBottom := py(cursorY - fieldLayout.RowGap) // bottom of the last input row after per-row caption expansion
	if !expanded && labActiveGroup != "overall" {
		if wideConcept {
			sideX := leftX + fieldsW + laboratoryConceptGap
			stack := laboratorySideActions(inputPanelTop, inputPanelBottom)
			if !scalePanel {
				layoutLaboratoryInfoCard(labReadinessPanel, sideX, inputPanelTop, readinessW, max32(80, stack.InfoH))
			} else {
				setPos(labScaleModelLabel, sideX, inputPanelTop, readinessW, 26)
				setPos(labScaleModelCombo, sideX, inputPanelTop+34, readinessW, 240)
				infoY := inputPanelTop + 72
				infoBottom := inputPanelTop + stack.InfoH
				layoutLaboratoryInfoCard(labScaleModelInfo, sideX, infoY, readinessW, max32(72, infoBottom-infoY))
			}
		}
	}
	show(labConclusionTitle, labConclusionOpen || labActiveGroup == "overall")
	show(labConclusionEdit, labConclusionOpen || labActiveGroup == "overall")
	show(labInfluenceChart, false)
	show(labTrendChart, false)
	show(labTrendMetricCombo, false)
	show(labScopeObjectsEdit, labActiveGroup == "overall" && labOverviewScopeCode == laboratoryScopeSelected)
	show(labRiskFilterCombo, labActiveGroup == "overall" && labOverviewScopeCode != laboratoryScopeObject)
	show(labGroupNameEdit, labActiveGroup == "overall" && labOverviewScopeCode == laboratoryScopeSelected)
	show(labSavedGroupCombo, labActiveGroup == "overall" && labOverviewScopeCode == laboratoryScopeSelected)
	show(labSaveGroupButton, labActiveGroup == "overall" && labOverviewScopeCode == laboratoryScopeSelected)
	show(labTrendSortCombo, labActiveGroup == "overall" && labOverviewScopeCode != laboratoryScopeObject)
	show(labDetailObjectCombo, labActiveGroup == "overall" && labOverviewScopeCode != laboratoryScopeObject)
	show(labScopeLabel, false)
	show(labScopeCombo, false)
	show(labInfographicButton, labActiveGroup == "overall")
	show(labComplexAnalysisButton, labActiveGroup == "overall")
	show(labCalculationBasisButton, true)
	show(labStiffChart, expanded && labActiveGroup == "scale" && labScaleModelCode != "water_mixing")
	show(labMixingChart, expanded && labActiveGroup == "scale" && labScaleModelCode == "water_mixing")
	show(labContinueButton, labActiveGroup == "overall")
	show(labCopyButton, expanded)
	geometry := calculateLaboratoryBottomGeometry(h, expanded)
	if !expanded {
		// Keep the five-row input matrix compact enough to leave a useful history
		// area on a 1080p desktop. The previous geometry spent a full extra row on
		// a redundant informational caption and reduced history to one visible item.
		fieldsBottom := inputPanelBottom
		geometry.PanelTop = fieldsBottom
		geometry.CommentY = geometry.PanelTop
		geometry.InfoY = geometry.PanelTop
		// At normal height keep the airy 18 px band. On short logical clients
		// reserve that space for history instead; this is the lower panel the user
		// reported as disappearing after RC121.
		geometry.HistoryY = min32(h-laboratoryBottomSafeMargin, fieldsBottom+max32(fieldLayout.HistoryGap, 14))
		geometry.HistoryH = max32(0, h-laboratoryBottomSafeMargin-geometry.HistoryY)
	}
	if labActiveGroup == "overall" {
		// RC371: the infographic no longer consumes the main analysis page.
		// A single accent action opens the complete visualization in a dedicated
		// window. This gives the conclusion and history the vertical space they
		// need on 1366x768 and high-DPI Windows layouts.
		buttonY := py(summaryVisualY) + 4
		// Keep all three analysis actions inside the output column. The right
		// action column starts at actionX and must never cover the last caption.
		buttonAreaW := min32(outputW, actionX-242-12)
		buttonW := (buttonAreaW - 24) / 3
		setPos(labInfographicButton, 242, buttonY, buttonW, 44)
		setPos(labComplexAnalysisButton, 254+buttonW, buttonY, buttonW, 44)
		setPos(labCalculationBasisButton, 266+2*buttonW, buttonY, buttonAreaW-24-2*buttonW, 44)
		show(labInfluenceChart, false)
		show(labTrendChart, false)
		show(labTrendMetricCombo, false)
		show(labScopeLabel, false)
		show(labScopeCombo, false)
		show(labScopeObjectsEdit, false)
		show(labRiskFilterCombo, false)
		show(labGroupNameEdit, false)
		show(labSavedGroupCombo, false)
		show(labSaveGroupButton, false)
		show(labTrendSortCombo, false)
		show(labDetailObjectCombo, false)

		geometry.ConclusionTitleY = buttonY + 54
		geometry.ConclusionY = geometry.ConclusionTitleY + 28
		geometry.PanelTop = buttonY
		// Reserve the action column, but donate the infographic height to the
		// conclusion and previous-analysis history.
		availableBottom := h - laboratoryBottomSafeMargin
		minHistory := int32(280)
		historyY := max32(geometry.ConclusionY+112, availableBottom-minHistory)
		geometry.HistoryY = min32(historyY, availableBottom-118)
		geometry.HistoryH = max32(0, availableBottom-geometry.HistoryY)
		geometry.InfoY = geometry.HistoryY - 10
		geometry.CommentY = geometry.InfoY
		geometry.ConclusionH = max32(76, geometry.InfoY-geometry.ConclusionY-8)
	}

	if expanded {
		if labActiveGroup == "scale" {
			chartH := min32(210, max32(160, geometry.ConclusionH/3))
			if labScaleModelCode == "water_mixing" {
				setPos(labMixingChart, 242, geometry.ConclusionTitleY, outputW, chartH)
				pInvalidateRect.Call(uintptr(labMixingChart), 0, 0)
			} else {
				setPos(labStiffChart, 242, geometry.ConclusionTitleY, outputW, chartH)
				pInvalidateRect.Call(uintptr(labStiffChart), 0, 0)
			}
			setPos(labConclusionTitle, 242, geometry.ConclusionTitleY+chartH+8, outputW, 22)
			conclusionY := geometry.ConclusionTitleY + chartH + 34
			conclusionH := max32(80, geometry.InfoY-conclusionY-8)
			layoutLaboratoryInfoCard(labConclusionEdit, 242, conclusionY, outputW, conclusionH)
		} else {
			setPos(labConclusionTitle, 242, geometry.ConclusionTitleY, outputW, 28)
			layoutLaboratoryInfoCard(labConclusionEdit, 242, geometry.ConclusionY, outputW, geometry.ConclusionH)
		}
	}
	clearCurrentVisible := false
	clearCurrentBottom := int32(0)
	if !expanded && labActiveGroup != "overall" {
		if wideConcept {
			sideX := leftX + fieldsW + laboratoryConceptGap
			stack := laboratorySideActions(inputPanelTop, inputPanelBottom)
			calcW := max32(122, readinessW*42/100)
			setPos(labAnalyzeButton, sideX, stack.PrimaryY, readinessW-calcW-8, stack.ButtonH)
			setPos(labCalculationBasisButton, sideX+readinessW-calcW, stack.PrimaryY, calcW, stack.ButtonH)
			secondaryW := (readinessW - 8) / 2
			setPos(labExportButton, sideX, stack.SecondaryY, secondaryW, stack.ButtonH)
			setPos(labSaveButton, sideX+secondaryW+8, stack.SecondaryY, readinessW-secondaryW-8, stack.ButtonH)
		} else {
			// Narrow windows use two rows so every action remains readable.
			stripY := inputPanelBottom + 10
			columnW := (contentW - 8) / 2
			setPos(labAnalyzeButton, leftX, stripY, columnW, 40)
			setPos(labCalculationBasisButton, leftX+columnW+8, stripY, contentW-columnW-8, 40)
			setPos(labExportButton, leftX, stripY+48, columnW, 40)
			setPos(labSaveButton, leftX+columnW+8, stripY+48, contentW-columnW-8, 40)
			geometry.HistoryY = min32(h-laboratoryBottomSafeMargin, stripY+98)
			geometry.HistoryH = max32(0, h-laboratoryBottomSafeMargin-geometry.HistoryY)
		}
		// RC572: the former off-screen «Очистить введённое» control must not
		// reserve the whole lower half of the page. On the normal input screen
		// all four primary actions already live inside the input envelope; history
		// starts immediately below it. The clear-current action is hidden here and
		// remains available in the expanded/maintenance workflow instead of
		// pushing the history ListView beyond the client area.
		clearCurrentVisible = false
	} else {
		const actionH int32 = 36
		const actionStep int32 = 44
		setPos(labAnalyzeButton, actionX, geometry.PanelTop, actionW, actionH)
		setPos(labExportButton, actionX, geometry.PanelTop+actionStep, actionW, actionH)
		setPos(labSaveButton, actionX, geometry.PanelTop+2*actionStep, actionW, actionH)
		if labActiveGroup == "overall" {
			setPos(labContinueButton, actionX, geometry.PanelTop+3*actionStep, actionW, actionH)
		} else {
			setPos(labCalculationBasisButton, actionX, geometry.PanelTop+3*actionStep, actionW, actionH)
		}
	}
	copyY := geometry.PanelTop + 3*44
	if labActiveGroup == "overall" || expanded {
		copyY += 44
	}
	setPos(labCopyButton, actionX, copyY, actionW, 36)
	show(labClearCurrentButton, clearCurrentVisible)
	if clearCurrentVisible {
		setPos(labClearCurrentButton, actionX, copyY+52, actionW, 42)
		clearCurrentBottom = copyY + 94
	}

	// RC559: the history/search block may never start under the action column.
	// Compute the bottom of the last actually visible action and push history
	// below it, preserving the original bottom edge where possible.
	historyBottom := geometry.HistoryY + geometry.HistoryH
	minHistoryY := geometry.HistoryY
	if expanded {
		minHistoryY = max32(minHistoryY, copyY+36+12)
	} else if labActiveGroup == "overall" {
		minHistoryY = max32(minHistoryY, geometry.PanelTop+3*44+36+12)
	} else {
		// RC572: normal input actions are already packed inside inputPanelBottom.
		// Do not reserve space for hidden/off-screen controls — that was the direct
		// cause of the blank lower panel and disappeared «Предыдущие анализы» table.
		minHistoryY = max32(minHistoryY, inputPanelBottom+8)
		if clearCurrentVisible {
			minHistoryY = max32(minHistoryY, clearCurrentBottom+8)
		}
	}
	if minHistoryY > geometry.HistoryY {
		geometry.HistoryY = minHistoryY
		geometry.HistoryH = max32(0, historyBottom-minHistoryY)
	}

	// Historical comments remain stored, while the working form uses this space for conclusions.
	show(labCommentLabel, false)
	show(labCommentEdit, false)
	setPos(labInfo, 242, geometry.InfoY, outputW, 24)
	// The per-section informational sentence duplicates the active tab and count.
	// Hide it in the normal input view and give that vertical space to history.
	show(labInfo, expanded && labActiveGroup != "overall")
	// Never make the previous-analysis area disappear entirely. On short client
	// heights keep a compact history drawer (title + list) and temporarily hide
	// search/delete controls; at normal height the full toolbar returns.
	const historyMinimumH int32 = 90
	// RC572: if a normal laboratory screen has at least a compact drawer worth
	// of physical space below the input matrix, force that drawer to the bottom.
	// Never leave an empty unlabelled rectangle where the table should be.
	if !expanded && labActiveGroup != "overall" && geometry.HistoryH < historyMinimumH {
		bottom := h - laboratoryBottomSafeMargin
		wantedY := max32(inputPanelBottom+8, bottom-132)
		if bottom-wantedY >= historyMinimumH {
			geometry.HistoryY = wantedY
			geometry.HistoryH = bottom - wantedY
		}
	}
	historyVisible := geometry.HistoryH >= historyMinimumH
	historyCompact := historyVisible && geometry.HistoryH < 210
	show(labHistoryTitle, historyVisible)
	show(labHistoryList, historyVisible)
	show(labFilterLabel, historyVisible && !historyCompact)
	show(labFilterEdit, historyVisible && !historyCompact)
	show(labDeleteButton, historyVisible && !historyCompact)
	show(labClearButton, historyVisible && !historyCompact)
	if historyVisible {
		historyW := contentW
		if historyCompact {
			// Compact drawer for minimum-height / high-DPI windows. It is better to
			// keep one visible historical row than to remove the whole block.
			headerY := geometry.HistoryY + 1
			setPos(labHistoryTitle, 242, headerY, historyW, 18)
			listY := headerY + 19
			listH := max32(40, geometry.HistoryY+geometry.HistoryH-listY-2)
			setPos(labHistoryList, 242, listY, historyW, listH)
		} else {
			// Extend the history block across the full content width: the action column
			// is only needed above. On medium widths place the search row below the
			// buttons so it never collides with the action column or the list header.
			const (
				deleteButtonW int32 = 126
				clearButtonW  int32 = 160
				buttonGap     int32 = 10
				headerTopPad  int32 = 8
				headerH       int32 = 32
				listGap       int32 = 8
				filterLabelW  int32 = 54
			)
			headerY := geometry.HistoryY + headerTopPad
			if historyW < 1120 {
				clearX := 242 + historyW - clearButtonW
				deleteX := clearX - buttonGap - deleteButtonW
				titleW := max32(180, deleteX-242-12)
				setPos(labHistoryTitle, 242, headerY+6, titleW, 24)
				setPos(labDeleteButton, deleteX, headerY, deleteButtonW, headerH)
				setPos(labClearButton, clearX, headerY, clearButtonW, headerH)
				searchY := headerY + headerH + 6
				setPos(labFilterLabel, 242, searchY+6, filterLabelW, 22)
				setPos(labFilterEdit, 242+filterLabelW+6, searchY, historyW-filterLabelW-6, headerH)
				listY := searchY + headerH + listGap
				listH := max32(44, geometry.HistoryY+geometry.HistoryH-listY-2)
				setPos(labHistoryList, 242, listY, historyW, listH)
			} else {
				searchBlockW := min32(340, max32(286, historyW*26/100))
				searchX := 242 + historyW - searchBlockW
				clearX := searchX - buttonGap - clearButtonW
				deleteX := clearX - buttonGap - deleteButtonW
				titleW := max32(190, deleteX-242-12)
				setPos(labHistoryTitle, 242, headerY+6, titleW, 24)
				setPos(labDeleteButton, deleteX, headerY, deleteButtonW, headerH)
				setPos(labClearButton, clearX, headerY, clearButtonW, headerH)
				setPos(labFilterLabel, searchX, headerY+6, filterLabelW, 22)
				setPos(labFilterEdit, searchX+filterLabelW+6, headerY, searchBlockW-filterLabelW-6, headerH)
				listY := headerY + headerH + listGap
				listH := max32(44, geometry.HistoryY+geometry.HistoryH-listY-2)
				setPos(labHistoryList, 242, listY, historyW, listH)
			}
		}
		// The overall tab is a report view. Reapply this after all geometry updates:
		// hidden field editors must not remain visible behind the conclusion/history
		// panels after a DPI change or a short-window resize.
		if labActiveGroup == "overall" {
			setLaboratoryFieldsVisible(false)
			for _, control := range []syscall.Handle{labFieldsPrev, labFieldsNext, labFieldsPageLabel} {
				show(control, false)
			}
		}
	}
	laboratoryRedraw(RDW_INVALIDATE | RDW_ALLCHILDREN)
}

func mergeImportedLabRecords(in []LabRecord, importedAt string) int {
	if len(in) == 0 {
		return 0
	}
	if !labRecordsLoaded {
		labRecords = loadLabRecords()
	}
	have := map[string]bool{}
	for _, r := range labRecords {
		have[r.ID] = true
	}
	added := 0
	for _, r := range in {
		if r.ID == "" || have[r.ID] {
			continue
		}
		r.ImportedAt = importedAt
		if appendLabRecord(r) == nil {
			labRecords = append(labRecords, r)
			have[r.ID] = true
			added++
		}
	}
	sort.SliceStable(labRecords, func(i, j int) bool { return parseDate(labRecords[i].Date).After(parseDate(labRecords[j].Date)) })
	return added
}

func labRecordFieldTotal(record LabRecord) int {
	group, ok := labGroupByCode(record.GroupCode)
	if !ok {
		return 0
	}
	_, total := laboratoryHistoryFieldCoverage(group, record.Values, record.ScaleModel)
	return total
}

func layoutLaboratoryCurrentClient() {
	var rc RECT
	pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&rc)))
	layoutLaboratory(rc.Right-rc.Left, rc.Bottom-rc.Top)
}
