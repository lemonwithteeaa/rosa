from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
 'version 708+': any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(708,800)),
 'selected measures button id': 'ID_RESULT_SELECTED_INFO' in report and '= 9770' in report[report.index('ID_RESULT_SELECTED_INFO'):report.index('ID_RESULT_SELECTED_INFO')+80],
 'requested caption': 'ИНФОРМАЦИЯ ПО ВЫБРАННЫМ МЕРОПРИЯТИЯМ' in report,
 'selected information action': 'func resultSelectedMeasuresInfoText() string' in report and 'openResultSelectedMeasuresInfo()' in report,
 'simplified benefit modeling only': 'Упрощённый Benefit Modeling' in report,
 'four decision actions same style': 'resultDecisionAction := h == resultSelectedInfoButton || h == resultUseSelectedButton || h == resultSaveButton || h == resultApproveButton' in main and 'br = brushNav' in main,
 'right three geometry retained': 'useW, saveW, approveW := int32(154), int32(174), int32(184)' in report,
 'info occupies decision free area': 'decisionRegionW := max32(420, contentW-decisionButtonsW-14)' in report and 'decisionTextW := decisionRegionW - infoW - infoGap' in report,
 'long info caption can wrap': 'resultSelectedInfoButton' in report and 'resultSelectedInfoButton' not in report[report.index('func isResultSingleLineButton'):report.index('func applyResultListReferenceStyle')],
 'legacy action captions still centered': 'isResultSingleLineButton(h)' in main and 'DT_CENTER | DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX' in main,
 'owner draw stability retained': 'func drawResultCard' in report and 'drawResultCard(di)' in main,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit(f'{len(failed)}/{len(checks)} failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
