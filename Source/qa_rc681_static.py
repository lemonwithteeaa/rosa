from pathlib import Path
import json,re
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
seed=json.loads((root/'rules_seed.json').read_text(encoding='utf-8'))
flow=(root/'methods_world_practice.go').read_text(encoding='utf-8')
ref=(root/'methods_reference_dialog.go').read_text(encoding='utf-8')
forecast=(root/'methods_forecast_details.go').read_text(encoding='utf-8')
rules_go=(root/'rules.go').read_text(encoding='utf-8')

def norm(s):
    s=s.lower().replace('ё','е')
    s=re.sub(r'[/+()\-]',' ',s)
    return ' '.join(s.split())
methods=seed['method_catalog']; rules=seed['rules']; sources=seed['sources']
method_names=[norm(m['name']) for m in methods]
rule_names={norm(r['method']) for r in rules}
source_ids={s['id'] for s in sources}
stages={'ПРЕДОТВРАЩЕНИЕ','УДАЛЕНИЕ','ОБОРУДОВАНИЕ','РЕЖИМ','ТКРС / ЗАКАНЧИВАНИЕ','КОНТРОЛЬ'}
# Normalize rule complication families used in the handbook.
def family(c):
    x=c.lower()
    if 'аспо' in x or 'парафин' in x or 'асфальтен' in x: return 'АСПО'
    if 'соле' in x or 'сульфид' in x: return 'Солеотложения'
    if 'микроб' in x: return 'Микробиология'
    if 'корроз' in x: return 'Коррозия'
    if 'механ' in x or 'пес' in x: return 'Механические примеси'
    if 'свобод' in x and 'газ' in x: return 'Свободный газ'
    if 'газогидрат' in x or 'гидрат' in x: return 'Гидраты'
    if 'вязк' in x: return 'Высокая вязкость'
    if 'температур' in x: return 'Высокая температура'
    return ''
control_families={family(r.get('complication','')) for r in rules if r.get('stage')=='КОНТРОЛЬ'}
required_control={'АСПО','Солеотложения','Коррозия','Механические примеси','Свободный газ','Гидраты','Высокая вязкость','Высокая температура'}
checks={
 'version 681': 'const appBuildVersion = "7.3.11.681-RC"' in main,
 'schema 6 or newer': seed.get('schema_version',0)>=6,
 'world-practice source base expanded': len(sources)>=70,
 'method catalogue expanded': len(methods)>=130,
 'no duplicate method captions': len(method_names)==len(set(method_names)),
 'no Latin words in visible method names': all(not re.search(r'[A-Za-z]{2,}',m['name']) for m in methods),
 'every catalogue method has a rule': all(n in rule_names for n in method_names),
 'every rule has stage': all(r.get('stage') in stages for r in rules),
 'every rule has required inputs': all(bool(r.get('required_inputs')) for r in rules),
 'every rule has contraindications': all(bool(r.get('contraindications')) for r in rules),
 'every rule has result control': all(bool(r.get('control')) for r in rules),
 'every rule sources resolve': all(bool(r.get('source_ids')) and all(s in source_ids for s in r.get('source_ids',[])) for r in rules),
 'all eight complications have explicit monitoring': required_control <= control_families,
 'six-stage matrix exists': all(stage in flow for stage in stages),
 'matrix supports multipurpose methods': 'methodsPracticeCategories' in flow and 'for _, category := range categories' in flow,
 'legacy duplicate TM-31 removed from seed': all(m['id']!='TM-31' for m in methods),
 'legacy duplicate removed from saved catalog migration': 'method.ID == "TM-31"' in rules_go and 'seenMethodNames' in rules_go,
 'brand is not base RVK method name': all('ПИЛОТ' not in m['name'] for m in methods),
 'brand is not base container method name': all('ТРИЛ-' not in m['name'] for m in methods),
 'reference bold italic only whole headings': 'upperHeading :=' in ref and 't == strings.ToUpper(t)' in ref,
 'forecast bold italic only whole headings': 'wholeHeading :=' in forecast and 'trimmed == strings.ToUpper(trimmed)' in forecast,
}
for name,ok in checks.items(): print(('PASS' if ok else 'FAIL')+': '+name)
print(f"sources={len(sources)} methods={len(methods)} rules={len(rules)}")
print(f"{sum(checks.values())}/{len(checks)} PASS")
raise SystemExit(0 if all(checks.values()) else 1)
