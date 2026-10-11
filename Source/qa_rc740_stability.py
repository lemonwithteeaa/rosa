from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
lab = (root / "laboratory.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")

checks = {
    "release version 744+": bool(re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)) and int(re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M).group(1)) >= 744,
    "archive worker has panic boundary": "runApprovedRegistryExportSafely(record, exportFormat)" in report,
    "worker converts panic to visible error": 'не удалось сформировать отчёт %s: внутренняя ошибка экспорта' in report and "Приложение продолжает работу" in report,
    "worker panic details are logged": 'approved export PANIC format=%s panic=%v' in report,
    "laboratory complication switch has reentrancy guard": "labGroupTransitionBusy" in lab,
    "laboratory failed switch restores previous group": 'switchLabGroup(previousGroup, false)' in lab and 'Предыдущая вкладка восстановлена' in lab,
    "laboratory transition resets guard after panic": "defer func() {\n\t\tlabGroupTransitionBusy = false" in lab,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit(1)
