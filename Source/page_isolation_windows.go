//go:build windows

package main

import "syscall"

// All writes are made on the main Win32 thread. A HWND belongs to precisely
// one page; only explicitly exclusive controls acquire a subview scope.
var pageControlSubview = map[syscall.Handle]int{}
var pageActiveSubview = map[int]int{}
var labGroupGeneration uintptr // UI thread; invalidates queued history selections.
var unownedInputReported = map[syscall.Handle]bool{}

func registerPageSubviewControl(page, scope int, controls ...syscall.Handle) {
	for _, h := range controls {
		if h == 0 {
			continue
		}
		if pageControlOwner[h] != page {
			diagLog("UI subview owner conflict hwnd=%d expected=%d actual=%d", h, page, pageControlOwner[h])
			continue
		}
		pageControlSubview[h] = scope
	}
}

func setPageSubview(page, scope int) {
	pageActiveSubview[page] = scope
}

func pageControlSubviewMatches(h syscall.Handle, page int) bool {
	return pageSubviewMatches(pageControlSubview[h], pageActiveSubview[page])
}

// Child notifications can originate from a ListView's header or another
// native subwindow. Walk up to the registered ListView before deciding ownership.
func pageInputSourceAllowed(h syscall.Handle) bool {
	if pageSwitching || pageVisualUpdateDepth[currentPage] > 0 {
		return false
	}
	if h == 0 {
		return true
	} // menu/posted command; guarded by its handler
	for depth := 0; h != 0 && depth < 8; depth++ {
		if owner, known := pageControlOwner[h]; known {
			return pageInputMayBeHandled(owner, currentPage, pageControlVisible[h], false, pageControlSubviewMatches(h, owner))
		}
		if isPersistentChromeControl(h) {
			return true
		}
		parent, _, _ := pGetParent.Call(uintptr(h))
		if parent == uintptr(hMain) || parent == 0 || parent == uintptr(h) {
			// A direct child of the main window must be registered or be chrome.
			// Modal windows have their own dispatcher and do not reach this path.
			if !unownedInputReported[h] {
				unownedInputReported[h] = true
				diagLog("UI ignored unowned child input hwnd=%d active=%d", h, currentPage)
			}
			return false
		}
		h = syscall.Handle(parent)
	}
	return false
}
