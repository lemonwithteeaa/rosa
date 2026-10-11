from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
checks = {
    "dedicated compact hint font": (
        'hFontResultHint = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(14-1)' in main
        or 'hFontResultHint = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(13-1)' in main
    ),
    "hint uses dedicated font": 'WM_SETFONT, uintptr(hFontResultHint), 1' in report,
    "hint exactly above three buttons": 'setPos(resultDecisionHint, actionsX, decisionY, actionsW, 16)' in report,
    "no RC715 left expansion": 'actionsW+140' not in report and 'hintX := right - hintW' not in report,
}
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if not all(checks.values()):
    raise SystemExit("RC716 result hint layout failed")
print(f"{len(checks)}/{len(checks)} PASS")
