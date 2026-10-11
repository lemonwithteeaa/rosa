from pathlib import Path
root=Path(__file__).resolve().parent
report=(root/'report_workspace.go').read_text(encoding='utf-8')
methods=(root/'methods_workflow.go').read_text(encoding='utf-8')
main=(root/'main.go').read_text(encoding='utf-8')
block=report[report.index('func registryMetricCard'):report.index('func resultInlineTextWidth')]
handler=report[report.index('func handleResultWorkspaceCommand'):report.index('func layoutResultWorkspace')]
checks={
 'version 732': 'const appBuildVersion = "7.3.11.732-RC"' in main,
 'archive cards informational': 'SS_NOTIFY' not in block and 'hMain, 0)' in block,
 'archive cards keep owner draw': 'SS_OWNERDRAW|SS_NOPREFIX' in block,
 'no archive click dispatch': 'id >= ID_ARCHIVE_METRIC_CARD_BASE' not in handler,
 'no archive chevron': 'archive && archiveMetricCardIndex(di.HwndItem) >= 0' not in report,
 'section7 cards still clickable': 'SS_OWNERDRAW|SS_NOPREFIX|SS_NOTIFY' in report[report.index('func resultSectionCard'):report.index('func resultSectionCardPage')],
 'archive large value retained': 'if archive {\n\t\tprimaryFont = hFontSection' in report,
 'archive large evidence retained': 'if archive && hFontHeaderSmall != 0' in report,
 'availability wider retained': 'availabilityW := min32(220, max32(184' in methods,
 'availability compact caption retained': 'h == methodsAvailabilityButton && hFontButtonCompact != 0' in main,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL'), '-', k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit('FAIL: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
