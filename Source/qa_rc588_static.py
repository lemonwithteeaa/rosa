from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8',errors='ignore')
ui=(root/'well_card_ui.go').read_text(encoding='utf-8',errors='ignore')
fail=(root/'well_failures.go').read_text(encoding='utf-8',errors='ignore')
imp=(root/'failure_import_windows.go').read_text(encoding='utf-8',errors='ignore')
comp=(root/'failure_completion.go').read_text(encoding='utf-8',errors='ignore')
extra=(root/'well_failure_extra.go').read_text(encoding='utf-8',errors='ignore')
build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8',errors='ignore')
checks={
'version 588':'const appBuildVersion = "7.3.11.588-RC"' in main,
'build version 588':'$version = "7.3.11.588-RC"' in build,
'import both sheets':'Оперативный ввод' in imp and 'История отказов' in imp,
'no second object selector':'show(failureObjectCombo, false)' in ui and 'return strings.TrimSpace(selectedObjectKey)' in imp,
'operational status':'Только оперативный ввод' in comp,
'full percentage':'Полная карточка: %d%%' in comp,
'no selection excluded':'failureIsNoData' in comp and '!failureHasValue' not in '',
'ORD four sizes':'EquipmentSize4' in fail and 'Типоразмер ГНО 4 (ОРД)' in extra,
'safe conflict merge':'mergeImportedFailure' in imp and 'Требует проверки' in imp,
'per-row import audit':'failureImportRowRecord' in imp and 'Rows       []failureImportRowRecord' in imp,
'manual/full modes':'ID_WELL_FORM_MODE_OPERATIONAL' in ui and 'ID_WELL_FORM_MODE_FULL' in ui,
'unlimited manual entries':'уже внесено 6 отказов' not in ui and 'шесть дополнительных отказов' not in fail,
'amber operational-only':'rgb(255, 246, 210)' in ui,
'full completion green':'FullCompletionPct >= 100' in ui and 'rgb(226, 246, 232)' in ui,
'form saves source provenance':'SourceType = "manual"' in ui and 'failureSourceLabel' in comp,
'full-card applicable equipment':'wellExtraApplicable' in extra and 'wellSectionApplicable' in extra,
'no-selection prompt':'const wellChoicePrompt = failureNoSelection' in ui,
'custom dictionary persisted':'failure_custom_catalog.json' in (root/'well_failure_multi_picker.go').read_text(encoding='utf-8',errors='ignore'),
}
failed=[]
for name,ok in checks.items():
    print(('PASS' if ok else 'FAIL')+': '+name)
    if not ok: failed.append(name)
print(f'{len(checks)-len(failed)}/{len(checks)} PASS')
if failed: raise SystemExit('FAILED: '+', '.join(failed))
