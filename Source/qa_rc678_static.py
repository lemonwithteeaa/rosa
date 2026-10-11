from pathlib import Path
import json, re

root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
flow = (root/'methods_workflow.go').read_text(encoding='utf-8')
methods = (root/'assessment_methods.go').read_text(encoding='utf-8')
forecast = (root/'methods_forecast_details.go').read_text(encoding='utf-8')
eng = (root/'methods_engineering_reference.go').read_text(encoding='utf-8')
dialog = (root/'methods_reference_dialog.go').read_text(encoding='utf-8')
rules = json.loads((root/'rules_seed.json').read_text(encoding='utf-8'))

versions = re.findall(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
m = int(versions[-1]) if versions else 0
names = [x.get('name','') for x in rules.get('method_catalog',[])]
ids = {x.get('id','') for x in rules.get('method_catalog',[])}
source_ids = {x.get('id','') for x in rules.get('sources',[])}
rule_ids = {x.get('id','') for x in rules.get('rules',[])}

checks = {
    'version 678 or newer': m >= 678,
    'UPLD catalog entry': 'Установка с погружным линейным двигателем (УПЛД)' in names and 'OS-UPLD' in ids,
    'RVK technological catalog entry': 'Резонансно-волновое воздействие для снижения солеотложений' in names and 'OS-SCALE-RVK-PILOT' in ids,
    'UPLD source and rule': 'oil_avtomatika_upld_2026' in source_ids and 'OS-UPLD-VISC-R' in rule_ids,
    'RVK source and rule': 'pilot_rvk_official' in source_ids and 'OS-SCALE-RVK-PILOT-R' in rule_ids,
    'UPLD is treated as equipment candidate': '"уплд"' in methods and '"погружным линейным двигателем"' in methods,
    'RVK technology is matched to scale': '"резонансно волнов"' in methods,
    'UPLD matched to viscosity': 'viscosity := containsAny' in methods and '"уплд"' in methods[methods.index('viscosity := containsAny'):methods.index('solids := containsAny')],
    'UPLD allowed into Methods workflow but still equipment': 'if !strings.Contains(n, "уплд")' in flow and 'assessmentMethodIsEquipmentDesign(candidate.Name)' in flow,
    'full UPLD engineering reference': all(x in eng for x in ('УПЛД — вариант механизированной добычи', 'СОСТАВ СИСТЕМЫ', 'Погружной линейный электродвигатель'.lower() if False else 'погружной линейный электродвигатель')),
    'full RVK engineering reference': all(x in eng for x in ('Резонансно-волновое воздействие рассматривается', 'РВК «ПИЛОТ» приведён только как пример', 'ЗАЯВЛЕННЫЙ ПРИНЦИП ДЕЙСТВИЯ', 'ОПИ')),
    'reference directions include split viscosity gas hydrate temp': all(x in dialog for x in ('{"viscosity", "высокая вязкость"}', '{"free_gas", "свободный газ"}', '{"hydrates", "гидраты"}', '{"high_temp", "высокая температура"}')),
    'forecast body explicitly clears bold and italic': 'DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize' in forecast,
    'forecast only whole uppercase headings emphasized': 'wholeHeading := trimmed != "" && trimmed == strings.ToUpper(trimmed)' in forecast and '!strings.HasPrefix(trimmed, "•")' in forecast,
    'forecast headings are bold italic': 'DwEffects:   CFE_BOLD | cfeItalic' in forecast,
    'forecast static heading fonts are italic and cards are plain': 'methodsForecastDetailHeadingFont' in forecast and '600, 1, 0, 0, 204' in forecast and 'methodsFont(methodsTypeBody, false)' in forecast,
    'long Russian method titles trigger measured two-card page': 'methodsCandidateTitlesNeedTwoCardPage' in flow and 'methodsMeasuredTextHeight(title, cardW-32, methodsTypeBody, true)' in flow and 'perPage = 2' in flow,
    'catalog has at least 85 entries and 84 unique methods': len(names) >= 85 and len({re.sub(r'\s+',' ',x.strip().casefold()) for x in names if x.strip()}) >= 84,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
