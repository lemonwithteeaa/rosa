from pathlib import Path
root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
report = (root/'report_workspace.go').read_text(encoding='utf-8')
checks = {
    'RC721 baseline retained': '// RC721 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.721-RC"' in main,
    'legacy add button removed': 'resultAddVariantButton = button(' not in report and 'resultAddVariantButton = 0' in report,
    'duplicate caption': '"ДУБЛИРОВАТЬ ВАРИАНТ"' in report,
    'compare caption retained': '"СРАВНИТЬ ВАРИАНТЫ"' in report,
    'two equal action widths': 'copyW, compareW := int32(236), int32(236)' in report,
    'duplicate hover text': 'Создать копию выбранного варианта для изменения методов, прогноза или экономического сценария.' in report,
    'compare hover text': 'Сопоставить варианты по наработке, CAPEX/OPEX, чистой выгоде и основанию выбора.' in report,
    'tooltips registered': 'TTM_ADDTOOLW' in report and 'TTF_IDISHWND | TTF_SUBCLASS' in report,
    'duplicate selected row': 'func duplicateSelectedResultScenario() error' in report and 'selectedResultScenario()' in report and 'setAlternativeScenarioActiveByID(key, selected.ID)' in report,
    'no add command path': 'case ID_RESULT_ADD_SCHEME:' not in report,
    'comparison no longer opens methods reference': 'func openResultScenarioComparison()' in report and 'openMethodsReferenceDialog(' not in report[report.index('func openResultScenarioComparison()'):report.index('func resultVariantCustomDraw')],
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+' - '+k)
if failed: raise SystemExit('RC721 variants QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
