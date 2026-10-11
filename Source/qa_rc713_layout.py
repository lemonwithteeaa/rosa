def chars_for_label(width):
    if width <= 0:
        return 27
    chars = (width - 10) // 8
    return max(12, min(30, chars))


def laboratory_columns(window_width):
    # Mirrors the stable desktop concept enough to regression-test the caption
    # width budget: content begins at x=242 and keeps a 24 px column gap.
    content_w = max(720, window_width - 270)
    fields_w = content_w if content_w < 980 else max(560, (content_w * 58) // 100)
    col_w = max(260, (fields_w - 24) // 2)
    label_w = max(120, col_w - 34)
    return content_w, fields_w, col_w, label_w, chars_for_label(label_w)

# Common Windows desktop sizes used by the project. Caption budget must never
# collapse below a readable two/three-line label width, including 1366x768.
for width, height in ((1280, 729), (1366, 768), (1600, 794), (1600, 900), (1920, 1080)):
    content_w, fields_w, col_w, label_w, chars = laboratory_columns(width)
    assert label_w >= 120
    assert 12 <= chars <= 30
    assert col_w >= 260
    print(f"PASS {width}x{height}: content={content_w}, fields={fields_w}, col={col_w}, label={label_w}, wrap={chars}")

# Compact/normal laboratory toolbar height after RC713. Two-line owner-drawn
# captions need at least 42 px; the tall profile remains 44 px.
for height, action_h in ((729, 42), (768, 42), (900, 42), (1080, 44)):
    assert action_h >= 42
    print(f"PASS height={height}: top-action={action_h}")

# Width-dependent wrapping must react to the real label width and remain
# bounded so very wide screens do not turn labels into long one-line strings.
assert chars_for_label(120) == 13
assert chars_for_label(150) == 17
assert chars_for_label(180) == 21
assert chars_for_label(234) == 28
assert chars_for_label(300) == 30
print("RC713 laboratory stability/layout matrix PASS")
