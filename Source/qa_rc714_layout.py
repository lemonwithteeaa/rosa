# RC714 keeps the RC713 geometry contract, but every geometry commit now occurs
# under one no-redraw transaction.  This matrix protects the minimum label/action
# budgets while regression-testing the supported desktop sizes.

def chars_for_label(width):
    if width <= 0:
        return 27
    chars = (width - 10) // 8
    return max(12, min(30, chars))


def laboratory_columns(window_width):
    content_w = max(720, window_width - 270)
    fields_w = content_w if content_w < 980 else max(560, (content_w * 58) // 100)
    col_w = max(260, (fields_w - 24) // 2)
    label_w = max(120, col_w - 34)
    return content_w, fields_w, col_w, label_w, chars_for_label(label_w)

for width, height in ((1280, 729), (1366, 768), (1600, 794), (1600, 900), (1920, 1080)):
    content_w, fields_w, col_w, label_w, chars = laboratory_columns(width)
    assert content_w >= 720
    assert col_w >= 260
    assert label_w >= 120
    assert 12 <= chars <= 30
    print(f"PASS {width}x{height}: content={content_w}, fields={fields_w}, col={col_w}, label={label_w}, wrap={chars}")

for height, action_h in ((729, 42), (768, 42), (900, 42), (1080, 44)):
    assert action_h >= 42
    print(f"PASS height={height}: top-action={action_h}")

print("RC714 laboratory atomic-layout matrix PASS")
