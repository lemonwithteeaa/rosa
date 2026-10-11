from pathlib import Path
root=Path(__file__).resolve().parent
a=(root/'auth.go').read_text(encoding='utf-8')
m=(root/'main.go').read_text(encoding='utf-8')
checks={
 'version': 'const appBuildVersion = "7.3.11.395-RC"' in m,
 'single header height': 'authHeaderHeight           int32 = 112' in a,
 'body starts below header': 'ch > authHeaderHeight' in a and 'ch-authHeaderHeight' in a,
 'stretch starts at header': 'pStretchBlt.Call(hdc, 0, uintptr(authHeaderHeight)' in a,
 'bottom anchored vertical crop': 'sy = authBackgroundSourceHeight - cropH' in a,
 'right anchored horizontal crop': 'sx = authBackgroundSourceWidth - cropW' in a,
 'old 104 start removed': 'pStretchBlt.Call(hdc, 0, 104,' not in a,
}
for name,ok in checks.items(): print(('PASS' if ok else 'FAIL'), name)
if not all(checks.values()): raise SystemExit(1)
