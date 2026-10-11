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
    'pager contained in header': 'y+2, pageButtonW, 34' in methods and 'cardTop := y + 41' in methods,
    'responsive readable methods controls': 'toolbarReserve, toolbarH := int32(42), int32(38)' in methods and 'if h < 960 {' in methods,
    'workspace controls have safe inset': 'workspaceInset := int32(9)' in methods and all(x in methods for x in ['leftInnerX', 'rightInnerX']),
    'cards keep bottom padding': 'workspaceH-48-cardGapY' in methods,
    'full catalogue caption has guaranteed width': 'showAllW := min32(210, max32(186' in methods and 'methodsFont(methodsTypeBody, true)' in main,
    'failure section tabs use standard font': 'id >= ID_WELL_SECTION_BASE && id < ID_WELL_SECTION_BASE+7' in main and 'if sectionCaption && hFontButton != 0' in main,
    'catalogue mode has active visual state': 'if methodsShowAll {' in main[main.index('if h == methodsShowAllButton'):],
    'forecast result is emphasized': 'if child == methodsForecastResult' in main and 'rgb(232, 245, 233)' in main and 'forecastUsable / 4' in methods,
    'selected plan count is visible': 'в плане: %d' in methods,
    'long candidate titles use measured height': 'methodsCandidateCardHeights(titleTextH, detailTextH, cardH)' in methods,
    'candidate cards favor readability': 'perPage := 4' in methods and 'perPage := 6' not in methods,
    'candidate owner draw uses matching font': 'if methodCandidateCard {' in main and 'drawFont = methodsFont(methodsTypeBody, true)' in main,
    'four-level Methods type scale': all(x in typography for x in ['methodsTypeDisplay = 26','methodsTypeSection = 19','methodsTypeBody    = 16','methodsTypeCompact = 14']),
    'forecast uses shared body token': 'methodsForecastResult' in typography and
        'methodsTypeBody, true, 0' in typography[typography.index('// The result remains'):typography.index('for i, card')],
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
    'register chooser face is aligned': (('selectorGap := int32(14)' in failure_ui and 'selectorButtonW, 30)' in failure_ui) or ('selectorGap := int32(10)' in failure_ui and 'show(failureObjectFrame, false)' in failure_ui and 'selectorButtonW, 30)' in failure_ui)),
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
        'failure title and status use separate rows': 'statusY := py(302)' in failure_ui and 'setPos(wellFormHeading, 242, py(270), contentW, 28)' in failure_ui,
    'failure header no longer duplicates object caption': 'setText(wellFormHeading, heading)' in failure_ui and 'heading+" · "+wellFailureObjectCaption' not in failure_ui,
    'failure form relayouts on open': 'updateWellCardVisibility()\n\tresize()' in failure_ui,
    'history wording is current': 'История отказов · 36 мес.' in failure_ui and 'Предыстория · 36 мес.' not in failure_ui,
    'registry blank button is explicit': 'СКАЧАТЬ БЛАНК РЕЕСТРА ОТКАЗОВ' in failure_ui,
    'classification edits invalidate full approval': 'r.FullClassificationConfirmed = false' in (root/'assessment_classification.go').read_text(),
    'methods reject stale classification records': 'assessmentDecisionConsistentForMethods' in methods,
    'method approval bound to classification snapshot': 'classificationSummariesEquivalent' in methods and 'latestApprovedMethodDecisionRaw' in methods,
        'reclass accepts exact program proposal': 'assessmentFinalClassForDecision' in (root/'assessment_classification.go').read_text() and 'return program, nil' in (root/'assessment_classification.go').read_text(),
    'technologist override requires comment': 'assessmentDecisionRequiresComment' in (root/'assessment_classification.go').read_text() and 'при отклонении от реклассификации' in (root/'assessment_classification.go').read_text(),
    'methods require accepted program snapshot': 'expected := strings.TrimSpace(r.ProgramClassAtDecision)' in methods,
    'current registry blank embedded': '//go:embed РОСА_БЛАНК_РЕЕСТРА_ОТКАЗОВ_632-RC.xlsx' in (root/'failure_blank_template.go').read_text(),
    'registry upload button wraps to two rows': 'ЗАГРУЗИТЬ ЗАПОЛНЕННЫЙ\\r\\nРЕЕСТР ОТКАЗОВ' in failure_ui and 'toolbarH := int32(54)' in failure_ui,
    'classification row choice is explicitly pending': 'ОЖИДАЕТ ОБЩЕГО УТВЕРЖДЕНИЯ' in (root/'assessment_classification.go').read_text(),
    'full classification approval is full-width separate row': 'setPos(assessConfirmFullButton, 242, approveY, contentW, approveH)' in (root/'assessment_classification.go').read_text(),
    'short workspace keeps approval and archive visible':
        ('historyVisible := h >= 820' in (root/'assessment_classification.go').read_text()) or
        ('historyVisible := true' in (root/'assessment_classification.go').read_text() and
         'detailAndActions' in (root/'assessment_classification.go').read_text()),
    'approval resolves current selected source snapshot': 'resolved, err := assessmentFinalClassForDecision(r.DecisionMode, r.TechnologistClass, program)' in (root/'assessment_classification.go').read_text() and 'решение по осложнению «%s» устарело' not in (root/'assessment_classification.go').read_text(),
    'classification summary is wrapped and concise': 'Итог переносится в «Методы» только после нажатия' in (root/'assessment_classification.go').read_text(),
    'pending choices are blocked from Methods': 'Pending row-level choices are intentionally not exposed as method inputs' in methods,
    'row choice does not write approval history': 'appendAssessmentDecisionHistory(r)' not in (root/'assessment_classification.go').read_text()[ (root/'assessment_classification.go').read_text().index('func confirmAssessmentFinalClass'): (root/'assessment_classification.go').read_text().index('func confirmAssessmentFullClassification') ],
    'classification batch checkboxes': 'LVS_EX_CHECKBOXES' in main and 'assessmentCheckedCompareDirections' in (root/'assessment_classification.go').read_text(),
    'batch decisions stay pending': 'confirmAssessmentFinalClasses' in (root/'assessment_classification.go').read_text() and 'FullClassificationConfirmed = false' in (root/'assessment_classification.go').read_text(),
    'teaching examples have production rates': all(x in (root/'demo_seed.go').read_text() for x in ['teachingWorkflowRegimeValues','LiquidRate: strconv.FormatFloat(w.LiquidRate','WaterCut: strconv.FormatFloat(w.WaterCut','OilRate: strconv.FormatFloat(w.OilRate']),
    'teaching watercut no longer uses completeness percent': 'bundledDemoWatercutPattern.FindStringSubmatch(name)' not in (root/'demo_seed.go').read_text(),
    'version at least 641 from 632 baseline': int(re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main).group(1)) >= 641,
    'approved ICO hash': hashlib.sha256((root/'OF_Model_icon_round.ico').read_bytes()).hexdigest() == 'd63b3dea920b18114fd62da4ddaa92cad91979663e732e523242909d06ac244a',
    'runtime ICO embedded': '//go:embed OF_Model_icon_round.ico' in main,
    'shared switch handler': all('switchCurrentUser()' in main[main.index(x):main.index(x)+130] for x in ['if id == ID_HEADER_EXIT {','case ID_HEADER_APPROVE:']),
    'user caption': 'const userSwitchCaption = "Смена пользователя"' in switch,
    'user switch returns visible dashboard': all(x in switch for x in ['ensureDataWithVisibleTransition()','activatePage(ID_DASH, true)','SW_MAXIMIZE','WM_SETICON']),
    'release uses source version': 're.search' in release and 'appBuildVersion' in release,
    'mandatory binary icon verification': 'verify_icon_resource.py' in release,
    'grades outside comparator': 'ai := scores[out[i].Method.Name]' in methods and 'aj := scores[out[j].Method.Name]' in methods,
    'hidden tables not repopulated': 'listAdd(' not in methods[methods.index('func refreshMethodsVisualTables'):methods.index('func methodsKeyDataText')],
    'native checkboxes': '0x00000003' in methods and 'BM_SETCHECK' in methods,
    'native cards have no owner draw': 'BS_OWNERDRAW' not in methods[methods.index('func methodsSafeCandidateButton'):methods.index('func methodsScrollableCard')],
    'draft toggle': 'setCombinedMethods(key, next)' in methods,
    'explicit empty draft cannot be approved': 'выберите хотя бы один метод для утверждения' in methods,
    'reference proportions': (
        ('leftW := contentW * 58 / 100' in methods and 'layoutMethodsWorkspacePanelsStacked' in methods)
        or ('leftW := contentW * 50 / 100' in methods and 'midW := contentW * 24 / 100' in methods
            and 'layoutMethodsWorkspacePanels(x, midX, rightX' in methods)
        # RC663 widens the implementation column so seven teaching fields stay readable
        # at the ~900 px reference height without shrinking/overlapping controls.
        or ('leftW := contentW * 44 / 100' in methods and 'midW := contentW * 25 / 100' in methods
            and 'if rightW < 270' in methods and 'layoutMethodsWorkspacePanels(x, midX, rightX' in methods)
    ),
    'adaptive paging': 'perPage = 2' in methods and 'perPage := 4' in methods,
    'plan, archive and formats remain': all(x in methods for x in ['openMethodsPlanEditor','methodsArchiveList','exportMethodsWorkflowReport("xlsx")','exportMethodsWorkflowReport("docx")','exportMethodsWorkflowReport("pdf")']) or all(x in methods for x in ['methodsPlanButton','methodsArchiveList','exportMethodsWorkflowReport("xlsx")','exportMethodsWorkflowReport("docx")','exportMethodsWorkflowReport("pdf")']),
}
for name, ok in checks.items(): print(('PASS' if ok else 'FAIL')+': '+name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
