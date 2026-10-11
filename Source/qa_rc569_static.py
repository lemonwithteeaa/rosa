from pathlib import Path
import json, sys
main = Path('main.go').read_text(encoding='utf-8')
workflow = Path('methods_workflow.go').read_text(encoding='utf-8')
reference = Path('methods_reference.go').read_text(encoding='utf-8')
dialog = Path('methods_reference_dialog.go').read_text(encoding='utf-8')
effect = Path('methods_effect_model.go').read_text(encoding='utf-8')
rep = Path('methods_replication.go').read_text(encoding='utf-8')
lab_trials = Path('laboratory_inhibitor_trials.go').read_text(encoding='utf-8')
well_choices = Path('well_failure_choices.go').read_text(encoding='utf-8')
russian_terms = Path('russian_terms.go').read_text(encoding='utf-8')
seed = json.loads(Path('rules_seed.json').read_text(encoding='utf-8'))
checks = {
    'version 569': 'const appBuildVersion = "7.3.11.569-RC"' in main,
    'replication button': '"ТИРАЖИРОВАНИЕ"' in workflow and 'ID_METHODS_REPLICATION' in workflow,
    'reference replication tab': 'methodsReferenceReplication' in dialog and '"ТИРАЖИРОВАНИЕ"' in dialog,
    'three engineering metrics': all(x in reference for x in ['ПРИМЕНИМОСТЬ МЕТОДА, %','ДОСТОВЕРНОСТЬ ОЦЕНКИ, %','ВЕРОЯТНОСТЬ ПОДТВЕРЖДЕНИЯ ЭФФЕКТА, %']),
    'probability interval stored': all(x in effect for x in ['ProbabilityLow','ProbabilityHigh','ProbabilityObjects']),
    'minimum empirical sample': 'sample < 5 || objects < 3' in effect,
    'replication weights': all(x in rep for x in ['0.30*float64(out.MechanismFit)','0.20*float64(out.OperatingFit)','0.15*float64(out.EquipmentFit)','0.20*float64(out.Repeatability)','0.10*float64(out.Durability)','0.05*float64(out.OperationalReproducibility)']),
    'mass rollout hard gate': all(x in rep for x in ['a.ProbabilitySample >= 10','a.ProbabilityObjects >= 5','a.ProbabilityLow >= 55','out.QualifiedAnalogCount >= 10','out.Score >= 88']),
    'pilot first wording': 'сначала пилот' in rep.lower(),
    'equipment stays in Autotechnolog': '«Автотехнологе»' in rep,
    'monitoring no MRP bonus': 'мониторинг повышает управляемость и достоверность программы' in effect,
    'seed schema 3': seed.get('schema_version') == 3,
    'expanded sources': len(seed.get('sources',[])) >= 24,
    'expanded methods': len(seed.get('method_catalog',[])) >= 42,
    'gas operational control': any(m.get('id') == 'OS-GAS-VSD-CTRL' for m in seed.get('method_catalog',[])),
    'solids drawdown control': any(m.get('id') == 'OS-SOLIDS-DRAWDOWN' for m in seed.get('method_catalog',[])),
    'MEC monitoring': any(m.get('id') == 'OS-SCALE-MEC-MON' for m in seed.get('method_catalog',[])),
    'MIC monitoring': any(m.get('id') == 'OS-MIC-MON' for m in seed.get('method_catalog',[])),
    'localized probability label': 'Вероятность, %' in workflow and 'P эффекта' not in workflow,
    'localized dose response': 'ЗАВИСИМОСТЬ «ДОЗА–ЭФФЕКТ» (dose–response)' in lab_trials and 'DOSE-RESPONSE' not in lab_trials,
    'localized MIC label': 'Микробиологически обусловленная коррозия (MIC)' in well_choices,
    'localized method reference': all(x in reference for x in ['температуру начала кристаллизации парафина (WAT)','минимальную эффективную концентрацию (MEC)','многофазная работа с газожидкостной смесью (multiphase gas handling)','реагент для снижения вязкости (viscosity reducer)']),
    'russian-first rule documented': 'английский термин допускается' in russian_terms.lower() and 'в скобках' in russian_terms.lower(),
}
failed=[]
for name,ok in checks.items():
    print(('PASS' if ok else 'FAIL')+': '+name)
    if not ok: failed.append(name)
if failed:
    print('FAILED:', ', '.join(failed)); sys.exit(1)
