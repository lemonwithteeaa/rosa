from pathlib import Path
import sys
root = Path('.')
files = {
    'workflow': (root/'methods_workflow.go').read_text(encoding='utf-8'),
    'reference': (root/'methods_reference.go').read_text(encoding='utf-8'),
    'lab': (root/'laboratory_inhibitor_trials.go').read_text(encoding='utf-8'),
    'choices': (root/'well_failure_choices.go').read_text(encoding='utf-8'),
    'terms': (root/'russian_terms.go').read_text(encoding='utf-8'),
}
checks = {
    'вероятность — русская подпись': 'Вероятность, %' in files['workflow'] and 'P эффекта' not in files['workflow'],
    'доза–эффект — русский заголовок': 'ЗАВИСИМОСТЬ «ДОЗА–ЭФФЕКТ» (dose–response)' in files['lab'] and 'DOSE-RESPONSE' not in files['lab'],
    'MIC — русский термин первым': 'Микробиологически обусловленная коррозия (MIC)' in files['choices'],
    'WAT — русский термин первым': 'температуру начала кристаллизации парафина (WAT)' in files['reference'],
    'MEC — русский термин первым': 'минимальную эффективную концентрацию (MEC)' in files['reference'],
    'gas handling — русский термин первым': 'многофазная работа с газожидкостной смесью (multiphase gas handling)' in files['reference'],
    'viscosity reducer — русский термин первым': 'реагент для снижения вязкости (viscosity reducer)' in files['reference'],
    'правило русификации закреплено': 'английский термин допускается только после русского эквивалента в скобках' in files['terms'].lower(),
}
failed=[]
for name, ok in checks.items():
    print(('ПРОЙДЕНО' if ok else 'ОШИБКА') + ': ' + name)
    if not ok: failed.append(name)
if failed:
    sys.exit(1)
print(f'ИТОГ: {len(checks)}/{len(checks)} ПРОЙДЕНО')
