from pathlib import Path
from zipfile import ZipFile

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8-sig") for name in [
    "main.go", "object_selection.go", "laboratory.go",
    "laboratory_complex_analysis.go", "demo_seed.go", "laboratory_pass.go",
    "repository.go", "storage_sqlite_windows.go", "assessment_classification.go",
]}

complex_source = files["laboratory_complex_analysis.go"]
load_body = complex_source.split("func complexLoadObjects(", 1)[1].split("func complexObjectNumber(", 1)[0]

checks = {
    "version_542": 'const appBuildVersion = "7.3.11.542-RC"' in files["main.go"],
    "one_caption_index": all(x in files["object_selection.go"] for x in [
        "func readableObjectKeyIndex()", "func readableObjectKeyFromIndex(",
    ]),
    "history_reuses_caption_index": all(x in files["laboratory.go"] for x in [
        "captions := readableObjectKeyIndex()",
        "laboratoryHistoryEntryColumnsWithCaptions(labHistoryEntries[i], captions)",
    ]),
    "complex_uses_snapshot": all(x in complex_source for x in [
        "prepareComplexAnalysisIndex()", "complexCaptionIndex = readableObjectKeyIndex()",
        "complexPassIndex = make(map[string][]LabPassRecord)",
    ]),
    "no_per_object_repository_query": "repositoryLabPassesForObject" not in load_body,
    "demo_is_one_batch": all(x in files["demo_seed.go"] for x in [
        "beginLabPassBulkWrite()", "flushLabPassBulkWrite()", "durableIDs[receipt.PassID]",
    ]),
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
    raise SystemExit("RC542 static QA failed: " + ", ".join(failed))
