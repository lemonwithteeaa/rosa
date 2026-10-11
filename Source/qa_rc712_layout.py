PAGE_Y_OFFSET = 88


def section7_layout(width, height):
    x = 242
    content_w = max(720, width - 270)
    right = x + content_w
    top = 94 + PAGE_Y_OFFSET
    bottom = height - 18
    compact = height <= 930
    very = height < 820

    object_y, object_h, cards_gap, cards_h, variants_h = top + 58, 58, 10, 84, 132
    button_h = 38
    decision_budget, decision_card_h = 82, 104
    archive_button_h = 34
    if compact:
        object_y, object_h, cards_gap, cards_h, variants_h = top + 54, 48, 8, 60, 86
        decision_budget, decision_card_h = 68, 90
    if very:
        object_y, object_h, cards_gap, cards_h, variants_h = top + 52, 44, 6, 50, 70
        button_h = 34
        decision_budget, decision_card_h = 64, 82
        archive_button_h = 30

    cards_y = object_y + object_h + cards_gap
    variant_title_y = cards_y + cards_h + 8
    variants_y = variant_title_y + button_h + 6
    decision_y = variants_y + variants_h + 8

    action_gap = 8
    method_w, economy_w, approve_w = 220, 180, 200
    actions_w = method_w + economy_w + approve_w + 2 * action_gap
    actions_x = right - actions_w
    decision_w = actions_x - x - 14
    archive_tools_y = decision_y + decision_budget
    archive_y = archive_tools_y + archive_button_h + 6
    archive_h = max(150, bottom - archive_y)
    refresh_x = right - 190
    return locals()


for width, height in ((1280, 729), (1366, 768), (1600, 794), (1600, 900), (1920, 1080)):
    row = section7_layout(width, height)
    assert row["decision_w"] >= 380
    assert row["decision_card_h"] >= 82
    assert row["archive_y"] + row["archive_h"] <= height
    assert row["archive_h"] >= 150
    assert row["refresh_x"] + 190 == row["right"]
    assert row["actions_x"] + row["actions_w"] == row["right"]
    print(
        f"PASS {width}x{height}: decision={row['decision_w']}x{row['decision_card_h']}, "
        f"archive={row['archive_h']}, refresh_x={row['refresh_x']}"
    )

print("RC712 section-7 decision layout PASS")
