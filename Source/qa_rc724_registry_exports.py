from pathlib import Path
root=Path(__file__).resolve().parent
report=(root/'report_workspace.go').read_text(encoding='utf-8')
workflow=(root/'workflow_pages.go').read_text(encoding='utf-8')
export=(root/'laboratory_report_export.go').read_text(encoding='utf-8')
checks={
 'three registry export buttons': all(x in report for x in ['СКАЧАТЬ PDF','СКАЧАТЬ WORD','СКАЧАТЬ EXCEL']),
 'registry exports use selected approved record': 'func selectedApprovedRegistryReport()' in report and 'func exportSelectedApprovedRegistryReport' in report,
 'registry rows open full report': 'openSelectedApprovedRegistryReport()' in report and 'approvedRegistryDetailedReport(record)' in report,
 'dedicated report viewer': 'openResultTextViewer("РОСА — Утверждённый отчёт"' in report,
 'all sections 1-7 exported': all(f'РАЗДЕЛ {i}.' in report for i in range(1,8)),
 'calculations preserved as historical snapshot': 'ПРИЛОЖЕНИЕ. РАСЧЁТНЫЕ ДАННЫЕ НА МОМЕНТ УТВЕРЖДЕНИЯ' in report and 'b.WriteString(snapshot)' in report,
 'pdf export wired': 'ID_ARCHIVE_EXPORT_PDF' in report and 'exportSelectedApprovedRegistryReport("pdf")' in report,
 'word export wired': 'ID_ARCHIVE_EXPORT_DOCX' in report and 'exportSelectedApprovedRegistryReport("docx")' in report,
 'excel export wired': 'ID_ARCHIVE_EXPORT_XLSX' in report and 'exportSelectedApprovedRegistryReport("xlsx")' in report,
 'archive page handles commands': 'currentPage == ID_ARCHIVE' in workflow,
 'numbered sections styled in office exports': 'strings.HasPrefix(upper, "РАЗДЕЛ ")' in export,
 'selection hint describes click and exports': 'двойной щелчок — открыть отчёт' in report and 'выгрузка — по выбранной записи' in report,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
if failed: raise SystemExit('RC724 registry export QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
