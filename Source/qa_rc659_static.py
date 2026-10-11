from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
auth = (root / 'auth.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
archive = (root / 'methods_plan_archive.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
visual = flow[flow.index('func refreshMethodsVisualTables'):flow.index('func methodsTeachingPreviewKey')]
layout_start = flow.index('exportGap := int32(8)')
layout = flow[layout_start:flow.index('layoutMethodsSectionBands(', layout_start)]
checks = {
    'version 659 or newer': bool(m) and int(m.group(1)) >= 659,
    'password caption is centred over the input only':
        '550, passwordEditW, labelH' in auth,
    'window is anchored to the nearest monitor work area':
        'pMonitorFromWindow' in main and 'pGetMonitorInfoW' in main and
        'mi.RcWork.Right-mi.RcWork.Left' in main,
    'plan refreshes on ordinary Methods rendering':
        'refreshMethodsPlanTable(methodsClassificationReady)' in visual,
    'teaching examples have a visible fallback route':
        'func methodsTeachingPreviewKey' in flow and
        'methodsTeachingPreviewKey(key)' in flow and
        'teachingCrossHistoryPoints(methodsTeachingPreviewKey(key)' in flow and
        'teachingMethodArchiveDecisions(objectKey, time.Now())' in archive,
    'export buttons share one row and cannot overlap archive header':
        'archiveHeaderH := int32(24)' in layout and
        ('archiveHeaderH = 16' in layout or 'archiveHeaderH = 14' in layout) and
        'methodsExportPDF, exportsX, y, exportW, archiveHeaderH' in layout and
        'archiveY := y + archiveHeaderH + archiveHeaderGap' in layout,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
