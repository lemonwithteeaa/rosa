#!/usr/bin/env python3
from __future__ import annotations
import csv, math, pathlib, sys

HERE = pathlib.Path(__file__).resolve().parent
PATH = HERE / "pressure_selected.tsv"
EXPECTED = [1.0, 100.0, 500.0, 1000.0]

if not PATH.exists():
    raise SystemExit("NOT_EXECUTED: pressure_selected.tsv отсутствует. Сначала фактически запустите официальный PHREEQC; скрипт не создаёт golden сам.")

with PATH.open("r", encoding="utf-8-sig", errors="replace", newline="") as f:
    rows = list(csv.DictReader(f, delimiter="\t"))

# PHREEQC selected output may contain an initial speciation line before the four
# REACTION_PRESSURE steps. Keep rows whose user-punched pressure matches targets.
found = []
for row in rows:
    raw = row.get("pressure_atm") or row.get("pressure_atm ") or ""
    try:
        p = float(raw)
    except ValueError:
        continue
    for target in EXPECTED:
        if abs(p-target) <= max(1e-8, abs(target)*1e-8):
            found.append((target, row))
            break

by_target = {}
for target, row in found:
    by_target[target] = row
missing = [p for p in EXPECTED if p not in by_target]
if missing:
    raise SystemExit(f"FAIL: нет строк для pressure_atm={missing}")

si_cols = ["SI_Calcite_user", "SI_Gypsum_user", "SI_Barite_user", "SI_Celestite_user"]
for target in EXPECTED:
    row = by_target[target]
    for col in si_cols:
        try:
            value = float((row.get(col) or "").strip())
        except ValueError:
            raise SystemExit(f"FAIL: {col} при {target} atm не число: {row.get(col)!r}")
        if not math.isfinite(value):
            raise SystemExit(f"FAIL: {col} при {target} atm не конечен: {value}")

print("PRESSURE_REFERENCE_OK")
for target in EXPECTED:
    r = by_target[target]
    print(target, *(r.get(c, "") for c in si_cols))
