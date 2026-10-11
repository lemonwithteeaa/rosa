from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
typography = (root / 'methods_typography.go').read_text(encoding='utf-8')
forecast = (root / 'methods_forecast_details.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 653 or newer': bool(m) and int(m.group(1)) >= 653,
    'methods typography visibly enlarged': all(token in typography for token in (
        'methodsTypeDisplay = 26', 'methodsTypeSection = 19',
        'methodsTypeBody    = 16', 'methodsTypeCompact = 14')),
    'page title receives larger vertical space': ('pageTitleH, pageTitleAdvance := int32(32), int32(40)' in flow or
        'pageTitleH, pageTitleAdvance := int32(34), int32(42)' in flow) and
        ('setPos(methodsPageTitle, x, top, contentW, pageTitleH)' in flow or
         'setPos(methodsPageTitle, x, top, titleW, pageTitleH)' in flow),
    'context header is taller and continuous': 'contextHeaderH := int32(22)' in flow and
        'setPos(methodsContextHeaderBand, x, y, contentW, contextHeaderH)' in flow,
    'object rows have independent readable heights': 'objectCodeH, objectContextH := int32(24), int32(20)' in flow and
        'y+contextHeaderH+4+objectCodeH' in flow,
    'four technical cards are taller': 'metricFrameH := int32(44)' in flow,
    'metric caption and value cannot overlap': 'metricCaptionH := int32(18)' in flow and
        'metricY+metricCaptionH+2' in flow and 'metricValueH := int32(21)' in flow,
    'technical card gutters are increased': 'metricGap := int32(10)' in flow and
        'metricXs[i]+5' in flow,
    'forecast cards are enlarged': 'int32(60), int32(28)' in flow and
        'forecastCardH, forecastNoteH = 36, 16' in flow,
    'forecast title has more height': 'forecastTitleH, forecastTitleAdvance := int32(22), int32(24)' in flow and
        'setPos(methodsForecastTitle, x+sectionTextInset, y, contentW-sectionTextInset*2, forecastTitleH)' in flow,
    'short screens retain compact no-clipping geometry': 'if h < 960' in flow and
        ('contextHeaderH, metricFrameH = 16, 30' in flow or 'contextHeaderH, metricFrameH = 20, 32' in flow) and
        'workspaceMaxH = 190' in flow and 'workspaceH := max32(120,' in flow,
    'whole-line forecast emphasis remains enforced': 'wholeHeading :=' in forecast and
        'offset+trimLen' in forecast,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
