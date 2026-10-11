from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
typo = (root / 'methods_typography.go').read_text(encoding='utf-8')

checks = {
    'version 674': 'const appBuildVersion = "7.3.11.674-RC"' in main,
    'plan title uses common methods header style': 'methodsForecastTitle || child == methodsPlanTitle || child == methodsArchiveTitle' in main,
    'plan title participates in common font fitting': 'methodsForecastTitle, methodsPlanTitle, methodsArchiveTitle' in typo,
    'section brush is neutral not blue': 'rgb(248, 250, 252)' in typo and 'rgb(232, 242, 253)' not in typo[typo.index('func methodsSectionBrush'):typo.index('// Measure using the actual Windows font')],
    'workspace panels are white': 'pCreateSolidBrush.Call(uintptr(rgb(255, 255, 255)))' in typo,
    'compact pinned-object rows have safe height': 'contextHeaderH, metricFrameH = 20, 32' in flow and 'objectCodeH, objectContextH = 20, 18' in flow,
    'compact plan reserves two rows': 'planReserve = 100' in flow and 'planListH = max32(74' in flow and 'planReserve = 90' in flow and 'planListH = max32(68' in flow,
    'real forecast card no longer pale blue': 'if child == methodsForecastGain' in main and 'return uintptr(brushWhite)' in main[main.index('if child == methodsForecastGain'):main.index('if child == methodsForecastResult')],
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
