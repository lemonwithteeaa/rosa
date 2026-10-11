from pathlib import Path
root = Path(__file__).resolve().parent
workflow = (root/'methods_workflow.go').read_text(encoding='utf-8')
main = (root/'main.go').read_text(encoding='utf-8')
checks = {
    'version 570': 'const appBuildVersion = "7.3.11.570-RC"' in main,
    'reference image': (root/'Дашборд_методов_и_мероприятий_РЕФЕРЕНС.png').exists(),
    'object tech card': 'ТЕХНИЧЕСКИЙ РЕЖИМ ОБЪЕКТА' in workflow,
    'classification strip': 'УТВЕРЖДЁННАЯ КЛАССИФИКАЦИЯ ИЗ РАЗДЕЛА 3' in workflow,
    'three-zone basis': 'ОСНОВАНИЕ ВЫБОРА' in workflow,
    'three-zone params': 'ПАРАМЕТРЫ РЕАЛИЗАЦИИ' in workflow,
    'horizontal forecast': 'ПРОГНОЗ НАРАБОТКИ ПОСЛЕ УТВЕРЖДЕНИЯ МЕТОДА' in workflow,
    'archive visible gate': 'showArchive :=' in workflow and 'show(methodsArchiveList, true)' in workflow,
    'reference action labels': all(x in workflow for x in ['СПРАВОЧНИК','ТИРАЖИРОВАНИЕ','УТВЕРДИТЬ МЕТОД','СФОРМИРОВАТЬ ПЛАН']),
    'responsive widths': all(x in workflow for x in ['contentW :=','leftW :=','midW :=','rightW :=']),
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL') + ' - ' + k)
if failed: raise SystemExit('FAILED: ' + ', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
