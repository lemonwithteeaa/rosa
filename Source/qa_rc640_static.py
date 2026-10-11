from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
implementation = (root / 'methods_implementation.go').read_text(encoding='utf-8')
reference = (root / 'methods_reference_dialog.go').read_text(encoding='utf-8')

checks = {
    'version 640': 'const appBuildVersion = "7.3.11.640-RC"' in main,
    'expanded implementation editor': ', 1120, 820,' in implementation,
    'program header palette': all(x in implementation for x in ['methodImplementationHeaderBand', 'brushNav', 'methodImplementationAccent', 'brushHeader']),
    'editable dropdown parameters': '"COMBOBOX"' in implementation and 'CBS_DROPDOWN' in implementation,
    'old clipped fields removed': 'ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL, 270' not in implementation,
    'context choice catalogs': 'methodImplementationFieldOptions' in implementation and all(x in implementation for x in ['case "basis":', 'case "point":', 'case "periodicity":', 'case "control":']),
    'measurement unit follows dosage basis': 'field("unit", "Единица измерения", "гр/м³")' in implementation and implementation.index('field("basis", "База дозирования"') < implementation.index('field("unit", "Единица измерения"') < implementation.index('field("rate", "Концентрация / расход"'),
    'measurement unit dropdown values': all(x in implementation for x in ['"гр/м³"', '"гр/тн"', '"тн"', '"м³"']),
    'seven dosage fields supported': 'methodsImplementationFieldCount         = 7' in implementation,
    'visible visual hierarchy': all(x in implementation for x in ['РЕАГЕНТ И ДОЗИРОВАНИЕ', 'ВВОД И ПЕРИОДИЧНОСТЬ', 'КОНТРОЛЬ РЕЗУЛЬТАТА', 'methodImplementationNoteBand']),
    'white parameters card with red accent': all(x in implementation for x in ['methodImplementationFieldsPanel', 'methodImplementationFieldsAccent', 'return uintptr(brushWhite)', 'return uintptr(brushHeader)']),
    'branded owner-drawn actions': 'drawMethodImplementationButton' in implementation and implementation.count('BS_OWNERDRAW') >= 5,
    'primary actions use brand colors': all(x in implementation for x in ['case methodsImplementationEditorChooseScheme:', 'fill = brushNav', 'case methodsImplementationEditorSave:', 'fill = brushHeader']),
    'unit is a strict dropdown': 'f.Key == "unit"' in implementation and 'CBS_DROPDOWNLIST' in implementation and 'CB_SETCURSEL' in implementation,
    'scheme chooser has branded header': all(x in implementation for x in ['methodImplementationSchemeChooserHeader', 'methodImplementationSchemeChooserAccent', 'ВЫБОР СХЕМЫ ХИМИЗАЦИИ']),
    'scheme chooser has separate list and detail headings': all(x in implementation for x in ['СОХРАНЁННЫЕ СХЕМЫ', 'СОСТАВ ВЫБРАННОЙ СХЕМЫ']),
    'scheme chooser is expanded and unclipped': ', 1040, 700,' in implementation and 'RichEdit20W' in implementation,
    'scheme chooser actions are branded': 'methodsImplementationSchemeApply, methodsImplementationSchemeCancel' in implementation and 'BS_OWNERDRAW' in implementation,
    'reference shows saved scheme count': 'СОХРАНЁННЫЕ СХЕМЫ · %d' in reference,
    'scheme card has explicit hierarchy': all(x in implementation for x in ['СХЕМА ХИМИЗАЦИИ\\r\\n', 'ПАРАМЕТРЫ\\r\\n', 'Тип реализации:']),
    'scheme rows are two-line cards': 'drawMethodChemicalSchemeListItem' in implementation and all(x in implementation for x in ['titleRect := RECT', 'metaRect := RECT', 'methodsTypeCompact']),
    'scheme cards show key dosage data': all(x in implementation for x in ['methodChemicalSchemeFieldValue(scheme, "reagent")', 'methodChemicalSchemeFieldValue(scheme, "rate")', 'methodChemicalSchemeFieldValue(scheme, "unit")', 'methodChemicalSchemeFieldValue(scheme, "basis")']),
    'scheme cards use brand selection': all(x in implementation for x in ['fill = brushNav', 'accent := RECT', 'brushHeader']),
    'chooser scheme list is owner drawn': 'LBS_OWNERDRAWFIXED|LBS_HASSTRINGS' in implementation and 'LB_SETITEMHEIGHT' in implementation,
    'reference scheme list is owner drawn': 'LBS_OWNERDRAWFIXED|LBS_HASSTRINGS' in reference and 'drawMethodChemicalSchemeListItem(di, methodsReferenceCurrentSchemes)' in reference,
    'persistent full chemical schemes': all(x in implementation for x in ['method_chemical_schemes.json', 'MethodChemicalScheme', 'saveMethodChemicalScheme', 'СОХРАНИТЬ ТЕКУЩУЮ СХЕМУ В СПРАВОЧНИК']),
    'scheme chooser applies all fields': all(x in implementation for x in ['ВЫБРАТЬ СХЕМУ ХИМИЗАЦИИ ИЗ СПРАВОЧНИКА', 'compatibleMethodChemicalSchemes', 'applyMethodChemicalScheme']),
    'missing scheme guidance': 'Заполните параметры вручную и сохраните всю схему в справочник' in implementation,
    'stored implementation reagents are reused': 'ensureMethodImplementationRecordsLoaded()' in implementation and 'field.Key == "reagent"' in implementation,
    'manual changes stay audited': 'appendMethodImplementationAudit' in implementation,
    'reference has full schemes subsection': all(x in reference for x in ['methodsReferenceTabReagents', '"РЕАГЕНТЫ"', 'СОХРАНЁННЫЕ СХЕМЫ ХИМИЗАЦИИ', 'ПОЛНАЯ СХЕМА ХИМИЗАЦИИ']),
    'exact scheme duplicate warning': 'схема химизации с такой совокупностью параметров уже существует. Дублирование не выполнено' in implementation and 'MB_ICONWARNING' in implementation,
    'reference shows scheme card': 'methodChemicalSchemeDetails(methodsReferenceCurrentSchemes[i])' in reference,
    'single reagent add removed from reference': 'saveMethodReagent(value)' not in reference,
    'three separate parameter groups': 'methodImplementationGroupPanels' in implementation and 'groupRects := [3][4]int32' in implementation,
    'dosage cards visibly split': '[3][4]int32{{20, 108, 514, 238}, {546, 108, 538, 238}, {20, 358, 1064, 162}}' in implementation,
    'parameter cards have borders': 'WS_CHILD|WS_VISIBLE|WS_BORDER, r[0], r[1], r[2], r[3]' in implementation,
    'reagent reference card has border': 'methodImplementationReferencePanel = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER' in implementation,
    'two-column field grid': all(x in implementation for x in ['methodImplementationEditorSlot', 'int32(slot%2)*244', 'width = 228']),
    'object badge in header': 'methodImplementationObjectBadge' in implementation and 'ОБЪЕКТ · ' in implementation,
    'dedicated reagent reference card': all(x in implementation for x in ['methodImplementationReferencePanel', '04 · РЕАГЕНТЫ · СПРАВОЧНИК СХЕМ ХИМИЗАЦИИ', 'methodImplementationReagentSummary']),
    'quiet action footer': all(x in implementation for x in ['СОХРАНИТЬ ПАРАМЕТРЫ', 'ВОССТАНОВИТЬ ПРЕДЛОЖЕННЫЕ', 'methodImplementationBottomRule']),
    'single day abbreviation period': 'this prevents "сут.."' in (root / 'methods_workflow.go').read_text(encoding='utf-8'),
    'basis contribution has no embedded terminal period': 'fmt.Sprintf(" ≈ +%.0f сут"' in (root / 'methods_workflow.go').read_text(encoding='utf-8'),
    'main methods workspace is stacked': all(x in (root / 'methods_workflow.go').read_text(encoding='utf-8') for x in ['leftW := contentW * 58 / 100', 'layoutMethodsWorkspacePanelsStacked', 'rightBottomY := y + rightTopH + rightCardGap']),
    'main reagent block title explicit': '03 · РЕАГЕНТЫ И ПАРАМЕТРЫ' in (root / 'methods_workflow.go').read_text(encoding='utf-8'),
    'dosage units follow basis': all(x in implementation for x in ['methodImplementationUnitOptionsForBasis', 'refreshMethodImplementationUnitCombo', 'strings.Contains(basis, "нефт")']),
    'manual scheme status': 'ручных изменений: %d' in implementation,
    'unit visually follows dosage basis': implementation.index('case "basis":\n\t\t\treturn 1') < implementation.index('case "unit":\n\t\t\treturn 2') < implementation.index('case "rate":\n\t\t\treturn 3'),
    'explicit reagent subsection': '04 · РЕАГЕНТЫ · СПРАВОЧНИК СХЕМ ХИМИЗАЦИИ' in implementation,
    'full scheme summary visible': all(x in implementation for x in ['База: %s · Дозирование: %s', 'Точка: %s · Периодичность: %s · Контроль: %s']),
    'main methods page exposes reagent workflow': all(x in (root / 'methods_workflow.go').read_text(encoding='utf-8') for x in ['03 · РЕАГЕНТЫ И ПАРАМЕТРЫ', 'ПАРАМЕТРЫ И РЕАГЕНТЫ']),
}

for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
