from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
  'version 722': 'const appBuildVersion = "7.3.11.722-RC"' in main,
  'dedicated comparison window': 'ROSAResultComparisonV2' in report and 'func resultCompareWndProc' in report,
  'native minimize maximize close': 'WS_OVERLAPPEDWINDOW|WS_VISIBLE' in report and 'case WM_SIZE:' in report and 'case WM_GETMINMAXINFO:' in report and 'case labMappingClose:' in report,
  'responsive layout': 'func resultCompareLayout' in report and 'setPos(resultCompareBody' in report,
  'unclipped compact window caption': 'РОСА — Сравнение вариантов' in report and '"СРАВНЕНИЕ ВАРИАНТОВ"' in report,
  'object subtitle separated from title': 'resultCompareObjectLabel = readableObjectKey(selectedObjectKey)' in report and 'resultCompareSubtitle' in report,
  'rich text heading formatter': 'func resultCompareFormatText' in report and 'CFE_BOLD | cfeItalic' in report,
  'variant headings emphasized': 'resultCompareIsVariantHeading' in report,
  'field labels emphasized': '"Методы:", "Доступность:", "Статус:", "Наработка:", "Benefit Modeling:", "Основание:", "Правило решения:"' in report,
  'body margins and white surface': 'emSetMargins' in report and 'emSetBkColor' in report,
  'tooltip topmost': '0x00000008, // WS_EX_TOPMOST' in report,
  'tooltip explicitly activated': 'ttmActivate' in report and 'ttmSetDelayTime' in report,
  'two tooltip texts preserved': 'Создать копию выбранного варианта' in report and 'Сопоставить варианты по наработке' in report,
  'no methods reference from compare': 'openMethodsReferenceDialog(' not in report[report.index('func openResultScenarioComparison()'):report.index('func resultVariantCustomDraw')],
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+' - '+k)
if failed: raise SystemExit('RC722 comparison QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
