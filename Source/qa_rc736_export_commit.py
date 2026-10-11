from pathlib import Path
root=Path(__file__).resolve().parent
s=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
 'version':'const appBuildVersion = "7.3.11.' in (root/'main.go').read_text(encoding='utf-8'),
 'destination_probe':'probeApprovedRegistryExportDestination' in s and 'os.CreateTemp(dir, ".rosa-export-check-*.tmp")' in s,
 'generate_temp':'writeApprovedRegistryExportTemp' in s and 'rosa-approved-report-*' in s,
 'validate_temp':'validateApprovedRegistryExportFile(tmp, format)' in s,
 'atomic_commit':'atomicWriteFile(finalPath, data, 0o644)' in s,
 'direct_fallback':'writeApprovedRegistryDirect' in s and 'os.O_TRUNC' in s,
 'validate_final':'validateApprovedRegistryExportFile(finalPath, format)' in s,
 'docx_structure':'word/document.xml' in s,
 'xlsx_structure':'xl/workbook.xml' in s,
 'pdf_signature':'%PDF-' in s,
 'success_after_stat':'Файл сохранён и проверен' in s and 'os.Stat(path)' in s,
 'diagnostics':'approved export START' in s and 'approved export OK' in s and 'approved export COMMIT FAIL' in s,
}
for k,v in checks.items(): print(f'{k}: {"PASS" if v else "FAIL"}')
if not all(checks.values()): raise SystemExit(1)
print(f'RC736 gate: {sum(checks.values())}/{len(checks)} PASS')
