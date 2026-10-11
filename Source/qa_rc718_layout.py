from pathlib import Path
root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
report = (root / 'report_workspace.go').read_text(encoding='utf-8')
checks = {
    'version 718': 'const appBuildVersion = "7.3.11.718-RC"' in main,
    'result captions lifted by 2px': 'if isResultRaisedCaptionButton(h)' in main and 'textRect.Top -= 2' in main and 'textRect.Bottom -= 2' in main,
    'upper scenario buttons included': all(x in report for x in ['resultAddVariantButton, resultCopyVariantButton, resultCompareButton', 'func isResultRaisedCaptionButton']),
    'lower decision buttons included': all(x in report for x in ['resultSelectedInfoButton, resultUseSelectedButton, resultApproveButton']),
    'export buttons included': all(x in report for x in ['resultOpenButton, resultExportPDFButton, resultExportDOCXButton, resultExportXLSXButton']),
    'hint centered over decision actions': 'WS_CHILD|SS_CENTER|SS_CENTERIMAGE|SS_NOPREFIX' in report and 'setPos(resultDecisionHint, actionsX, decisionY, actionsW, 16)' in report,
    'object operational values red': 'object identity, GNO value and current MRP are red' in report and 'seg{detail[:gnoAt], red}' in report and 'seg{detail[mrpValueStart:sutAt], red}' in report,
    'archive title gets dedicated row': 'archiveTitleY := decisionY + decisionCardH + 4' in report and 'setPos(resultArchiveTitle, x, archiveTitleY, decisionTextW, archiveButtonH)' in report,
    'archive may give up one row': 'max32(120, bottom-archiveY)' in report,
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ' - ' + name)
if failed:
    raise SystemExit('RC718 layout QA failed: ' + ', '.join(failed))
