from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
  'version 728 or newer': any(v in main for v in ['7.3.11.728-RC','7.3.11.729-RC']),
  'dedicated evidence font declared': 'hFontReportEvidence' in main,
  'evidence font is 12px regular': 'pCreateFontW.Call(^uintptr(12-1), 0, 0, 0, 400' in main,
  'evidence font cleanup': 'pDeleteObject.Call(uintptr(hFontReportEvidence))' in main,
  'metric cards use evidence font': 'drawReportText(di.HDC, secondaryR, secondary, evidenceFont' in report,
  'secondary bottom reserve': 'Bottom: di.RcItem.Bottom - 8' in report,
  'secondary starts close to primary': 'Top: primaryR.Bottom + 1' in report,
  'compact card threshold': 'if cardH <= 90' in report and 'labelH = 16' in report and 'primaryH = 22' in report,
  'safe min height 12': 'secondaryR.Bottom-secondaryR.Top >= 12' in report,
}
for k,v in checks.items():
    print(('PASS' if v else 'FAIL'), k)
if not all(checks.values()): raise SystemExit(1)
print(f'RC728 report-card no-clip gate {sum(checks.values())}/{len(checks)} PASS')
