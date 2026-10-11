from pathlib import Path
root = Path(__file__).resolve().parent
files = {n:(root/n).read_text(encoding='utf-8') for n in [
    'main.go','object_selection.go','laboratory.go','demo_seed.go','workflow_pages.go','mrp_forecast.go','assessment_classification.go'
]}
checks = {
    'version_539': 'const appBuildVersion = "7.3.11.539-RC"' in files['main.go'],
    'main_table_stable_height': 'desiredObjectH := max32(214' in files['main.go'],
    'selected_row_restored': 'selectListRow(objectsList, selected)' in files['main.go'],
    'single_global_object_transition': 'func applyGlobalObjectSelection' in files['object_selection.go'] and 'applyGlobalObjectSelection(openedKey, true)' in files['laboratory.go'],
    'off_main_warning_full_workflow': all(x in files['laboratory.go'] for x in ['проверить, дополнить или обновить','МРП и отказы','экономику','итоговое заключение']),
    'assessment_repin_action': 'ЗАКРЕПИТЬ И ПЕРЕСЧИТАТЬ' in files['main.go'],
    'teaching_examples_unique_keys': 'Учебный пример %03d · %s' in files['demo_seed.go'] and 'repairBundledDemoIdentity' in files['demo_seed.go'],
    'teaching_tech_context_explicit': 'синтетический техрежим только для тестирования' in files['demo_seed.go'],
    'teaching_mrp_history_not_confirmed': 'ClassificationConfirmed: false' in files['demo_seed.go'],
    'teaching_economics_explicit': 'УЧЕБНЫЙ РЕЖИМ:' in files['workflow_pages.go'],
    'classification_still_uses_open_record': 'labDrafts[groupCode]' in files['main.go'],
    'free_gas_not_faked': 'Лабораторный газовый фактор не заменяет этот показатель' in files['assessment_classification.go'],
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+' | '+k)
print(f"RESULT | {len(checks)-len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
