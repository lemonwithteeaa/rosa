from pathlib import Path
import hashlib
root=Path(__file__).resolve().parent
mw=(root/'methods_workflow.go').read_text(encoding='utf-8')
main=(root/'main.go').read_text(encoding='utf-8')
build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8')
verify=(root/'verify_icon_resource.py').read_text(encoding='utf-8')
checks={
 'version_576': 'const appBuildVersion = "7.3.11.576-RC"' in main,
 'unready_three_zone_layout': 'even before full classification the page keeps the same three-zone' in mw and 'show(methodsBasisTitle, true)' in mw and 'show(methodsImplementationTitle, true)' in mw,
 'unready_workspace_reserved': 'y += workspaceH + 10' in mw,
 'no_old_pink_placeholder': 'child == methodsEmptyState || child == methodsSuggestionInfo || child == methodsImplementation' in main and 'return uintptr(brushWhite)' in main,
 'classification_jump_button': 'ПЕРЕЙТИ К КЛАССИФИКАЦИИ' in mw and 'switchPage(ID_ASSESS)' in mw,
 'unready_content_not_blank': 'ОСНОВАНИЕ БУДУЩЕГО ВЫБОРА' in mw and 'ПОСЛЕ УТВЕРЖДЕНИЯ' in mw,
 'ready_card_grid_preserved': 'ID_METHODS_CARD_COUNT        = 6' in mw and 'cardRows := int32(3)' in mw,
 'approved_icon_hash': hashlib.sha256((root/'OF_Model_icon_round.ico').read_bytes()).hexdigest()=='d63b3dea920b18114fd62da4ddaa92cad91979663e732e523242909d06ac244a',
 'build_uses_approved_icon': 'OF_Model_icon_round.ico' in build and '7.3.11.576-RC' in build,
 'icon_identity_check': 'approved icon bytes embedded' in verify and 'payload not in b' in verify,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL'),k)
failed=[k for k,v in checks.items() if not v]
if failed: raise SystemExit('RC576 static QA failed: '+', '.join(failed))
print(f'RC576 STATIC QA: {len(checks)}/{len(checks)} PASS')
