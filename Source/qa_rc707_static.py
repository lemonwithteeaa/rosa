from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
 'version 707+': any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(707,800)),
 'owner draw cards': 'WS_CHILD|SS_OWNERDRAW|SS_NOPREFIX' in report and 'func drawResultCard' in report and 'drawResultCard(di)' in main,
 'object local red accent': 'di.HwndItem == resultObjectCard' in report and 'accent = brushHeader' in report,
 'decision local navy accent': 'di.HwndItem == resultDecisionCard' in report and 'accent = brushNav' in report,
 'six evidence cards': 'resultSectionCards' in report and '[6]syscall.Handle' in report,
 'single line action captions': 'DT_CENTER | DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX' in main and 'isResultSingleLineButton(h)' in main,
 'active scheme accent': 'rgb(255, 241, 244)' in report and 'rgb(145, 0, 24)' in report,
 'archive soft accent': 'rgb(238, 244, 249)' in report and 'func resultArchiveCustomDraw' in report,
 'no permanent list grid': 'LVS_EX_FULLROWSELECT | LVS_EX_DOUBLEBUFFER' in report,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit(f'{len(failed)}/{len(checks)} failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
