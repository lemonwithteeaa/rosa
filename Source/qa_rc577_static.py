from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
mw=(root/'methods_workflow.go').read_text(encoding='utf-8')
ac=(root/'assessment_classification.go').read_text(encoding='utf-8')
build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8')
checks={
 'version_577': 'const appBuildVersion = "7.3.11.577-RC"' in main and '7.3.11.577-RC' in build,
 'methods_actions_48px': 'btnH := int32(48)' in mw,
 'methods_four_equal_columns': 'btnW := (contentW - btnGap*3) / 4' in mw,
 'methods_title_separate_row': 'setPos(methodsSuggestionTitle, x, y, contentW, 30)' in mw and 'y += 32' in mw,
 'assessment_actions_50px': 'actionH := int32(50)' in main,
 'assessment_action_width_safe': 'actionW := min32(260, max32(220, contentW/4))' in main,
 'assessment_tabs_below_actions': 'tabY := py(190)' in ac,
 'assessment_tabs_44px': 'tabH := int32(44)' in ac,
 'old_overlap_removed': 'tabY := py(158)' not in ac,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL'),k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit('RC577 static QA failed: '+', '.join(failed))
print(f'RC577 STATIC QA: {len(checks)}/{len(checks)} PASS')
