from pathlib import Path
import json
root=Path('.')
s=(root/'methods_reference_dialog.go').read_text(encoding='utf-8')
m=(root/'main.go').read_text(encoding='utf-8')
w=(root/'methods_workflow.go').read_text(encoding='utf-8')
seed=json.loads((root/'rules_seed.json').read_text(encoding='utf-8'))
visible_blob=s+'\n'+w+'\n'+json.dumps(seed,ensure_ascii=False)
checks={
 'version 580':'const appBuildVersion = "7.3.11.580-RC"' in m,
 'dialog V5':'ROSAMethodsReferenceDialogV5' in s,
 'native top window controls':'WS_OVERLAPPEDWINDOW|WS_VISIBLE' in s,
 'no bottom close button':'methodsReferenceCloseButton' not in s and 'modalCreate("BUTTON", "ЗАКРЫТЬ"' not in s,
 'no bottom min max buttons':'methodsReferenceMinimizeButton' not in s and 'methodsReferenceMaxRestoreButton' not in s,
 'method source list':'methodsReferenceSourceListHandle' in s and 'ИСТОЧНИКИ · двойной щелчок' in s,
 'double click source':'notify == 2' in s and 'openMethodsReferenceSourceLinkPopup' in s,
 'copyable source link':'methodsSourcePopupCopy' in s and 'WM_COPY' in s and 'methodsReferenceSourceLink' in s,
 'all sources tab clickable':'populateAllMethodsReferenceSources' in s and 'ruleCatalog.Sources' in s,
 'method sources derived':'methodsReferenceSourcesForMethod' in s and 'r.SourceIDs' in s,
 'calm typography':'strings.Index(t, ":")' not in s and 'upperHeading :=' in s,
 'no technology module wording':'Технологического модуля' not in visible_blob and 'технологический модуль' not in visible_blob.lower(),
 'renamed embedded source':any(x.get('id')=='excel_tech_methods' and x.get('title')=='Встроенный справочник методов РОСА' for x in seed.get('sources',[])),
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+' - '+k)
print(f'{len(checks)-len(failed)}/{len(checks)} PASS')
if failed: raise SystemExit(1)
