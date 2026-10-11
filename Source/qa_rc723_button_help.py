from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
access=(root/'access_control.go').read_text(encoding='utf-8')
auth=(root/'auth.go').read_text(encoding='utf-8')
help_src=(root/'button_context_help.go').read_text(encoding='utf-8')
checks={
  'version 723': 'const appBuildVersion = "7.3.11.723-RC"' in main,
  'global context-help source': 'func registerButtonContextHelp' in help_src and 'func buttonContextHelpWndProc' in help_src,
  'right click intercepted': 'msg == wmRButtonUpButtonHelp || msg == WM_CONTEXTMENU' in help_src and 'showButtonContextHelp' in help_src,
  'main-page buttons registered': 'registerButtonContextHelp(z, class)' in main,
  'modal buttons registered': 'registerButtonContextHelp(z, class)' in access,
  'auth buttons registered': 'registerButtonContextHelp(z, class)' in auth,
  'all button classes covered': 'strings.EqualFold(strings.TrimSpace(className), "BUTTON")' in help_src,
  'right click never executes action': 'return 0' in help_src[help_src.index('if msg == wmRButtonUpButtonHelp || msg == WM_CONTEXTMENU'):help_src.index('oldAny, ok :=')],
  'root-owned visible popup': 'GetAncestor' in help_src and 'MB_OK|MB_ICONINFORMATION' in help_src,
  'variant duplicate explanation': 'Создаёт копию выбранного варианта решения' in help_src,
  'variant compare explanation': 'Открывает сравнение вариантов по методам' in help_src,
  'generic future-button fallback': 'Выполняет действие «%s» для текущего экрана' in help_src,
  'navigation help': 'captionLooksLikeSectionNavigation' in help_src,
  'common verbs covered': all(x in help_src for x in ['strings.Contains(c, "выбрать")','strings.Contains(c, "сохран")','strings.Contains(c, "обнов")','strings.Contains(c, "выгруз")','strings.Contains(c, "удал")']),
  'left-right usage text': 'Левая кнопка мыши — выполнить действие. Правая кнопка — показать эту подсказку.' in help_src,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+' - '+k)
if failed: raise SystemExit('RC723 button-help QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
