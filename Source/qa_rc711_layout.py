def section7_action_headers(window_width):
    x = 242
    content_w = max(720, window_width - 270)
    right = x + content_w
    gap = 8

    add_w = copy_w = compare_w = 208
    total = add_w + copy_w + compare_w + 2 * gap
    add_x = right - total
    copy_x = add_x + add_w + gap
    compare_x = copy_x + copy_w + gap

    exports_total = 104 + 86 + 92 + 92 + 3 * gap
    archive_header_w = max(180, content_w - exports_total - 12)
    hint_w = archive_header_w
    return locals()


for width in (1280, 1366, 1600, 1920):
    row = section7_action_headers(width)
    assert row["add_w"] == row["copy_w"] == row["compare_w"] == 208
    assert row["add_x"] >= row["x"] + 340
    assert row["compare_x"] + row["compare_w"] == row["right"]
    assert row["hint_w"] >= 600
    print(
        f"PASS width={width}: variants=3x{row['add_w']} px, "
        f"hint={row['hint_w']} px"
    )

print("RC711 section-7 action headers PASS")
