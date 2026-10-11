from pathlib import Path
import hashlib, sys
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8')
ref=(root/'methods_reference_dialog.go').read_text(encoding='utf-8')
impl=(root/'methods_implementation.go').read_text(encoding='utf-8')
workflow=(root/'methods_workflow.go').read_text(encoding='utf-8')
checks={
 'version_585':'const appBuildVersion = "7.3.11.585-RC"' in main,
 'build_version_585':'$version = "7.3.11.585-RC"' in build,
 'source_list_compact':'return prefix + " · " + title' in ref and 'parts = append(parts, strings.TrimSpace(src.Organization))' not in ref,
 'source_item_height':'LB_SETITEMHEIGHT' in ref and ref.count('0x01A0') >= 2,
 'source_heading_height':'headingH := int32(30)' in ref and 'sourceTitleH := int32(28)' in ref,
 'source_short_action_title':'ВСЕ ИСТОЧНИКИ · двойной щелчок' in ref,
 'implementation_edit_button':'ID_METHODS_IMPLEMENTATION_EDIT' in impl and 'ИЗМЕНИТЬ ПАРАМЕТРЫ' in workflow,
 'implementation_type_specific':all(x in impl for x in ['База дозирования','Тоннаж / концентрация','Критерии прекращения']),
 'implementation_water_oil_basis':'basis = "По воде"' in impl and 'basis = "По нефти"' in impl,
 'implementation_manual_marker':'ИЗМЕНЕНО ТЕХНОЛОГОМ' in impl and '[изменено]' in impl,
 'implementation_journal':'method_implementation_parameters_journal.jsonl' in impl and 'appendMethodImplementationAudit' in impl,
 'implementation_persistence':'method_implementation_parameters.json' in impl and 'saveMethodImplementationValues' in impl,
 'implementation_approved_snapshot':'Implementation    []MethodImplementationRecord' in workflow and 'Implementation: implementation' in workflow,
 'implementation_report':'ПАРАМЕТРЫ РЕАЛИЗАЦИИ' in workflow and 'methodImplementationReportText' in workflow,
 'forecast_core_unchanged':hashlib.sha256((root/'methods_effect_model.go').read_bytes()).hexdigest()=='741a6b2630994cfa7d1696c5c278648cb123decdd651d180545e1ebd5d705c3c',
 'icon_guard':all(x in build for x in ['embed_icon_resource.py','verify_icon_resource.py','OF_Model_icon_round.ico']),
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
print(f"TOTAL: {sum(checks.values())}/{len(checks)} PASS")
sys.exit(1 if failed else 0)
