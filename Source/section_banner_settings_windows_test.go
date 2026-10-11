//go:build windows

package main

import "testing"

func TestSectionBannerCommandIDsFitWMCommand(t *testing.T) {
	ids := []int{
		ID_SECTION_BANNER_SETTINGS,
		idSectionBannerSettingsSave,
		idSectionBannerSettingsCancel,
		idSectionNoticeLabel,
	}
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || id > 0xffff {
			t.Fatalf("control ID %d does not fit WM_COMMAND's 16-bit ID field", id)
		}
		if seen[id] {
			t.Fatalf("duplicate section banner control ID %d", id)
		}
		seen[id] = true
	}
}

func TestSectionBannerSettingsChecklistFitsDialog(t *testing.T) {
	const (
		rowTop       = 128
		rowStep      = 34
		rowHeight    = 32
		rowCount     = 10
		checkLeft    = 30
		checkWidth   = 800
		footerTop    = 482
		footerHeight = 30
		buttonTop    = 532
		buttonHeight = 42
	)
	lastRowBottom := rowTop + (rowCount-1)*rowStep + rowHeight
	if lastRowBottom > footerTop {
		t.Fatalf("last section label ends at %d, overlapping footer at %d", lastRowBottom, footerTop)
	}
	if footerTop+footerHeight > buttonTop {
		t.Fatalf("footer ends at %d, overlapping buttons at %d", footerTop+footerHeight, buttonTop)
	}
	if buttonTop+buttonHeight > sectionBannerSettingsWindowHeight {
		t.Fatalf("settings buttons end at %d, outside dialog height %d", buttonTop+buttonHeight, sectionBannerSettingsWindowHeight)
	}
	if checkLeft+checkWidth > sectionBannerSettingsWindowWidth-10 {
		t.Fatalf("section labels end at %d, outside dialog width %d", checkLeft+checkWidth, sectionBannerSettingsWindowWidth)
	}
}
