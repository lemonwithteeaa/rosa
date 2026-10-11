#!/usr/bin/env python3
from pathlib import Path
import csv, math, sys, hashlib

BASE = Path(__file__).resolve().parent
GOLDEN = BASE / 'pressure_golden.tsv'
MANIFEST = BASE / 'pressure_golden_manifest.txt'
OUT = BASE / 'pressure_comparison_report.txt'
EXPECTED_PRESSURES = [1.0, 100.0, 500.0, 1000.0]
PHASES = ['Calcite','Gypsum','Barite','Celestite']
TOL_PRESSURE = 1e-6
SIGNIFICANT_DELTA = 1e-4

def fail(msg):
    print('PRESSURE_COMPARISON_NOT_READY: ' + msg)
    sys.exit(2)

def read_manifest(path):
    if not path.exists():
        fail('pressure_golden_manifest.txt отсутствует; сначала примите внешний golden.')
    d = {}
    for line in path.read_text(encoding='utf-8-sig').splitlines():
        if '=' in line:
            k,v = line.split('=',1)
            d[k.strip()] = v.strip()
    return d

def first_key(headers, candidates):
    norm = {h.strip().lower(): h for h in headers}
    for c in candidates:
        if c.lower() in norm:
            return norm[c.lower()]
    for h in headers:
        hl = h.strip().lower()
        for c in candidates:
            if c.lower() in hl:
                return h
    return None

if not GOLDEN.exists():
    fail('pressure_golden.tsv отсутствует; сначала RUN_PRESSURE_REFERENCE.cmd, затем ACCEPT_PRESSURE_GOLDEN.cmd.')
manifest = read_manifest(MANIFEST)
if manifest.get('status') != 'ACCEPTED_EXTERNAL_GOLDEN':
    fail('manifest не имеет status=ACCEPTED_EXTERNAL_GOLDEN.')

with GOLDEN.open('r', encoding='utf-8-sig', newline='') as f:
    rows = list(csv.DictReader(f, delimiter='\t'))
if not rows:
    fail('golden пуст.')
headers = list(rows[0].keys())
pkey = first_key(headers, ['pressure_atm','pressure'])
if not pkey:
    fail('не найдена колонка давления.')
phase_keys = {}
for ph in PHASES:
    phase_keys[ph] = first_key(headers, [f'SI_{ph}_user', f'si_{ph}', ph])
    if not phase_keys[ph]:
        fail(f'не найдена колонка SI для {ph}.')

parsed = []
for row in rows:
    try:
        p = float(str(row[pkey]).strip())
    except Exception:
        continue
    vals = {}
    ok = True
    for ph,k in phase_keys.items():
        try:
            vals[ph] = float(str(row[k]).strip())
        except Exception:
            ok = False
            break
    if ok:
        parsed.append((p, vals))
if len(parsed) < 4:
    fail(f'ожидалось минимум 4 валидные строки, найдено {len(parsed)}.')
parsed.sort(key=lambda x:x[0])
matched=[]
for target in EXPECTED_PRESSURES:
    cand=min(parsed,key=lambda x:abs(x[0]-target))
    if abs(cand[0]-target) > max(TOL_PRESSURE, target*1e-6):
        fail(f'нет контрольной точки около {target:g} atm; ближайшая {cand[0]:g}.')
    matched.append(cand)
base=matched[0][1]
lines=[]
lines.append('ROSA PRESSURE GOLDEN COMPARISON')
lines.append('status=EXTERNAL_GOLDEN_COMPARED_TO_CURRENT_ROSA_PRESSURE_RESPONSE')
lines.append('important=Текущая РОСА не применяет давление к SI; её pressure-delta по определению 0 до отдельной реализации.')
lines.append('golden_sha256='+hashlib.sha256(GOLDEN.read_bytes()).hexdigest())
lines.append('significant_delta_threshold='+str(SIGNIFICANT_DELTA))
lines.append('')
max_delta=0.0
for p, vals in matched:
    parts=[]
    for ph in PHASES:
        delta=vals[ph]-base[ph]
        max_delta=max(max_delta,abs(delta))
        parts.append(f'{ph}: external_SI={vals[ph]:+.9g}; external_delta_vs_1atm={delta:+.9g}; ROSA_pressure_delta_current=+0')
    lines.append(f'P={p:g} atm | '+' | '.join(parts))
lines.append('')
if max_delta > SIGNIFICANT_DELTA:
    lines.append('decision=PRESSURE_EFFECT_DETECTED_EXTERNALLY__ROSA_IMPLEMENTATION_REQUIRED_BEFORE_ENABLE')
    lines.append('rule=Не включать pressure-поправку автоматически. Следующий шаг — выбрать физически обоснованную реализацию и сверить каждую фазу с golden.')
else:
    lines.append('decision=NO_MATERIAL_PRESSURE_EFFECT_IN_THIS_BENCHMARK_AT_SELECTED_THRESHOLD')
    lines.append('rule=Это относится только к данному эталонному составу/температуре и не доказывает отсутствие pressure-эффекта в других системах.')
OUT.write_text('\n'.join(lines)+'\n', encoding='utf-8')
print('PRESSURE_COMPARISON_OK')
print(OUT)
