//go:build windows

package main

import "syscall"

// Reusable typography and sizing tokens for ROSA workspaces. The economics
// workspace adopts them in 815-RC. Other sections continue using their own
// stable layouts until individually validated against this shared system.
// Native font handles are reused: this adds no GDI allocations to navigation.
type rosaUIRole uint8

const (
	rosaUIBody rosaUIRole = iota
	rosaUICaption
	rosaUIAction
	rosaUISectionHeading
	rosaUIMetric
)
const (
	rosaUIFontMinimumCaption      = int32(14)
	rosaUIFontMinimumAction       = int32(17)
	rosaUIButtonHeight            = int32(40)
	rosaUIInputHeight             = int32(30)
	rosaUIButtonPaddingHorizontal = int32(12)
)

func rosaUIFont(role rosaUIRole) syscall.Handle {
	switch role {
	case rosaUICaption:
		return hFontHeaderSmall // 14px / 700
	case rosaUIAction:
		return hFontButton // 17px / 600
	case rosaUISectionHeading:
		return hFontBold // 17px / 600
	case rosaUIMetric:
		return hFontSystemName // 23px / 600
	default:
		return hFont // 17px / 400
	}
}
func rosaUISetFont(h syscall.Handle, role rosaUIRole) {
	if h != 0 {
		if f := rosaUIFont(role); f != 0 {
			pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(f), 1)
		}
	}
}

// Central control inventory prevents accidental fallback to a 10px font on
// economics buttons. Only this workspace is opted in; other pages are untouched.
func rosaUIEconomicsAction(h syscall.Handle) bool {
	if h == 0 {
		return false
	}
	switch h {
	case econTemplateSaveButton, econTemplateLoadButton, econUseObjectButton,
		econCalculateAllButton, econCalculateButton, econFullResultButton,
		econChartButton, econMethodBaseButton, econMethodNextButton,
		econ812Details, econBasicButton, econExtendedButton:
		return true
	}
	return false
}

func rosaUIApplyEconomicsStyles() {
	for _, h := range []syscall.Handle{
		econTemplateSaveButton, econTemplateLoadButton, econUseObjectButton,
		econCalculateAllButton, econCalculateButton, econFullResultButton,
		econChartButton, econMethodBaseButton, econMethodNextButton,
		econ812Details, econBasicButton, econExtendedButton,
	} {
		rosaUISetFont(h, rosaUIAction)
	}
	for _, h := range []syscall.Handle{
		econInputsTitle, econSchemesTitle, econCompareTitle, econ813BreakTitle,
	} {
		rosaUISetFont(h, rosaUISectionHeading)
	}
	for _, h := range []syscall.Handle{
		econModeInfo, econDecisionStatus, econObjectLabel, econ813ScenarioHint,
	} {
		rosaUISetFont(h, rosaUICaption)
	}
	for _, h := range econFieldLabels {
		rosaUISetFont(h, rosaUIBody)
	}
	for _, h := range econ812Units {
		rosaUISetFont(h, rosaUICaption)
	}
	for _, h := range econ812States {
		rosaUISetFont(h, rosaUICaption)
	}
	for _, h := range economicsInputHandles() {
		rosaUISetFont(h, rosaUIBody)
	}
	rosaUISetFont(econFundFilterCombo, rosaUIBody)
	rosaUISetFont(econCompareList, rosaUIBody)
	setListHeaderFont(econCompareList, rosaUIFont(rosaUIAction))
}
