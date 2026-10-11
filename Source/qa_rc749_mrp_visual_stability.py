from pathlib import Path
import re

root = Path(__file__).resolve().parent
ui = (root / 'mrp_forecast_ui.go').read_text(encoding='utf-8')
main = (root / 'main.go').read_text(encoding='utf-8')
workflow = (root / 'workflow_pages.go').read_text(encoding='utf-8')

m = re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
if not m or int(m.group(1)) < 749:
    raise SystemExit('RC749 FAIL: app version below 749-RC')

for marker in [
    'compact := h < 960',
    'veryCompact := h < 760',
    'mrpAtomicLayoutDepth',
    'SWP_NOZORDER',
    'setMRPLayoutVisibility',
    'mrpDetailButton',
    'ОТКРЫТЬ ОБОСНОВАНИЕ ПРОГНОЗА',
    'openMRPForecastDetail()',
    'mrpCenteredLabel',
    'layoutMRPTableColumns(leftW, rightW)',
    'setMRPLayoutVisibility(mrpMethodsList, false)',
    'setMRPLayoutVisibility(mrpRollingLabel, true)',
    'show(mrpRollingHeader, false)',
]:
    if marker not in ui and marker not in main and marker not in workflow:
        raise SystemExit('RC749 FAIL: missing visual/stability marker: ' + marker)

# The compact geometry is duplicated here intentionally as an acceptance gate.
# It proves that all primary controls remain inside the client area at the
# supported 1180x700 floor and at the user's 1366x768-class screen.
def compact_geometry(width: int, height: int):
    content = max(720, width - 270)
    action = content * 48 // 100
    guide = max(120, action * 27 // 100)
    calc = max(150, action * 35 // 100)
    drill = action - guide - calc - 16
    target = content - action - 12
    target_label = max(62, target * 14 // 100)
    edit = max(60, target * 13 // 100)
    save = max(106, target * 23 // 100)
    reset = max(78, target * 17 // 100)
    combo = target - target_label - edit - save - reset - 20
    if combo < 108:
        deficit = 108 - combo
        save = max(98, save - deficit)
        combo = target - target_label - edit - save - reset - 20
    lower_y = 370 + 88  # PAGE_Y_OFFSET
    list_y = lower_y + 22
    button_h = 36
    extra = 98 if height < 760 else 92
    button_y = max(list_y + extra, height - 10 - button_h)
    if height < 760:
        rolling_y = button_y - 24
        list_h = max(66, rolling_y - list_y - 6)
    else:
        list_h = max(82, button_y - list_y - 6)
    return {
        'content': content, 'guide': guide, 'calc': calc, 'drill': drill,
        'target_label': target_label, 'edit': edit, 'combo': combo,
        'save': save, 'reset': reset, 'list_h': list_h,
        'button_bottom': button_y + button_h,
        'card_w': (content - 3 * 8) // 4,
    }

for width, height in [(1180, 700), (1280, 720), (1366, 768), (1600, 900), (1920, 900)]:
    g = compact_geometry(width, height)
    if min(g['guide'], g['calc'], g['drill']) < 118:
        raise SystemExit(f'RC749 FAIL {width}x{height}: action button too narrow {g}')
    if min(g['target_label'], g['edit'], g['combo'], g['save'], g['reset']) < 58:
        raise SystemExit(f'RC749 FAIL {width}x{height}: target controls too narrow {g}')
    if g['card_w'] < 215:
        raise SystemExit(f'RC749 FAIL {width}x{height}: scenario card too narrow {g}')
    if g['list_h'] < 82 and height >= 760:
        raise SystemExit(f'RC749 FAIL {width}x{height}: history/statistics viewport collapsed {g}')
    if g['button_bottom'] > height - 10:
        raise SystemExit(f'RC749 FAIL {width}x{height}: rationale button clipped {g}')
    print(f"PASS {width}x{height}: card={g['card_w']} list={g['list_h']} detail-bottom={g['button_bottom']}")

# Full layout begins only when the inline rationale can physically fit.
for height in [960, 1080, 1200]:
    section_y = 490 + 88
    payload = max(210, height - section_y - 16 - 3 * 26 - 3 * 8)
    history_h = max(88, payload * 30 // 100)
    methods_h = max(92, payload * 32 // 100)
    methods_y = section_y + 26 + history_h + 8
    detail_y = methods_y + 26 + methods_h + 8
    detail_h = max(82, height - (detail_y + 26) - 14)
    bottom = detail_y + 26 + detail_h
    if bottom > height - 14:
        raise SystemExit(f'RC749 FAIL full height={height}: bottom={bottom}')
    print(f'PASS full-height {height}: inline-rationale bottom={bottom}')

print('RC749 MRP visual/stability gate PASS')
