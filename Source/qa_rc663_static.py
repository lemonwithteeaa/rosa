from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
demo = (root / 'demo_seed.go').read_text(encoding='utf-8')
flow = (root / 'methods_workflow.go').read_text(encoding='utf-8')
impl = (root / 'methods_implementation.go').read_text(encoding='utf-8')
multi = (root / 'multi_method_scenario.go').read_text(encoding='utf-8')
background = (root / 'methods_background.go').read_text(encoding='utf-8')
classification = (root / 'assessment_classification.go').read_text(encoding='utf-8')
teaching = (root / 'teaching_cross_history.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 663 or newer': bool(m) and int(m.group(1)) >= 663,
    'legacy well-number matcher remains exact and bounded':
        'for _, token := range []string{"скважина", "скв.", "скв", "№", "#"}' in demo and
        'well == "6"' in demo and 'const canonicalCrossExampleObject' in demo,
    'method decision lookup uses equivalent object identity':
        'methodsObjectKeysEquivalent(approvedMethodDecisions[i].ObjectKey, objectKey)' in flow,
    'teaching methods are restored deterministically':
        'teachingActiveMethodDecision(key, summary)' in flow and
        'setCombinedMethods(key, lesson.Methods)' in flow and
        'Фильтр / сепаратор мехпримесей' in teaching,
    'combined method drafts support caption aliases':
        'func combinedMethodsForObject(objectKey string)' in multi and
        'methodsObjectIdentityKey(key) == identity' in multi,
    'classification clears every equivalent draft':
        'clearCombinedMethods(strings.TrimSpace(selectedObjectKey))' in classification,
    'background draft detection uses alias-safe accessor':
        'combinedMethodsForObject(key)' in background,
    'teaching implementation has concrete values for all three complications':
        'учебный образец К-01' in impl and '"rate": "35"' in impl and
        'учебный образец С-02' in impl and '"rate": "30"' in impl and
        'Фильтрация / сепарация мехпримесей по учебному режиму' in impl,
    '900px layout gives the workbench more room':
        'if h < 960' in flow and 'planReserve = 100' in flow and 'archiveReserve = 70' in flow and
        'workspaceMaxH = 190' in flow and
        'leftW := contentW * 44 / 100' in flow and 'midW := contentW * 25 / 100' in flow,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
