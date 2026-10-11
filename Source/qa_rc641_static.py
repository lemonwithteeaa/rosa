from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
implementation = (root / 'methods_implementation.go').read_text(encoding='utf-8')
workflow = (root / 'methods_workflow.go').read_text(encoding='utf-8')

checks = {
    'version at least 641': int(re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main).group(1)) >= 641,
    'editor is vertically scrollable': 'WS_VISIBLE|WS_VSCROLL' in implementation and 'case WM_VSCROLL:' in implementation,
    'mouse wheel scroll': 'case 0x020A: // WM_MOUSEWHEEL' in implementation,
    'scroll range tracks client height': 'methodImplementationEditorContentHeight - clientH' in implementation,
    'scroll moves child controls': 'SW_SCROLLCHILDREN' in implementation and 'methodImplementationScrollWindowEx.Call' in implementation,
    'editor height increased': ', 1120, 880,' in implementation,
    'object badge wraps instead of ellipsis': 'objectCaption := "ОБЪЕКТ ·\\r\\n"' in implementation and '620, 10, 454, 70' in implementation,
    'object badge ellipsis removed': 'SS_CENTER|SS_CENTERIMAGE|SS_ENDELLIPSIS, 798, 23, 276, 42' not in implementation,
    'dosage long fields full width': 'labelY = 147 + int32(slot)*47' in implementation and 'width = 474' in implementation,
    'old 228px dosage controls removed': 'int32(slot%2)*244' not in implementation and 'width = 228' not in implementation,
    'chemical scheme summary expanded': '36, 565, 500, 108' in implementation,
    'scheme summary uses separate lines': 'База: %s\\r\\nДозирование: %s\\r\\nТочка: %s\\r\\nПериодичность: %s\\r\\nКонтроль: %s' in implementation,
    'reference panel expanded': '20, 530, 1064, 154' in implementation,
    'footer moved below reference': '20, 744, 1064, 1' in implementation,
    'save button widened': '844, 758, 240, 44' in implementation,
    'cancel button widened': '682, 758, 150, 44' in implementation,
    'reset button widened': '20, 758, 320, 44' in implementation,
    'forecast equal width grid': 'forecastUsable / 4' in workflow and 'currentW := cardW' in workflow and 'gainW := cardW' in workflow,
    'forecast centered inset': 'currentX := x + forecastInset' in workflow,
    'double day period regression retained': 'this prevents "сут.."' in workflow and 'fmt.Sprintf(" ≈ +%.0f сут"' in workflow,
}

for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
