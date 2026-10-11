from pathlib import Path
import json, re

root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
methods = (root/'assessment_methods.go').read_text(encoding='utf-8')
workflow = (root/'methods_workflow.go').read_text(encoding='utf-8')
ref = (root/'methods_reference.go').read_text(encoding='utf-8')
dialog = (root/'methods_reference_dialog.go').read_text(encoding='utf-8')
eng = (root/'methods_engineering_reference.go').read_text(encoding='utf-8')
effect = (root/'methods_effect_model.go').read_text(encoding='utf-8')
rep = (root/'methods_replication.go').read_text(encoding='utf-8')
rules = json.loads((root/'rules_seed.json').read_text(encoding='utf-8'))
rulesgo = (root/'rules.go').read_text(encoding='utf-8')

method_names = [m.get('name','') for m in rules.get('method_catalog',[])]
unique_methods = {re.sub(r'\s+',' ',m.strip().casefold()) for m in method_names if m.strip()}
source_visible = []
for s in rules.get('sources',[]):
    for key in ('organization','title','kind'):
        source_visible.append(str(s.get(key,'')))
latin = re.compile(r'[A-Za-z]{2,}')

required_methods = [
    'Ингибитор образования парафиновых отложений',
    'Ингибитор осаждения асфальтенов',
    'Растворение отложений сульфида железа хелатирующим или малокоррозионным составом с контролем сероводорода',
    'Гравийная набивка для контроля пескопроявления',
    'Химическое закрепление призабойной зоны для ограничения выноса песка',
    'Газожидкостный смеситель перед УЭЦН для работы при повышенной доле свободного газа',
    'Локальный обогрев зоны риска гидратообразования',
    'Деэмульгатор для разрушения устойчивой водонефтяной эмульсии',
    'Термостойкий погружной электродвигатель с высокотемпературной изоляцией',
]

checks = {
    'version 677 or newer': int(re.findall(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)[-1]) >= 677,
    'catalog substantially expanded': len(unique_methods) >= 80,
    'all key researched method classes are present': all(x in method_names for x in required_methods),
    'direction-specific groups split gas hydrate temperature viscosity': all(x in methods for x in (
        'return "viscosity"', 'return "free_gas"', 'return "hydrates"', 'return "high_temp"')),
    'workflow uses direction-specific method group': 'assessmentMethodCandidatesForGroup(assessmentMethodGroupForDirection(d))' in workflow,
    'viscosity keeps relevant legacy equipment': all(x in methods for x in ('"уэвн"', '"винтовая насосная"', '"нкт увеличенного диаметра"')),
    'hydrate keeps thermal cable': '"нагревательная кабельная линия"' in methods,
    'high temperature keeps shroud and thermometry': '"кожух уэцн"' in methods and '"термометрия перед спуском"' in methods,
    'method names contain no English words': not any(latin.search(x) for x in method_names),
    'source visible names are Russianized': not any(latin.search(x) for x in source_visible),
    'saved built-in catalog cannot reintroduce legacy wording': 'seedSources[v.ID] = v' in rulesgo and 'seedMethods[v.ID] = v' in rulesgo and 'seedRules[v.ID] = v' in rulesgo,
    'reference source prose is Russianized': all(x not in ref for x in ('• SLB:', '• AMPP', 'run life', 'uptime', '(ESP)', '(MIC)')),
    'effect and analogy prose contains no legacy English explanations': all(x not in effect+rep for x in ('(run life)', '(uptime)', '(ESP)', '(MIC)', '(VSD)', '(rollout)', '(gas handling)', '(scale control)', '(squeeze life)')),
    'long engineering reference is attached to every handbook card': 'b.WriteString(methodEngineeringReferenceText(method.Name))' in dialog,
    'engineering reference covers FeS safety': 'КРИТИЧЕСКАЯ БЕЗОПАСНОСТЬ' in eng and 'токсичный сероводород' in eng,
    'engineering reference covers sand gas hydrate viscosity high temperature': all(x in eng for x in (
        'противопесочный', 'газожидкостный смеситель', 'локальный обогрев зоны риска гидратообразования',
        'деэмульгатор для разрушения', 'термостойкий погружной электродвигатель')),
    'normal handbook text explicitly clears bold and italic': 'DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize' in dialog and 'DwEffects:' not in dialog[dialog.index('normal := CHARFORMATW'):dialog.index('pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, ^uintptr(0))')],
    'uppercase section headings keep separate emphasis from field labels': 'upperHeading := t == strings.ToUpper(t)' in dialog and 'upperHeading && !schemeHeading' in dialog and 'fieldLabels := []string{' in dialog,
    'handbook headings are bold italic': 'DwEffects:   CFE_BOLD | cfeItalic' in dialog,
    'visible handbook heading labels also use bold italic font': 'pCreateFontW.Call(^uintptr(uiTextHeight-1), 0, 0, 0, 600, 1' in dialog and 'methodsReferenceHeadingFont' in dialog and 'methodsReferenceTitleFont' in dialog,
    'handbook subtitle now says detailed': 'Подробная инженерная база · каждый метод открывается по клику' in dialog,
}

for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
