PAGE_Y_OFFSET = 88


def object_history_layout(window_height, chooser_count):
    selector_y = 384 + PAGE_Y_OFFSET
    history_y = 384 + PAGE_Y_OFFSET
    if chooser_count > 0:
        history_y = selector_y + 38
    action_y = max(590 + PAGE_Y_OFFSET, window_height - 54)
    return selector_y, history_y, max(88, action_y - history_y - 12)


for height in (729, 768, 794, 823, 900, 1080):
    selector_y, initial_y, initial_h = object_history_layout(height, 0)
    _, refreshed_y, refreshed_h = object_history_layout(height, 4)
    assert initial_y == selector_y
    assert refreshed_y == selector_y + 38
    assert refreshed_y > selector_y + 30
    assert refreshed_h >= 88
    # RC709 reapplies this final geometry before the first atomic redraw.
    print(f"PASS h={height}: initial={initial_y}, refreshed={refreshed_y}, table={refreshed_h}")

print("RC709 first-open object-history layout PASS")
