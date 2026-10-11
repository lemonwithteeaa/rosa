from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
lab=(root/'laboratory.go').read_text(encoding='utf-8')
dlg=(root/'laboratory_infographic_dialog.go').read_text(encoding='utf-8')
checks={
    'version RC371': 'const appBuildVersion = "7.3.11.371-RC"' in main,
    'accent infographic button': 'ОТКРЫТЬ ИНФОГРАФИКУ ОСЛОЖНЕНИЙ' in lab,
    'main influence hidden': 'show(labInfluenceChart, false)' in lab,
    'main trend hidden': 'show(labTrendChart, false)' in lab,
    'dialog opens': 'openLaboratoryInfographicDialog()' in main,
    'dialog has influence chart': 'drawLaboratoryInfluenceChart(di)' in dlg,
    'dialog has trend chart': 'drawLaboratoryTrendChart(di)' in dlg,
    'dialog has scope selector': 'laboratoryScopeNames' in dlg,
    'dialog has metric selector': 'laboratoryTrendMetrics' in dlg,
    'unified pass assessment': 'repositoryLabPassesForObject(objectKey)' in main,
    'saved pass only': 'pass.Status != labPassSavedStatus' in main,
    'canonical units': 'canonicalizeLaboratoryValues(group, values, units)' in main,
    'assessment actions red': 'assessmentAction :=' in main,
    'assessment tabs red': 'assessmentTab :=' in main,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL')+' | '+name)
if not all(checks.values()): raise SystemExit(1)
