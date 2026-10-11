from pathlib import Path
from zipfile import ZipFile

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8-sig") for name in [
    "main.go", "startup_loading.go", "demo_seed.go", "laboratory_pass.go",
    "repository.go", "repository_windows.go", "storage_sqlite_windows.go",
    "assessment_classification.go",
]}

checks = {
    "version_541": 'const appBuildVersion = "7.3.11.541-RC"' in files["main.go"],
    "startup_pumps_messages": all(x in files["startup_loading.go"] for x in [
        "pumpStartupLoadingMessages", "PeekMessageW", "setStartupLoadingProgress",
    ]),
    "demo_is_one_batch": all(x in files["demo_seed.go"] for x in [
        "beginLabPassBulkWrite()", "flushLabPassBulkWrite()", "durableIDs[receipt.PassID]",
    ]),
    "sqlite_batch_transaction": all(x in files["storage_sqlite_windows.go"] for x in [
        "insertLabPassBatch", "withWriteTransaction",
    ]),
    "jsonl_batch_single_sync": "appendLabPassLegacyJSONLBatch" in files["laboratory_pass.go"],
    "classification_uses_exact_open_pass": all(x in files["main.go"] for x in [
        'labPassDrafts[normalizeLaboratoryObjectKey(activeObject)+"|"+labActivePassID]',
        "открытая лабораторная запись", "canonicalizeLaboratoryValues",
    ]),
    "expert_decision_not_automated": all(x in files["assessment_classification.go"] for x in [
        "Программное предположение не заменяет решение технолога",
        "confirmAssessmentFinalClass",
    ]),
}

examples = sorted((root / "demo_examples").glob("*.xlsx"))
checks["exactly_100_examples"] = len(examples) == 100
checks["all_examples_valid_xlsx"] = len(examples) == 100 and all(
    ZipFile(path).testzip() is None for path in examples
)

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(f"{'PASS' if ok else 'FAIL'}: {name}")
if failed:
    raise SystemExit("RC541 static QA failed: " + ", ".join(failed))
