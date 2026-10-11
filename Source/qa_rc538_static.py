from pathlib import Path

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8") for name in [
    "main.go", "laboratory.go", "assessment_classification.go", "object_selection.go",
]}
all_source = "\n".join(files.values())
checks = {
    "version_538": 'const appBuildVersion = "7.3.11.538-RC"' in files["main.go"],
    "table_readable_height": all(x in files["main.go"] for x in ["maxObjectH := max32(140", "max32(140, desiredObjectH)"]),
    "teaching_objects_on_main": all(x in files["main.go"] for x in ["Saved laboratory and teaching records remain selectable", "pass.Status != labPassSavedStatus", "state := TechObjectState{ObjectKey: key"]),
    "lab_button_approves_visible_object": all(x in files["laboratory.go"] for x in ["approveOpenLaboratoryObject", "ЗАКРЕПИТЬ ЭТОТ ОБЪЕКТ", "selectedObjectKey = openedKey"]),
    "approval_keeps_open_pass": all(x in files["laboratory.go"] for x in ["labPinnedDraftID = labActivePassID", "Классификация осложнений рассчитана по открытой лабораторной записи"]),
    "off_main_repin_warning": all(x in files["laboratory.go"] for x in ["Перезакрепить объект вне Главной?", "проверить, дополнить или обновить", "МРП и отказы", "итоговое заключение"]),
    "assessment_update_can_approve": all(x in files["main.go"] for x in ["case ID_ASSESS_PROBABILITY:", "approveOpenLaboratoryObject()"]),
    "classification_reads_open_values": all(x in files["main.go"] for x in ["labDrafts[groupCode]", "выбранная лабораторная запись"]),
    "classification_source_truthful": all(x in files["assessment_classification.go"] for x in ["assessmentLatestLab(d)", "данных недостаточно для класса"]),
    "method_classes_present": all(x in files["assessment_classification.go"] for x in ['return "К3"', 'return "С2"', 'return "А2"', 'return "М3"', 'return "В3"']),
    "method_class_test_hook": "assessmentProgramClassFromRecord" in files["assessment_classification.go"],
    "free_gas_not_faked": "Лабораторный газовый фактор не заменяет этот показатель" in files["assessment_classification.go"],
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " | " + name)
print(f"RESULT | {len(checks)-len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
