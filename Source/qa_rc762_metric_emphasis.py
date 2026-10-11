from pathlib import Path

root = Path(__file__).resolve().parent
methods = (root / "methods_reference_dialog.go").read_text(encoding="utf-8")
economics = (root / "economics_fund_archive.go").read_text(encoding="utf-8")

labels = [
    "Осложнения:", "Методы:", "Доступность:", "МРП:", "Основание:", "План:",
    "Контроль:", "Критерии прекращения:", "Дополнительное условие:",
]
checks = {
    "method history has an explicit set of field labels":
        all(f'"{label}"' in methods for label in labels),
    "method and scheme headings get red bold italic formatting":
        'schemeHeading := strings.HasPrefix(t, "Схема ") || strings.HasPrefix(t, "МЕТОД ")' in methods and
        'DwEffects: CFE_BOLD | cfeItalic, CrTextColor: uint32(rgb(183, 20, 37))' in methods,
    "field labels get bold navy without styling the values":
        'DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82))' in methods and
        'labelLength := len(utf16.Encode([]rune(label)))' in methods,
    "economics detail table emphasizes the metric column":
        'hdr.HwndFrom == economicsFundDialogDetail' in economics and
        'pSelectObject.Call(uintptr(draw.NMCD.HDC), uintptr(hFontHeaderBold))' in economics and
        'draw.ClrText = uint32(rgb(15, 48, 82))' in economics,
    "economics heading remains the separate red accent":
        'pSetTextColor.Call(wParam, rgb(183, 20, 37))' in economics,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit(1)
