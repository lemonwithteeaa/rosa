"""Run the actual pure Go model on Linux; this is not a native Windows UI test.

Go AST extraction copies declarations unchanged. OS/data access uses explicit
fixtures. The production Windows tests are compiled separately.
"""
from pathlib import Path
import json, os, subprocess, tempfile

root = Path(__file__).resolve().parent
go_root = root.parent/'Компилятор'/'go-1.23.2'
go = str(go_root/'bin'/'go')
env = dict(os.environ, GOROOT=str(go_root), GOOS='linux', GOARCH='amd64', CGO_ENABLED='1' if os.environ.get('ROSA_QA_RACE') else '0')
extractor = r'''package main
import("encoding/json";"go/ast";"go/parser";"go/token";"os";"strings")
func main(){ out:=map[string]string{}; for _,path:=range os.Args[1:] { b,_:=os.ReadFile(path); fset:=token.NewFileSet(); f,e:=parser.ParseFile(fset,path,b,0); if e!=nil {panic(e)}; for _,d:=range f.Decls { if fn,ok:=d.(*ast.FuncDecl);ok { out[fn.Name.Name]=string(b[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset]) }; if gd,ok:=d.(*ast.GenDecl);ok { for _,sp:=range gd.Specs {if ts,ok:=sp.(*ast.TypeSpec);ok {out[ts.Name.Name]="type "+string(b[fset.Position(ts.Pos()).Offset:fset.Position(ts.End()).Offset])}} } } }; _=strings.TrimSpace; json.NewEncoder(os.Stdout).Encode(out) }
'''
files = ['multi_method_scenario.go','dynamogram_ui.go','main.go','methods_effect_model.go','methods_workflow.go','methods_evaluation.go','method_runlife_history.go','teaching_cross_history.go','teaching_method_effect.go','demo_seed.go','rules.go','assessment_methods.go','assessment_classification.go','well_failures.go','mrp_forecast.go','dynamogram.go','rc617_methods_regression_test.go','rc618_methods_regression_test.go','methods_background.go','target_runlife.go','methods_parameter_panel.go','methods_basis_panel.go','methods_reference.go','rc619_visual_reference_test.go','rc620_all_methods_test.go','rc623_candidate_cards_test.go']
names = '''normalizeMethodList methodsForWorkflowApproval MethodEffectAssessment methodWorkflowSuggestion assessmentMethodCandidate teachingCrossHistoryPoint teachingCrossHistoryPoints teachingWorkflowOrdinal teachingMethodEffect sameMethodFamily combineMethodEffectPercents normalizeMethodText clampFloat clampInt minInt MethodHistoryBasis methodsEvaluationKey methodsEvaluationScope beginMethodsEvaluation methodApplicabilityForSuggestion calculateMethodApplicability methodEffectReferenceBand methodEvidenceScore historyScoreForMethod methodReliabilityLevel methodFailureEvidence failureTextForMethodMatch failureMatchesMethodDirection methodTextMatchesDirection assessmentMethodNeedsLabSelection assessmentMethodIsEquipmentDesign FailureEvent methodHistoryBasisForObject calculateMethodHistoryBasis historyForWell medianFloat64 empiricalQuantile methodSuggestionsForObject buildMethodSuggestionsForObject methodDirectionByName methodClassSeverity assessmentClassDirection newMethodsEvaluationScope calculateMethodApplicabilityFromSnapshot methodProbabilityResult collectMethodSuggestionsForObject collectAllMethodSuggestionsForObject rankMethodSuggestions VersionRecord Outcome empiricalMethodProbabilityFromSnapshot indexMethodOutcomes methodNamesComparable methodProbabilityObservation normalizeTargetLiftType methodsBackgroundResult methodsBackgroundResultIsCurrent methodsBaselineFromSnapshot MRPObservation MRPRegimeSnapshot methodTextMatchesNormalizedDirection methodNamesComparableNormalized methodProbabilityOutcomeObservation methodProbabilityCohortRow prepareMethodProbabilityCohort empiricalMethodProbabilityFromCohort refreshMethodsWorkflowUI invalidateMethodsBackground completeMethodsEvaluation boolToUintptr methodsParameterRowHeights methodsBasisItemText selectedMethodReferenceText methodVisualType methodPriorityLabel formatMethodApplicability methodForecastSnapshotForSelectedObject mergeAllMethodSuggestions methodSuggestionOutsideCurrentProfile methodOutsideProfileAssessment rankMethodSuggestionsWithLimit methodsCandidateCardHeights MethodCatalogEntry'''.split()
test_names = ['TestRC617UncheckedDraftDoesNotApproveSelectedFallback','TestRC617TeachingForecastsDerivedFromEffects','TestRC617RefreshCacheEndsWithAction','TestRC617ProductionDoesNotReceiveTeachingEffects','TestRC618StaleWorkerCannotUpdateAnotherSessionOrObject','TestRC618ProbabilityUsesImmutableIndexedOutcomes','TestRC618ObjectBaselineWindowAndFallback','BenchmarkRC618ProbabilityIndexed50000','TestRC618NumericLessonsRankBeforeQualitativeActions','TestRC619ParameterRowsFitAllWrappedValuesOrFallback','TestRC619ReferenceUsesPreparedObjectEstimate','TestRC619BasisNumberingIsSeparateFromFullText','TestRC620AllMethodsIncludesWholeCatalogWithoutDuplicates','TestRC620RecommendedModeStillCapsCandidateWork','TestRC623LongMethodTitleKeepsMeasuredHeight','TestRC623CandidateCardHeightDegradesWithoutOverflow']
with tempfile.TemporaryDirectory(prefix='rosa629-qa-') as temp:
    dst=Path(temp)
    (dst/'extract.go').write_text(extractor)
    subprocess.run([go,'build','-o',str(dst/'extract'),str(dst/'extract.go')],env=env,check=True)
    declarations=json.loads(subprocess.check_output([str(dst/'extract')]+[str(root/f) for f in files]))
    code='package main\nimport("encoding/json";"sync";"sync/atomic";"fmt";"math";"regexp";"sort";"strconv";"strings";"time")\n'
    code+='\n\n'.join(declarations[n] for n in names)
    # Execute the actual native layout without a display: only setPos/show are
    # recorded. This checks coordinates, not font rendering or native painting.
    import re
    layout = declarations['layoutMethodsWorkflow']
    handles = sorted(set(re.findall(r'\bmethods[A-Z]\w*\b',layout))-set(declarations)-{
        'methodsCardsPerPage','methodsCandidatePage','methodsClassificationReady',
        'methodsCandidateCards','methodsCandidateDetails','methodsPreparedScope','methodsPreparedKey','methodsShowAll','methodsMeasuredTextHeight'})
    code += '\n'+layout+'\n'+declarations['min32']+'\n'+declarations['max32']+'\n'
    for i,name in enumerate(handles,1): code+=f'\nvar {name} uintptr = {i}\n'
    code += r'''
const PAGE_Y_OFFSET=88
var methodsCardsPerPage=6
var methodsCandidatePage int
var methodsClassificationReady bool
var methodWorkflowSuggestions []methodWorkflowSuggestion
var methodsCandidateCards=[6]uintptr{100,101,102,103,104,105}
var methodsCandidateDetails=[6]uintptr{110,111,112,113,114,115}
type rectangle struct{x,y,w,h int32}
var rectangles=map[uintptr]rectangle{}
var visibility=map[uintptr]bool{}
func setPos(handle uintptr,x,y,w,h int32){rectangles[handle]=rectangle{x,y,w,h}}
func show(handle uintptr,v bool){visibility[handle]=v}
func fitWellFailureListColumns(list uintptr,base []int32){}
func py(y int32)int32{return y+PAGE_Y_OFFSET}
func diagLog(format string, args ...any){}
func refreshMethodsCandidateCards(){}
func fitMethodsVisibleText(){}
func fitMethodsControlText(h uintptr,size int,bold bool,inset int32){}
func getText(h uintptr)string{return "Контроль остаточной концентрации ингибитора солеотложений"}
func methodsMeasuredTextHeight(text string,width int32,size int,bold bool)int32{return 40}
func hideMethodsParameterPanel(){}
func layoutMethodsParameterPanel(x,y,w,h int32){}
func hideMethodsBasisPanel(){}
func layoutMethodsBasisPanel(x,y,w,h int32){}
func layoutMethodsClassChips(x,y,w,h int32){}
var methodsWorkspacePanels=[3]uintptr{200,201,202}
var methodsBasisNumbers=[5]uintptr{210,211,212,213,214}
var methodsBasisItems=[5]uintptr{220,221,222,223,224}
var methodsParameterLabels=[6]uintptr{230,231,232,233,234,235}
var methodsParameterValues=[6]uintptr{240,241,242,243,244,245}
func layoutMethodsWorkspacePanels(x,mx,rx,y,lw,mw,rw,h int32){
 rectangles[methodsWorkspacePanels[0]]=rectangle{x,y,lw,h}
 rectangles[methodsWorkspacePanels[1]]=rectangle{mx,y,mw,h}
 rectangles[methodsWorkspacePanels[2]]=rectangle{rx,y,rw,h}
}
var methodsPreparedScope *methodsEvaluationScope
var methodsPreparedKey,selectedObjectKey,methodsShowAllObject string
var methodsShowAll bool

'''
    code+=r'''
var methodsEvaluation *methodsEvaluationScope
var bundledDemoObjectPattern = regexp.MustCompile(`(?i)^Учебный пример\s+(\d{3})\s+·\s+(.+)$`)
var failureEvents []FailureEvent
var combinedScenarioMethods = map[string][]string{}
type methodForecastSnapshot struct { Low, Base, High float64; Source string; History MethodHistoryBasis }
var assessmentClassDirections = []assessmentClassDirection{{Name:"Солеотложения",LabGroup:"scale"},{Name:"Коррозия",LabGroup:"corrosion"}}
type AssessmentClassRecord struct{Direction, FinalClass string}
var ruleCatalog struct{MethodCatalog []MethodCatalogEntry}
func assessmentMethodSource(m MethodCatalogEntry)(string,string){return "Справочник","catalog"}
func assessmentMethodStatus(name string)string{return "Проверен"}
func methodsReferenceDirectionText(name string)string{return "Все направления"}
func approvedAssessmentForMethods(key string)(string,bool,[]AssessmentClassRecord){ return "",true,[]AssessmentClassRecord{{"Солеотложения","С2"},{"Коррозия","К2"}} }
func assessmentMethodCandidatesForGroup(group string)[]assessmentMethodCandidate {
 out:=[]assessmentMethodCandidate{}; for i:=0;i<24;i++ { name:=fmt.Sprintf("Ингибитор солеотложений %02d",i); if group=="corrosion" {name=fmt.Sprintf("Ингибитор коррозии %02d",i)}; out=append(out,assessmentMethodCandidate{Name:name,Source:"Технологический модуль"}) }; return out
}
func collectHistoryForWell(key string, now time.Time)[]FailureEvent { return failureEvents }
func empiricalMethodEffectProbability(method,direction,key string)(bool,int,int,int,int,int,int){ return false,0,0,0,0,0,0 }
func failureDate(s string)(time.Time,error){return time.Parse("02.01.2006",s)}
func dash(s string)string{if s=="" {return "—"};return s}
const methodForecastHistoryMonths=36
'''
    code+=r'''
var methodsBackgroundGeneration atomic.Uint64
var methodsBackgroundMu sync.Mutex
var methodsBackgroundReady *methodsBackgroundResult
var methodsBackgroundBusy bool
var methodsResumeSelectionName string
func methodsImplementationText(key string)string{return "Все параметры реализации"}
func assessmentMethodDetailText(s assessmentMethodCandidate)string{return "Проверенные источники"}
func calculateMethodForecastSnapshot()methodForecastSnapshot{panic("unexpected uncached forecast in reference")}
var versions []VersionRecord
var outcomes []Outcome
var postedMethodsTokens=make(chan uint64,16)
var hMain uintptr=1
const ID_ACTIVITIES=4
const WM_APP_METHODS_DONE=0x8006
var currentPage=ID_ACTIVITIES
var renderedMethods int
func methodsSelectedSuggestion()(methodWorkflowSuggestion,bool){return methodWorkflowSuggestion{},false}
type snapshotState struct{CurrentMRP string}
func currentSelectedState()(snapshotState,bool){return snapshotState{"876"},true}
func toFloat(s string)float64{n,_:=strconv.ParseFloat(s,64);return n}
func mrpObservationHistoryForObject(key string)[]MRPObservation{return nil}
func methodObjectEquipment(key string)string{return "УЭЦН"}
func setText(h uintptr,s string){}
func relayoutMethodsWorkflowCurrent(){}
func renderMethodsWorkflowUI(){renderedMethods++}
func showMethodsSafeFallback(s string){panic(s)}
type nativeStub struct{post bool}
func(n nativeStub)Call(args ...uintptr)(uintptr,uintptr,error){if n.post{postedMethodsTokens<-uint64(args[2])};return 1,0,nil}
var pEnableWindow=nativeStub{}
var pPostMessageW=nativeStub{true}
var pInvalidateRect=nativeStub{}
'''
    (dst/'model.go').write_text(code)
    tests='package main\nimport("fmt";"math";"strings";"testing";"time")\n'+'\n\n'.join(declarations[n] for n in test_names)
    tests+=r'''
func TestRC618RealAsyncEntryAndStaleCompletion(t *testing.T){
 selectedObjectKey="OLD"
 versions=make([]VersionRecord,50000)
 outcomes=make([]Outcome,50000)
 for i:=range versions {id:=fmt.Sprintf("v%d",i);versions[i]=VersionRecord{VersionID:id,Method:"Ингибитор солеотложений",ObjectKey:"OLD",Equipment:"УЭЦН",Complication:"Солеотложения"};outcomes[i]=Outcome{VersionID:id,EffectAttribution:"Подтверждено специалистом",ObservedEffect:"Наблюдается улучшение"}}
 started:=time.Now()
 refreshMethodsWorkflowUI()
 elapsed:=time.Since(started)
 if elapsed>time.Second {t.Fatalf("entry blocked UI thread: %s",elapsed)}
 oldToken:=methodsBackgroundGeneration.Load()
 if !methodsBackgroundBusy || renderedMethods!=0 {t.Fatal("entry did not return during calculation")}
 selectedObjectKey="NEW";versions=nil;outcomes=nil
 refreshMethodsWorkflowUI()
 newToken:=methodsBackgroundGeneration.Load()
 timeout:=time.NewTimer(5*time.Second);defer timeout.Stop()
 for {select {case token:=<-postedMethodsTokens:completeMethodsEvaluation(token);if token==newToken {goto done};case <-timeout.C:t.Fatal("worker did not complete")}}
 done:
 if methodsPreparedKey!="NEW" || methodsBackgroundBusy || renderedMethods!=1 {t.Fatalf("completion state: key=%s busy=%v render=%d",methodsPreparedKey,methodsBackgroundBusy,renderedMethods)}
 completeMethodsEvaluation(oldToken)
 if methodsPreparedKey!="NEW" || renderedMethods!=1 {t.Fatal("stale completion changed UI")}
 t.Logf("actual entry function, 50000 records: %s; stale job rejected",elapsed)
 invalidateMethodsBackground();selectedObjectKey=""
}
func TestRC617CandidateEvaluationReuse(t *testing.T){
 failureEvents=[]FailureEvent{{Date:"01.09.2026",CycleStartDate:"01.05.2024",StopCause:"Солеотложения",ConclusionApproved:true}}
 defer func(){failureEvents=nil}()
 finish:=beginMethodsEvaluation()
 out:=methodSuggestionsForObject("OBJ")
 if len(out)!=30 {t.Fatalf("candidate cap: %d",len(out))}
 count:=len(methodsEvaluation.assessments)
 for _,s:=range out {methodApplicabilityForSuggestion(s,"OBJ",time.Now());methodApplicabilityForSuggestion(s,"OBJ",time.Now())}
 if count!=48 || len(methodsEvaluation.assessments)!=count {t.Fatalf("candidates evaluated repeatedly: %d -> %d",count,len(methodsEvaluation.assessments))}
 finish()
 if methodsEvaluation!=nil {t.Fatal("retained evaluation")}
}
func BenchmarkRC617CandidateRefresh(b *testing.B){
 for n:=0;n<b.N;n++ {finish:=beginMethodsEvaluation(); suggestions:=methodSuggestionsForObject("OBJ");for _,s:=range suggestions {methodApplicabilityForSuggestion(s,"OBJ",time.Now())};finish()}
}
func TestRC617ActualLayoutCoordinates(t *testing.T){
 for _,size:=range [][2]int32{{1280,729},{1366,768},{1600,900},{1920,1080}} {
  for _,ready:=range []bool{false,true} {
   methodsClassificationReady=ready
   methodWorkflowSuggestions=make([]methodWorkflowSuggestion,30)
   rectangles=map[uintptr]rectangle{};visibility=map[uintptr]bool{}
   layoutMethodsWorkflow(size[0],size[1])
   for handle,r:=range rectangles {if visible,ok:=visibility[handle];ok&&!visible {continue};if r.w<1||r.h<1||r.x+r.w>size[0]||r.y+r.h>size[1]-12 {t.Fatalf("layout %v ready=%v handle=%d rect=%+v",size,ready,handle,r)} }
   if rectangles[methodsArchiveList].h<40 {t.Fatalf("unusable archive at %v: %+v",size,rectangles[methodsArchiveList])}
   if ready {
    groups:=[][]uintptr{{methodsAvailableTitle,methodsShowAllButton,methodsCandidatePrev,methodsCandidateNext,methodsCandidateCards[0]},{methodsBasisTitle,methodsBasisNumbers[0],methodsBasisItems[0]},{methodsImplementationTitle,methodsParameterLabels[0],methodsParameterValues[0],methodsImplementationEditButton}}
    for i,controls:=range groups {p:=rectangles[methodsWorkspacePanels[i]];for _,handle:=range controls {r,ok:=rectangles[handle];if !ok {continue};if r.x<p.x+6||r.x+r.w>p.x+p.w-6||r.y<p.y+2||r.y+r.h>p.y+p.h-5 {t.Fatalf("control touches workspace border at %v: panel=%+v handle=%d rect=%+v",size,p,handle,r)}}}
   }
   t.Logf("%dx%d ready=%v cards/page=%d archive height=%d",size[0],size[1],ready,methodsCardsPerPage,rectangles[methodsArchiveList].h)
  }
 }
}
'''
    (dst/'model_test.go').write_text(tests)
    (dst/'go.mod').write_text('module rosa629qa\ngo 1.22\n')
    (dst/'extract.go').unlink()
    args=[go,'test','-v','-bench=RC618','-benchtime=200ms','.']
    if os.environ.get('ROSA_QA_RACE'): args.insert(2,'-race')
    subprocess.run(args,cwd=dst,env=env,check=True)
print('Portable checks passed. Native Windows UI/login execution is not included.')
