//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// Window-level stability instruments. The presentation logic remains on the
// Win32 owner thread; the observer posts messages but never touches controls.
const (
	WS_CLIPCHILDREN_FRAME        = 0x02000000
	WM_APP_UI_PULSE       uint32 = 0x80EF
	WM_APP_LAYOUT_PENDING uint32 = 0x80EE
	frameNoRedraw                = 0x0008
	frameNoSize                  = 0x0001
	frameNoMove                  = 0x0002
	frameNoZOrder                = 0x0004
	frameNoActivate              = 0x0010
	frameShowWindow              = 0x0040
	frameHideWindow              = 0x0080
)

var (
	lastUIResponseNs  atomic.Int64
	pulsePending      atomic.Bool
	mainLayoutQueued  bool // only main Win32 thread may read/write
	mainLayoutRunning bool // only main Win32 thread may read/write
	mainLayoutReady   bool // set after the first valid client-area layout
)

func nowNano() int64 { return time.Now().UnixNano() }

func recordUIPulse() {
	lastUIResponseNs.Store(nowNano())
	pulsePending.Store(false)
}

// requestMainLayout coalesces resize storms into a single queued layout.
// Call only on the UI thread. WM_SIZE itself must not do expensive layouts.
func requestMainLayout() {
	if hMain == 0 || mainLayoutQueued || mainLayoutRunning {
		return
	}
	mainLayoutQueued = true
	ok, _, _ := pPostMessageW.Call(uintptr(hMain), uintptr(WM_APP_LAYOUT_PENDING), 0, 0)
	if ok == 0 {
		mainLayoutQueued = false
	}
}

func executeMainLayout() {
	// A page activation may have already laid out the latest client size after
	// this message was queued. Do not repeat the full layout on another page.
	if !mainLayoutQueued {
		return
	}
	mainLayoutQueued = false
	if mainLayoutRunning || hMain == 0 {
		return
	}
	mainLayoutRunning = true
	started := time.Now()
	defer func() {
		mainLayoutRunning = false
		reportSlowFrame("layout", currentPage, started, 150*time.Millisecond)
	}()
	page := currentPage
	beginPageVisualUpdate(page)
	defer endPageVisualUpdate(page)
	resize()
	mainLayoutReady = true
	// The batch completes in the defer even if a layout callback fails.
}

// A geometry batch is confined to the Win32 owner thread. Duplicate position
// updates collapse to the last rectangle; Begin/Defer/EndWindowPos applies all
// moves as one native window-management transaction. Fonts/readback must be
// adjusted *after* endLayoutBatch, never while positions are pending.
type layoutMove struct {
	h               syscall.Handle
	x, y, w, height int32
}

var layoutBatchDepth int
var layoutPending []layoutMove
var layoutPendingIndex = map[syscall.Handle]int{}

func beginLayoutBatch() { layoutBatchDepth++ }

func queueLayoutPosition(h syscall.Handle, x, y, w, height int32) {
	if index, ok := layoutPendingIndex[h]; ok {
		layoutPending[index] = layoutMove{h, x, y, w, height}
		return
	}
	layoutPendingIndex[h] = len(layoutPending)
	layoutPending = append(layoutPending, layoutMove{h, x, y, w, height})
}

func endLayoutBatch() {
	if layoutBatchDepth <= 0 {
		return
	}
	layoutBatchDepth--
	if layoutBatchDepth != 0 {
		return
	}
	pending := layoutPending
	layoutPending = nil
	clear(layoutPendingIndex)
	changed := make([]layoutMove, 0, len(pending))
	for _, move := range pending {
		if move.h == 0 || move.w <= 0 || move.height <= 0 {
			continue
		}
		var rect RECT
		if ok, _, _ := pGetWindowRect.Call(uintptr(move.h), uintptr(unsafe.Pointer(&rect))); ok != 0 {
			parent, _, _ := pGetParent.Call(uintptr(move.h))
			origin := POINT{X: rect.Left, Y: rect.Top}
			if parent != 0 {
				pScreenToClient.Call(parent, uintptr(unsafe.Pointer(&origin)))
			}
			if origin.X == move.x && origin.Y == move.y && rect.Right-rect.Left == move.w && rect.Bottom-rect.Top == move.height {
				continue
			}
		}
		changed = append(changed, move)
	}
	if len(changed) == 0 {
		return
	}
	hdwp, _, _ := pBeginDeferWindowPos.Call(uintptr(len(changed)))
	if hdwp != 0 {
		for _, move := range changed {
			next, _, _ := pDeferWindowPos.Call(hdwp, uintptr(move.h), 0, uintptr(move.x), uintptr(move.y), uintptr(move.w), uintptr(move.height), uintptr(frameNoZOrder|frameNoActivate))
			if next == 0 {
				hdwp = 0
				break
			}
			hdwp = next
		}
		if hdwp != 0 {
			if ok, _, _ := pEndDeferWindowPos.Call(hdwp); ok != 0 {
				return
			}
		}
	}
	// Rare API failure: apply the final geometry once per window instead of
	// leaving an incomplete page. This also handles low-resource Win32 cases.
	for _, move := range changed {
		pSetWindowPos.Call(uintptr(move.h), 0, uintptr(move.x), uintptr(move.y), uintptr(move.w), uintptr(move.height), uintptr(frameNoZOrder|frameNoActivate))
	}
	diagLog("LAYOUT batch fallback count=%d", len(changed))
}

func noRedrawVisibility(h syscall.Handle, visible bool) {
	// Windows must invalidate both the revealed child and the uncovered parent.
	// SWP_NOREDRAW suppressed this and left persistent white fragments.
	flags := uintptr(frameNoSize | frameNoMove | frameNoZOrder | frameNoActivate)
	if visible {
		flags |= frameShowWindow
	} else {
		flags |= frameHideWindow
	}
	pSetWindowPos.Call(uintptr(h), 0, 0, 0, 0, 0, flags)
}

func reportSlowFrame(kind string, page int, started time.Time, threshold time.Duration) {
	elapsed := time.Since(started)
	if elapsed < threshold {
		return
	}
	limitedHealthLog(fmt.Sprintf("SLOW kind=%s page=%d duration_ms=%d", kind, page, elapsed.Milliseconds()))
}

func limitedHealthLog(line string) {
	root := dataDir
	if root == "" {
		root = appDataDir()
	}
	if root == "" {
		return
	}
	dir := filepath.Join(root, "health")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	path := filepath.Join(dir, "frame_health.log")
	if stat, err := os.Stat(path); err == nil && stat.Size() > 2<<20 {
		_ = os.Remove(path + ".old")
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(f, "%s | %s\n", time.Now().Format("2006-01-02T15:04:05.000"), line)
	_ = f.Close()
}

// A stall report is diagnostic only. It deliberately does not close, restart,
// hide or restore the window, which could destroy pending user input.
func dumpUIStall(duration time.Duration) {
	limitedHealthLog(fmt.Sprintf("STALL message_loop_no_heartbeat_ms=%d", duration.Milliseconds()))
	buf := make([]byte, 256*1024)
	n := runtime.Stack(buf, true)
	root := dataDir
	if root == "" {
		root = appDataDir()
	}
	if root == "" {
		return
	}
	dir := filepath.Join(root, "health")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	// Only the last report is kept. Stacks contain source paths and stay local.
	_ = os.WriteFile(filepath.Join(dir, "last_stall_stack.txt"), buf[:n], 0600)
}

func startUIWatchdog(hwnd syscall.Handle) func() {
	recordUIPulse()
	stopped := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		tick := time.NewTicker(750 * time.Millisecond)
		defer tick.Stop()
		var lastWarning time.Time
		for {
			select {
			case <-stopped:
				return
			case <-tick.C:
				if !pulsePending.Swap(true) {
					ok, _, _ := pPostMessageW.Call(uintptr(hwnd), uintptr(WM_APP_UI_PULSE), 0, 0)
					if ok == 0 {
						pulsePending.Store(false)
					}
				}
				elapsed := time.Since(time.Unix(0, lastUIResponseNs.Load()))
				if elapsed >= 3*time.Second && time.Since(lastWarning) >= 10*time.Second {
					lastWarning = time.Now()
					dumpUIStall(elapsed)
				}
			}
		}
	}()
	return func() { close(stopped); <-finished }
}
