PAGE_Y_OFFSET = 88


def decision_row(window_width, window_height):
    x = 242
    content_w = max(720, window_width - 270)
    right = x + content_w
    compact = window_height <= 930
    very_compact = window_height < 820
    decision_h = 72
    if compact:
        decision_h = 58
    if very_compact:
        decision_h = 54

    gap = 8
    use_w, save_w, approve_w = 154, 174, 184
    buttons_w = use_w + save_w + approve_w + 2 * gap
    region_w = max(420, content_w - buttons_w - 14)
    info_w = min(304, max(168, region_w - 292 - gap))
    text_w = region_w - info_w - gap
    button_h = min(50, max(44, decision_h - 4))

    info_x = x + text_w + gap
    use_x = x + region_w + 14
    save_x = use_x + use_w + gap
    approve_x = save_x + save_w + gap
    return locals()


for width, height in ((1280, 729), (1366, 768), (1600, 794), (1600, 900), (1920, 1080)):
    row = decision_row(width, height)
    assert row["button_h"] == 50, (width, height, row["button_h"])
    assert row["button_h"] <= row["decision_h"] - 4
    assert row["text_w"] >= 292
    assert row["info_w"] >= 168
    assert row["info_x"] + row["info_w"] <= row["use_x"] - 14
    assert row["approve_x"] < row["right"]
    print(
        f"PASS {width}x{height}: button={row['button_h']} px, "
        f"decision={row['text_w']} px, methods={row['info_w']} px"
    )

print("RC710 decision-action layout matrix PASS")
