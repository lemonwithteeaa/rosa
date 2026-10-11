from pathlib import Path
import re
root=Path(__file__).resolve().parent
files={p.name:p.read_text(encoding='utf-8',errors='ignore') for p in root.glob('*.go')}
main=files['main.go']; mw=files['methods_workflow.go']; tc=files['teaching_cross_history.go']; tr=files['target_runlife.go']; me=files['methods_effect_model.go']; demo=files['demo_seed.go']
checks=[]
def ck(name, cond): checks.append((name,bool(cond)))

ck('version 616', 'const appBuildVersion = "7.3.11.616-RC"' in main)
ck('profile caption restored', '"Смена\\r\\nпользователя"' in main and 'setText(hHeaderApprove, "Смена\\r\\nпользователя")' in main)
ck('profile action remains login return', 'runOfflineLogin()' in main[main.index('case ID_HEADER_APPROVE:'):main.index('case ID_HEADER_HELP:')])
ck('profile is quiet outlined service action', 'if h == hHeaderApprove {' in main and 'primary = false' in main[main.index('if h == hHeaderApprove {'):main.index('if h == hHeaderApprove {')+250])
ck('methods refresh panic isolated', 'METHODS refresh recovered panic=' in main and 'showMethodsSafeFallback' in main)
ck('methods layout panic isolated', 'METHODS layout recovered panic=' in mw)
ck('safe methods fallback exists', 'func showMethodsSafeFallback' in mw and 'Методы открыты в безопасном режиме' in mw)
ck('unsafe hidden methods customdraw removed from wndproc', 'hdr.HwndFrom == methodsCandidateList' not in main)
ck('method cards use native standard button', 'methodsSafeCandidateButton' in mw and 'BS_OWNERDRAW' not in mw[mw.index('func methodsSafeCandidateButton'):mw.index('func methodsScrollableCard')])
ck('reference proportions retained', 'leftW := contentW * 44 / 100' in mw and 'midW := contentW * 27 / 100' in mw)
ck('three reference zones retained', all(x in mw for x in ['01 · ДОСТУПНЫЕ МЕТОДЫ','02 · ОСНОВАНИЕ ВЫБОРА','03 · ПАРАМЕТРЫ РЕАЛИЗАЦИИ']))
ck('plan editor preserved', 'СФОРМИРОВАТЬ ПЛАН' in mw and 'show(methodsPlanList, false)' in mw)
ck('target minimum is 1000', 'minTargetRunlifeDays     = 1000.0' in tr and 'defaultTargetRunlifeDays = 1000.0' in tr)
ck('methods target headline 1000+', 'ЦЕЛЕВОЙ ОРИЕНТИР ≥ 1000 СУТ.' in mw)
ck('target gap shown in method cards', 'нужен комплекс' in mw and 'К цели ≥ %.0f сут.' in mw)
ck('target gap shown in selected basis', 'остаточный разрыв %.0f сут. — требуется комплекс мероприятий' in mw)
ck('combined forecast stores target', 'TargetDays    float64' in me and 'TargetDays: target' in me)
ck('combined forecast does not fake target floor', 'ForecastBase = out.BaselineBase * (1 + out.EffectBasePct/100)' in me and 'остаточный разрыв' in me)
ck('teaching source has four distinct equipment types', all(x in tc for x in ['Equipment: "УЭЦН"','Equipment: "УШГН"','Equipment: "УЭВН"','Equipment: "ОРД (УЭЦН + ШГН)"']))
ck('teaching actual MRP examples exceed one year', all(x in tc for x in ['MRPDays: 610','MRPDays: 845','MRPDays: 455','MRPDays: 730']))
ck('teaching current MRP spans toward 1000+', 'mrpBase := []int{610, 845, 455, 730}' in demo and 'mrpBand := ((n - 1) / 4 % 5) * 70' in demo)
ck('teaching central forecasts all 1000+', 'forecastBase := []float64{1100, 1200, 1300, 1420}' in tc)
ck('teaching cautious forecasts target-oriented', 'forecastLow := []float64{1000, 1080, 1160, 1260}' in tc)
ck('mixed complications retained', 'АСПО + высокая вязкость' in tc and 'Соли + коррозия + мехпримеси' in tc)
ck('methods archive four row reserve', 'archiveReserve := int32(142)' in mw)
ck('forecast title dynamic target', 'ПРОГНОЗ НАРАБОТКИ · ЦЕЛЕВОЙ ОРИЕНТИР ≥ %.0f СУТ.' in mw)
ck('methods target contribution can be qualitative', 'вклад качественный · нужен комплекс' in mw)

passed=sum(v for _,v in checks)
for name,v in checks: print(('PASS' if v else 'FAIL')+': '+name)
print(f'{passed}/{len(checks)} PASS')
raise SystemExit(0 if passed==len(checks) else 1)
