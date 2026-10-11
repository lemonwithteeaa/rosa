from pathlib import Path
import hashlib
import re

root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
methods = (root/'methods_workflow.go').read_text(encoding='utf-8')
switch = (root/'user_switch.go').read_text(encoding='utf-8')
release = (root/'build_release.py').read_text(encoding='utf-8')
background = (root/'methods_background.go').read_text(encoding='utf-8')
reference = (root/'methods_reference.go').read_text(encoding='utf-8')
parameter = (root/'methods_parameter_panel.go').read_text(encoding='utf-8')
typography = (root/'methods_typography.go').read_text(encoding='utf-8')
failure_ui = (root/'well_card_ui.go').read_text(encoding='utf-8')
failure_extra = (root/'well_failure_extra.go').read_text(encoding='utf-8')
failure_import = (root/'failure_import_windows.go').read_text(encoding='utf-8')
checks = {
    'show all methods command': 'ID_METHODS_SHOW_ALL' in methods and 'ПОКАЗАТЬ ВСЕ МЕТОДЫ' in methods,
    'full catalog merge': 'mergeAllMethodSuggestions' in methods and 'ruleCatalog.MethodCatalog' in methods,
    'full catalog is unbounded': 'rankMethodSuggestionsWithLimit(candidates, scores, 0)' in background,
    'outside profile has no artificial score': 'methodOutsideProfileAssessment' in background,
    'pager contained in header': 'y+3, pageButtonW, 30' in methods and 'cardTop := y + 41' in methods,
    'workspace controls have safe inset': 'workspaceInset := int32(7)' in methods and all(x in methods for x in ['leftInnerX', 'midInnerX', 'rightInnerX']),
    'cards keep bottom padding': 'workspaceH-48-cardGapY' in methods,
    'full catalogue caption has guaranteed width': 'showAllW := min32(210, max32(186' in methods and 'methodsFont(methodsTypeCompact, true)' in main,
    'catalogue mode has active visual state': 'if methodsShowAll {' in main[main.index('if h == methodsShowAllButton'):],
    'forecast result is emphasized': 'if child == methodsForecastResult' in main and 'rgb(232, 245, 233)' in main and 'resultW := forecastUsable * 30 / 100' in methods,
    'selected plan count is visible': 'в плане: %d' in methods,
    'long candidate titles use measured height': 'methodsCandidateCardHeights(titleTextH, detailTextH, cardH)' in methods,
    'candidate cards favor readability': 'perPage := 4' in methods and 'perPage := 6' not in methods,
    'candidate owner draw uses matching font': 'if methodCandidateCard {' in main and 'drawFont = methodsFont(methodsTypeBody, true)' in main,
    'four-level Methods type scale': all(x in typography for x in ['methodsTypeDisplay = 24','methodsTypeSection = 15','methodsTypeBody    = 13','methodsTypeCompact = 12']),
    'forecast uses shared section token': 'fitMethodsControlText(methodsForecastResult, methodsTypeSection, true, 0)' in typography,
    'Methods type fitting uses only tokens': 'for _, size := range []int{methodsTypeDisplay, methodsTypeSection, methodsTypeBody, methodsTypeCompact}' in typography,
    'no former one-off Methods sizes': all(x not in typography for x in ['methodsFont(19,','methodsFont(16,','methodsFont(14,','methodsFont(23,']),

    'complete workspace panels': 'layoutMethodsWorkspacePanels' in methods,
    'classification chips': 'layoutMethodsClassChips' in methods,

    'reference does not search fleet': 'methodReplicationAssessmentForSuggestion' not in reference[reference.index('func selectedMethodReferenceText'):reference.index('func methodsReferenceText')],
    'reference uses prepared evaluation': 'finish:=beginMethodsEvaluation()' in reference.replace(' ', '').replace('\t',''),
    'parameters wrap without horizontal scrolling': 'ES_MULTILINE' in parameter and 'ES_AUTOHSCROLL' not in parameter and 'WS_HSCROLL' not in parameter,
    'measured parameter row heights': 'methodsParameterRowHeights' in parameter and 'methodsMeasuredTextHeight' in parameter,
    'complete parameter fallback': 'if limit > 2' not in (root/'methods_implementation.go').read_text(),
    'numbered native basis panel': 'layoutMethodsBasisPanel' in methods,

    'background candidates': 'go func()' in background and 'calculateMethodApplicabilityFromSnapshot' in background,
    'no automatic fleet replication': 'methodReplicationAssessmentForSuggestion' not in methods[methods.index('func refreshMethodsSuggestionInfo'):methods.index('func relayoutMethodsWorkflowCurrent')],
    'no survival training in UI snapshot': 'calculateMethodForecastSnapshot()' not in background,
    'session cancels worker': 'invalidateMethodsBackground()' in switch,
    'actual GDI text measurement': 'DT_CALCRECT' in (root/'methods_typography.go').read_text(),
    'parameter rows retained': 'layoutMethodsParameterPanel' in methods,

    'classification approve ID does not log out': 'ID_ASSESS_CONFIRM_FULL    = 9610' in main and 'ID_HEADER_EXIT            = 9441' in main,
    'register chooser face is aligned': 'selectorGap := int32(14)' in failure_ui and 'selectorButtonW, 30)' in failure_ui,
    'manual form has deliberate spacing': 'toolbarGap := int32(14)' in failure_ui and 'actionInset := int32(18)' in failure_ui and 'formInset := int32(8)' in failure_ui,
    'manual tab uses calm active accent': 'Both top-level failure tabs use the same calm navigation accent' in main,
    'catalogue actions are quiet': 'Reference/catalogue actions stay quiet' in main,
    'failure sections use NKT before Pump and keep Rods': '3 ● НКТ' in failure_ui and '4 ● НАСОС' in failure_ui and '6 ● ШТАНГИ' in failure_ui and 'wellSectionButtons [7]' in failure_extra,
    'failure actions are on separate row': 'actionControls := []syscall.Handle{wellCatalogButton, wellReferenceButton, wellManualEntryButton}' in failure_ui,
    'rod block contains seven fields': all(('"'+x+'"') in failure_extra for x in ['BH','BI','BJ','BK','BL','BM','BN']),
    'rod manual fields stay free text': 'case "BJ", "BK", "BL", "BN"' in failure_extra,
    'all editable selectors show no selection': 'CB_SETCURSEL, 0, 0' in failure_extra and 'failureNoSelection' in failure_extra,
    'register object chooser is visible locally': 'registerChooser :=' in failure_ui and 'failurePreviewObjectKey' in failure_import,
    'Methods archive consumes full width': 'fitWellFailureListColumns(methodsArchiveList' in methods,
    'obsolete ORD GNO 3 and 4 absent': '"I", "Типоразмер ГНО 3' not in failure_extra and '"J", "Типоразмер ГНО 4' not in failure_extra,
    'version 626': 'const appBuildVersion = "7.3.11.626-RC"' in main,
    'approved ICO hash': hashlib.sha256((root/'OF_Model_icon_round.ico').read_bytes()).hexdigest() == 'd63b3dea920b18114fd62da4ddaa92cad91979663e732e523242909d06ac244a',
    'runtime ICO embedded': '//go:embed OF_Model_icon_round.ico' in main,
    'shared switch handler': all('switchCurrentUser()' in main[main.index(x):main.index(x)+130] for x in ['if id == ID_HEADER_EXIT {','case ID_HEADER_APPROVE:']),
    'user caption': 'const userSwitchCaption = "Смена пользователя"' in switch,
    'user switch returns visible dashboard': all(x in switch for x in ['ensureDataWithVisibleTransition()','activatePage(ID_DASH, true)','SW_MAXIMIZE','WM_SETICON']),
    'release uses source version': 're.search' in release and 'appBuildVersion' in release,
    'mandatory binary icon verification': 'verify_icon_resource.py' in release,
    'grades outside comparator': 'ai := scores[out[i].Method.Name]' in methods and 'aj := scores[out[j].Method.Name]' in methods,
    'hidden tables not repopulated': 'listAdd(' not in methods[methods.index('func refreshMethodsVisualTables'):methods.index('func methodsKeyDataText')],
    'native checkboxes': 'BS_CHECKBOX' in methods and 'BM_SETCHECK' in methods,
    'native cards have no owner draw': 'BS_OWNERDRAW' not in methods[methods.index('func methodsSafeCandidateButton'):methods.index('func methodsScrollableCard')],
    'draft toggle': 'setCombinedMethods(key, next)' in methods,
    'explicit empty draft cannot be approved': 'выберите хотя бы один метод для утверждения' in methods,
    'reference proportions': 'leftW := contentW * 44 / 100' in methods and 'midW := contentW * 27 / 100' in methods,
    'adaptive paging': 'perPage = 2' in methods and 'perPage := 4' in methods,
    'plan, archive and formats remain': all(x in methods for x in ['openMethodsPlanEditor','methodsArchiveList','exportMethodsWorkflowReport("xlsx")','exportMethodsWorkflowReport("docx")','exportMethodsWorkflowReport("pdf")']) or all(x in methods for x in ['methodsPlanButton','methodsArchiveList','exportMethodsWorkflowReport("xlsx")','exportMethodsWorkflowReport("docx")','exportMethodsWorkflowReport("pdf")']),
}
for name, ok in checks.items(): print(('PASS' if ok else 'FAIL')+': '+name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
