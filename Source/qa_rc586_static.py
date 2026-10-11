from pathlib import Path
import hashlib, sys
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
workflow=(root/'methods_workflow.go').read_text(encoding='utf-8')
plan=(root/'methods_plan_archive.go').read_text(encoding='utf-8')
build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8')
checks={
 'version_586':'const appBuildVersion = "7.3.11.586-RC"' in main,
 'build_version_586':'$version = "7.3.11.586-RC"' in build,
 'plan_statuses':all(x in plan for x in ['Черновик','Выбран','Утверждён','Назначен','Выполнен','Отменён']),
 'plan_columns':all(x in workflow for x in ['{"Состояние", 120}', '{"Плановая дата", 120}', '{"Ответственный", 160}', '{"Контроль", 205}', '{"Параметры", 130}']),
 'plan_editor':'ПЛАН ИСПОЛНЕНИЯ МЕРОПРИЯТИЯ' in plan and 'openMethodsPlanEditorAtRow' in plan,
 'plan_dates_responsible':all(x in main for x in ['Responsible      string', 'ControlDate      string', 'ControlCriterion string', 'DecisionID       string']),
 'plan_validation':'Для состояния «Назначен»' in plan and 'Для состояния «Выполнен»' in plan,
 'plan_audit':'method_plan_journal.jsonl' in plan and 'appendMethodPlanAudit' in plan,
 'plan_parameter_link':'Сохранённый снимок параметров версии решения' in plan and 'methodPlanSnapshotText' in plan,
 'approval_per_method_plan':'methodPlanItemsForApprovedDecision' in workflow and 'd.Plan = clonePlanItems(planItems)' in workflow,
 'decision_version_fields':all(x in workflow for x in ['SchemaVersion', 'VersionNumber', 'RestoredFromID', 'SupersedesID', 'Plan              []PlanItem']),
 'archive_version_status':'methodArchiveVersionStatus(d)' in workflow and 'Версия %d · восстановлено из v%s' in plan,
 'archive_preview':'АРХИВНАЯ ВЕРСИЯ РЕШЕНИЯ' in plan and 'openMethodsArchiveVersionAtRow' in plan,
 'restore_confirmation':'Восстановить выбранную архивную версию?' in plan and 'r != 6' in plan,
 'restore_new_version':'restoreMethodDecisionAsNewVersion' in plan and 'd.RestoredFromID = old.ID' in plan and 'approvedMethodDecisions = append(approvedMethodDecisions, d)' in plan,
 'restore_no_overwrite':'Текущая запись НЕ будет перезаписана' in plan,
 'restore_implementation':'applyMethodImplementationSnapshotsToMemory' in plan and 'saveJSON(methodImplementationPath(), methodImplementationRecords)' in plan,
 'report_plan':'ПЛАН ИСПОЛНЕНИЯ' in workflow and 'methodPlanControlDisplay' in workflow,
 'report_archive_versions':'methodDecisionStoredVersion(d)' in workflow and 'methodArchiveVersionStatus(d)' in workflow,
 'methods_button':'НАСТРОИТЬ ПЛАН' in workflow,
 'methods_double_click_editor':'openMethodsPlanEditorAtRow(i)' in main,
 'archive_double_click':'openMethodsArchiveVersionAtRow(i)' in main,
 'global_plan_kept':'if currentPage == ID_ACTIVITIES' in main and 'n := generatePlan()' in main,
 'forecast_core_unchanged':hashlib.sha256((root/'methods_effect_model.go').read_bytes()).hexdigest()=='741a6b2630994cfa7d1696c5c278648cb123decdd651d180545e1ebd5d705c3c',
 'icon_guard':all(x in build for x in ['embed_icon_resource.py','verify_icon_resource.py','OF_Model_icon_round.ico']),
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
print(f"TOTAL: {sum(checks.values())}/{len(checks)} PASS")
sys.exit(1 if failed else 0)
