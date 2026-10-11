from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 654 or newer': bool(m) and int(m.group(1)) >= 654,
    'methods content joins navigation and keeps the right safety edge':
        'x := int32(230)' in flow and 'w-242' in flow,
    'upper title is proportionally reduced': all(token in flow for token in (
        'pageTitleH, pageTitleAdvance := int32(34), int32(42)',
        'pageTitleH, pageTitleAdvance = 32, 34',
        'pageTitleH, pageTitleAdvance = 30, 32')),
    'upper context block is proportionally reduced': all(token in flow for token in (
        'contextHeaderH := int32(22)', 'metricFrameH := int32(44)',
        'metricCaptionH := int32(18)', 'metricValueH := int32(21)')),
    'forecast block is compacted': 'forecastCardH, forecastNoteH := int32(60), int32(28)' in flow,
    'lower workflow receives explicit plan and archive reserves':
        'planReserve := int32(132)' in flow and 'archiveReserve := int32(154)' in flow,
    'methods workspace grows while reserving lower sections': 'workspaceMaxH := int32(300)' in flow and
        'workspaceAvailable :=' in flow and 'workspaceH := max32(120, min32(workspaceMaxH,' in flow,
    'short layouts retain both lower sections': 'planReserve = 100' in flow and 'archiveReserve = 70' in flow,
    'DPI changes re-anchor the maximized workspace': 'case WM_DPICHANGED:' in main and
        'anchorMainWindowToMonitor(hwnd)' in main and 'pMonitorFromWindow' in main,
    'display changes clear off-screen geometry': 'case WM_DISPLAYCHANGE:' in main and
        'anchorMainWindowToMonitor(hwnd)' in main,
    'restored minimum fits common work area': 'mmi.PtMinTrackSize.X = 1180' in main and
        'mmi.PtMinTrackSize.Y = 700' in main,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
