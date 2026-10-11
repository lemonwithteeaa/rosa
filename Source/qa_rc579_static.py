from pathlib import Path
import json
root=Path('.')
s=(root/'methods_reference_dialog.go').read_text(encoding='utf-8')
m=(root/'main.go').read_text(encoding='utf-8')
seed=json.loads((root/'rules_seed.json').read_text(encoding='utf-8'))
checks={
 'version 579':'const appBuildVersion = "7.3.11.579-RC"' in m,
 'dialog V4':'ROSAMethodsReferenceDialogV4' in s,
 'clickable method list':'methodsReferenceMethodListHandle' in s and 'LBS_NOTIFY' in s and 'LBN_SELCHANGE' in s,
 'all catalog methods populate':'for _, m := range ruleCatalog.MethodCatalog' in s and len(seed.get('method_catalog',[])) >= 40,
 'method card on selection':'openSelectedMethodsReferenceCard' in s and 'methodsReferenceMethodCardText' in s,
 'method sources in card':'ИСТОЧНИКИ И ДОКАЗАТЕЛЬНАЯ БАЗА' in s and 'ruleSource(sourceID)' in s,
 'method limitations in card':'Проверить до применения' in s and 'Ограничения' in s,
 'minimize control':'methodsReferenceMinimize' in s and 'СВЕРНУТЬ' in s and 'swMinimize' in s,
 'maximize restore control':'methodsReferenceMaxRestore' in s and 'РАЗВЕРНУТЬ' in s and 'ВОССТАНОВИТЬ' in s,
 'close control':'methodsReferenceClose' in s and 'ЗАКРЫТЬ' in s,
 'methods split layout':'listW :=' in s and 'cardX :=' in s,
 'autotechnolog boundary':'подбор конструкции ГНО остаётся в «Автотехнологе»' in s,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+' - '+k)
print(f'{len(checks)-len(failed)}/{len(checks)} PASS')
if failed: raise SystemExit(1)
