from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
s=(root/'report_workspace.go').read_text(encoding='utf-8')
start=s.index('func exportSelectedApprovedRegistryReport(format string)')
end=s.index('func selectedFullReport()', start)
body=s[start:end]
helper_start=s.index('func exportApprovedRegistryReportToChosenPath')
helper_end=s.index('func exportSelectedApprovedRegistryReport', helper_start)
helper=s[helper_start:helper_end]
checks={
 'version':'7.3.11.737-RC' in main,
 'probe_advisory':'approved export destination probe advisory FAIL' in body and 'probeApprovedRegistryExportDestination(path)' in body,
 'probe_does_not_abort':'probeApprovedRegistryExportDestination(path); err != nil' in body and 'msg("Выгрузка утверждённого отчёта", err.Error()' not in body,
 'real_extension_stage':'os.CreateTemp(dir, "ROSA_EXPORT_STAGE-*"+ext)' in helper,
 'stage_same_destination_dir':'dir := filepath.Dir(filepath.Clean(path))' in helper,
 'payload_writer':'writeApprovedRegistryExportPayload(stagePath, format, r, report)' in helper,
 'validated_before_commit':'validateApprovedRegistryExportFile(stagePath, format)' in helper,
 'no_hidden_tmp_atomic_writer':'atomicWriteFile(' not in helper,
 'same_dir_replace':'replaceFileAtomic(stagePath, path)' in helper,
 'direct_fallback':'writeApprovedRegistryDirect(path, data)' in helper,
 'direct_if_stage_blocked':'writeApprovedRegistryExportPayload(path, format, r, report)' in helper,
 'final_validation':'validateApprovedRegistryExportFile(path, format)' in helper,
 'success_after_stat':'Файл сохранён и проверен' in body and 'os.Stat(path)' in body,
 'full_report_content':'approvedRegistryDetailedReport(r)' in body,
 'detailed_xlsx':'writeFullReportSectionXLSX(path, r, true)' in s,
}
for k,v in checks.items(): print(f'{k}: {"PASS" if v else "FAIL"}')
if not all(checks.values()): raise SystemExit(1)
print(f'RC737 gate: {sum(checks.values())}/{len(checks)} PASS')
