from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
panels = (root / 'methods_visual_panels.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 650 or newer': bool(m) and int(m.group(1)) >= 650,
    'three aligned workspace columns': 'layoutMethodsWorkspacePanels(x, midX, rightX' in flow,
    'reference proportions': (('leftW := contentW * 50 / 100' in flow and 'midW := contentW * 24 / 100' in flow) or ('leftW := contentW * 44 / 100' in flow and 'midW := contentW * 25 / 100' in flow and 'if rightW < 270' in flow)),
    'implementation wording matches reference': '03 · ПАРАМЕТРЫ РЕАЛИЗАЦИИ' in flow,
    'five framed reference bands': all(name in panels for name in [
        'methodsContextBand', 'methodsClassificationBand', 'methodsActionBand',
        'methodsForecastBand', 'methodsArchiveBand']),
    'section bands are laid out': 'layoutMethodsSectionBands(' in flow,
    'stacked right workspace removed from active layout': 'layoutMethodsWorkspacePanelsStacked(x, rightX' not in flow,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
