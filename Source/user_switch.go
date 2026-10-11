//go:build windows

package main

const userSwitchCaption = "Смена пользователя"

var userSwitchInProgress bool

// Both historical button IDs have exactly the same lifecycle. Previously
// ID_HEADER_APPROVE omitted loading and revealing the window after login.
func switchCurrentUser() {
	if userSwitchInProgress {
		return
	}
	userSwitchInProgress = true
	defer func() { userSwitchInProgress = false }()
	if persistentStore != nil {
		persistentStore.close()
		persistentStore = nil
	}
	pShowWindow.Call(uintptr(hMain), SW_HIDE)
	clearCurrentSession()
	if !runOfflineLogin() {
		pDestroyWindowAuth.Call(uintptr(hMain))
		return
	}
	// No UI-only draft or approval cache may survive a profile change.
	invalidateMethodsBackground()
	methodsEvaluation = nil
	methodWorkflowSuggestions = nil
	methodsCandidatePage = 0
	methodsShowAll = false
	methodsShowAllObject = ""
	methodImplementationRecordsLoaded = false
	methodImplementationRecords = nil
	assessmentLabPassSnapshotValid = false
	approvedMethodsLoaded = false
	approvedMethodDecisions = nil
	assessmentClassStoreLoaded = false
	assessmentClassStore = nil
	combinedScenarioMethods = map[string][]string{}
	pageBackStack, pageForwardStack = nil, nil
	selectedObjectKey, candidateObjectKey = "", ""
	labPassesLoaded = false
	ensureDataWithVisibleTransition()
	candidateObjectKey = selectedObjectKey
	setText(hUserLabel, currentUserLabel())
	setText(hHeaderApprove, userSwitchCaption)
	loadAppIconsFallback()
	pSendMessageW.Call(uintptr(hMain), WM_SETICON, ICON_BIG, uintptr(hIconBig))
	pSendMessageW.Call(uintptr(hMain), WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
	activatePage(ID_DASH, true)
	pShowWindow.Call(uintptr(hMain), SW_MAXIMIZE)
	pUpdateWindow.Call(uintptr(hMain))
}
