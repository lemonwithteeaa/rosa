from pathlib import Path
main=Path('main.go').read_text(encoding='utf-8')
dlg=Path('outcome_details_dialog.go').read_text(encoding='utf-8')
workflow=Path('workflow_pages.go').read_text(encoding='utf-8')
checks={
'version RC372':'const appBuildVersion = "7.3.11.372-RC"' in main,
'outcome detail button':'ID_OUTCOME_DETAILS' in main and 'Причина / дефектовка / эффект' in main,
'failure cause field':'FailureCause' in main and 'Причина завершения цикла / отказа' in dlg,
'defect finding field':'DefectFinding' in main and 'Дефектовка / фактически выявленное состояние оборудования' in dlg,
'evidence source':'EvidenceSource' in main and 'Источник подтверждения' in dlg,
'next cycle':'NextCycleStartDate' in main and 'Следующий эксплуатационный цикл' in dlg,
'observed effect':'ObservedEffect' in main and 'Наблюдаемое изменение после мероприятия' in dlg,
'causal separation':'причинность не подтверждена' in dlg and 'не интерпретируется РОСА как причинный эффект автоматически' in workflow,
'preserve structured data':'preserveOutcomeDetails(&o, outcomeFor(v.VersionID))' in main,
'result report includes fact':'ФАКТИЧЕСКИЙ РЕЗУЛЬТАТ И ДЕФЕКТОВКА' in workflow,
}
for name,ok in checks.items():
    print(('PASS' if ok else 'FAIL')+' | '+name)
if not all(checks.values()):
    raise SystemExit(1)
