from pathlib import Path
root = Path(__file__).resolve().parent
workflow = (root/'methods_workflow.go').read_text(encoding='utf-8')
reference = (root/'methods_reference.go').read_text(encoding='utf-8')
dialog = (root/'methods_reference_dialog.go').read_text(encoding='utf-8')
main = (root/'main.go').read_text(encoding='utf-8')
checks = {
    'version 571': 'const appBuildVersion = "7.3.11.571-RC"' in main,
    'reference composition preserved': all(x in workflow for x in ['ТЕХНИЧЕСКИЙ РЕЖИМ ОБЪЕКТА','УТВЕРЖДЁННАЯ КЛАССИФИКАЦИЯ ИЗ РАЗДЕЛА 3','ОСНОВАНИЕ ВЫБОРА','ПАРАМЕТРЫ РЕАЛИЗАЦИИ','ПРОГНОЗ НАРАБОТКИ ПОСЛЕ УТВЕРЖДЕНИЯ МЕТОДА']),
    'compact candidate columns': 'Прим. / дост. / вероятность' in workflow and 'centerListHeaders(methodsCandidateList, 5)' in workflow,
    'five-point basis': all(x in workflow for x in ['1. Класс и механизм:', '2. Применимость:', '3. Достоверность оценки:', '4. Вероятность подтверждения эффекта:', '5. Прогноз и переносимость:']),
    'priority visible': 'Приоритет / подход' in workflow and 'methodPriorityLabel' in workflow,
    'top candidate suggested not approved': 'selectedComboIndex = 1' in workflow and 'не утверждает его' in workflow,
    'selection preserved': 'previousMethod :=' in workflow and 'normalizeMethodText(previousMethod)' in workflow,
    'selected row highlighted': 'selectListRow(methodsCandidateList' in workflow,
    'selected method full card': 'КАРТОЧКА МЕТОДА — ПОЛНАЯ ИНФОРМАЦИЯ' in reference and 'selectedMethodReferenceText' in reference,
    'reference remembers selected method': 'methodsReferenceMethodsBody' in dialog,
    'technology boundary': '«Автотехнологе»' in reference,
    'reference image retained': (root/'Дашборд_методов_и_мероприятий_РЕФЕРЕНС.png').exists(),
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL') + ' - ' + k)
if failed: raise SystemExit('FAILED: ' + ', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
