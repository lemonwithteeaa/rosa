"""Immutable release gate for the approved native welcome page.

The ordinary regression tests can pass after a stale auth.go is copied back.
This gate binds the intended welcome geometry, assets and release EXE together.
Run it before compiling and again with --exe after icon embedding.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
LOCK = ROOT / "WELCOME_RELEASE_LOCK.json"


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def block_hashes(source: str, names: list[str]) -> dict[str, str]:
    out: dict[str, str] = {}
    for name in names:
        start = f"// WELCOME_444_{name}_BEGIN"
        end = f"// WELCOME_444_{name}_END"
        if source.count(start) != 1 or source.count(end) != 1:
            raise SystemExit(f"WELCOME LOCK FAIL: missing/duplicate {name} block")
        a = source.index(start)
        b = source.index(end, a) + len(end)
        out[name] = sha256(source[a:b].encode("utf-8"))
    return out


def check_source(lock: dict) -> None:
    source = (ROOT / "auth.go").read_text(encoding="utf-8")
    actual = block_hashes(source, list(lock["blocks"]))
    if actual != lock["blocks"]:
        raise SystemExit(f"WELCOME LOCK FAIL: protected blocks changed: {actual}")
    for name, expected in lock["assets"].items():
        actual_asset = sha256((ROOT / name).read_bytes())
        if actual_asset != expected:
            raise SystemExit(f"WELCOME LOCK FAIL: asset changed: {name}")
    required = [
        "comboH := int32(42)",
        "authNumberFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(28-1)",
        "authInfoLeadFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(26-1)",
        "featureNumX := rightCardX + 28",
        "authRosaSubtitle, 0, 82, 312, 44",
        "authRosaLogo, 28, 4, 256, 78",
    ]
    for marker in required:
        if marker not in source:
            raise SystemExit(f"WELCOME LOCK FAIL: required geometry missing: {marker}")
    forbidden = ["buildLoginStandaloneIconBitmap(false", "featureIconControls[i] = authCreateCentered"]
    for marker in forbidden:
        if marker in source:
            raise SystemExit(f"WELCOME LOCK FAIL: removed decoration returned: {marker}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--exe", type=Path)
    parser.add_argument("--strict-exe", action="store_true", help="require the exact archived EXE hash")
    args = parser.parse_args()
    lock = json.loads(LOCK.read_text(encoding="utf-8"))
    check_source(lock)
    if args.exe:
        if not args.exe.is_file() or args.exe.stat().st_size < 1024:
            raise SystemExit(f"WELCOME LOCK FAIL: missing or incomplete EXE: {args.exe}")
        actual = sha256(args.exe.read_bytes())
        if args.strict_exe and actual != lock["exe_sha256"]:
            raise SystemExit(f"WELCOME LOCK FAIL: EXE fingerprint mismatch: {actual}")
        print(f"PASS: welcome release lock (EXE sha256={actual})")
        return
    print("PASS: welcome release lock")


if __name__ == "__main__":
    main()
