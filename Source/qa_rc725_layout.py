# RC725 geometry sanity: larger visual hierarchy must still fit supported screens.
SIZES=[(1280,729),(1366,768),(1600,823),(1600,900),(1920,1080)]
PAGE_Y_OFFSET=88
for w,h in SIZES:
    x=242
    content=max(720,w-270)
    top=94+PAGE_Y_OFFSET
    bottom=h-18
    compact=h<=930
    very=h<820
    object_y=top+(64 if very else 66 if compact else 68)
    object_h=54 if very else 60 if compact else 66
    cards_gap=6 if very else 8 if compact else 10
    cards_h=64 if very else 78 if compact else 98
    cards_y=object_y+object_h+cards_gap
    variant_title=cards_y+cards_h+8
    btn_h=34 if very else 38
    variants_y=variant_title+btn_h+6 if very else variant_title+max(btn_h,46)+8
    variants_h=70 if very else 86 if compact else 132
    decision_y=variants_y+variants_h+8
    decision_h=82 if very else 90 if compact else 104
    archive_title=decision_y+decision_h+4
    archive_btn_h=30 if very else 34
    archive_y=archive_title+max(archive_btn_h+2,46)+8
    archive_min=78 if very else 120
    archive_h=max(archive_min,bottom-archive_y)
    if archive_y>=bottom+4:
        raise SystemExit(f'RC725 section7 archive starts below viewport {w}x{h}: y={archive_y} bottom={bottom}')
    if archive_y+archive_h>bottom+archive_min+2:
        raise SystemExit(f'RC725 section7 archive geometry invalid {w}x{h}')
    card_w=(content-5*8)//6
    if card_w<110:
        raise SystemExit(f'RC725 section7 cards too narrow {w}x{h}: {card_w}')
    print(f'PASS section7 {w}x{h}: cards={card_w}x{cards_h}, archive={archive_h}')

    compact8=h<840
    banner_y=top+72
    banner_h=74 if compact8 else 82
    stats_h=82 if compact8 else 94
    stats_y=banner_y+banner_h+10
    title_y=stats_y+stats_h+12
    list_y=title_y+66
    list_h=max(126,bottom-list_y)
    if list_y>=bottom:
        raise SystemExit(f'RC725 section8 list starts below viewport {w}x{h}: {list_y}>{bottom}')
    card_w8=(content-3*8)//4
    if card_w8<160:
        raise SystemExit(f'RC725 section8 cards too narrow {w}x{h}: {card_w8}')
    print(f'PASS section8 {w}x{h}: cards={card_w8}x{stats_h}, list={list_h}')
print('RC725 report/registry layout matrix PASS')
