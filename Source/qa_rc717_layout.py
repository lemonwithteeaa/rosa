from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
checks = {
    "version 717": 'const appBuildVersion = "7.3.11.717-RC"' in main,
    "dedicated 13px hint font": 'hFontResultHint = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(13-1)' in main,
    "hint uses dedicated font": 'WM_SETFONT, uintptr(hFontResultHint), 1' in report,
    "hint exactly above three buttons": 'setPos(resultDecisionHint, actionsX, decisionY, actionsW, 16)' in report,
    "caption keeps full final word": 'Выберите строку сценария — затем открывайте карточку метода или расчёт эффекта"' in report,
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " - " + name)
if failed:
    raise SystemExit("RC717 layout QA failed: " + ", ".join(failed))
