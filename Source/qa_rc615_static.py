from pathlib import Path
root=Path(__file__).resolve().parent
files={p.name:p.read_text(encoding='utf-8',errors='ignore') for p in root.glob('*.go')}
main=files['main.go']; mw=files['methods_workflow.go']; tc=files['teaching_cross_history.go']; tf=files['teaching_failure_history.go']; lab=files['laboratory.go']; ac=files['assessment_classification.go']; mpa=files['methods_plan_archive.go']; demo=files['demo_seed.go']
checks=[]
def ck(name, cond):
    checks.append((name,bool(cond)))

ck('version 615', 'const appBuildVersion = "7.3.11.615-RC"' in main)
ck('reference proportions 44/27/29', 'leftW := contentW * 44 / 100' in mw and 'midW := contentW * 27 / 100' in mw)
ck('zone 01 numbered', '01 · ДОСТУПНЫЕ МЕТОДЫ' in mw)
ck('zone 02 numbered', '02 · ОСНОВАНИЕ ВЫБОРА' in mw)
ck('zone 03 numbered', '03 · ПАРАМЕТРЫ РЕАЛИЗАЦИИ' in mw)
ck('scrollable basis vertical only', 'func methodsScrollableCard' in mw and 'WS_VSCROLL' in mw and 'ES_AUTOVSCROLL' in mw and 'WS_HSCROLL' not in mw[mw.index('func methodsScrollableCard'):mw.index('func buildMethodsWorkflowUI')])
ck('basis uses scrollable card', 'methodsSuggestionInfo = methodsScrollableCard' in mw)
ck('implementation uses scrollable card', 'methodsImplementation = methodsScrollableCard' in mw)
ck('persistent plan hidden', 'show(methodsPlanTitle, false)' in mw and 'show(methodsPlanList, false)' in mw and 'show(methodsPlanComment, false)' in mw)
ck('plan action preserved', 'methodsPlanButton = button' in mw and 'СФОРМИРОВАТЬ ПЛАН' in mw)
ck('archive title', 'ПРЕДЫДУЩИЕ РЕШЕНИЯ ПО ОБЪЕКТАМ' in mw)
ck('archive version column', '{"Версия", 82}' in mw)
ck('archive four-row reserve', 'archiveReserve := int32(142)' in mw)
ck('candidate selection page-aware', 'methodsCandidateIndexForCardID(id)' in main)
ck('candidate cards never shrink', 'id >= ID_METHODS_CARD_BASE && id < ID_METHODS_CARD_BASE+ID_METHODS_CARD_COUNT' in main[main.index('func buttonUsesUniformStandardFont'):main.index('func button(')])
ck('selected card calm fill', 'methodCandidateSelected' in main and 'br = brushBorder' in main)
ck('forecast green removed', 'child == methodsForecastGain' in main and 'return uintptr(brushWhite)' in main[main.index('if child == methodsForecastGain'):main.index('if child == methodsForecastGain')+350])
ck('card metrics three-way', 'Прим. %d%% · дост. %d%% · %s' in mw and 'ProbabilityAvailable' in mw)
ck('basis exactly five numbered lines', all(f'"{i}. ' in mw[mw.index('func methodsBasisSummaryText'):mw.index('func methodVisualType')] for i in range(1,6)))
ck('cross history helper exists', 'func teachingCrossHistoryPoints' in tc)
ck('cross history exactly four templates', tc.count('Equipment:') >= 4 and 'offsets := []int{5, 14, 24, 33}' in tc)
ck('four equipment contexts', all(x in tc for x in ['"УЭЦН"','"УШГН"','"УЭВН"','"ОРД (УЭЦН + ШГН)"']))
ck('mixed ASPО viscosity', 'АСПО + высокая вязкость' in tc and 'А2 + В2' in tc)
ck('mixed salts corrosion solids', 'Соли + коррозия + мехпримеси' in tc and 'С2 + К1 + М2' in tc)
ck('failure history uses shared timeline', 'teachingCrossHistoryPoints(objectKey, now)' in tf)
ck('mrp history uses shared timeline', 'teachingCrossHistoryPoints(objectKey, time.Now())' in demo)
ck('lab history uses shared teaching rows', 'teachingLaboratoryHistoryEntries(selectedObjectKey, time.Now())' in lab)
ck('assessment history uses shared teaching rows', 'teachingAssessmentHistoryRows(selectedObjectKey, time.Now())' in ac)
ck('methods archive uses shared teaching rows', 'teachingMethodArchiveDecisions(selectedObjectKey, time.Now())' in mw)
ck('teaching lab restore protected', 'entry.Kind == "teaching"' in lab and 'не заменяет текущий лабораторный анализ' in lab)
ck('teaching assessment restore protected', 'teachingAssessmentHistoryRows(selectedObjectKey, time.Now())' in main and 'не восстанавливается как производственное решение' in main)
ck('teaching method restore protected', 'Синтетическая учебная версия не восстанавливается' in mpa)
ck('teaching method status explicit', 'return "УЧЕБНЫЙ ПРИМЕР"' in mpa)
ck('four-type current equipment rotation', 'ОРД: УЭЦН + ШГН (учебный контекст)' in demo)


# Late method-plan feedback contracts retained from 593–594.
plan_main=files['main.go']; plan_src=files['methods_plan_archive.go']
ck('plan structured outcome stored', all(x in plan_main for x in ['ControlOutcome   string','ControlComment   string','CompletedAt      string','ExecutionResult  string']))
ck('plan editor expanded', '920, 735' in plan_src)
ck('plan four control outcomes', all(x in plan_src for x in ['Оценка ещё не завершена','Эффект подтверждён','Подтверждён частично','Не подтверждён']))
ck('plan completed requires fact', 'фактическую дату выполнения и результат' in plan_src)
ck('pending control requires comment', 'Если контроль ещё не завершён' in plan_src)
ck('restored version drops old fact', 'Historical execution/control facts' in plan_src and 'p0.CompletedAt, p0.ExecutionResult = "", ""' in plan_src)
ck('methods report exports fact and outcome', 'итог контроля %s; комментарий %s' in mw)

passed=sum(v for _,v in checks)
for name,v in checks: print(('PASS' if v else 'FAIL')+': '+name)
print(f'{passed}/{len(checks)} PASS')
raise SystemExit(0 if passed==len(checks) else 1)
