//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unsafe"
)

// assessmentMethodCandidate is a non-prescriptive candidate shown next to the
// complication assessment. It does not represent an automatic recommendation.
type assessmentMethodCandidate struct {
	ID         string
	Name       string
	Source     string
	SourceKind string
	Status     string
	Note       string
}

var (
	assessMethodCandidates []assessmentMethodCandidate
	assessSelectedRow      int = -1
	// Этап 2 RC365: выбранный пользователем кандидат запоминается отдельно для
	// каждого объекта и направления. Это черновой выбор интерфейса, а не
	// подтверждённое мероприятие и не коэффициент прогноза МРП.
	assessmentMethodSelection = map[string]string{}
)

func assessmentMethodSelectionKey(objectKey, groupCode string) string {
	return strings.TrimSpace(objectKey) + "|" + strings.TrimSpace(groupCode)
}

func assessmentMethodSource(m MethodCatalogEntry) (title, kind string) {
	if s, ok := ruleSource(m.SourceID); ok {
		title = strings.TrimSpace(s.Title)
		kind = strings.TrimSpace(s.Kind)
		if title == "" {
			title = m.SourceID
		}
		if kind == "" {
			kind = strings.TrimSpace(s.Status)
		}
		return title, kind
	}
	if m.SourceID == "excel_tech_methods" {
		return "Технологический модуль", "исходный справочник"
	}
	return dash(m.SourceID), "источник требует уточнения"
}

func assessmentMethodIsEquipmentDesign(name string) bool {
	n := normalizeMethodText(name)
	needles := []string{
		"гно", "уэцн", "ушгн", "уэвн", "винтовая насосная", "насосная установка",
		"ну специсполнения", "мультифазный насос", "газосепаратор", "диспергатор",
		"штанговращатель", "нкт увеличенного диаметра", "хвостовик", "заглубление ну",
		"кожух пэд", "центраторы уэцн", "термостойком исполнении", "шнп с покрытием",
		"противопесочный", "гравийная набивка", "гидроразрыв пласта", "пескоотделитель", "гидроциклон",
		"коррозионностойкие сплавы", "газожидкостный смеситель", "двухступенчатая газосепарация",
		"термостойкий погружной", "металлическим сильфоном", "термостойкая кабельная линия",
		"уплд", "погружным линейным двигателем", "расширяемый противопесочный экран", "полимерная футеровка", "газовый якорь", "роторный газосепаратор", "электроприводная винтовая", "квалифицированный по фактической вязкости", "скважинный электрический", "индукционный нагреватель", "защитные покрытия рабочих органов", "термостойкая эксплуатационная компоновка",
	}
	for _, s := range needles {
		if strings.Contains(n, normalizeMethodText(s)) {
			return true
		}
	}
	return false
}

func assessmentMethodNeedsLabSelection(name string) bool {
	n := normalizeMethodText(name)
	for _, s := range []string{
		"ингибитор", "деэмульгатор", "биоцид", "растворитель", "увр",
		"снижения вязкости", "разрушения эмульсии", "комплексон", "диспергатор", "депрессор",
		"хелат", "поглотитель кислорода", "нитратная обработка", "улучшения текучести", "твердый ингибитор", "твёрдый ингибитор", "углеводородным разбавителем", "промывочная вода",
	} {
		if strings.Contains(n, normalizeMethodText(s)) {
			return true
		}
	}
	return false
}

func assessmentMethodStatus(name string) string {
	switch {
	case assessmentMethodIsEquipmentDesign(name):
		return "Проверить в «Автотехнологе»"
	case assessmentMethodNeedsLabSelection(name):
		return "Требуется лабораторный подбор"
	default:
		return "Требуется техническая проверка"
	}
}

func assessmentMethodGroupForDirection(d assessmentClassDirection) string {
	switch strings.TrimSpace(d.Name) {
	case "Высокая вязкость":
		return "viscosity"
	case "Свободный газ":
		return "free_gas"
	case "Гидраты":
		return "hydrates"
	case "Высокая температура":
		return "high_temp"
	default:
		return d.LabGroup
	}
}

func assessmentMethodMatchesGroup(groupCode, name string) bool {
	n := normalizeMethodText(name)
	containsAny := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(n, normalizeMethodText(value)) {
				return true
			}
		}
		return false
	}
	corrosion := containsAny("корроз", "скрн", "протектор", "трил к", "кислородопоглот", "поглотитель кислорода", "коррозионностойкие сплавы", "ингибитор коррозии", "полимерная футеровка", "купоны", "электрохимические зонды", "электрорезистивные зонды", "толщинометр", "дефектоскоп", "подотложечн")
	scale := containsAny("соле", "соли", "комплексон", "хелат", "кислот", "виброакуст", "медленно высвобождаем", "периодическая обработка пласта", "капиллярную линию", "сульфат", "несовместимых вод", "сульфида железа", "растворённого железа", "рвк пилот", "резонансно волнов", "галит", "промывочная вода", "умягчение воды", "предварительное осаждение", "пересыщения солями", "минеральных отложений")
	aspo := containsAny("аспо", "парафин", "асфальтен", "скреб", "теплонос", "увр", "нагреватель", "магнит", "трил п см", "антиадгезион", "органических отложений") && !containsAny("высокая температура")
	viscosity := containsAny("вязкост", "деэмульг", "улучшения текучести", "разрушения эмульсии", "тепловая обработка", "уэвн", "винтовая насосная", "нкт увеличенного диаметра", "уплд", "погружным линейным двигателем", "расширяемый противопесочный экран", "полимерная футеровка", "газовый якорь", "роторный газосепаратор", "электроприводная винтовая", "квалифицированный по фактической вязкости", "скважинный электрический", "индукционный нагреватель", "защитные покрытия рабочих органов", "термостойкая эксплуатационная компоновка", "разбавлен", "легким углеводородным", "электроприводная винтовая", "фактической вязкости", "паротеплов", "индукцион")
	solids := containsAny("мехприм", "фильтр", "сепаратор мех", "износостой", "нормализация забоя", "песк", "гравийн", "проппант", "гидроциклон", "закрепление призабойной", "депресс", "вывод на режим", "расширяемый противопесочный", "селективное заканчивание", "размещение смолы", "полимерным покрытием", "песчаной пробки", "обратного оседания", "безопасному потоку", "механических примесей")
	microbiology := containsAny("биоцид", "микроб", "бактер", "свб", "нитрат", "биоплен", "биоплён")
	freeGas := containsAny("свободного газа", "газосепаратор", "газожидкост", "двухступенчатая газосепарация", "частотно регулируемый привод", "мультифаз", "хвостовик", "заглубление ну", "кожух уэцн", "роторный газосепаратор", "газовый якорь", "наполнению насоса")
	hydrates := containsAny("гидрат", "метанол", "моноэтиленгликоль", "антиагломерант", "кинетический ингибитор", "теплоизоляц", "нагревательная кабельная линия", "осушение газа", "свободной воды", "двусторонняя контролируемая депрессуризация", "нагретым гликолем", "границы гидратообразования")
	highTemp := containsAny("термостой", "высокотемператур", "температуры двигателя", "металлическим сильфоном", "гидрозащита", "кабельная линия", "кожух уэцн", "кожух пэд", "термометрия перед спуском", "абразивно коррозионной среды", "ограничение частоты и нагрузки", "паротепловых циклов") && !hydrates
	switch groupCode {
	case "corrosion":
		return corrosion
	case "scale":
		return scale
	case "aspo":
		return aspo
	case "viscosity":
		return viscosity
	case "solids":
		return solids
	case "microbiology":
		return microbiology
	case "free_gas":
		return freeGas
	case "hydrates":
		return hydrates
	case "high_temp":
		return highTemp
	case "gases":
		return freeGas || hydrates || highTemp
	default:
		return false
	}
}

func assessmentMethodCandidatesForGroup(groupCode string) []assessmentMethodCandidate {
	seen := map[string]bool{}
	out := make([]assessmentMethodCandidate, 0, 16)
	for _, m := range ruleCatalog.MethodCatalog {
		if !assessmentMethodMatchesGroup(groupCode, m.Name) {
			continue
		}
		key := normalizeMethodText(m.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		source, sourceKind := assessmentMethodSource(m)
		note := "Кандидат из справочника методов. Выбор не означает автоматического назначения мероприятия."
		if assessmentMethodIsEquipmentDesign(m.Name) {
			note = "РОСА фиксирует диагностический контекст. Подбор и проверка компоновки выполняются в «Автотехнологе»."
		}
		out = append(out, assessmentMethodCandidate{
			ID: m.ID, Name: m.Name, Source: source, SourceKind: sourceKind,
			Status: assessmentMethodStatus(m.Name), Note: note,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		// The Technology Module is the canonical first layer; verified open-source
		// additions follow it and never displace the corporate working list.
		iExcel := strings.Contains(strings.ToLower(out[i].Source), "технологичес")
		jExcel := strings.Contains(strings.ToLower(out[j].Source), "технологичес")
		if iExcel != jExcel {
			return iExcel
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func populateAssessmentMethodCombo(groupCode string) {
	assessMethodCandidates = assessmentMethodCandidatesForGroup(groupCode)
	if assessMethodsCombo == 0 {
		return
	}
	pSendMessageW.Call(uintptr(assessMethodsCombo), CB_RESETCONTENT, 0, 0)
	placeholder := "— выберите метод для проверки —"
	pSendMessageW.Call(uintptr(assessMethodsCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(placeholder))))
	for _, method := range assessMethodCandidates {
		prefix := "[Модель] "
		if !strings.Contains(strings.ToLower(method.Source), "технологичес") {
			prefix = "[Справочно] "
		}
		caption := prefix + method.Name
		pSendMessageW.Call(uintptr(assessMethodsCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
	}
	selectedIndex := 0
	savedID := assessmentMethodSelection[assessmentMethodSelectionKey(selectedObjectKey, groupCode)]
	if savedID != "" {
		for i, method := range assessMethodCandidates {
			if method.ID == savedID || normalizeMethodText(method.Name) == normalizeMethodText(savedID) {
				selectedIndex = i + 1
				break
			}
		}
	}
	pSendMessageW.Call(uintptr(assessMethodsCombo), CB_SETCURSEL, uintptr(selectedIndex), 0)
	if len(assessMethodCandidates) == 0 {
		setText(assessMethodInfo, "В справочнике нет сопоставленных кандидатов. Это не означает отсутствие возможных мероприятий; требуется инженерная проверка.")
	} else if selectedIndex > 0 {
		method := assessMethodCandidates[selectedIndex-1]
		setText(assessMethodInfo, method.Status+" • "+method.Source+" • сохранённый черновой выбор")
	} else {
		setText(assessMethodInfo, fmt.Sprintf("Кандидатов: %d. [Модель] — из встроенной базы РОСА; [Справочно] — отраслевое дополнение. Автоматического выбора нет.", len(assessMethodCandidates)))
	}
}

func rememberSelectedAssessmentMethod() {
	if assessSelectedRow < 0 || assessSelectedRow >= len(assessDirectionRows) {
		return
	}
	key := assessmentMethodSelectionKey(selectedObjectKey, assessDirectionRows[assessSelectedRow].Group.Code)
	if key == "|" {
		return
	}
	method, ok := selectedAssessmentMethod()
	if !ok {
		delete(assessmentMethodSelection, key)
		return
	}
	if method.ID != "" {
		assessmentMethodSelection[key] = method.ID
	} else {
		assessmentMethodSelection[key] = method.Name
	}
}

func selectedAssessmentMethod() (assessmentMethodCandidate, bool) {
	if assessMethodsCombo == 0 {
		return assessmentMethodCandidate{}, false
	}
	idx, _, _ := pSendMessageW.Call(uintptr(assessMethodsCombo), CB_GETCURSEL, 0, 0)
	if int(idx) <= 0 || int(idx) > len(assessMethodCandidates) {
		return assessmentMethodCandidate{}, false
	}
	return assessMethodCandidates[int(idx)-1], true
}

func assessmentMethodDetailText(method assessmentMethodCandidate) string {
	evidence := "Критерии: уточняются по объекту.\r\nОграничения: требуют технической проверки."
	if rules := methodRulesForName(method.Name); len(rules) > 0 {
		r := rules[0]
		criteria := "—"
		if len(r.Criteria) > 0 {
			criteria = strings.Join(r.Criteria, "; ")
		}
		limitations := "—"
		if len(r.Limitations) > 0 {
			limitations = strings.Join(r.Limitations, "; ")
		}
		evidence = fmt.Sprintf("Назначение: %s.\r\nКритерии проверки: %s.\r\nОграничения: %s.", dash(r.Purpose), criteria, limitations)
	}
	nextStep := "Следующее действие: проверить ограничения метода на данных выбранного объекта и зафиксировать основание решения."
	if strings.Contains(strings.ToLower(method.Status), "лаборатор") {
		nextStep = "Следующее действие: выполнить лабораторный подбор/совместимость и только после этого формировать мероприятие."
	} else if strings.Contains(strings.ToLower(method.Status), "автотехнолог") {
		nextStep = "Следующее действие: передать фактические данные объекта в «Автотехнолог» для проверки оборудования/компоновки."
	}
	base := fmt.Sprintf("МЕТОД ДЛЯ ПРОВЕРКИ: %s\r\nСтатус: %s\r\nИсточник: %s\r\nТип источника: %s\r\n%s\r\n%s\r\n%s\r\n\r\nВажно: РОСА не назначает метод автоматически. Выбор в списке означает только кандидат на инженерную проверку.", method.Name, method.Status, method.Source, dash(method.SourceKind), evidence, method.Note, nextStep)
	return base + methodEngineeringReferenceText(method.Name)
}

func assessmentDraftSummaryForObject(objectKey string) string {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return ""
	}
	names := make([]string, 0, 4)
	for _, item := range plans {
		if !strings.EqualFold(strings.TrimSpace(item.ObjectKey), objectKey) || !strings.Contains(strings.ToLower(item.Status), "черновик из оценки осложнений") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(item.Action, "Проверить применимость метода:"))
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return fmt.Sprintf("Черновики из «Оценки осложнений»: %s. Они не передаются в экономику и не изменяют прогноз МРП, пока специалист не подтвердит применимость и не сформирует сценарий мероприятия.", strings.Join(names, "; "))
}

func createAssessmentMethodDraftPlan() (string, error) {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return "", fmt.Errorf("сначала выберите объект")
	}
	if assessSelectedRow < 0 || assessSelectedRow >= len(assessDirectionRows) {
		return "", fmt.Errorf("сначала выберите направление осложнения")
	}
	method, ok := selectedAssessmentMethod()
	if !ok {
		return "", fmt.Errorf("выберите метод-кандидат в выпадающем списке")
	}
	if assessmentMethodIsEquipmentDesign(method.Name) {
		return "", fmt.Errorf("метод «%s» относится к оборудованию/компоновке. Проверьте его в «Автотехнологе»; РОСА не создаёт по нему проектное решение", method.Name)
	}
	row := assessDirectionRows[assessSelectedRow]
	action := "Проверить применимость метода: " + method.Name
	for _, p := range plans {
		if strings.EqualFold(strings.TrimSpace(p.ObjectKey), strings.TrimSpace(selectedObjectKey)) && strings.EqualFold(strings.TrimSpace(p.Action), action) && strings.Contains(strings.ToLower(p.Status), "черновик") {
			return method.Name, nil
		}
	}
	versionID := ""
	if v, ok := latestVersionForSelectedObject(); ok {
		versionID = v.VersionID
	}
	basis := fmt.Sprintf("Кандидат из оценки осложнений: %s. Результат: %s. Данные: %s; источник: %s; актуальность: %s. Статус метода: %s. Выбор не подтверждает причинность и не начисляет эффект на МРП.", row.Group.Name, row.Summary.State, row.CoverageText, row.Source, row.Freshness, method.Status)
	plans = append(plans, PlanItem{
		ID:        fmt.Sprintf("assess-%d", time.Now().UnixNano()),
		DueDate:   "—",
		ObjectKey: selectedObjectKey,
		Action:    action,
		Basis:     basis,
		Status:    "Черновик из оценки осложнений — требует подтверждения",
		VersionID: versionID,
	})
	_ = saveJSON(filepath.Join(dataDir, "plan.json"), plans)
	return method.Name, nil
}

func refreshAssessmentMethodDetail() {
	if assessSelectedRow < 0 || assessSelectedRow >= len(assessDirectionRows) {
		return
	}
	row := assessDirectionRows[assessSelectedRow]
	showAssessmentDirectionDetail(row)
}
