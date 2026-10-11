from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 656 or newer': bool(m) and int(m.group(1)) >= 656,
    'workspace joins navigation and remains inside the right window edge':
        'x := int32(230)' in flow and 'w-242' in flow,
    'section labels use a real inner text inset':
        'sectionTextInset := int32(10)' in flow and
        flow.count('x+sectionTextInset') >= 6,
    'compact object stack controls the following section position':
        'metricBottom := metricY + metricFrameH' in flow and
        'objectBottom := y + contextHeaderH + 4 + objectCodeH + objectContextH' in flow and
        'y = max32(metricBottom, objectBottom) + contextBottomGap' in flow,
    'lower area is shared by the restored plan and visible archive':
        'planReserve := int32(132)' in flow and 'archiveReserve := int32(154)' in flow,
    'compact screens reserve a larger central workspace':
        'pageTitleH, pageTitleAdvance = 30, 32' in flow and
        'workspaceMaxH = 190' in flow and 'workspaceH := max32(120,' in flow,
    'object context remains free from forced ellipsis':
        'methodsObjectContext = label(ID_ACTIVITIES' in flow and
        'methodsObjectContext = labelEllipsis' not in flow,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
