from pathlib import Path
src=(Path(__file__).resolve().parent/'assessment_classification.go').read_text(encoding='utf-8')
checks={
 'summary uses two lines':'Итог переносится в «Методы» только после нажатия' in src,
 'approval owns full width':'setPos(assessConfirmFullButton, 242, approveY, contentW, approveH)' in src,
 'choice buttons share first row':'choiceW := (contentW - buttonGap) / 2' in src,
 'short screens hide archive':'historyVisible := h >= 820' in src,
 'final approval re-resolves selected source':'resolved, err := assessmentFinalClassForDecision(r.DecisionMode, r.TechnologistClass, program)' in src,
 'obsolete stale blocker removed':'решение по осложнению «%s» устарело' not in src,
}
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
# coordinate model for 768-class window: approval must remain entirely visible.
def model(h):
    tabY=190+88; tabH=44; overallY=tabY+tabH+8; overallH=52; listY=overallY+overallH+6
    historyY=h-12
    listH=112 if h<820 else 156
    maxListH=historyY-listY-185
    if maxListH<listH: listH=max(112,maxListH)
    editorY=listY+listH+8
    choicesY=editorY+22+42+8
    approveY=choicesY+40+7
    approveBottom=approveY+42
    detailY=approveBottom+7
    return approveY,approveBottom,detailY,historyY
for h in (729,768,800):
    ay,ab,dy,hy=model(h)
    ok=ab <= h-8 and dy < hy
    checks[f'approval visible at {h}px']=ok
    print(('PASS' if ok else 'FAIL')+f': approval visible at {h}px (approve={ay}:{ab}, detailY={dy}, bottomLimit={hy})')
raise SystemExit(0 if all(checks.values()) else 1)
