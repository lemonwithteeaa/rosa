//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

// RC681: the method handbook is organised as an engineering matrix rather than
// a flat vendor/product list.  Every catalogue method is placed into one of the
// six decisions a technologist actually makes: prevention, removal, equipment,
// operating regime, workover/completion, or monitoring/control.
const (
	methodsStagePrevention = "ПРЕДОТВРАЩЕНИЕ"
	methodsStageRemoval    = "УДАЛЕНИЕ"
	methodsStageEquipment  = "ОБОРУДОВАНИЕ"
	methodsStageRegime     = "РЕЖИМ"
	methodsStageWorkover   = "ТКРС / ЗАКАНЧИВАНИЕ"
	methodsStageControl    = "КОНТРОЛЬ"
)

var methodsPracticeStageOrder = []string{
	methodsStagePrevention,
	methodsStageRemoval,
	methodsStageEquipment,
	methodsStageRegime,
	methodsStageWorkover,
	methodsStageControl,
}

type methodsPracticeProfile struct {
	Title             string
	Inputs            []string
	Contraindications []string
	Control           []string
}

var methodsPracticeProfiles = map[string]methodsPracticeProfile{
	"aspo": {
		Title:             "АСПО — парафин и асфальтены",
		Inputs:            []string{"состав фактического осадка с раздельной оценкой парафиновой и асфальтеновой части", "температура начала кристаллизации парафина и температура застывания", "реология нефти по рабочему диапазону температур", "давление начала осаждения асфальтенов и траектория давление–температура", "профиль давления и температуры по глубине", "совместимость химии с нефтью, водой, эластомерами и другими реагентами", "история очисток, перепада давления, загрузки ГНО и отказов"},
		Contraindications: []string{"не объединять парафин и асфальтены без подтверждения состава осадка", "не переносить лабораторную эффективность реагента на скважину без проверки точки ввода, времени контакта и температурного окна", "тепловые и растворяющие обработки не применять без проверки материалов, пожарной безопасности и риска повторного выпадения ниже по потоку"},
		Control:           []string{"межоперационный период и частота очисток", "масса и состав снятых отложений", "перепад давления, дебит, ток и нагрузка ГНО", "температурный профиль", "остаточная концентрация реагента при наличии валидированной методики", "состояние НКТ и ГНО после подъёма"},
	},
	"scale": {
		Title:             "Солеотложения и сульфид железа",
		Inputs:            []string{"полный ионный состав пластовой, закачиваемой и попутной воды", "доли смешения вод и расход воды", "давление и температура по тракту", "минералогия фактического осадка", "расчёт пересыщения и место начала кристаллизации", "минимальная эффективная концентрация ингибитора и совместимость реагента с рассолом", "для FeS — Fe общий/растворённый, H₂S, pH, окислительно-восстановительные условия и продукты коррозии"},
		Contraindications: []string{"не выбирать кислоту без минералогии: сульфатные соли и FeS требуют отдельной химии", "не допускать подпороговой концентрации ингибитора без проверки динамического теста и остаточной концентрации", "обработку FeS кислотой не проводить без оценки выделения H₂S, коррозии и безопасного отвода продуктов реакции", "инфраструктурные методы водоподготовки не назначать как локальную операцию одной скважины без проверки системы ППД"},
		Control:           []string{"остаточная концентрация ингибитора относительно минимальной эффективной", "ионный баланс и динамика пересыщения", "минералогия и масса отложений", "перепад давления и продуктивность", "ток/нагрузка ГНО", "для FeS — железо, H₂S и продукты реакции", "наработка до очистки/отказа"},
	},
	"corrosion": {
		Title:             "Коррозия",
		Inputs:            []string{"CO₂, H₂S, растворённый O₂, pH, хлориды и минерализация", "обводнённость и режим потока", "давление, температура и скорость потока", "фактическая общая и локальная скорость коррозии", "морфология повреждений и состояние поднятого оборудования", "отложения и риск подотложечной коррозии", "совместимость ингибитора с другими реагентами и неметаллическими материалами"},
		Contraindications: []string{"не назначать поглотитель кислорода без подтверждённого кислородного фактора", "ингибитор не должен подменять устранение механического повреждения, эрозии или неподходящего материала", "покрытие/футеровку не выбирать без проверки температур, диаметров, сцепления и ремонтопригодности", "мониторинг одним показателем не считать достаточным доказательством эффективности"},
		Control:           []string{"коррозионные купоны и/или зонды там, где конструктивно возможно", "ультразвуковая толщинометрия и дефектоскопия", "железо в воде с учётом ограничений метода", "остаточная концентрация ингибитора", "локальная и общая скорость коррозии", "частота течей/отказов и результаты разбора поднятого оборудования"},
	},
	"microbiology": {
		Title:             "Микробиологически обусловленная коррозия",
		Inputs:            []string{"репрезентативные пробы воды и биоплёнки", "микробиологические показатели культуральными и/или молекулярными методами", "H₂S, сульфаты/сульфиды, железо, pH и температура", "морфология коррозии", "режим закачки/добычи и время застоя", "совместимость биоцида с водой, реагентами и материалами"},
		Contraindications: []string{"наличие микроорганизмов само по себе не доказывает причинность коррозии", "биоцид не назначать только по единичной пробе без оценки биоплёнки и эксплуатационного контекста", "нитратную обработку применять только после микробиологического и технологического обоснования", "разрушение биоплёнки без последующего контроля может временно увеличить вынос продуктов и коррозионную нагрузку"},
		Control:           []string{"динамика микробных групп/генетических маркеров", "биоплёнка", "H₂S и сульфиды", "коррозионная скорость", "остаточная концентрация биоцида при валидированной методике", "состояние фильтров/оборудования и повторяемость обрастания"},
	},
	"solids": {
		Title:             "Механические примеси и пескопроявление",
		Inputs:            []string{"гранулометрический состав с распределением размеров", "концентрация, минералогия и твёрдость частиц", "происхождение частиц: пласт, проппант, продукты коррозии/солей", "геомеханика и история пескопроявления", "дебит, депрессия и динамика выноса при изменении режима", "история ГРП/ТКРС", "геометрия заканчивания, допустимый перепад и риск эрозии/закупоривания"},
		Contraindications: []string{"фильтр без гранулометрии может быстро закупориться или пропускать опасную фракцию", "повышение дебита ограничивать безопасным потоком через экран/гравийную набивку, а не только депрессией", "устьевой пескоотделитель не защищает погружной насос от уже прошедших в него частиц", "химическое закрепление не применять без оценки потери проницаемости и совместимости с пластом"},
		Control:           []string{"концентрация и гранулометрия выносимых частиц", "перепад на фильтре/экране", "эрозия и износ рабочих органов", "дебит и продуктивность", "ток/вибрация/наполнение насоса", "объём песка после остановок", "состояние экрана, гравийной набивки и насоса после подъёма"},
	},
	"free_gas": {
		Title:             "Свободный газ",
		Inputs:            []string{"доля свободного газа на приёме насоса", "газовый фактор и давление насыщения", "давление на приёме и динамический уровень", "дебит жидкости/газа и нестационарность притока", "геометрия ствола и возможность отвода газа в затруб", "температура двигателя", "для ШГН — наполнение насоса и динамограмма"},
		Contraindications: []string{"газосепаратор не выбирать без расчёта производительности и доли газа", "газообработчик и сепаратор решают разные задачи и не должны считаться взаимозаменяемыми", "повышение противодавления не считать универсальным способом борьбы с газом", "изменение частоты/глубины спуска проверять на приток, охлаждение двигателя и допустимый рабочий диапазон насоса"},
		Control:           []string{"давление на приёме", "доля свободного газа/газовый фактор", "ток, частота и температура двигателя", "число срывов подачи и остановок", "стабильность дебита", "для ШГН — коэффициент наполнения и динамограмма", "наработка оборудования"},
	},
	"hydrates": {
		Title:             "Гидраты",
		Inputs:            []string{"состав газа и воды", "фазовая граница гидратообразования", "профиль давления и температуры и величина переохлаждения", "наличие свободной воды", "расходы газа/жидкости и время пребывания", "режимы остановки и пуска", "совместимость ингибитора с коррозионной и другой химией"},
		Contraindications: []string{"не применять неконтролируемую быструю разгерметизацию при сформированной пробке", "кинетический ингибитор и антиагломерант не считать эквивалентами метанола/МЭГ: механизмы и рабочие окна различны", "антиагломерант требует подходящей жидкой углеводородной фазы и лабораторной проверки", "любое удаление пробки выполнять по отдельному плану с анализом запертого давления"},
		Control:           []string{"запас до фазовой границы гидратообразования по давлению–температуре", "расход и концентрация ингибитора", "перепад давления и признаки ограничения потока", "температура после дросселирования/остановки", "содержание воды", "события остановки и безопасный перезапуск"},
	},
	"viscosity": {
		Title:             "Высокая вязкость и эмульсия",
		Inputs:            []string{"вязкость по диапазону температур и скоростей сдвига", "температура застывания и реологическая модель", "обводнённость и тип/устойчивость эмульсии", "состав нефти и воды", "давление/температура по тракту", "характеристики насоса при фактической вязкости", "мощность, ток и ограничения оборудования"},
		Contraindications: []string{"не смешивать собственную высокую вязкость нефти и вязкость устойчивой эмульсии", "не применять разбавитель без проверки асфальтеновой стабильности, совместимости и экономики возврата/утилизации", "тепловые методы требуют проверки целостности скважины и материалов", "паспортную характеристику УЭЦН на воде не использовать без вязкостной коррекции/испытаний"},
		Control:           []string{"вязкость и температура", "обводнённость и устойчивость эмульсии", "дебит", "давление на приёме", "ток/мощность/КПД насоса", "удельное энергопотребление", "межремонтный период и характер отказов"},
	},
	"high_temp": {
		Title:             "Высокая температура",
		Inputs:            []string{"температура флюида и профиль температуры по глубине", "расчётный нагрев двигателя под нагрузкой", "дебит охлаждающего потока вокруг двигателя", "газосодержание", "состав среды и коррозионная активность", "температурные рейтинги двигателя, гидрозащиты, кабеля, вводов, эластомеров, датчиков и соединений", "история термоциклов"},
		Contraindications: []string{"температурный класс всей системы определяется самым слабым элементом", "не выбирать оборудование только по пластовой/устьевой температуре без расчёта собственного нагрева", "обычные эластомеры и кабели не применять за пределами температурного рейтинга", "после паротеплового воздействия учитывать термоциклические напряжения и требования целостности скважины"},
		Control:           []string{"температура двигателя и на приёме", "давление на приёме", "ток/мощность/частота", "сопротивление изоляции и состояние кабеля при ремонте", "вибрация и защитные срабатывания", "температурная история/термоциклы", "разбор отказов после подъёма"},
	},
}

func methodsPracticeCategories(name string) []string {
	checks := []struct {
		category string
		group    string
	}{
		{"aspo", "aspo"},
		{"scale", "scale"},
		{"corrosion", "corrosion"},
		{"microbiology", "microbiology"},
		{"solids", "solids"},
		{"free_gas", "free_gas"},
		{"hydrates", "hydrates"},
		{"viscosity", "viscosity"},
		{"high_temp", "high_temp"},
	}
	out := []string{}
	seen := map[string]bool{}
	add := func(category string) {
		if category == "" || seen[category] {
			return
		}
		seen[category] = true
		out = append(out, category)
	}
	for _, check := range checks {
		if assessmentMethodMatchesGroup(check.group, name) {
			add(check.category)
		}
	}
	// Older corporate methods can have deliberately generic captions such as
	// "защитное покрытие". Use their exact rule complication as a second signal
	// so the world-practice matrix does not lose them merely because the caption
	// itself lacks a keyword.
	nameKey := normalizeMethodText(name)
	for _, rule := range ruleCatalog.Rules {
		if normalizeMethodText(rule.Method) != nameKey {
			continue
		}
		c := strings.ToLower(strings.TrimSpace(rule.Complication))
		switch {
		case strings.Contains(c, "микроб"):
			add("microbiology")
		case strings.Contains(c, "аспо") || strings.Contains(c, "парафин") || strings.Contains(c, "асфальтен"):
			add("aspo")
		case strings.Contains(c, "соле") || strings.Contains(c, "сульфид железа"):
			add("scale")
		case strings.Contains(c, "корроз"):
			add("corrosion")
		case strings.Contains(c, "механ") || strings.Contains(c, "пес"):
			add("solids")
		case strings.Contains(c, "гидрат"):
			add("hydrates")
		case strings.Contains(c, "свобод") && strings.Contains(c, "газ"):
			add("free_gas")
		case strings.Contains(c, "вязк") || strings.Contains(c, "эмульс"):
			add("viscosity")
		case strings.Contains(c, "температур"):
			add("high_temp")
		}
	}
	return out
}

func methodsPracticeCategory(name string) string {
	n := normalizeMethodText(name)
	switch {
	case strings.Contains(n, "биоцид") || strings.Contains(n, "микроб") || strings.Contains(n, "биоплен") || strings.Contains(n, "биоплен") || strings.Contains(n, "нитратн"):
		return "microbiology"
	case strings.Contains(n, "сульфид железа") || strings.Contains(n, "fes"):
		return "scale"
	case assessmentMethodMatchesGroup("hydrates", name):
		return "hydrates"
	case assessmentMethodMatchesGroup("high_temp", name):
		return "high_temp"
	case assessmentMethodMatchesGroup("free_gas", name):
		return "free_gas"
	case assessmentMethodMatchesGroup("viscosity", name):
		return "viscosity"
	case assessmentMethodMatchesGroup("solids", name):
		return "solids"
	case assessmentMethodMatchesGroup("corrosion", name):
		return "corrosion"
	case assessmentMethodMatchesGroup("scale", name):
		return "scale"
	case assessmentMethodMatchesGroup("aspo", name):
		return "aspo"
	}
	return ""
}

func methodsPracticeStage(name string) string {
	nameKey := normalizeMethodText(name)
	for _, rule := range ruleCatalog.Rules {
		if normalizeMethodText(rule.Method) != nameKey {
			continue
		}
		for _, allowed := range methodsPracticeStageOrder {
			if strings.EqualFold(strings.TrimSpace(rule.Stage), allowed) {
				return allowed
			}
		}
	}
	n := nameKey
	// Monitoring/control first so a phrase containing an equipment name does
	// not hide the fact that the method is purely diagnostic.
	if strings.Contains(n, "контроль") || strings.Contains(n, "мониторинг") || strings.Contains(n, "термометр") || strings.Contains(n, "толщинометр") || strings.Contains(n, "дефектоскоп") || strings.Contains(n, "купон") || strings.Contains(n, "зонд") || strings.Contains(n, "расчет и контроль") || strings.Contains(n, "расчет границы") {
		return methodsStageControl
	}
	if strings.Contains(n, "удаление") || strings.Contains(n, "раствор") || strings.Contains(n, "промыв") || strings.Contains(n, "очист") || strings.Contains(n, "нормализац") || strings.Contains(n, "разложени") || strings.Contains(n, "депрессуризац") || strings.Contains(n, "комплексон") || strings.Contains(n, "хелат") || strings.Contains(n, "кислот") || strings.Contains(n, "гнкт") {
		return methodsStageRemoval
	}
	if strings.Contains(n, "гравийн") || strings.Contains(n, "закрепление призабой") || strings.Contains(n, "закрепление пропп") || strings.Contains(n, "размещени") || strings.Contains(n, "селективное заканчив") || strings.Contains(n, "гидроразрыв") || strings.Contains(n, "пластовая обработка") || strings.Contains(n, "squeeze") || strings.Contains(n, "сквиз") || strings.Contains(n, "паротепловая обработка") || strings.Contains(n, "эксплуатационная компоновка") {
		return methodsStageWorkover
	}
	if strings.Contains(n, "режим") || (strings.Contains(n, "депресс") && !strings.Contains(n, "депрессор")) || strings.Contains(n, "частот") || strings.Contains(n, "скорости шгн") || strings.Contains(n, "давления и температуры") || strings.Contains(n, "ограничение эксплуатационного расхода") || strings.Contains(n, "заглубление") || strings.Contains(n, "глубины спуска") {
		return methodsStageRegime
	}
	if assessmentMethodIsEquipmentDesign(name) || strings.Contains(n, "фильтр") || strings.Contains(n, "экран") || strings.Contains(n, "футеров") || strings.Contains(n, "покрыти") || strings.Contains(n, "пескоотдел") || strings.Contains(n, "гидроциклон") || strings.Contains(n, "газовый якор") || strings.Contains(n, "нагреватель") || strings.Contains(n, "изоляц") || strings.Contains(n, "кабель") || strings.Contains(n, "теплоизоляц") {
		return methodsStageEquipment
	}
	return methodsStagePrevention
}

func methodsPracticeProfileFor(name string) methodsPracticeProfile {
	categories := methodsPracticeCategories(name)
	if len(categories) == 0 {
		return methodsPracticeProfile{}
	}
	seen := map[string]bool{}
	appendUnique := func(dst *[]string, values ...string) {
		for _, value := range values {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			*dst = append(*dst, value)
		}
	}
	profile := methodsPracticeProfile{}
	titles := []string{}
	for _, category := range categories {
		p := methodsPracticeProfiles[category]
		if p.Title == "" {
			continue
		}
		titles = append(titles, p.Title)
		appendUnique(&profile.Inputs, p.Inputs...)
		appendUnique(&profile.Contraindications, p.Contraindications...)
		appendUnique(&profile.Control, p.Control...)
	}
	profile.Title = strings.Join(titles, " / ")
	return profile
}

func methodsPracticeMethodMatrix() map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for _, m := range ruleCatalog.MethodCatalog {
		categories := methodsPracticeCategories(m.Name)
		if len(categories) == 0 {
			continue
		}
		stage := methodsPracticeStage(m.Name)
		for _, category := range categories {
			if out[category] == nil {
				out[category] = map[string][]string{}
			}
			// Exact duplicate names are intentionally hidden from each matrix cell
			// even if a legacy installation still carries a duplicate record.
			exists := false
			for _, current := range out[category][stage] {
				if normalizeMethodText(current) == normalizeMethodText(m.Name) {
					exists = true
					break
				}
			}
			if !exists {
				out[category][stage] = append(out[category][stage], m.Name)
			}
		}
	}
	for _, stages := range out {
		for _, stage := range methodsPracticeStageOrder {
			sort.SliceStable(stages[stage], func(i, j int) bool { return strings.ToLower(stages[stage][i]) < strings.ToLower(stages[stage][j]) })
		}
	}
	return out
}

func methodsWorldPracticeMatrixText() string {
	matrix := methodsPracticeMethodMatrix()
	order := []string{"aspo", "scale", "corrosion", "microbiology", "solids", "free_gas", "hydrates", "viscosity", "high_temp"}
	var b strings.Builder
	b.WriteString("МАТРИЦА МИРОВОЙ ПРАКТИКИ: ОТ ПРИЧИНЫ К КОНТРОЛЮ\r\n")
	b.WriteString("Каждое направление разложено по шести типам решения: предотвращение → удаление → оборудование → режим → ТКРС/заканчивание → контроль. Пустая ячейка означает, что отдельный метод для этой стадии в текущем контексте не выделен; это не разрешение выдумывать мероприятие.\r\n\r\n")
	for _, category := range order {
		profile := methodsPracticeProfiles[category]
		if profile.Title == "" {
			continue
		}
		b.WriteString(strings.ToUpper(profile.Title) + "\r\n")
		for _, stage := range methodsPracticeStageOrder {
			methods := matrix[category][stage]
			if len(methods) == 0 {
				fmt.Fprintf(&b, "%s: —\r\n", stage)
				continue
			}
			fmt.Fprintf(&b, "%s: %s.\r\n", stage, strings.Join(methods, "; "))
		}
		b.WriteString("\r\n")
	}
	return b.String()
}

func methodsPracticeDetailedCard(name string) string {
	p := methodsPracticeProfileFor(name)
	if p.Title == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("\r\n\r\nМЕСТО В ИНЖЕНЕРНОЙ МАТРИЦЕ\r\n")
	fmt.Fprintf(&b, "Осложнение: %s. Стадия решения: %s.\r\n", p.Title, methodsPracticeStage(name))
	b.WriteString("\r\nТРЕБУЕМЫЕ ИСХОДНЫЕ ДАННЫЕ\r\n")
	b.WriteString("• " + strings.Join(p.Inputs, "; ") + ".\r\n")
	b.WriteString("\r\nПРОТИВОПОКАЗАНИЯ И ОГРАНИЧЕНИЯ\r\n")
	b.WriteString("• " + strings.Join(p.Contraindications, "; ") + ".\r\n")
	b.WriteString("\r\nКОНТРОЛЬ РЕЗУЛЬТАТА\r\n")
	b.WriteString("• " + strings.Join(p.Control, "; ") + ".\r\n")
	return b.String()
}
