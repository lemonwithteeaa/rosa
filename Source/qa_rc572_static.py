from pathlib import Path

main = Path('main.go').read_text(encoding='utf-8')
methods = Path('methods_workflow.go').read_text(encoding='utf-8')
ref = Path('methods_reference_dialog.go').read_text(encoding='utf-8')
lab = Path('laboratory.go').read_text(encoding='utf-8')

checks = {
    'version 572': 'const appBuildVersion = "7.3.11.573-RC"' in main,
    'candidate row click handler': 'case methodsCandidateList:' in main and 'openMethodsCandidateReference(i)' in main,
    'method name cell opens detail': 'subItem == 1 || subItem == 4' in main,
    'priority plaque opens detail': 'subItem == 4' in main,
    'double click opens detail': 'hdr.Code == NM_DBLCLK' in main and 'openMethodsCandidateReference(i)' in main,
    'double click no longer adds': 'selectMethodsCandidateRow(i, hdr.Code == NM_DBLCLK)' not in main,
    'plan method names clickable': 'case methodsPlanList:' in main and 'openMethodsReferenceByName(names[i])' in main,
    'full-size resizable method card': 'WS_OVERLAPPEDWINDOW|WS_VISIBLE' in ref and 'case WM_SIZE:' in ref,
    'method card has priority explanation': 'ПОЧЕМУ ТАКОЙ ПРИОРИТЕТ' in Path('methods_reference.go').read_text(encoding='utf-8'),
    'candidate clickable visual accent': 'methodsCandidateCustomDraw' in methods and 'draw.SubItem == 4' in methods,
    'history not pushed by hidden clear control': 'copyY+52+36+12' not in lab,
    'history blank-area guard': 'Never leave an empty unlabelled rectangle where the table should be' in lab,
    'history header still explicit': 'Предыдущие анализы' in lab,
    'method table headers non-empty': all(x in methods for x in ['Осложнение / класс','Метод / мероприятие','Прим. / дост. / вероятность','Ожидаемый вклад','Приоритет / подход']),
    'global blank column guard': 'headerText = "Данные"' in main,
    'global blank row guard': 'hasVisibleData' in main and 'never render anonymous blank rows' in main,
    'reference removes permanent plan table': 'show(methodsPlanList, false)' in methods and 'archiveY := y + 31' in methods,
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ' | ' + name)
print(f"RESULT {len(checks)-len(failed)}/{len(checks)}")
raise SystemExit(1 if failed else 0)
