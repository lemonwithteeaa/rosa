from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
panels = (root / 'methods_visual_panels.go').read_text(encoding='utf-8')
forecast = (root / 'methods_forecast_details.go').read_text(encoding='utf-8')
archive = (root / 'methods_plan_archive.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 651 or newer': bool(m) and int(m.group(1)) >= 651,
    'four technical values are separate': all(name in flow for name in [
        'methodsTechEquipmentValue', 'methodsTechMRPValue',
        'methodsTechLiquidValue', 'methodsTechWatercutValue']),
    'technical values are red': 'rgb(190, 24, 49)' in main and 'methodsTechEquipmentValue' in main,
    'classification rows are centered vertically': 'SS_CENTERIMAGE' in panels,
    'workspace joins navigation and text keeps an inner inset':
        'x := int32(230)' in flow and 'sectionTextInset := int32(10)' in flow,
    'forecast help uses program header': all(name in forecast for name in [
        'methodsForecastDetailHeaderBand', 'methodsForecastDetailAccent',
        'layoutMethodsForecastDetail', 'WM_GETMINMAXINFO']),
    'forecast help wraps and has margins': 'emSetMargins' in forecast and 'WS_HSCROLL' not in forecast,
    'archive preview uses program header': all(name in archive for name in [
        'methodArchiveHeaderBand', 'methodArchiveAccent',
        'layoutMethodArchiveDialog', 'WM_GETMINMAXINFO']),
    'archive actions are dynamically laid out':
        'setPos(methodArchiveRestoreButton' in archive and
        ('setPos(methodArchiveCloseButton' in archive or 'modalCreate("BUTTON", "ЗАКРЫТЬ"' not in archive),
    'archive preview wraps and has margins': 'emSetMargins' in archive and 'WS_HSCROLL' not in archive,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
