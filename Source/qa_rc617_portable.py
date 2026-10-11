"""Run the actual pure Go model on Linux; this is not a native Windows UI test.

Go AST extraction copies declarations unchanged. OS/data access uses explicit
fixtures. The production Windows tests are compiled separately.
"""
from pathlib import Path
import json, os, subprocess, tempfile

root = Path(__file__).resolve().parent
go_root = root.parent/'Компилятор'/'go-1.23.2'
go = str(go_root/'bin'/'go')
env = dict(os.environ, GOROOT=str(go_root), GOOS='linux', GOARCH='amd64', CGO_ENABLED='0')
extractor = r'''package main
import("encoding/json";"go/ast";"go/parser";"go/token";"os";"strings")
func main(){ out:=map[string]string{}; for _,path:=range os.Args[1:] { b,_:=os.ReadFile(path); fset:=token.NewFileSet(); f,e:=parser.ParseFile(fset,path,b,0); if e!=nil {panic(e)}; for _,d:=range f.Decls { if fn,ok:=d.(*ast.FuncDecl);ok { out[fn.Name.Name]=string(b[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset]) }; if gd,ok:=d.(*ast.GenDecl);ok { for _,sp:=range gd.Specs {if ts,ok:=sp.(*ast.TypeSpec);ok {out[ts.Name.Name]="type "+string(b[fset.Position(ts.Pos()).Offset:fset.Position(ts.End()).Offset])}} } } }; _=strings.TrimSpace; json.NewEncoder(os.Stdout).Encode(out) }
'''
files = ['multi_method_scenario.go','dynamogram_ui.go','main.go','methods_effect_model.go','methods_workflow.go','methods_evaluation.go','method_runlife_history.go','teaching_cross_history.go','teaching_method_effect.go','demo_seed.go','rules.go','assessment_methods.go','assessment_classification.go','well_failures.go','mrp_forecast.go','dynamogram.go','rc617_methods_regression_test.go']
names = '''normalizeMethodList methodsForWorkflowApproval MethodEffectAssessment methodWorkflowSuggestion assessmentMethodCandidate teachingCrossHistoryPoint teachingCrossHistoryPoints teachingWorkflowOrdinal teachingMethodEffect sameMethodFamily combineMethodEffectPercents normalizeMethodText clampFloat clampInt minInt MethodHistoryBasis methodsEvaluationKey methodsEvaluationScope beginMethodsEvaluation methodApplicabilityForSuggestion calculateMethodApplicability methodEffectReferenceBand methodEvidenceScore historyScoreForMethod methodReliabilityLevel methodFailureEvidence failureTextForMethodMatch failureMatchesMethodDirection methodTextMatchesDirection assessmentMethodNeedsLabSelection assessmentMethodIsEquipmentDesign FailureEvent methodHistoryBasisForObject calculateMethodHistoryBasis historyForWell medianFloat64 empiricalQuantile methodSuggestionsForObject buildMethodSuggestionsForObject methodDirectionByName methodClassSeverity assessmentClassDirection'''.split()
test_names = ['TestRC617UncheckedDraftDoesNotApproveSelectedFallback','TestRC617TeachingForecastsDerivedFromEffects','TestRC617RefreshCacheEndsWithAction','TestRC617ProductionDoesNotReceiveTeachingEffects']
with tempfile.TemporaryDirectory(prefix='rosa617-qa-') as temp:
    dst=Path(temp)
    (dst/'extract.go').write_text(extractor)
    subprocess.run([go,'build','-o',str(dst/'extract'),str(dst/'extract.go')],env=env,check=True)
    declarations=json.loads(subprocess.check_output([str(dst/'extract')]+[str(root/f) for f in files]))
    code='package main\nimport("fmt";"math";"regexp";"sort";"strconv";"strings";"time")\n'
    code+='\n\n'.join(declarations[n] for n in names)
    # Execute the actual native layout without a display: only setPos/show are
    # recorded. This checks coordinates, not font rendering or native painting.
    import re
    layout = declarations['layoutMethodsWorkflow']
    handles = sorted(set(re.findall(r'\bmethods[A-Z]\w*\b',layout))-set(declarations)-{
        'methodsCardsPerPage','methodsCandidatePage','methodsClassificationReady',
        'methodsCandidateCards','methodsCandidateDetails'})
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
func py(y int32)int32{return y+PAGE_Y_OFFSET}
func diagLog(format string, args ...any){panic(fmt.Sprintf(format,args...))}
func refreshMethodsCandidateCards(){}
'''
    code+=r'''
var methodsEvaluation *methodsEvaluationScope
var bundledDemoObjectPattern = regexp.MustCompile(`(?i)^Учебный пример\s+(\d{3})\s+·\s+(.+)$`)
var failureEvents []FailureEvent
var combinedScenarioMethods = map[string][]string{}
type methodForecastSnapshot struct { Low, Base, High float64; Source string; History MethodHistoryBasis }
var assessmentClassDirections = []assessmentClassDirection{{Name:"Солеотложения",LabGroup:"scale"},{Name:"Коррозия",LabGroup:"corrosion"}}
type AssessmentClassRecord struct{Direction, FinalClass string}
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
    (dst/'model.go').write_text(code)
    tests='package main\nimport("fmt";"math";"strings";"testing";"time")\n'+'\n\n'.join(declarations[n] for n in test_names)
    tests+=r'''
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
   t.Logf("%dx%d ready=%v cards/page=%d archive height=%d",size[0],size[1],ready,methodsCardsPerPage,rectangles[methodsArchiveList].h)
  }
 }
}
'''
    (dst/'model_test.go').write_text(tests)
    (dst/'go.mod').write_text('module rosa617qa\ngo 1.22\n')
    (dst/'extract.go').unlink()
    subprocess.run([go,'test','-v','-bench=RC617','-benchtime=200ms','.'],cwd=dst,env=env,check=True)
print('Portable checks passed. Native Windows UI/login execution is not included.')
