from pathlib import Path

main = Path('main.go').read_text(encoding='utf-8')
methods = Path('methods_workflow.go').read_text(encoding='utf-8')
assessment = Path('assessment_classification.go').read_text(encoding='utf-8')
ref = Path('Дашборд_методов_и_мероприятий_РЕФЕРЕНС.png')

checks = {
    'version 574': 'const appBuildVersion = "7.3.11.575-RC"' in main,
    'approved reference retained': ref.exists(),
    'four technical cards retained': all(x in methods for x in ['methodsTechEquipment','methodsTechMRP','methodsTechLiquid','methodsTechWatercut']),
    'technical cards are one horizontal row': 'metricW := (techW - metricGap*3) / 4' in methods and '(metricW+metricGap)*3' in methods,
    'compact classification card': 'methodsClassSummary = methodsCardLabel' in methods and 'show(methodsClassList, false)' in methods,
    'no empty methods grid': '!methodsClassificationReady || len(methodWorkflowSuggestions) == 0' in methods and 'show(methodsEmptyState, true)' in methods,
    'three-zone working area': 'contentW*54/100' in methods and 'contentW*23/100' in methods,
    'method list remains interactive': 'openMethodsCandidateReference' in methods,
    'explicit add action retained': '"+ ДОБАВИТЬ"' in methods,
    'four forecast cards retained': all(x in methods for x in ['methodsForecastCurrent','methodsForecastGain','methodsForecastResult','methodsForecastBasis']),
    'archive minimum height increased': 'archiveH := max32(96, bottom-archiveY)' in methods,
    'dashboard journal gets more height': 'maxObjectH := max32(180, remaining-214)' in main,
    'off-page assessment layout blocked': 'if currentPage == ID_ASSESS' in main and 'if currentPage != ID_ASSESS' in assessment,
    'classification summary has quiet card style': 'child == methodsClassSummary' in main and 'return uintptr(brushWhite)' in main,
    'red/blue action hierarchy retained': 'h == methodsApproveButton' in main and 'br = brushNav' in main and 'ID_GEN_PLAN' in main,
    'global empty header guard retained': 'headerText = "Данные"' in main,
    'global blank row guard retained': 'never render anonymous blank rows' in main,
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ' | ' + name)
print(f'RESULT {len(checks)-len(failed)}/{len(checks)}')
raise SystemExit(1 if failed else 0)
