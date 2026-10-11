from pathlib import Path
import hashlib, sys
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
ref=(root/'methods_reference_dialog.go').read_text(encoding='utf-8')
build=(root/'build_windows_gui.ps1').read_text(encoding='utf-8')
checks={
 'version_584':'const appBuildVersion = "7.3.11.584-RC"' in main,
 'build_version_584':'$version = "7.3.11.584-RC"' in build,
 'approved_icon_hash':hashlib.sha256((root/'OF_Model_icon_round.ico').read_bytes()).hexdigest()=='d63b3dea920b18114fd62da4ddaa92cad91979663e732e523242909d06ac244a',
 'build_embeds_icon':'embed_icon_resource.py' in build and 'OF_Model_icon_round.ico' in build,
 'build_verifies_icon':'verify_icon_resource.py' in build and 'OF_Model_icon_round.ico' in build,
 'reference_native_caption': 'hwnd, _, _ := pCreateWindowExW.Call(\n\t\t0,' in ref,
 'reference_has_app_icon':'HIcon:         hIconBig' in ref and 'HIconSm:       hIconSmall' in ref,
 'reference_window_sets_icons':'WM_SETICON, ICON_BIG' in ref and 'WM_SETICON, ICON_SMALL' in ref,
 'reference_header_compact':'headerH := int32(76)' in ref and 'tabsY := int32(88)' in ref and 'contentY := int32(150)' in ref,
 'reference_title_font':'methodsReferenceTitleHandle), WM_SETFONT, uintptr(hFontSystemName)' in ref,
 'reference_subtitle_font':'methodsReferenceSubtitleHandle), WM_SETFONT, uintptr(hFont)' in ref,
 'reference_one_strong_heading':'RC584: one strong card title only' in ref and 'cf.DwEffects = CFE_BOLD' in ref,
 'source_problem_reference':(root/'РЕФЕРЕНС_584_СПРАВОЧНИК_ПРОБЛЕМНЫЕ_МЕСТА.png').exists(),
 'source_icon_reference':(root/'РЕФЕРЕНС_584_ИКОНОЧНЫЙ_ДЕФЕКТ.png').exists(),
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL'),k)
if not all(checks.values()): sys.exit(1)
print(f'{sum(checks.values())}/{len(checks)} PASS')
