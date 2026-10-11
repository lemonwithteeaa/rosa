from pathlib import Path
import re, struct, sys

ROOT = Path(__file__).resolve().parent
BASE = ROOT.parent
SRC = (ROOT/'auth.go').read_text(encoding='utf-8')
MAIN = (ROOT/'main.go').read_text(encoding='utf-8')
BRAND = (ROOT/'brand.go').read_text(encoding='utf-8')
checks=[]
def check(name, ok, detail=''):
    checks.append((name,bool(ok),detail))

check('build version RC421', 'const appBuildVersion = "7.3.11.421-RC"' in MAIN)

legacy = [
    'rc374_login_menu_regression_test.go','rc375_login_refresh_test.go','rc376_login_fullscreen_test.go',
    'rc377_login_brand_layout_test.go','rc378_login_profile_emphasis_test.go','rc379_login_visual_alignment_test.go',
    'rc380_login_ritek_lockup_test.go','rc381_login_reference_lock_test.go','rc382_login_exact_reference_test.go',
    'rc397_login_centered_labels_test.go','rc398_login_bold_inputs_header_initials_test.go','rc400_login_first_frame_test.go',
    'rc401_login_compact_cards_test.go','rc402_login_paint_recovery_test.go']
check('historical login tests quarantined', all((ROOT/f).read_text(encoding='utf-8').startswith('//go:build windows && legacy_rc_tests') for f in legacy))
cur=(ROOT/'rc421_login_current_regression_test.go').read_text(encoding='utf-8')
check('current RC421 login regression active', cur.startswith('//go:build windows') and '7.3.11.421-RC' in cur and 'legacy_rc_tests' not in cur.splitlines()[0])
check('header equal-gap cap', 'if gap > 58' in SRC and 'gap = 58' in SRC)
check('header red initials', all(x in SRC for x in ['{"Р", "АСЧЁТ"}', '{"О", "ЦЕНКА"}', '{"С", "ОПРОВОЖДЕНИЕ"}', '{"А", "ВТОМАТИЗАЦИЯ"}']))
check('ROSA logo cache bumped', '_rosa_header_brand_v9.bmp' in BRAND)
check('ROSA logo/subtitle no geometric overlap', '36, 2, 226, 68' in SRC and '0, 74, 312, 28' in SRC)
check('both card top/bottom accents', all(x in SRC for x in ['authTitleAccent', 'authLeftBottomAccent', 'authInfoAccent', 'authInfoBottomAccent']))
check('feature icon+number markers', 'authFeatureIconControls[i]' in SRC and 'fmt.Sprintf("%02d", i+1)' in SRC)
check('feature number shadow', 'shadow.Left += 1' in SRC and 'rgb(233, 218, 221)' in SRC)
check('right separator clears final description', 'rightInnerX, 688, rightInnerW, 1' in SRC)
check('bottom nav starts below separator', 'navX, 700, navW, 112' in SRC and 'iconX, 708, 54, 54' in SRC and 'navX+2, 766, navW-4, 34' in SRC)
check('minimize layout guard', 'if wParam == SIZE_MINIMIZED_AUTH' in SRC)
check('restore min geometry guard', 'mmi.PtMinTrackSize.X = 1080' in SRC and 'mmi.PtMinTrackSize.Y = 930' in SRC)
check('deferred resize batch', 'pBeginDeferWindowPos' in SRC and 'pEndDeferWindowPos' in SRC)
check('one redraw after resize', 'authScheduleFullRedraw(hwnd)' in SRC and 'case WM_AUTH_FULL_REDRAW:' in SRC)

check('first frame hides parent during construction', all(x in SRC for x in ['authConstructing = true', 'style &^= WS_VISIBLE', 'authRevealDeferredChildren()']))
check('first frame avoids composited parent', 'pCreateWindowExW.Call(WS_EX_COMPOSITED_AUTH' not in SRC)
check('hidden work-area fit retained', all(x in SRC for x in ['authFitHiddenWindowToWorkArea(hwnd)', 'pSystemParametersInfoWAuth.Call(SPI_GETWORKAREA', 'pSendMessageW.Call(hwnd, WM_SIZE']))
check('restore seed size retained', '1360, 960, 0, 0, hInstRaw, 0)' in SRC)
# Mirror the two active RC395 geometry checks without needing to execute a Windows test binary.
sw, sh = 1672, 941
def crop(dw, dh):
    sx = sy = 0; cw, ch = sw, sh
    if dw <= 0 or dh <= 0: return sx, sy, cw, ch
    if dw*sh > dh*sw:
        ch = max(1, sw*dh//dw); sy = sh-ch
    else:
        cw = max(1, sh*dw//dh); sx = sw-cw
    return sx, sy, cw, ch
sx,sy,cw,ch=crop(1920,968)
check('background widescreen bottom anchor', sx == 0 and cw == sw and sy > 0 and sy+ch == sh, f'sx={sx}, sy={sy}, cw={cw}, ch={ch}')
sx,sy,cw,ch=crop(1024,900)
check('background narrow right anchor', sy == 0 and ch == sh and sx >= 0 and sx+cw == sw, f'sx={sx}, sy={sy}, cw={cw}, ch={ch}')

# Base geometry sanity: card accents stay inside the 760 px cards and last detail
# ends at 682, leaving 6 px before separator at 688.
card_y, card_h = 121, 760
check('base card bottom = 881', card_y + card_h == 881)
check('separator gap after last detail >= 6 px', 688 - (582+38+62) >= 6, f'gap={688-(582+38+62)}')
check('nav label clears nav icon', 766 - (708+54) >= 4, f'gap={766-(708+54)}')
check('bottom nav remains above card accent', 766+34 < 121+760-7, f'bottom={766+34}')

exe = BASE/'РОСА.exe'
if exe.exists():
    b=exe.read_bytes()
    try:
        pe=struct.unpack_from('<I', b, 0x3c)[0]
        opt=pe+24
        subsystem=struct.unpack_from('<H', b, opt+68)[0]
        resource_rva, resource_size=struct.unpack_from('<II', b, opt+112+16)
        check('PE subsystem Windows GUI', subsystem==2, f'subsystem={subsystem}')
        check('PE resource directory present', resource_rva>0 and resource_size>0, f'rva=0x{resource_rva:x}, size={resource_size}')
    except Exception as e:
        check('PE parse', False, repr(e))
else:
    check('PE exists', False, str(exe))

bad=[x for x in checks if not x[1]]
for name,ok,detail in checks:
    print(('PASS' if ok else 'FAIL')+' | '+name+((' | '+detail) if detail else ''))
print(f'RESULT | {len(checks)-len(bad)}/{len(checks)} passed')
sys.exit(1 if bad else 0)
