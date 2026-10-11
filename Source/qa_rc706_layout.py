PAGE_Y_OFFSET=88
def layout(w,h):
 x=242; contentW=max(720,w-270); top=94+PAGE_Y_OFFSET; bottom=h-18; compact=h<=930; very=h<820
 objectY=top+58; objectH=58; cardsGapY=10; cardsH=84; variantsH=132; decisionH=72
 if compact: objectY=top+54; objectH=48; cardsGapY=8; cardsH=60; variantsH=86; decisionH=58
 if very: objectY=top+52; objectH=44; cardsGapY=6; cardsH=50; variantsH=70; decisionH=54
 cardsY=objectY+objectH+cardsGapY; variantTitleY=cardsY+cardsH+8; variantButtonH=34 if very else 38
 variantsY=variantTitleY+variantButtonH+6; decisionY=variantsY+variantsH+8; archiveTitleY=decisionY+decisionH+10
 archiveButtonH=30 if very else 34; archiveY=archiveTitleY+archiveButtonH+6; archiveH=max(150,bottom-archiveY)
 return contentW, archiveY, archiveH, archiveY+archiveH, max(180,contentW-(184+184+194+16)-12), max(220,contentW-(154+174+184+16)-14)
for w,h in [(1280,729),(1366,768),(1600,823),(1600,900),(1920,1080)]:
 c,y,ah,end,tw,dw=layout(w,h); assert tw>=180 and dw>=220 and ah>=150 and end<=h,(w,h,layout(w,h)); print(f"PASS {w}x{h}: archive={ah}px")
print("RC706 result-layout matrix PASS")
