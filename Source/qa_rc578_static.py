from pathlib import Path
s = Path('methods_reference_dialog.go').read_text(encoding='utf-8')
m = Path('main.go').read_text(encoding='utf-8')
checks = {
    'version 578': '7.3.11.578-RC' in m,
    'dialog V3': 'ROSAMethodsReferenceDialogV3' in s,
    'navy header band': 'methodsReferenceHeaderBand' in s and 'brushNav' in s,
    'red accent line': 'methodsReferenceAccent' in s and 'brushHeader' in s,
    'owner-drawn tabs': s.count('BS_OWNERDRAW') >= 5,
    'four equal tabs': 'tabW := available / 4' in s,
    'active tab state': 'methodsReferenceActiveSection' in s,
    'graphite body': 'rgb(43, 46, 50)' in s,
    'navy hierarchy': 'rgb(15, 48, 82)' in s,
    'red key hierarchy': 'rgb(190, 24, 49)' in s,
    'compact window cap': 'min32(1480' in s and 'min32(860' in s,
    'concise shell title': 'РОСА — Справочник' in s,
    'footer boundary note': 'подбор конструкции ГНО остаётся в «Автотехнологе»' in s,
    'legacy V2 removed': 'ROSAMethodsReferenceDialogV2' not in s,
}
failed = [k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL') + ' - ' + k)
print(f'{len(checks)-len(failed)}/{len(checks)} PASS')
if failed: raise SystemExit(1)
