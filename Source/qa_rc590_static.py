from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8", errors="ignore")
build = (root / "build_windows_gui.ps1").read_text(encoding="utf-8", errors="ignore")
methods = (root / "methods_workflow.go").read_text(encoding="utf-8", errors="ignore")
effects = (root / "methods_effect_model.go").read_text(encoding="utf-8", errors="ignore")
multi = (root / "multi_method_scenario.go").read_text(encoding="utf-8", errors="ignore")

checks = {
    "version 590": 'const appBuildVersion = "7.3.11.590-RC"' in main,
    "build version 590": '$version = "7.3.11.590-RC"' in build,
    "technical cards centered": "methodsCenteredCardLabel" in methods and "SS_CENTER|SS_NOPREFIX" in methods,
    "plan primary caption": '"СФОРМИРОВАТЬ ПЛАН"' in methods,
    "compact action line": "readyW := contentW - approveW - planW" in methods,
    "reference proportions": "contentW * 47 / 100" in methods and "contentW * 25 / 100" in methods,
    "six card grid": "ID_METHODS_CARD_COUNT" in methods and "cardRows := int32(3)" in methods,
    "forecast four cards": all(name in methods for name in (
        "methodsForecastCurrent", "methodsForecastGain", "methodsForecastResult", "methodsForecastBasis"
    )),
    "archive uses remaining client height": "archiveH := max32(1, bottom-archiveY)" in methods,
    "three methods scenario tested without reducing six-method limit": "const maxMethodsPerScenario = 6" in multi,
    "residual risk combination": "residual *= 1 - r" in effects,
    "approved evidence gates numeric effect": "approved > 0" in effects and "confirmedCauseShare" in effects,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit("FAILED: " + ", ".join(failed))
