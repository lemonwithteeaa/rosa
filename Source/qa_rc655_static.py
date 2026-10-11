from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 655 or newer': bool(m) and int(m.group(1)) >= 655,
    'object context has no ellipsis clipping style':
        'methodsObjectContext = label(ID_ACTIVITIES' in flow and
        'methodsObjectContext = labelEllipsis' not in flow,
    'context does not repeat field already shown above':
        'fmt.Sprintf("скв. %s · %s", dash(st.Well), dash(st.ObjectPlast))' in flow,
    'old long three-part context removed':
        'fmt.Sprintf("%s / скв. %s / %s", dash(st.Field), dash(st.Well), dash(st.ObjectPlast))' not in flow,
    'object context still uses adaptive font fitting':
        'methodsObjectCode, methodsObjectContext' in (root / 'methods_typography.go').read_text(encoding='utf-8'),
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
