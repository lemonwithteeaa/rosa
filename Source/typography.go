//go:build windows

package main

import "syscall"

var hFontPanel syscall.Handle

// All interface fonts use Segoe UI. Bold changes weight, never text size.
// Heights are logical pixels; section and brand sizes have explicit roles.
const (
	uiTextHeight            = 17
	uiPanelHeight           = uiTextHeight
	uiHeadingHeight         = 30
	uiBrandHeight           = 31
	uiHeaderBrandTextHeight = 34
	uiSystemNameHeight      = 23
	uiCaptionHeight         = 18
	uiCompactHeight         = 16
)
