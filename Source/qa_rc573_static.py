from pathlib import Path

main = Path('main.go').read_text(encoding='utf-8')
methods = Path('methods_workflow.go').read_text(encoding='utf-8')
ref = Path('methods_reference_dialog.go').read_text(encoding='utf-8')

checks = {
    'version 573': 'const appBuildVersion = "7.3.11.573-RC"' in main,
    'four technical metric cards': all(x in methods for x in ['methodsTechEquipment','methodsTechMRP','methodsTechLiquid','methodsTechWatercut']),
    'legacy tech textbox hidden': 'show(methodsKeyData, false)' in methods,
    'classification duplicate table hidden': 'show(methodsClassList, false)' in methods,
    'readiness banner exists': 'methodsReadyBanner' in methods and 'СНАЧАЛА УТВЕРДИТЕ ПОЛНУЮ КЛАССИФИКАЦИЮ' in methods,
    'no empty table state': 'methodsEmptyState' in methods and 'No empty grid' in methods,
    'methods title cannot overlap actions': 'buttonsX-x-12' in methods,
    'working area only shown when ready': '!methodsClassificationReady || len(methodWorkflowSuggestions) == 0' in methods,
    'four forecast cards': all(x in methods for x in ['methodsForecastCurrent','methodsForecastGain','methodsForecastResult','methodsForecastBasis']),
    'legacy forecast textbox hidden': 'show(methodsForecast, false)' in methods,
    'forecast cards refreshed': 'refreshMethodsDashboardCards' in methods and 'combinedMethodForecastForNames' in methods,
    'blue approve action': 'h == methodsApproveButton' in main and 'br = brushNav' in main,
    'quiet reference actions': 'h == methodsReferenceButton || h == methodsReplicationButton' in main,
    'green ready banner': 'child == methodsReadyBanner' in main and 'brushInputValid' in main,
    'warning empty state': 'child == methodsEmptyState' in main and 'brushWarning' in main,
    'reference image retained': Path('Дашборд_методов_и_мероприятий_РЕФЕРЕНС.png').exists(),
    'full-size method card retained': 'WS_OVERLAPPEDWINDOW|WS_VISIBLE' in ref and 'case WM_SIZE:' in ref,
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ' | ' + name)
print(f'RESULT {len(checks)-len(failed)}/{len(checks)}')
raise SystemExit(1 if failed else 0)
