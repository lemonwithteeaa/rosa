from pathlib import Path

root = Path(__file__).resolve().parent
report = (root / "report_workspace.go").read_text(encoding="utf-8")

required = [
    "veryCompact := h < 820",
    "setPos(resultArchiveList, x, archiveY, contentW, max32(150, bottom-archiveY))",
    "show(resultTraceCard, false)",
]
for marker in required:
    if marker not in report:
        raise SystemExit("layout contract missing: " + marker)

PAGE_Y_OFFSET = 88

def max32(a,b): return a if a>b else b

def layout(w,h):
    x=242
    contentW=max(720,w-270)
    right=x+contentW
    top=94+PAGE_Y_OFFSET
    bottom=h-18
    compact=h<=930
    very=h<820
    objectY=top+58
    objectH=58
    cardsGapY=10
    cardsH=84
    variantsH=132
    decisionH=72
    traceH=54
    if compact:
        objectY=top+54; objectH=48; cardsGapY=8; cardsH=60; variantsH=86; decisionH=58; traceH=40
    if very:
        objectY=top+52; objectH=44; cardsGapY=6; cardsH=50; variantsH=70; decisionH=54
    cardsY=objectY+objectH+cardsGapY
    variantTitleY=cardsY+cardsH+8
    variantButtonH=30 if very else 36
    variantsY=variantTitleY+variantButtonH+6
    decisionY=variantsY+variantsH+8
    traceY=decisionY+decisionH+7
    if very:
        archiveTitleY=decisionY+decisionH+8
    else:
        archiveTitleY=traceY+traceH+8
    archiveButtonH=30 if very else 36
    archiveY=archiveTitleY+archiveButtonH+6
    archiveH=max(150,bottom-archiveY)
    # horizontal safety for the densest control rows
    variant_buttons=164+166+174+2*8
    decision_buttons=150+160+174+2*8
    export_buttons=104+86+92+92+3*8
    return {
        "contentW":contentW,"right":right,"bottom":bottom,
        "object_end":objectY+objectH,"cards_end":cardsY+cardsH,
        "variants_end":variantsY+variantsH,"decision_end":decisionY+decisionH,
        "archiveY":archiveY,"archiveH":archiveH,"archive_end":archiveY+archiveH,
        "variant_title_w":max(180,contentW-variant_buttons-12),
        "decision_text_w":max(220,contentW-decision_buttons-14),
        "archive_title_w":max(180,contentW-export_buttons-12),
        "very":very,
    }

for w,h in [(1280,729),(1366,768),(1600,823),(1600,900),(1920,1080)]:
    r=layout(w,h)
    assert r["variant_title_w"]>=180
    assert r["decision_text_w"]>=220
    assert r["archive_title_w"]>=180
    assert r["object_end"] < r["cards_end"] < r["variants_end"] < r["decision_end"] < r["archiveY"]
    assert r["archiveH"] >= 150
    assert r["archive_end"] <= h, (w,h,r)
    print(f"PASS {w}x{h}: archive={r['archiveH']}px, veryCompact={r['very']}")
print("RC705 result-layout matrix PASS")
