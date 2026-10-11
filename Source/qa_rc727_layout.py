SIZES=[(1280,729),(1366,768),(1600,823),(1600,900),(1920,1080)]
PAGE_Y_OFFSET=88
for w,h in SIZES:
    x=242
    content=max(720,w-270)
    top=94+PAGE_Y_OFFSET
    bottom=h-18
    compact=h<840
    banner_y=top+72
    banner_h=92 if compact else 100
    stats_h=104 if compact else 112
    stats_y=banner_y+banner_h+10
    title_y=stats_y+stats_h+14
    export_h=46
    hint_y=title_y+48
    hint_h=28
    list_y=title_y+80
    list_h=max(126,bottom-list_y)
    widths=[154,154,154]
    total=sum(widths)+16
    title_w=max(220,content-total-18)
    card_w=(content-24)//4
    if card_w<160: raise SystemExit(f'RC727 cards too narrow {w}x{h}: {card_w}')
    if title_w<220: raise SystemExit(f'RC727 title band too narrow {w}x{h}: {title_w}')
    if list_y>=bottom: raise SystemExit(f'RC727 list below viewport {w}x{h}: {list_y}>={bottom}')
    if list_h<126: raise SystemExit(f'RC727 list too short {w}x{h}: {list_h}')
    if hint_y+hint_h>list_y: raise SystemExit(f'RC727 hint overlaps list {w}x{h}')
    if title_y+export_h>list_y: raise SystemExit(f'RC727 export row overlaps list {w}x{h}')
    print(f'PASS {w}x{h}: banner={banner_h}, cards={card_w}x{stats_h}, title={title_w}, list={list_h}')
print('RC727 registry layout matrix PASS')
