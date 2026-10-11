from pathlib import Path
root = Path(__file__).resolve().parent
checks=[]
def check(name, cond):
    checks.append((name,bool(cond)))
main=(root/'main.go').read_text(encoding='utf-8')
lab=(root/'laboratory.go').read_text(encoding='utf-8')
auth=(root/'auth.go').read_text(encoding='utf-8')
mrp=(root/'mrp_forecast_ui.go').read_text(encoding='utf-8')
check('version_322', 'const appBuildVersion = "7.3.11.322-RC"' in main)
check('archive_in_resize_nav', 'ID_RESULT, ID_ARCHIVE}' in main)
check('archive_caption', '"8. Архивы"' in main)
check('archive_temp_guard', 'runningFromArchiveTemp' in main and 'rar$exa' in main.lower())
check('archive_message', 'распакуйте архив' in main.lower())
check('assessment_object_regular_bold', 'assessObjectInfo != 0 && hFontBold != 0' in main)
check('probability_button_compact', 'caption == "Вероятность осложнений"' in main)
check('orp_compact_only', 'f.Key == "orp" && hFontButtonCompact != 0' in lab)
check('orp_restore_bold', 'else if hFontBold != 0' in lab)
check('auth_label_font_17', 'pCreateFontW.Call(^uintptr(17-1)' in auth)
check('auth_profile_font_18', 'pCreateFontW.Call(^uintptr(18-1)' in auth)
check('auth_label_height_30', 'labelH, editH, editGap := int32(30), int32(36), int32(2)' in auth)
check('mrp_object_regular_bold', 'mrpObjectLabel != 0 && hFontBold != 0' in mrp)
failed=[n for n,ok in checks if not ok]
for n,ok in checks:
    print(('PASS' if ok else 'FAIL')+': '+n)
print(f'RESULT: {len(checks)-len(failed)}/{len(checks)}')
raise SystemExit(1 if failed else 0)
