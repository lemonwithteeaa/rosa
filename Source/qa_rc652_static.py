from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
panels = (root / 'methods_visual_panels.go').read_text(encoding='utf-8')
typography = (root / 'methods_typography.go').read_text(encoding='utf-8')
forecast = (root / 'methods_forecast_details.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 652 or newer': bool(m) and int(m.group(1)) >= 652,
    'four independent metric frames': 'methodsTechMetricFrames [4]syscall.Handle' in panels,
    'metric labels are centered both ways': 'methodsCenteredMetricLabel' in flow and 'SS_CENTER|SS_CENTERIMAGE' in flow,
    'metric values use larger shared token': 'methodsFont(methodsTypeSection, true)' in flow and 'fitMethodsControlText(handle, methodsTypeSection, true' in typography,
    'single continuous context header band': 'methodsContextHeaderBand' in panels and 'setPos(methodsContextHeaderBand, x, y, contentW, contextHeaderH)' in flow,
    'metric label and value geometry do not overlap': 'metricY+3' in flow and 'metricY+metricCaptionH+2' in flow,
    'forecast formatter resets complete document': 'func formatMethodsForecastDetailText' in forecast and 'EM_SETSEL, 0, ^uintptr(0)' in forecast,
    'forecast formatter bolds only whole headings': 'wholeHeading :=' in forecast and 'offset+trimLen' in forecast,
    'legacy fragment formatter is not used': 'formatFailureReportText(methodsForecastDetailEdit' not in forecast,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
