from pathlib import Path
import json, os, subprocess, tempfile
root=Path(__file__).resolve().parent
go_root=root.parent/'Компилятор'/'go-1.23.2'
go=str(go_root/'bin'/'go')
env=dict(os.environ,GOROOT=str(go_root),GOOS='linux',GOARCH='amd64',CGO_ENABLED='0')
extractor=r'''package main
import("encoding/json";"go/ast";"go/parser";"go/token";"os")
func main(){b,_:=os.ReadFile(os.Args[1]);fset:=token.NewFileSet();f,e:=parser.ParseFile(fset,os.Args[1],b,0);if e!=nil{panic(e)};want:=map[string]bool{};for _,n:=range os.Args[2:]{want[n]=true};out:=map[string]string{};for _,d:=range f.Decls{if fn,ok:=d.(*ast.FuncDecl);ok&&want[fn.Name.Name]{out[fn.Name.Name]=string(b[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])}};json.NewEncoder(os.Stdout).Encode(out)}'''
with tempfile.TemporaryDirectory() as td:
    td=Path(td); (td/'extract.go').write_text(extractor)
    assess_names=['assessmentFinalClassForDecision','assessmentDecisionRequiresComment']
    method_names=['assessmentDecisionConsistentForMethods','approvedAssessmentForMethods','latestApprovedMethodDecisionRaw','classificationSummariesEquivalent','latestApprovedMethodDecision']
    def extract(source,names):
        copied=td/(source.stem+'_source.txt')
        copied.write_text(source.read_text(encoding='utf-8'),encoding='utf-8')
        raw=subprocess.check_output([go,'run','extract.go',str(copied),*names],cwd=td,env=env,text=True)
        f=json.loads(raw); missing=[n for n in names if n not in f]
        if missing: raise SystemExit('missing production funcs: '+','.join(missing))
        return f
    a=extract(root/'assessment_classification.go',assess_names)
    m=extract(root/'methods_workflow.go',method_names)
    model='''package main\nimport("fmt";"strings")\n\ntype AssessmentClassRecord struct { ObjectKey,Direction,Prefix,TechnologistClass,FinalClass,DecisionMode,ProgramClassAtDecision string; FullClassificationConfirmed bool }\ntype assessmentClassDirection struct {Name,Prefix string}\nvar assessmentClassDirections=[]assessmentClassDirection{{"Солеотложения","С"},{"Коррозия","К"},{"Свободный газ","Г"}}\nvar classStore=map[string]AssessmentClassRecord{}\nfunc assessmentRecord(objectKey string,d assessmentClassDirection) AssessmentClassRecord{return classStore[objectKey+"|"+d.Name]}\ntype approvedMethodDecision struct{ObjectKey,Classification string}\nvar approvedMethodDecisions []approvedMethodDecision\nfunc ensureApprovedMethodsLoaded(){}\n\n'''+ '\n\n'.join(a[n] for n in assess_names)+ '\n\n' + '\n\n'.join(m[n] for n in method_names)
    (td/'model.go').write_text(model)
    tests=r'''package main
import("strings";"testing")
func rec(obj,dir,tech,program,final,mode string,full bool) AssessmentClassRecord{return AssessmentClassRecord{ObjectKey:obj,Direction:dir,TechnologistClass:tech,ProgramClassAtDecision:program,FinalClass:final,DecisionMode:mode,FullClassificationConfirmed:full}}
func TestAcceptReclassUsesProgramProposal(t *testing.T){got,err:=assessmentFinalClassForDecision("с корректировкой","С3","С1");if err!=nil||got!="С1"{t.Fatalf("got=%q err=%v",got,err)}}
func TestTechnologistDecisionKeepsTechAndNeedsCommentWhenDifferent(t *testing.T){got,err:=assessmentFinalClassForDecision("без корректировки","С3","С1");if err!=nil||got!="С3"{t.Fatalf("got=%q err=%v",got,err)};if !assessmentDecisionRequiresComment("без корректировки","С3","С1"){t.Fatal("override must require comment")};if assessmentDecisionRequiresComment("с корректировкой","С3","С1"){t.Fatal("accepted reclass must not require override comment")}}
func TestAcceptedReclassFlowsToMethods(t *testing.T){classStore=map[string]AssessmentClassRecord{"OBJ|Солеотложения":rec("OBJ","Солеотложения","С3","С1","С1","с корректировкой",true),"OBJ|Коррозия":rec("OBJ","Коррозия","Не относится","Не относится","Не относится","с корректировкой",true),"OBJ|Свободный газ":rec("OBJ","Свободный газ","Не относится","Не относится","Не относится","с корректировкой",true)};summary,ready,rows:=approvedAssessmentForMethods("OBJ");if !ready{t.Fatalf("not ready: %s",summary)};if len(rows)!=1||rows[0].FinalClass!="С1"{t.Fatalf("wrong rows: %#v",rows)};if !strings.Contains(summary,"Солеотложения — С1"){t.Fatalf("wrong summary: %s",summary)}}
func TestPendingRowChoiceDoesNotReachMethods(t *testing.T){classStore=map[string]AssessmentClassRecord{"OBJ|Солеотложения":rec("OBJ","Солеотложения","С3","С1","С1","с корректировкой",false),"OBJ|Коррозия":rec("OBJ","Коррозия","Не относится","Не относится","Не относится","с корректировкой",false),"OBJ|Свободный газ":rec("OBJ","Свободный газ","Не относится","Не относится","Не относится","с корректировкой",false)};summary,ready,rows:=approvedAssessmentForMethods("OBJ");if ready{t.Fatalf("pending choice became approved: %s",summary)};if len(rows)!=0{t.Fatalf("pending rows leaked to Methods: %#v",rows)};if !strings.Contains(summary,"НЕ УТВЕРЖДЕНО"){t.Fatalf("pending status not explicit: %s",summary)}}
func TestStaleTechValueCannotPassAsAcceptedReclass(t *testing.T){r:=rec("OBJ","Солеотложения","С3","С1","С3","с корректировкой",true);if assessmentDecisionConsistentForMethods(r){t.Fatal("stale tech class leaked through accepted reclass")}}
func TestMethodDecisionSnapshotStillExact(t *testing.T){classStore=map[string]AssessmentClassRecord{"OBJ|Солеотложения":rec("OBJ","Солеотложения","С3","С1","С1","с корректировкой",true),"OBJ|Коррозия":rec("OBJ","Коррозия","Не относится","Не относится","Не относится","с корректировкой",true),"OBJ|Свободный газ":rec("OBJ","Свободный газ","Не относится","Не относится","Не относится","с корректировкой",true)};approvedMethodDecisions=[]approvedMethodDecision{{ObjectKey:"OBJ",Classification:"УТВЕРЖДЕНО: Солеотложения — С3"}};if _,ok:=latestApprovedMethodDecision("OBJ");ok{t.Fatal("old method decision survived reclass")};summary,_,_:=approvedAssessmentForMethods("OBJ");approvedMethodDecisions=[]approvedMethodDecision{{ObjectKey:"OBJ",Classification:summary}};if _,ok:=latestApprovedMethodDecision("OBJ");!ok{t.Fatal("matching method decision not recognized")}}
'''
    (td/'model_test.go').write_text(tests)
    (td/'go.mod').write_text('module rosa631classification\ngo 1.22\n')
    subprocess.run([go,'test','-v','.'],cwd=td,env=env,check=True)
print('RC631 Classification -> Methods linkage checks passed.')
