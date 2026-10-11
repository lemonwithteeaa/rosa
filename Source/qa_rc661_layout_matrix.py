from dataclasses import dataclass


@dataclass
class Result:
    resolution: str
    scale: int
    no_lab_ok: bool
    with_lab_ok: bool
    history_visible: bool
    horizontal_scroll_required: bool


def classification_layout(width: int, height: int, has_lab: bool):
    # Mirrors layoutAssessmentInnerTabs. DPI-aware Win32 client coordinates stay
    # in the actual maximized client pixel space; scale affects font validation,
    # covered separately by the 42/48 px wrapped action heights.
    page_offset = 88
    tab_y = 190 + page_offset
    tab_h = 44
    overall_y = tab_y + tab_h + 8
    overall_h = 52
    list_y = overall_y + overall_h + 6
    history_list_h = 56 if height < 820 else max(64, min(112, height // 9))
    history_y = height - 12 - history_list_h - 22 - 4
    reserve = 120
    if has_lab:
        reserve = 178 if height < 820 else 230
    desired = min(194, 26 + min(8, 7) * 24)
    minimum = 64 if height < 820 else 104
    list_h = min(desired, max(minimum, history_y - list_y - reserve))
    editor_y = list_y + list_h + 8
    if not has_lab:
        detail_y = editor_y + 48 + 8
        detail_h = max(54, history_y - detail_y - 8)
    else:
        if height < 820:
            caption_h, comment_h, choice_h, approve_h, gap, row_gap = 18, 34, 38, 42, 4, 4
        else:
            caption_h, comment_h, choice_h, approve_h, gap, row_gap = 22, 42, 40, 42, 8, 7
        choices_y = editor_y + caption_h + comment_h + gap
        approve_y = choices_y + choice_h + row_gap
        detail_y = approve_y + approve_h + row_gap
        detail_h = max(20, history_y - detail_y - 8)
    detail_bottom = detail_y + detail_h
    content_w = max(720, width - 270)
    column_total = max(620, content_w - 44)
    return detail_bottom <= history_y, history_y > list_y, column_total < content_w


results = []
for width, height in [(1280, 729), (1366, 768), (1600, 900), (1920, 1080)]:
    for scale in (100, 125):
        no_lab_ok, history_visible, columns_fit = classification_layout(width, height, False)
        with_lab_ok, history_visible_lab, columns_fit_lab = classification_layout(width, height, True)
        results.append(Result(
            resolution=f"{width}x{height}",
            scale=scale,
            no_lab_ok=no_lab_ok,
            with_lab_ok=with_lab_ok,
            history_visible=history_visible and history_visible_lab,
            horizontal_scroll_required=not (columns_fit and columns_fit_lab),
        ))

failed = False
for row in results:
    ok = row.no_lab_ok and row.with_lab_ok and row.history_visible and not row.horizontal_scroll_required
    failed |= not ok
    print(
        ("PASS" if ok else "FAIL"),
        row.resolution,
        f"Windows {row.scale}%",
        f"no_lab={row.no_lab_ok}",
        f"with_lab={row.with_lab_ok}",
        f"history={row.history_visible}",
        f"horizontal_scroll={row.horizontal_scroll_required}",
    )

raise SystemExit(1 if failed else 0)
