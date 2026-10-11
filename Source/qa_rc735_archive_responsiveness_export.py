from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
ws = (root / "report_workspace.go").read_text(encoding="utf-8")
teach = (root / "teaching_alternative_scenarios.go").read_text(encoding="utf-8")
checks = {
    "version": '7.3.11.735-RC' in main,
    "no_heavy_migration_on_page_open": 'do NOT rebuild detailed section snapshots while a page is' in teach and 'fullReports[i].Sections = buildFullReportSectionSnapshot(record.ObjectKey' not in teach,
    "new_teaching_rows_defer_sections": 'SnapshotVersion: ""' in teach and 'Sections:        nil' in teach,
    "raw_selection_helper": 'func selectedApprovedRegistryReportRaw()' in ws,
    "save_dialog_before_enrichment": ws.find('path := chooseApprovedRegistryExportPath(r, format)') < ws.find('r = fullReportRecordForExport(r)'),
    "fallback_first_row": 'row = 0' in ws and 'selectListRow(archiveList, 0)' in ws,
    "persist_lazy_snapshot": 'func persistEnrichedTeachingFullReport' in ws,
    "export_pdf_handler": 'case ID_ARCHIVE_EXPORT_PDF:' in ws and 'exportSelectedApprovedRegistryReport("pdf")' in ws,
    "export_docx_handler": 'case ID_ARCHIVE_EXPORT_DOCX:' in ws and 'exportSelectedApprovedRegistryReport("docx")' in ws,
    "export_xlsx_handler": 'case ID_ARCHIVE_EXPORT_XLSX:' in ws and 'exportSelectedApprovedRegistryReport("xlsx")' in ws,
    "archive_autoselect": 'if len(approvedRegistryRows) > 0' in main and 'selectListRow(archiveList, 0)' in main,
}
failed = [k for k,v in checks.items() if not v]
for k,v in checks.items():
    print(('PASS' if v else 'FAIL'), k)
if failed:
    raise SystemExit('FAILED: ' + ', '.join(failed))
print(f'RC735 gate: {len(checks)}/{len(checks)} PASS')
