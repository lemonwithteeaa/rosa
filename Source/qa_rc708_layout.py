PAGE_Y_OFFSET=88

def layout(w,h):
    x=242; contentW=max(720,w-270); right=x+contentW; top=94+PAGE_Y_OFFSET; bottom=h-18
    compact=h<=930; very=h<820
    objectY=top+58; objectH=58; cardsGapY=10; cardsH=84; variantsH=132; decisionH=72
    if compact:
        objectY=top+54; objectH=48; cardsGapY=8; cardsH=60; variantsH=86; decisionH=58
    if very:
        objectY=top+52; objectH=44; cardsGapY=6; cardsH=50; variantsH=70; decisionH=54
    cardsY=objectY+objectH+cardsGapY
    variantTitleY=cardsY+cardsH+8
    variantButtonH=34 if very else 38
    variantsY=variantTitleY+variantButtonH+6
    decisionY=variantsY+variantsH+8
    gap=8
    useW,saveW,approveW=154,174,184
    decisionButtonsW=useW+saveW+approveW+2*gap
    decisionRegionW=max(420,contentW-decisionButtonsW-14)
    infoGap=8
    infoW=min(304,max(168,decisionRegionW-292-infoGap))
    decisionTextW=decisionRegionW-infoW-infoGap
    infoX=x+decisionTextW+infoGap
    useX=x+decisionRegionW+14
    saveX=useX+useW+gap
    approveX=saveX+saveW+gap
    archiveTitleY=decisionY+decisionH+10
    archiveButtonH=30 if very else 34
    archiveY=archiveTitleY+archiveButtonH+6
    archiveH=max(150,bottom-archiveY)
    return locals()

for w,h in [(1280,729),(1366,768),(1600,794),(1600,823),(1600,900),(1680,794),(1920,1080)]:
    d=layout(w,h)
    assert d['decisionTextW']>=292,(w,h,'decision',d['decisionTextW'])
    assert d['infoW']>=168,(w,h,'info',d['infoW'])
    assert d['infoX']+d['infoW'] <= d['useX']-14,(w,h,'info-overlap')
    assert d['useX']+d['useW']+8 <= d['saveX'],(w,h,'use/save')
    assert d['saveX']+d['saveW']+8 <= d['approveX'],(w,h,'save/approve')
    assert d['approveX'] <= d['right'] and d['right']-d['approveX']>=150,(w,h,'approve width',d['right']-d['approveX'])
    assert d['archiveH']>=150,(w,h,'archive',d['archiveH'])
    assert d['archiveY']+d['archiveH']<=h,(w,h,'bottom',d['archiveY']+d['archiveH'])
    print(f"PASS {w}x{h}: decision={d['decisionTextW']} info={d['infoW']} archive={d['archiveH']}")
print('RC708 result-layout matrix PASS')
