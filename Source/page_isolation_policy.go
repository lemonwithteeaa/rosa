package main

// A page owns its child controls. A subview is an exclusive surface inside a
// page, such as one assessment tab or one of the two section-5 workspaces.
// Shared controls have scope 0 and remain available in all subviews.
const (
	pageSubviewShared = iota
	pageSubviewAssessmentTech
	pageSubviewAssessmentLab
	pageSubviewAssessmentCompare
	pageSubviewMRPFleet
	pageSubviewMRPObject
)

func pageSubviewMatches(controlScope, activeScope int) bool {
	return controlScope == pageSubviewShared || controlScope == activeScope
}

// Input and presentation use the same ownership rule. A queued notification
// from a previously visible tab cannot act on the page that replaced it.
func pageInputMayBeHandled(owner, active int, wantedVisible, transitioning, scopeMatches bool) bool {
	return pageControlMayBeVisible(owner, active, wantedVisible, transitioning) && scopeMatches
}
