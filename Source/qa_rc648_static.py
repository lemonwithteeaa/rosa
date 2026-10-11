from pathlib import Path

root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
flow = (root/'methods_workflow.go').read_text(encoding='utf-8')
ref = (root/'methods_reference.go').read_text(encoding='utf-8')
learn = (root/'methods_learning.go').read_text(encoding='utf-8')

checks = {
    'version 648 or newer': __import__('re').search(r'const appBuildVersion = \"7\.3\.11\.(\d+)-RC\"', main) is not None and int(__import__('re').search(r'const appBuildVersion = \"7\.3\.11\.(\d+)-RC\"', main).group(1)) >= 648,
    'learning module exists': 'type learnedMethodSupport struct' in learn,
    'requires repeated distinct objects': 'support.Objects < 2' in learn,
    'learned methods merge into candidate list': 'mergeLearnedMethodSuggestions(objectKey, out)' in flow,
    'no automatic approval': 'Автоматически не выбирается' in learn and 'не применяется и не утверждается автоматически' in learn,
    'single methods remain available': 'Одиночный метод остаётся доступен' in learn,
    'historical combinations surfaced': 'ВОЗМОЖНЫЕ СОЧЕТАНИЯ С ДРУГИМИ МЕТОДАМИ' in learn,
    'historical application schemes surfaced': 'ВОЗМОЖНЫЕ СХЕМЫ ПРИМЕНЕНИЯ ИЗ ПРЕДЫДУЩИХ ДИАГНОСТИК' in learn,
    'analog dialog includes learned experience': 'learnedExperienceText(key, r.Method)' in ref,
    'persistent information line explains learning': 'аналоги из предыдущих диагностик, схемы применения и связки методов учитываются как подсказки' in flow,
    'user report no longer calls it replication': 'АНАЛОГИ / ПЕРЕНОС РЕШЕНИЯ' in flow and 'Нет выбранного/утверждённого метода для оценки тиражирования' not in flow,
    'implementation panel signals accumulated schemes': 'Накопленные схемы РОСА' in flow,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
