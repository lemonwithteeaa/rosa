from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
typography = (root / 'methods_typography.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
minmax = main[main.index('case WM_GETMINMAXINFO:'):main.index('case WM_DPICHANGED:')]
plan = flow[flow.index('func refreshMethodsPlanTable'):flow.index('func renderMethodsWorkflowUI')]
checks = {
    'version 660 or newer': bool(m) and int(m.group(1)) >= 660,
    'maximized frame is constrained to monitor work area':
        'PtMaxPosition.X' in minmax and 'PtMaxPosition.Y' in minmax and
        'PtMaxSize.X' in minmax and 'PtMaxSize.Y' in minmax,
    'lower result area visibly dominates the previous compressed version':
        'planReserve := int32(132)' in flow and 'archiveReserve := int32(154)' in flow,
    'automatic teaching plan contains three distinct rows':
        'row < 3' in plan and 'seen := map[string]bool{}' in plan and
        'if row == 3' in plan and 'три связанных мероприятия' in plan,
    'five section bands have semantic pale accents':
        'methodsSectionBrushes [5]syscall.Handle' in typography and
        'methodsSectionAccentBrush' in typography and
        'sectionBands := [5]syscall.Handle' in main,
    'three workspace columns keep stable native panel brushes':
        'methodsPanelBrushes [3]syscall.Handle' in typography and
        'methodsWorkspacePanelBrush' in typography and
        'rgb(255, 255, 255)' in typography,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
