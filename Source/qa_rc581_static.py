from pathlib import Path
import json
root=Path('.')
s=(root/'methods_reference_dialog.go').read_text(encoding='utf-8')
a=(root/'auth.go').read_text(encoding='utf-8')
m=(root/'main.go').read_text(encoding='utf-8')
seed=json.loads((root/'rules_seed.json').read_text(encoding='utf-8'))
web=[x for x in seed.get('sources',[]) if x.get('url')]
local=[x for x in seed.get('sources',[]) if not x.get('url')]
checks={
 'version 581':'const appBuildVersion = "7.3.11.581-RC"' in m,
 'dialog V6':'ROSAMethodsReferenceDialogV6' in s,
 'visible minimize icon':'methodsReferenceMinimizeButton = modalCreate("BUTTON", "—"' in s,
 'visible maximize icon':'methodsReferenceMaxRestoreButton = modalCreate("BUTTON", "□"' in s and '"❐"' in s,
 'visible close icon':'methodsReferenceCloseButton = modalCreate("BUTTON", "×"' in s,
 'header controls positioned':'setPos(methodsReferenceMinimizeButton' in s and 'setPos(methodsReferenceCloseButton' in s,
 'no bottom close controls':'modalCreate("BUTTON", "ЗАКРЫТЬ"' not in s,
 'web opens in browser':'methodsReferenceOpenURL' in s and 'pShellExecuteW.Call' in s and '"open"' in s,
 'local sources download':'methodsReferenceExportLocalSource' in s and 'methodsReferenceDownloadsDir' in s and 'Downloads' in s,
 'internal tables export csv':'methodsReferenceWriteCSV' in s and 'excel_tech_methods' in s and 'excel_prelim_matrix' in s,
 'double click dispatches action':'notify == 2' in s and 'handleMethodsReferenceSourceAction(hwnd, src)' in s,
 'popup shows actual target':'openMethodsReferenceSourceLinkPopup(owner, src, path, "Скачанный файл")' in s and 'openMethodsReferenceSourceLinkPopup(owner, src, strings.TrimSpace(src.URL), "Интернет-ссылка")' in s,
 'all web sources real scheme': bool(web) and all(x['url'].startswith(('https://','http://')) for x in web),
 'local sources preserved': {x['id'] for x in local} >= {'excel_tech_methods','excel_prelim_matrix','excel_econ_main'},
 'welcome locked hotfix':'authHardenFeatureTitleGeometry()' in a and 'КОМПЛЕКСНАЯ ДИАГНОСТИКА' in a and 'item.x -= 32' in a,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+' - '+k)
print(f'{len(checks)-len(failed)}/{len(checks)} PASS')
if failed: raise SystemExit(1)
