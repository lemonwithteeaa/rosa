from pathlib import Path
import hashlib
root=Path(__file__).resolve().parent
mw=(root/'methods_workflow.go').read_text(encoding='utf-8')
main=(root/'main.go').read_text(encoding='utf-8')
build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8')
verify=(root/'verify_icon_resource.py').read_text(encoding='utf-8')
checks={
 'version_575': 'const appBuildVersion = "7.3.11.575-RC"' in main,
 'card_grid_ids': 'ID_METHODS_CARD_BASE' in mw and 'ID_METHODS_CARD_COUNT       = 6' in mw,
 'card_grid_layout': 'two columns of large interactive cards' in mw and 'cardRows := int32(3)' in mw,
 'dense_candidate_table_hidden': 'show(methodsCandidateList, false)' in mw,
 'clean_static_context_cards': 'methodsSuggestionInfo = methodsCardLabel' in mw and 'methodsImplementation = methodsCardLabel' in mw,
 'reference_bundled': (root/'Дашборд_методов_и_мероприятий_РЕФЕРЕНС.png').exists(),
 'build_uses_approved_icon': 'OF_Model_icon_round.ico' in build and '7.3.11.575-RC' in build,
 'icon_identity_check': 'approved icon bytes embedded' in verify and 'payload not in b' in verify,
 'approved_icon_hash': hashlib.sha256((root/'OF_Model_icon_round.ico').read_bytes()).hexdigest()=='d63b3dea920b18114fd62da4ddaa92cad91979663e732e523242909d06ac244a',
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL'),k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit('RC575 static QA failed: '+', '.join(failed))
print(f'RC575 STATIC QA: {len(checks)}/{len(checks)} PASS')
