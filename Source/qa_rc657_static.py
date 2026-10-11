from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
demo = (root / 'demo_seed.go').read_text(encoding='utf-8')
reference = (root / 'methods_reference_dialog.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
seed = demo[demo.index('func seedBundledDemoChecked()'):demo.index('func isSupersededTrainingPass')]
checks = {
    'version 657 or newer': bool(m) and int(m.group(1)) >= 657,
    'training seed reconciles every current passport through its linked failure':
        'for _, well := range corpus.Wells' in seed and 'failuresByID[p.FailureID]' in seed and
        'trainingLabPassRecord(w, event, p)' in seed,
    'accepted content rail removes the navigation gap':
        'x := int32(230)' in flow and 'contentW := max32(804, w-242)' in flow,
    'plan is visible between forecast and archive':
        'show(methodsPlanTitle, true)' in flow and 'show(methodsPlanList, true)' in flow and
        'y += planTitleH + 4 + planListH + 6' in flow,
    'teaching examples reach the plan':
        'teachingCrossHistoryPoints(methodsTeachingPreviewKey(key), time.Now())' in flow and
        'УЧЕБНЫЙ ПРИМЕР' in flow,
    'reference contains accent target action':
        'ИЗМЕНИТЬ ЦЕЛЕВОЙ ОРИЕНТИР' in reference and
        'methodsReferenceOpenTarget = true' in reference and 'activatePage(ID_MRP, true)' in reference,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
