from pathlib import Path
root=Path(__file__).resolve().parent
files={p.name:p.read_text(encoding='utf-8',errors='ignore') for p in root.glob('*.go')}
main=files['main.go']; ref=files['methods_reference_dialog.go']; mw=files['methods_workflow.go']; wp=files['workflow_pages.go']; bind=files['economics_decision_binding.go']; impl=files['methods_implementation.go']; arch=files['methods_plan_archive.go']; build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8')
checks={
'version 587':'const appBuildVersion = "7.3.11.587-RC"' in main,
'build version 587':'$version = "7.3.11.587-RC"' in build,
'source popup normal caption':'pCreateWindowExW.Call(0,' in ref and '860, 215' in ref,
'source popup no toolwindow':'pCreateWindowExW.Call(0x00000080' not in ref,
'source path wider':'92, 650, 34' in ref,
'copy separated':'686, 92, 132, 34' in ref,
'archive reserve':'archiveReserve := int32(205)' in mw,
'archive readable minimum':'archiveH := max32(150, bottom-archiveY)' in mw,
'economics binding persisted':'economics_decision_links.json' in bind,
'needs recalculation state':'требует пересчёта' in bind,
'current state':'расчёт актуален' in bind,
'approval invalidates economics':'markEconomicsNeedsRecalcForDecision(d, "утверждена новая версия технологического решения")' in mw,
'restore invalidates economics':'восстановлена архивная версия технологического решения' in arch,
'implementation edit invalidates economics':'изменены параметры реализации утверждённого метода' in impl,
'approved forecast transferred':'decision.ForecastBaseDays' in wp and 'methodDecisionStoredVersion(decision)' in wp,
'approved implementation transferred':'economicsApprovedDecisionText(decision)' in wp,
'decision version binding':'econBoundDecisionID' in wp,
'status displayed':'econDecisionStatus' in wp,
'deferral displayed':'Отсрочка ремонта, мес.' in wp,
'deferral calculation':'economicsDeferralMonths' in bind,
'OPEX explicit':'Эксплуатационные затраты (OPEX)' in wp,
'CAPEX explicit':'Капитальные / разовые затраты (CAPEX)' in wp,
'calculation marks current':'markEconomicsCalculatedForCurrentDecision' in wp,
'approved source stated':'Источник сценария: архив решений раздела 4' in bind,
'base oil price non-use transparent':'внешняя цена нефти в базовом режиме не используется' in bind,
'icon build embedding':'embed_icon_resource.py' in build and 'verify_icon_resource.py' in build,
}
for name,ok in checks.items(): print(('PASS' if ok else 'FAIL')+': '+name)
failed=[n for n,o in checks.items() if not o]
print(f'{len(checks)-len(failed)}/{len(checks)} PASS')
if failed: raise SystemExit(1)
