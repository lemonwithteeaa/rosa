"""Fail the build if the 7.3.11.444-RC welcome-window layout changes."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
MANIFEST = json.loads((ROOT / 'WELCOME_444_MANIFEST.json').read_text(encoding='utf-8'))
REFERENCE_SHA256 = 'c7b606a1e1620ae063afcf55f73d7cb592549ac64a3385813b377d807a283b46'

def digest(data):
    return hashlib.sha256(data).hexdigest()

def check(source):
    for name, expected in MANIFEST['blocks'].items():
        start, end = f'// WELCOME_444_{name}_BEGIN', f'// WELCOME_444_{name}_END'
        if source.count(start) != 1 or source.count(end) != 1:
            raise ValueError(f'{name}: markers missing or duplicated')
        begin, finish = source.index(start), source.index(end) + len(end)
        if begin >= finish or digest(source[begin:finish].encode()) != expected:
            raise ValueError(f'{name}: reference layout changed')
    for name, expected in MANIFEST['assets'].items():
        if digest((ROOT / name).read_bytes()) != expected:
            raise ValueError(f'{name}: reference asset changed')
    if MANIFEST['reference_exe_sha256'] != REFERENCE_SHA256:
        raise ValueError('reference EXE fingerprint changed')
    reference = ROOT.parent / 'Эталон_приветственного_окна_444' / 'РОСА_7.3.11.444-RC_ЭТАЛОН.exe'
    if reference.exists() and digest(reference.read_bytes()) != REFERENCE_SHA256:
        raise ValueError('reference EXE changed')

if __name__ == '__main__':
    source = (ROOT / 'auth.go').read_text(encoding='utf-8')
    check(source)
    try:
        check(source.replace('featureY := []int32{280, 390, 500, 610}', 'featureY := []int32{281, 390, 500, 610}', 1))
    except ValueError:
        pass
    else:
        raise SystemExit('FAIL: mutation was not detected')
    print('PASS: welcome 444 layout, assets, reference hash and mutation detection')
