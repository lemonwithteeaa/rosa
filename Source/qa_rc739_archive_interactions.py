from pathlib import Path
import re
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
 'version 744+ uses RC739 archive baseline': bool(re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)) and int(re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M).group(1)) >= 744,
 'dedicated archive export IDs': all(x in report for x in ['ID_ARCHIVE_EXPORT_PDF       = 18771','ID_ARCHIVE_EXPORT_DOCX      = 18772','ID_ARCHIVE_EXPORT_XLSX      = 18773']),
 'async export completion message': 'WM_APP_ARCHIVE_EXPORT_DONE = 0x8007' in main and 'approvedRegistryExportDone()' in main,
 'archive double click consumes event row': 'act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))' in main and 'selectListRow(archiveList, int(act.IItem))' in main,
 'double click still opens report': 'case archiveList:' in main and 'openSelectedApprovedRegistryReport()' in main,
 'open does not enrich on UI thread': 'record, ok := selectedApprovedRegistryReportRaw()' in report[report.index('func openSelectedApprovedRegistryReport'):report.index('func chooseApprovedRegistryExportPath')],
 'export runs in goroutine': 'go func(record FullReportRecord, exportFormat string)' in report,
 'buttons disabled while exporting': 'setApprovedRegistryExportBusy(true, format)' in report and 'pEnableWindow.Call(uintptr(h), 0)' in report,
 'buttons restored after exporting': 'setApprovedRegistryExportBusy(false, "")' in report and 'pEnableWindow.Call(uintptr(h), 1)' in report,
 'direct final-format write': 'writeApprovedRegistryExportDirectValidated' in report and 'writeApprovedRegistryExportPayload(path, format, r, report)' in report,
 'no staging inside RC739 direct writer': 'os.CreateTemp' not in report[report.index('func writeApprovedRegistryExportDirectValidated'):report.index('func runApprovedRegistryExport')],
 'multiple visible writable candidates': all(x in report for x in ['РОСА_Отчёты','Экспорт_отчётов','os.TempDir()']),
 'final file validated': 'validateApprovedRegistryExportFile(path, format)' in report[report.index('func writeApprovedRegistryExportDirectValidated'):report.index('func runApprovedRegistryExport')],
 'explorer reveals created file': 'explorer.exe' in report and '/select,"' in report,
 'worker panic is converted to an export error': 'runApprovedRegistryExportSafely' in report and 'recover()' in report and 'Приложение продолжает работу' in report,
}
# Detect duplicate numeric command IDs for the new archive range.
ids={}
for p in root.glob('*.go'):
    text=p.read_text(encoding='utf-8')
    for m in re.finditer(r'\b(ID_[A-Z0-9_]+)\s*=\s*(\d+)', text):
        ids.setdefault(int(m.group(2)), []).append((m.group(1),p.name))
checks['archive export IDs globally unique'] = all(len(ids.get(n,[]))==1 for n in (18771,18772,18773))
failed=[]
for name,ok in checks.items():
    print(('PASS' if ok else 'FAIL')+': '+name)
    if not ok: failed.append(name)
print(f'{len(checks)-len(failed)}/{len(checks)} PASS')
if failed: raise SystemExit(1)
