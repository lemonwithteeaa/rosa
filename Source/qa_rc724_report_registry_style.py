from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
 'RC724+ report styling retained': 'func drawReportMetricCard' in report,
 'section7 metric renderer': 'func drawReportMetricCard' in report and 'resultMetricCardIndex' in report,
 'reference red ordinal': 'rgb(203, 27, 43)' in report and 'ordinal' in report,
 'section7 hierarchy subtitles': 'resultVariantsSubtitle' in report and 'resultArchiveSubtitle' in report,
 'full report archive caption strengthened': 'Ранее сохранённые полные отчёты' in report,
 'evidence cards use value hierarchy': all(x in report for x in ['01  ОТКАЗЫ','04  МЕТОДЫ','05  НАРАБОТКА','06  ЭКОНОМИКА']),
 'section8 workspace builder': 'func buildApprovedRegistryUI()' in report and 'buildApprovedRegistryUI()' in main,
 'section8 four summary cards': '[4]syscall.Handle' in report and 'archiveRegistryStats' in report,
 'section8 approved-only banner': 'РЕЕСТР УТВЕРЖДЁННЫХ РЕШЕНИЙ' in report and 'Черновики и сохранённые варианты остаются в разделе 7' in report,
 'section8 target statistic': 'ЦЕЛЬ ≥ 1000 СУТ.' in report and 'ForecastDays >= 1000' in report,
 'section8 benefit statistic': 'ЧИСТАЯ ВЫГОДА' in report and 'totalBenefit' in report,
 'section8 terminology variant': '{"Вариант", 120}' in report,
 'section8 calm list': 'applyResultListReferenceStyle(archiveList)' in report,
 'section8 row styling': 'func approvedRegistryCustomDraw' in report and 'hdr.HwndFrom == archiveList' in main,
 'section8 responsive layout': 'func layoutApprovedRegistryWorkspace' in report and 'layoutApprovedRegistryWorkspace(w, h)' in main,
 'section8 summary refresh': 'refreshApprovedRegistrySummary(approvedRegistryRows)' in main,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
if failed: raise SystemExit('RC724 report/registry style QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
