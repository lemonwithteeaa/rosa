from pathlib import Path
root=Path(__file__).resolve().parent
report=(root/'report_workspace.go').read_text(encoding='utf-8')
methods=(root/'methods_workflow.go').read_text(encoding='utf-8')
main=(root/'main.go').read_text(encoding='utf-8')
checks={
 'version 731': 'const appBuildVersion = "7.3.11.731-RC"' in main,
 'archive cards clickable ids': 'ID_ARCHIVE_METRIC_CARD_BASE = 9910' in report and 'registryMetricCard("—", i' in report,
 'archive click dispatch': 'id >= ID_ARCHIVE_METRIC_CARD_BASE' in report and 'switchPage(ID_MRP)' in report and 'switchPage(ID_ECON)' in report,
 'archive larger caption': 'if archive && hFontBold != 0' in report,
 'archive larger value': 'if archive {\n\t\tprimaryFont = hFontSection' in report,
 'archive larger evidence': 'if archive && hFontHeaderSmall != 0' in report,
 'archive chevron': 'archive && archiveMetricCardIndex(di.HwndItem) >= 0' in report,
 'availability wider': 'availabilityW := min32(220, max32(184' in methods,
 'availability compact caption font': 'h == methodsAvailabilityButton && hFontButtonCompact != 0' in main,
 'availability taller row': 'pageTitleH, pageTitleAdvance = 30, 32' in methods,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL'), '-', k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit('FAIL: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
