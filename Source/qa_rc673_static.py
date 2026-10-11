from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 673 or newer': bool(m) and int(m.group(1)) >= 673,
    '668 remains the declared base only in release notes':
        '7.3.11.668-RC' in (root / 'ИЗМЕНЕНИЯ_7.3.11.673-RC.md').read_text(encoding='utf-8'),
    'compact geometry extends through 900px work areas':
        'if h < 960' in flow and 'workspaceMaxH = 190' in flow,
    'normal Full-HD geometry keeps enlarged workspace':
        'if h < 1200' in flow and 'workspaceMaxH = 270' in flow,
    'compact workbench keeps only two large cards':
        'if workspaceH < 230' in flow and 'perPage = 2' in flow,
    'candidate title measured with actual body font':
        'methodsMeasuredTextHeight(getText(card), cardW-32, methodsTypeBody, true)' in flow,
    'candidate detail measured with actual compact font':
        'methodsMeasuredTextHeight(getText(methodsCandidateDetails[i]), cardW-12, methodsTypeCompact, false)' in flow,
    'native button safety margin is reserved':
        'titleHeight := max32(44, titleTextHeight+12)' in flow and
        'detailHeight := max32(32, detailTextHeight+10)' in flow,
    'lower compact tables are explicitly budgeted':
        'planReserve = ' in flow and 'archiveReserve = ' in flow,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
