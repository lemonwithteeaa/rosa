from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/"main.go").read_text(encoding="utf-8")
report=(root/"report_workspace.go").read_text(encoding="utf-8")
checks={
"version 706+": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(706, 1000)),
"cards no edit scrollbars": 'func resultCard(' in report and 'create("STATIC"' in report and 'resultSectionCards[i] = resultCard' in report,
"lists are calm": 'applyResultListReferenceStyle(resultVariantsList)' in report and 'LVS_EX_FULLROWSELECT | LVS_EX_DOUBLEBUFFER' in report,
"buttons single line": 'isResultSingleLineButton(h)' in main and 'DT_CENTER | DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX' in main,
"buttons compact font": ('isResultSingleLineButton(h) && hFontButtonCompact != 0' in main) or ('isResultWorkspaceButton(h) && hFontButtonCompact != 0' in main),
"variant controls widened": ('addW, copyW, compareW := int32(184), int32(184), int32(194)' in report) or ('copyW, compareW := int32(236), int32(236)' in report),
"decision controls widened": 'useW, saveW, approveW := int32(154), int32(174), int32(184)' in report,
"active variant accent": 'rgb(255, 241, 244)' in report and 'rgb(145, 0, 24)' in report,
"archive accent": 'func resultArchiveCustomDraw' in report and 'row == 0' in report and 'rgb(238, 244, 249)' in report,
"archive draw hook": 'hdr.HwndFrom == resultArchiveList' in main and 'resultArchiveCustomDraw(lParam)' in main,
"trace folded into decision": 'resultDecisionControlLine' in report and 'archiveTitleY := decisionY + decisionH + 10' in report and 'show(resultTraceCard, false)' in report,
"white card surfaces": 'isResultCardControl(child)' in main and 'return uintptr(brushWhite)' in main,
}
for k,v in checks.items(): print(("PASS" if v else "FAIL")+": "+k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
