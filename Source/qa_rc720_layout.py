from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
report = (root / 'report_workspace.go').read_text(encoding='utf-8')

family = [
    'resultRefreshButton',
    'resultAddVariantButton', 'resultCopyVariantButton', 'resultCompareButton',
    'resultSelectedInfoButton', 'resultUseSelectedButton', 'resultApproveButton',
    'resultOpenButton', 'resultExportPDFButton', 'resultExportDOCXButton', 'resultExportXLSXButton',
]
checks = {
    'version 720': 'const appBuildVersion = "7.3.11.720-RC"' in main,
    'single result button family helper': 'func isResultWorkspaceButton' in report and all(x in report for x in family),
    'one compact caption font for section 7': 'if isResultWorkspaceButton(h) && hFontButtonCompact != 0' in main and 'drawFont = hFontButtonCompact' in main,
    'single safe caption inset': 'if isResultWorkspaceButton(h)' in main and 'textRect.Top += 3' in main and 'textRect.Bottom -= 3' in main,
    'one optical lift': 'func resultButtonOpticalLift' in report and 'return 2' in report and 'if lift := resultButtonOpticalLift(h); lift != 0' in main,
    'old per-row lift removed': 'func isResultRaisedCaptionButton' not in report and 'func isResultVariantCaptionButton' not in report,
    'single-line result buttons not double centered': '!isResultSingleLineButton(h)' in main,
    'approve remains wrapped': 'resultApproveButton' in report and 'resultApproveButton' not in report.split('func isResultSingleLineButton',1)[1].split('}',1)[0],
    'hint alignment preserved': 'setPos(resultDecisionHint, actionsX, decisionY, actionsW, 16)' in report,
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ' - ' + name)
if failed:
    raise SystemExit('RC720 layout QA failed: ' + ', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
