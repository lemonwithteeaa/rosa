from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8", errors="ignore")
ui = (root / "well_card_ui.go").read_text(encoding="utf-8", errors="ignore")
extra = (root / "well_failure_extra.go").read_text(encoding="utf-8", errors="ignore")
template = (root / "failure_blank_template.go").read_text(encoding="utf-8", errors="ignore")
methods = (root / "methods_workflow.go").read_text(encoding="utf-8", errors="ignore")
build = (root / "build_windows_gui.ps1").read_text(encoding="utf-8", errors="ignore")

checks = {
    "version 589": 'const appBuildVersion = "7.3.11.589-RC"' in main,
    "build version 589": '$version = "7.3.11.589-RC"' in build,
    "failure blank embedded": "//go:embed РОСА_ФОРМА_БАЗЫ_ОТКАЗОВ" in template,
    "failure blank action": "downloadFailureTemplate()" in ui,
    "four import steps": "1  СКАЧАТЬ БЛАНК" in ui and "4  ПРИНЯТЬ" in ui,
    "preliminary first page": "1 ● ПРЕДВАРИТЕЛЬНО" in ui and "ПРЕДВАРИТЕЛЬНО · Исходные сведения" in extra,
    "extended investigation pages": "2 ● РАССЛЕДОВАНИЕ" in ui and "5 ● КОМИССИЯ/ИТОГ" in ui,
    "preliminary fields retained": "wellFullCardMode = true" in ui and "wellFormSection = 0" in ui,
    "dynamic toolbar": "toolbarControls := []syscall.Handle{wellSectionButtons[0]}" in ui,
    "dynamic footer": "footerControls := []syscall.Handle{wellSaveDraft, wellSaveFailure}" in ui,
    "technical regime centered": "methodsCenteredCardLabel" in methods and "SS_CENTER|SS_NOPREFIX" in methods,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit("FAILED: " + ", ".join(failed))
