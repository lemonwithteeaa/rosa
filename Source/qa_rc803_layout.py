from pathlib import Path
import json, re
root = Path(__file__).resolve().parent
src = {n: (root / n).read_text(encoding='utf-8') for n in ('main.go','methods_workflow.go','methods_plan_archive.go','stability_frame_windows.go','laboratory.go','assessment_classification.go')}
corpus = json.loads((root / 'training_corpus.json').read_text(encoding='utf-8'))
main, methods, stability = src['main.go'], src['methods_workflow.go'], src['stability_frame_windows.go']
switch = src['assessment_classification.go'].split('func switchAssessmentInnerTab(')[1].split('\nfunc showAssessmentInnerControls(')[0]
archive = methods.split('func refreshMethodsArchive()')[1].split('\nfunc addSelectedMethodsWorkflowMethod(')[0]
checks = {
 '20 wells in embedded corpus': len(corpus['wells']) == 20,
 '60 failure-linked method decisions retained': sum(len(w['failures']) for w in corpus['wells']) == 60,
 '180 laboratory passes retained': sum(len(w['labPasses']) for w in corpus['wells']) == 180,
 'method archive iterates all 20 objects': 'ordinal <= 20' in archive and 'teachingMethodArchiveDecisions(objectKey, time.Now())' in archive,
 'method archive has no four-row placeholder': 'row < 4' not in archive,
 'real registry is also retained': 'approvedMethodDecisions[i]' in archive,
 'archive native row count is verified': 'LVM_GETITEMCOUNT' not in archive or 'lvmGetItemCount' in archive,
 'methods layout commits native geometry together': 'beginLayoutBatch()' in methods.split('func layoutMethodsWorkflow(')[1].split('func methodsCandidateTitlesNeedTwoCardPage(')[0],
 'lab layout commits native geometry together': 'beginLayoutBatch()' in src['laboratory.go'].split('func layoutLaboratory(w, h int32)')[1].split('\nfunc mergeImportedLabRecords(')[0],
 'assessment inner layout batches geometry': 'beginLayoutBatch()' in src['assessment_classification.go'].split('func layoutAssessmentInnerTabs()')[1],
 'native transaction is DeferWindowPos': 'pDeferWindowPos.Call' in stability and 'pEndDeferWindowPos.Call' in stability,
 'no top-level redraw suppression': 'pSendMessageW.Call(uintptr(hMain), WM_SETREDRAW' not in main,
 'no duplicate full-child frame invalidation': 'for _, h := range pageControls[page]' not in main.split('func invalidateVisibleFrame(')[1].split('\nfunc applyActivePageVisibility(')[0],
 'same-value caption repaints skipped': 'getText(h) == s' in main.split('func setText(')[1].split('func getText(')[0],
 'assessment inner tab does not rebuild all three ListViews': 'refreshAssessmentClassTabs()' not in switch,
 'method archive retains distinct labels for synthetic history': 'УЧЕБНЫЙ ПРИМЕР' in archive,
 'lab list avoids rebuilding identical native rows': 'labHistoryRenderSignature' in src['laboratory.go'] and 'signature != labHistoryRenderSignature' in src['laboratory.go'],
 'short window reserves dedicated archive area': 'archiveReserve = 185' in methods and 'workspaceMaxH = 120' in methods,
 'the archive object dialog includes synthetic and real histories': 'rows = append(rows, teachingMethodArchiveDecisions(objectKey, time.Now())...)' in src['methods_plan_archive.go'],
}
for label, ok in checks.items():
 print(('PASS' if ok else 'FAIL') + ': ' + label)
if not all(checks.values()): raise SystemExit(1)
print(f'CHECKS {sum(checks.values())}/{len(checks)}')
