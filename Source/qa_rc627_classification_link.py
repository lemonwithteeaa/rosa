from pathlib import Path
import json, os, subprocess, tempfile
root=Path(__file__).resolve().parent
go_root=root.parent/'Компилятор'/'go-1.23.2'
go=str(go_root/'bin'/'go')
env=dict(os.environ,GOROOT=str(go_root),GOOS='linux',GOARCH='amd64',CGO_ENABLED='0')
extractor=r'''package main
import("encoding/json";"go/ast";"go/parser";"go/token";"os")
func main(){b,_:=os.ReadFile(os.Args[1]);fset:=token.NewFileSet();f,e:=parser.ParseFile(fset,os.Args[1],b,0);if e!=nil{panic(e)};want:=map[string]bool{};for _,n:=range os.Args[2:]{want[n]=true};out:=map[string]string{};for _,d:=range f.Decls{if fn,ok:=d.(*ast.FuncDecl);ok&&want[fn.Name.Name]{out[fn.Name.Name]=string(b[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])}};json.NewEncoder(os.Stdout).Encode(out)}'''
names=['assessmentDecisionConsistentForMethods','approvedAssessmentForMethods','latestApprovedMethodDecisionRaw','classificationSummariesEquivalent','latestApprovedMethodDecision']
with tempfile.TemporaryDirectory() as td:
    td=Path(td)
    (td/'extract.go').write_text(extractor)
    source=td/'methods_workflow_source.txt'; source.write_text((root/'methods_workflow.go').read_text(encoding='utf-8'),encoding='utf-8')
    raw=subprocess.check_output([go,'run','extract.go',str(source),*names],cwd=td,env=env,text=True)
    f=json.loads(raw)
    missing=[n for n in names if n not in f]
    if missing: raise SystemExit('missing production funcs: '+','.join(missing))
    model='''package main\nimport "strings"\n\ntype AssessmentClassRecord struct { ObjectKey,Direction,Prefix,TechnologistClass,FinalClass,DecisionMode string; FullClassificationConfirmed bool }\ntype assessmentClassDirection struct {Name,Prefix string}\nvar assessmentClassDirections=[]assessmentClassDirection{{"Солеотложения","С"},{"Коррозия","К"},{"Свободный газ","Г"}}\nvar classStore=map[string]AssessmentClassRecord{}\nfunc assessmentRecord(objectKey string,d assessmentClassDirection) AssessmentClassRecord{return classStore[objectKey+"|"+d.Name]}\ntype approvedMethodDecision struct{ObjectKey,Classification string}\nvar approvedMethodDecisions []approvedMethodDecision\nfunc ensureApprovedMethodsLoaded(){}\n\n'''+ '\n\n'.join(f[n] for n in names)
    (td/'model.go').write_text(model)
    tests=r'''package main
import("strings";"testing")
func rec(obj,dir,tech,final,mode string,full bool) AssessmentClassRecord{return AssessmentClassRecord{ObjectKey:obj,Direction:dir,TechnologistClass:tech,FinalClass:final,DecisionMode:mode,FullClassificationConfirmed:full}}
func TestStaleAcceptedWithoutLabCannotLeakToMethods(t *testing.T){
 classStore=map[string]AssessmentClassRecord{
  "OBJ|Солеотложения":rec("OBJ","Солеотложения","Не относится","С3","принято без лаборатории",true),
  "OBJ|Коррозия":rec("OBJ","Коррозия","К2","К2","принято без лаборатории",true),
  "OBJ|Свободный газ":rec("OBJ","Свободный газ","","НД","принято без лаборатории",true),
 }
 _,ready,_:=approvedAssessmentForMethods("OBJ"); if ready{t.Fatal("stale S3 remained ready after technologist profile changed")}
}
func TestOnlyCurrentAcceptedComplicationsFlowDownstream(t *testing.T){
 classStore=map[string]AssessmentClassRecord{
  "OBJ|Солеотложения":rec("OBJ","Солеотложения","Не относится","Не относится","принято без лаборатории",true),
  "OBJ|Коррозия":rec("OBJ","Коррозия","К2","К2","принято без лаборатории",true),
  "OBJ|Свободный газ":rec("OBJ","Свободный газ","","НД","принято без лаборатории",true),
 }
 summary,ready,rows:=approvedAssessmentForMethods("OBJ"); if !ready{t.Fatalf("current classification not ready: %s",summary)}
 if len(rows)!=1||rows[0].Direction!="Коррозия"||rows[0].FinalClass!="К2"{t.Fatalf("wrong downstream rows: %#v",rows)}
 if strings.Contains(summary,"Солеотложения")||strings.Contains(summary,"Свободный газ"){t.Fatalf("inactive complication leaked: %s",summary)}
}
func TestMethodDecisionMustMatchExactClassificationSnapshot(t *testing.T){
 classStore=map[string]AssessmentClassRecord{
  "OBJ|Солеотложения":rec("OBJ","Солеотложения","Не относится","Не относится","принято без лаборатории",true),
  "OBJ|Коррозия":rec("OBJ","Коррозия","К2","К2","принято без лаборатории",true),
  "OBJ|Свободный газ":rec("OBJ","Свободный газ","","НД","принято без лаборатории",true),
 }
 approvedMethodDecisions=[]approvedMethodDecision{{ObjectKey:"OBJ",Classification:"УТВЕРЖДЕНО: Солеотложения — С3; Свободный газ — Г3"}}
 if _,ok:=latestApprovedMethodDecision("OBJ");ok{t.Fatal("old method decision survived changed classification")}
 summary,_,_:=approvedAssessmentForMethods("OBJ")
 approvedMethodDecisions=[]approvedMethodDecision{{ObjectKey:"OBJ",Classification:summary}}
 if _,ok:=latestApprovedMethodDecision("OBJ");!ok{t.Fatal("matching method decision was not recognized")}
}
'''
    (td/'model_test.go').write_text(tests)
    (td/'go.mod').write_text('module rosa627classification\ngo 1.22\n')
    subprocess.run([go,'test','-v','.'],cwd=td,env=env,check=True)
print('Classification -> Methods linkage checks passed.')
