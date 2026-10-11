from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
reference = (root / 'methods_reference_dialog.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
dpi = main[main.index('case WM_DPICHANGED:'):main.index('case WM_DISPLAYCHANGE:')]
anchor = main[main.index('func anchorMainWindowToMonitor'):main.index('type INITCOMMONCONTROLSEX')]
formatter = reference[reference.index('func formatMethodsReferenceText'):reference.index('func methodsReferenceCandidateFromCatalog')]
checks = {
    'version 658 or newer': bool(m) and int(m.group(1)) >= 658,
    'methods workspace touches the navigation rail':
        'x := int32(230)' in flow and 'contentW := max32(804, w-242)' in flow,
    'DPI handler rejects stale suggested coordinates':
        'pSetWindowPos.Call' not in dpi and 'anchorMainWindowToMonitor(hwnd)' in dpi and
        'pMonitorFromWindow' in anchor and 'RcWork' in anchor,
    'DPI handler reapplies both embedded icon sizes':
        'ICON_BIG' in dpi and 'ICON_SMALL' in dpi and 'loadEmbeddedAppIcons()' in dpi,
    'reference positions count RichEdit paragraph breaks once':
        'strings.ReplaceAll(s, "\\r\\n", "\\n")' in formatter and
        'if i < len(lines)-1' in formatter and 'offset++' in formatter,
    'reference headings and exact field labels receive distinct emphasis':
        'upperHeading := t == strings.ToUpper(t)' in formatter and
        'upperHeading && !schemeHeading' in formatter and
        'fieldLabels := []string{' in formatter and
        'DwEffects: CFE_BOLD' in formatter and
        'lineStart+trimLen' in formatter,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
