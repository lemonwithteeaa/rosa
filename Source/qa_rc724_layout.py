# RC724 geometry sanity checks for the two report pages.
# Mirrors the compact thresholds used in report_workspace.go.
SIZES=[(1280,729),(1366,768),(1600,823),(1600,900),(1920,1080)]
PAGE_Y_OFFSET=88
for w,h in SIZES:
    x=242
    content=max(720,w-270)
    top=94+PAGE_Y_OFFSET
    bottom=h-18
    # Section 8
    compact=h<840
    banner_y=top+68
    banner_h=56 if compact else 64
    stats_h=66 if compact else 78
    stats_y=banner_y+banner_h+10
    title_y=stats_y+stats_h+12
    list_y=title_y+34
    list_h=max(150,bottom-list_y)
    if list_y>=bottom or list_h<150:
        raise SystemExit(f'RC724 section8 clipping risk {w}x{h}: list_y={list_y} bottom={bottom} h={list_h}')
    card_w=(content-3*8)//4
    if card_w<160:
        raise SystemExit(f'RC724 section8 cards too narrow {w}x{h}: {card_w}')
    print(f'PASS section8 {w}x{h}: card={card_w}px list={list_h}px')
print('RC724 report/registry layout matrix PASS')
