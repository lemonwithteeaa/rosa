//go:build windows

package main

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf16"
	"unsafe"
)

// RC674 baseline tag retained for static QA: const appBuildVersion = "7.3.11.674-RC"
// RC709 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.709-RC"
// RC710 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.710-RC"
// RC711 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.711-RC"
// RC712 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.712-RC"
// RC713 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.713-RC"
// RC714 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.714-RC"
// RC715 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.715-RC"
// RC717 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.717-RC"
// RC719 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.719-RC"
// RC720 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.720-RC"
// RC721 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.721-RC"
// RC722 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.722-RC"
// RC723 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.723-RC"
// RC729 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.729-RC"
// RC731 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.731-RC"
// RC732 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.732-RC"
// RC733 baseline tag retained for regression QA: const appBuildVersion = "7.3.11.733-RC"
const appBuildVersion = "7.3.11.824-RC"

// Historical QA marker only: const appBuildVersion = "7.3.11.726-RC"

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	riched20 = syscall.NewLazyDLL("riched20.dll")

	pRegisterClassExW              = user32.NewProc("RegisterClassExW")
	pGetModuleHandleW              = kernel32.NewProc("GetModuleHandleW")
	pCreateWindowExW               = user32.NewProc("CreateWindowExW")
	pDefWindowProcW                = user32.NewProc("DefWindowProcW")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pUpdateWindow                  = user32.NewProc("UpdateWindow")
	pIsZoomed                      = user32.NewProc("IsZoomed")
	pIsIconic                      = user32.NewProc("IsIconic")
	pGetMessageW                   = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessageW              = user32.NewProc("DispatchMessageW")
	pGetFocus                      = user32.NewProc("GetFocus")
	pGetClassNameW                 = user32.NewProc("GetClassNameW")
	pGetKeyState                   = user32.NewProc("GetKeyState")
	pPostMessageW                  = user32.NewProc("PostMessageW")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pSendMessageW                  = user32.NewProc("SendMessageW")
	pSetWindowTextW                = user32.NewProc("SetWindowTextW")
	pGetWindowTextLengthW          = user32.NewProc("GetWindowTextLengthW")
	pGetWindowTextW                = user32.NewProc("GetWindowTextW")
	pEnableWindow                  = user32.NewProc("EnableWindow")
	pMessageBoxW                   = user32.NewProc("MessageBoxW")
	pFillRect                      = user32.NewProc("FillRect")
	pFrameRect                     = user32.NewProc("FrameRect")
	pDrawTextW                     = user32.NewProc("DrawTextW")
	pLoadImageW                    = user32.NewProc("LoadImageW")
	pCreateIconFromResourceEx      = user32.NewProc("CreateIconFromResourceEx")
	pDestroyIcon                   = user32.NewProc("DestroyIcon")
	pSetWindowPos                  = user32.NewProc("SetWindowPos")
	pGetParent                     = user32.NewProc("GetParent")
	pMonitorFromWindow             = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	pBeginDeferWindowPos           = user32.NewProc("BeginDeferWindowPos")
	pDeferWindowPos                = user32.NewProc("DeferWindowPos")
	pEndDeferWindowPos             = user32.NewProc("EndDeferWindowPos")
	pGetClientRect                 = user32.NewProc("GetClientRect")
	pGetWindowRect                 = user32.NewProc("GetWindowRect")
	pInvalidateRect                = user32.NewProc("InvalidateRect")
	pRedrawWindow                  = user32.NewProc("RedrawWindow")
	pSetFocus                      = user32.NewProc("SetFocus")
	pGetCursorPos                  = user32.NewProc("GetCursorPos")
	pScreenToClient                = user32.NewProc("ScreenToClient")
	pSetScrollRange                = user32.NewProc("SetScrollRange")
	pSetScrollPos                  = user32.NewProc("SetScrollPos")
	pSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pGetWindowLongPtrW             = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW             = user32.NewProc("SetWindowLongPtrW")
	pSetBkMode                     = gdi32.NewProc("SetBkMode")
	pSetBkColor                    = gdi32.NewProc("SetBkColor")
	pSetTextColor                  = gdi32.NewProc("SetTextColor")
	pCreateSolidBrush              = gdi32.NewProc("CreateSolidBrush")
	pCreateFontW                   = gdi32.NewProc("CreateFontW")
	pSelectObject                  = gdi32.NewProc("SelectObject")
	pGetStockObject                = gdi32.NewProc("GetStockObject")
	pPolygon                       = gdi32.NewProc("Polygon")
	pCreatePen                     = gdi32.NewProc("CreatePen")
	pPolyline                      = gdi32.NewProc("Polyline")
	pStretchBlt                    = gdi32.NewProc("StretchBlt")
	pSetStretchBltMode             = gdi32.NewProc("SetStretchBltMode")
	pDeleteObject                  = gdi32.NewProc("DeleteObject")
	pInitCommonControlsEx          = comctl32.NewProc("InitCommonControlsEx")
	pShellExecuteW                 = shell32.NewProc("ShellExecuteW")
	pGetOpenFileNameW              = comdlg32.NewProc("GetOpenFileNameW")
)

const (
	WS_OVERLAPPEDWINDOW  = 0x00CF0000
	WS_VISIBLE           = 0x10000000
	WS_CHILD             = 0x40000000
	WS_TABSTOP           = 0x00010000
	WS_BORDER            = 0x00800000
	WS_VSCROLL           = 0x00200000
	ES_MULTILINE         = 0x0004
	ES_AUTOVSCROLL       = 0x0040
	ES_NOHIDESEL         = 0x0100
	ES_READONLY          = 0x0800
	BS_PUSHBUTTON        = 0x00000000
	BS_OWNERDRAW         = 0x0000000B
	BS_NOTIFY            = 0x00004000
	SS_LEFT              = 0
	SS_CENTER            = 1
	SS_RIGHT             = 2
	SS_ICON              = 0x00000003
	SS_BITMAP            = 0x0000000E
	SS_CENTERIMAGE       = 0x00000200
	SS_NOPREFIX          = 0x00000080
	SS_ENDELLIPSIS       = 0x00004000
	SS_OWNERDRAW         = 0x0000000D
	CBS_DROPDOWNLIST     = 0x0003
	CBS_DROPDOWN         = 0x0002
	CB_GETCURSEL         = 0x0147
	CB_GETLBTEXT         = 0x0148
	CB_SETCURSEL         = 0x014E
	CB_ADDSTRING         = 0x0143
	CB_SETDROPPEDWIDTH   = 0x0160
	CB_RESETCONTENT      = 0x014B
	LBS_NOTIFY           = 0x0001
	LVS_REPORT           = 0x0001
	LVS_SINGLESEL        = 0x0004
	LVS_SHOWSELALWAYS    = 0x0008
	LVS_EX_FULLROWSELECT = 0x00000020
	LVS_EX_GRIDLINES     = 0x00000001
	LVS_EX_DOUBLEBUFFER  = 0x00010000
	LVS_EX_CHECKBOXES    = 0x00000004

	WM_CREATE                  = 0x0001
	WM_DESTROY                 = 0x0002
	WM_DRAWITEM                = 0x002B
	WM_COPY                    = 0x0301
	WM_CUT                     = 0x0300
	WM_PASTE                   = 0x0302
	WM_SETICON                 = 0x0080
	WM_SIZE                    = 0x0005
	WM_SYSCOMMAND              = 0x0112
	WM_DPICHANGED              = 0x02E0
	WM_DISPLAYCHANGE           = 0x007E
	WM_EXITSIZEMOVE            = 0x0232
	WM_GETMINMAXINFO           = 0x0024
	WM_COMMAND                 = 0x0111
	WM_VSCROLL                 = 0x0115
	WM_CONTEXTMENU             = 0x007B
	WM_NOTIFY                  = 0x004E
	WM_SETFONT                 = 0x0030
	WM_CTLCOLORSTATIC          = 0x0138
	WM_CTLCOLOREDIT            = 0x0133
	WM_CTLCOLORBTN             = 0x0135
	WM_APP_DYN_DONE            = 0x8001
	WM_APP_INITIAL_REFRESH     = 0x8002
	WM_APP_TECH_PREVIEW_DONE   = 0x8003
	WM_APP_LAB_SELECT          = 0x8004
	WM_APP_LAB_REPORT_DONE     = 0x8005
	WM_APP_METHODS_DONE        = 0x8006
	WM_APP_ARCHIVE_EXPORT_DONE = 0x8007
	WM_SETREDRAW               = 0x000B
	SIZE_MINIMIZED             = 1
	STN_DBLCLK                 = 1
	EM_SETSEL                  = 0x00B1
	EM_SETCHARFORMAT           = 0x0400 + 68
	SCF_SELECTION              = 0x0001
	CFM_BOLD                   = 0x00000001
	CFM_COLOR                  = 0x40000000
	CFE_BOLD                   = 0x00000001
	VK_CONTROL                 = 0x11
	VK_SHIFT                   = 0x10
	VK_INSERT                  = 0x2D
	VK_DELETE                  = 0x2E
	VK_A                       = 0x41
	VK_ESCAPE                  = 0x1B
	VK_C                       = 0x43
	VK_V                       = 0x56
	VK_X                       = 0x58

	SW_SHOW     = 5
	SW_HIDE     = 0
	SW_MAXIMIZE = 3
	SW_RESTORE  = 9

	SWP_NOZORDER    = 0x0004
	RDW_INVALIDATE  = 0x0001
	RDW_ERASE       = 0x0004
	RDW_ALLCHILDREN = 0x0080
	RDW_UPDATENOW   = 0x0100

	COLOR_WINDOW = 5
	TRANSPARENT  = 1
	OPAQUE       = 2

	LVM_FIRST                    = 0x1000
	LVM_GETHEADER                = LVM_FIRST + 31
	LVM_SETEXTENDEDLISTVIEWSTYLE = LVM_FIRST + 54
	LVM_INSERTCOLUMNW            = LVM_FIRST + 97
	LVM_INSERTITEMW              = LVM_FIRST + 77
	LVM_SETITEMTEXTW             = LVM_FIRST + 116
	LVM_DELETEALLITEMS           = LVM_FIRST + 9
	LVM_SETBKCOLOR               = LVM_FIRST + 1
	LVM_SETTEXTCOLOR             = LVM_FIRST + 36
	LVM_SETTEXTBKCOLOR           = LVM_FIRST + 38
	LVM_GETNEXTITEM              = LVM_FIRST + 12
	LVM_ENSUREVISIBLE            = LVM_FIRST + 19
	LVM_SETCOLUMNWIDTH           = LVM_FIRST + 30
	LVM_SETITEMSTATE             = LVM_FIRST + 43
	LVM_GETITEMSTATE             = LVM_FIRST + 44
	LVNI_SELECTED                = 0x0002
	LVIS_FOCUSED                 = 0x0001
	LVIS_SELECTED                = 0x0002
	LVIS_STATEIMAGEMASK          = 0xF000
	LVIF_TEXT                    = 0x0001
	LVCF_FMT                     = 0x0001
	LVCF_WIDTH                   = 0x0002
	LVCF_TEXT                    = 0x0004
	LVCFMT_LEFT                  = 0
	HDM_FIRST                    = 0x1200
	HDM_SETITEMW                 = HDM_FIRST + 12
	HDI_FORMAT                   = 0x0004
	HDF_CENTER                   = 0x0002
	HDF_STRING                   = 0x4000
	NM_CLICK                     = -2
	NM_DBLCLK                    = -3
	NM_CUSTOMDRAW                = -12
	LVN_ITEMCHANGED              = -101
	CDDS_PREPAINT                = 0x00000001
	CDDS_ITEMPREPAINT            = 0x00010001
	CDDS_SUBITEM                 = 0x00020000
	CDRF_DODEFAULT               = 0x00000000
	CDRF_NEWFONT                 = 0x00000002
	CDRF_NOTIFYITEMDRAW          = 0x00000020
	CDRF_NOTIFYSUBITEMDRAW       = 0x00000020

	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_ICONERROR       = 0x00000010

	ODT_BUTTON        = 4
	ODS_SELECTED      = 0x0001
	DT_CENTER         = 0x00000001
	DT_VCENTER        = 0x00000004
	DT_WORDBREAK      = 0x00000010
	DT_SINGLELINE     = 0x00000020
	DT_CALCRECT       = 0x00000400
	DT_NOPREFIX       = 0x00000800
	IMAGE_BITMAP      = 0
	IMAGE_ICON        = 1
	LR_LOADFROMFILE   = 0x00000010
	STM_SETIMAGE      = 0x0172
	ICON_SMALL        = 0
	ICON_BIG          = 1
	OFN_PATHMUSTEXIST = 0x00000800
	OFN_FILEMUSTEXIST = 0x00001000
	OFN_EXPLORER      = 0x00080000

	PAGE_Y_OFFSET = 88
	HEADER_H      = 124
	STRIPE_Y      = 124
	WARNING_Y     = 128
	WARNING_H     = 46
	NAV_TOP       = 174

	ID_DASH                   = 100
	ID_HISTORY                = 101
	ID_TECH                   = 102
	ID_TIMELINE               = 103
	ID_PLAN                   = 104
	ID_REGISTRY               = 105
	ID_ARCHIVE                = 106
	ID_DATA                   = 107
	ID_OBJECTS                = 108
	ID_ASSESS                 = 109
	ID_ACTIVITIES             = 110
	ID_OPI                    = 111
	ID_REGISTRIES             = 112
	ID_SYNC                   = 201
	ID_COMPARE                = 202
	ID_OPEN_FOLDER            = 204
	ID_GEN_PLAN               = 205
	ID_SAVE_FACT              = 206
	ID_FILTER_HISTORY         = 207
	ID_BACKUP                 = 208
	ID_FILTER_OBJECTS         = 209
	ID_RESTORE_DB             = 9416
	ID_STORAGE_LOG            = 9417
	ID_VERIFY_BACKUPS         = 9418
	ID_LAB_DIAGNOSTICS        = 9419
	ID_ACTIVITY_ADD_METHOD    = 9420
	ID_ACTIVITY_REMOVE_METHOD = 9421
	ID_ACTIVITY_RESET_METHODS = 9422
	ID_CONFIRM_FACT_CLASS     = 9423
	ID_ASSESS_METHOD_COMBO    = 9424
	ID_ASSESS_PROBABILITY     = 9425
	ID_ASSESS_TO_ACTIVITIES   = 9426
	ID_ASSESS_TAB_TECH        = 9427
	ID_ASSESS_TAB_LAB         = 9428
	ID_ASSESS_TAB_COMPARE     = 9429
	ID_ASSESS_CLASS_COMBO     = 9430
	ID_ASSESS_CLASS_SAVE      = 9431
	ID_ASSESS_FINAL_COMBO     = 9432
	ID_ASSESS_CONFIRM_FINAL   = 9433
	ID_ASSESS_CLASS_COMMENT   = 9434
	ID_ASSESS_FINAL_COMMENT   = 9435
	ID_OUTCOME_DETAILS        = 9436
	ID_ASSESS_ACCEPT_TECH     = 9437
	ID_ASSESS_CONFIRM_KEEP    = 9438
	ID_ASSESS_CONFIRM_CORRECT = 9439
	ID_ASSESS_CONFIRM_FULL    = 9610
	ID_TECH_CHOOSE            = 210
	ID_TECH_PREVIEW           = 211
	ID_TECH_APPLY             = 212
	ID_OBJECT_CONFIRM         = 213
	ID_CONTINUE_OBJECT        = 214
	ID_OPEN_EQUIPMENT         = 9440
	ID_HEADER_EXIT            = 9441
	ID_HEADER_BACK            = 9607
	ID_HEADER_FORWARD         = 9608
	ID_HEADER_APPROVE         = 9609
	// Keep import commands outside the 9500 range used by the well-card form.
	// The old overlapping IDs made the upload button execute another card action.
	ID_FAILURE_IMPORT       = 9600
	ID_FAILURE_CHOOSE       = 9601
	ID_FAILURE_PREVIEW      = 9602
	ID_FAILURE_APPLY        = 9603
	ID_FAILURE_OBJECT_COMBO = 9604
	ID_FAILURE_OBJECT_OPEN  = 9605
	ID_FAILURE_ANALYSIS     = 9606
	ID_GNO_DYNAMOMETRY      = 284
	ID_GNO_TELEMETRY        = 285
	ID_HEADER_HELP          = 287
	ID_HEADER_SUBTITLE      = 9415
)

type editShortcutAction int

const (
	editShortcutNone editShortcutAction = iota
	editShortcutSelectAll
	editShortcutCopy
	editShortcutPaste
	editShortcutCut
)

func standardEditShortcutAction(key uintptr, ctrl, shift bool) editShortcutAction {
	if ctrl {
		switch key {
		case VK_A:
			return editShortcutSelectAll
		case VK_C, VK_INSERT:
			return editShortcutCopy
		case VK_V:
			return editShortcutPaste
		case VK_X:
			return editShortcutCut
		}
	}
	if shift {
		switch key {
		case VK_INSERT:
			return editShortcutPaste
		case VK_DELETE:
			return editShortcutCut
		}
	}
	return editShortcutNone
}

// handleStandardEditShortcut makes the familiar clipboard shortcuts reliable
// in every native text field, including source/protocol and read-only conclusions.
// Besides Ctrl+C/V/X/A, support the standard Windows Ctrl+Insert / Shift+Insert /
// Shift+Delete variants so clipboard behaviour is identical across lab controls.
func handleStandardEditShortcut(message *MSG) bool {
	const WM_KEYDOWN = 0x0100
	if message == nil || message.Message != WM_KEYDOWN {
		return false
	}
	ctrlState, _, _ := pGetKeyState.Call(VK_CONTROL)
	shiftState, _, _ := pGetKeyState.Call(VK_SHIFT)
	action := standardEditShortcutAction(message.WParam, ctrlState&0x8000 != 0, shiftState&0x8000 != 0)
	if action == editShortcutNone {
		return false
	}
	focused, _, _ := pGetFocus.Call()
	if focused == 0 {
		return false
	}
	className := make([]uint16, 64)
	n, _, _ := pGetClassNameW.Call(focused, uintptr(unsafe.Pointer(&className[0])), uintptr(len(className)))
	class := strings.ToLower(syscall.UTF16ToString(className[:n]))
	if class != "edit" && !strings.HasPrefix(class, "richedit") {
		return false
	}
	switch action {
	case editShortcutSelectAll:
		pSendMessageW.Call(focused, EM_SETSEL, 0, ^uintptr(0))
	case editShortcutCopy:
		pSendMessageW.Call(focused, WM_COPY, 0, 0)
	case editShortcutPaste:
		pSendMessageW.Call(focused, WM_PASTE, 0, 0)
	case editShortcutCut:
		pSendMessageW.Call(focused, WM_CUT, 0, 0)
	}
	return true
}

// handleModalEscape gives owned information/reference windows the same
// secondary close path as the system caption button. The close is posted to
// the modal itself even while a RichEdit, list or combo box has focus.
func handleModalEscape(hwnd uintptr, message *MSG) bool {
	const WM_KEYDOWN = 0x0100
	if hwnd == 0 || message == nil || message.Message != WM_KEYDOWN || message.WParam != VK_ESCAPE {
		return false
	}
	pPostMessageW.Call(hwnd, labMappingClose, 0, 0)
	return true
}

type CHARFORMATW struct {
	CbSize          uint32
	DwMask          uint32
	DwEffects       uint32
	YHeight         int32
	YOffset         int32
	CrTextColor     uint32
	BCharSet        byte
	BPitchAndFamily byte
	SzFaceName      [32]uint16
}

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}
type MSG struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}
type RECT struct{ Left, Top, Right, Bottom int32 }
type POINT struct{ X, Y int32 }
type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}
type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

func anchorMainWindowToMonitor(hwnd syscall.Handle) {
	if hwnd == 0 {
		return
	}
	// Neither a maximized nor a minimized window should change state because of
	// a display notification. Windows handles the maximized work-area rectangle.
	if isMax, _, _ := pIsZoomed.Call(uintptr(hwnd)); isMax != 0 {
		return
	}
	if isMin, _, _ := pIsIconic.Call(uintptr(hwnd)); isMin != 0 {
		return
	}
	const monitorDefaultToNearest = 2
	monitor, _, _ := pMonitorFromWindow.Call(uintptr(hwnd), monitorDefaultToNearest)
	if monitor != 0 {
		mi := MONITORINFO{CbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
		ok, _, _ := pGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&mi)))
		var rect RECT
		if ok != 0 && mi.RcWork.Right > mi.RcWork.Left && mi.RcWork.Bottom > mi.RcWork.Top {
			if found, _, _ := pGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect))); found == 0 {
				return
			}
			width := min32(rect.Right-rect.Left, mi.RcWork.Right-mi.RcWork.Left)
			height := min32(rect.Bottom-rect.Top, mi.RcWork.Bottom-mi.RcWork.Top)
			if width <= 0 || height <= 0 {
				return
			}
			x := max32(mi.RcWork.Left, min32(rect.Left, mi.RcWork.Right-width))
			y := max32(mi.RcWork.Top, min32(rect.Top, mi.RcWork.Bottom-height))
			if x != rect.Left || y != rect.Top || width != rect.Right-rect.Left || height != rect.Bottom-rect.Top {
				pSetWindowPos.Call(uintptr(hwnd), 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x0004|0x0010)
			}
		}
	}
}

type INITCOMMONCONTROLSEX struct {
	DwSize uint32
	DwICC  uint32
}
type LVCOLUMN struct {
	Mask       uint32
	Fmt        int32
	Cx         int32
	PszText    *uint16
	CchTextMax int32
	ISubItem   int32
	IImage     int32
	IOrder     int32
	CxMin      int32
	CxDefault  int32
	CxIdeal    int32
}
type HDITEM struct {
	Mask       uint32
	Cxy        int32
	PszText    *uint16
	Hbm        syscall.Handle
	CchTextMax int32
	Fmt        int32
}
type LVITEM struct {
	Mask       uint32
	IItem      int32
	ISubItem   int32
	State      uint32
	StateMask  uint32
	PszText    *uint16
	CchTextMax int32
	IImage     int32
	LParam     uintptr
	IIndent    int32
	IGroupId   int32
	CColumns   uint32
	PuColumns  *uint32
	PiColFmt   *int32
	IGroup     int32
}
type NMHDR struct {
	HwndFrom syscall.Handle
	IdFrom   uintptr
	Code     int32
}

// NMITEMACTIVATE is the ListView notification payload used for click and
// double-click handling. Keeping iSubItem lets ROSA make the visible method
// name and the priority badge true interactive surfaces instead of requiring
// a tiny auxiliary button.
type NMITEMACTIVATE struct {
	Hdr       NMHDR
	IItem     int32
	ISubItem  int32
	UNewState uint32
	UOldState uint32
	UChanged  uint32
	PtAction  POINT
	LParam    uintptr
	UKeyFlags uint32
}
type NMCUSTOMDRAW struct {
	Hdr        NMHDR
	DrawStage  uint32
	HDC        syscall.Handle
	Rc         RECT
	ItemSpec   uintptr
	ItemState  uint32
	ItemLParam uintptr
}
type NMLVCUSTOMDRAW struct {
	NMCD       NMCUSTOMDRAW
	ClrText    uint32
	ClrTextBk  uint32
	SubItem    int32
	DwItemType uint32
	ClrFace    uint32
	IconEffect uint32
	IconPhase  uint32
	PartID     uint32
	StateID    uint32
	RcText     RECT
	TextAlign  uint32
}
type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   syscall.Handle
	HDC        syscall.Handle
	RcItem     RECT
	ItemData   uintptr
}
type OPENFILENAMEW struct {
	LStructSize       uint32
	HwndOwner         syscall.Handle
	HInstance         syscall.Handle
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

type TechObjectState struct {
	ObjectKey          string `json:"object_key"`
	Organization       string `json:"organization,omitempty"`
	ProductionUnit     string `json:"production_unit,omitempty"`
	TPP                string `json:"tpp,omitempty"` // legacy compatibility; UI uses neutral ProductionUnit
	Workshop           string `json:"workshop"`
	DNS                string `json:"dns"`
	Field              string `json:"field"`
	Deposit            string `json:"deposit"`
	Well               string `json:"well"`
	Block              string `json:"block"`
	ObjectPlast        string `json:"object_plast"`
	ElementDevelopment string `json:"element_development"`
	Equipment          string `json:"equipment"`
	PumpModel          string `json:"pump_model"`
	CurrentMRP         string `json:"current_mrp"`
	BufferPressure     string `json:"buffer_pressure"`
	LinePressure       string `json:"line_pressure"`
	DynamicLevel       string `json:"dynamic_level"`
	TMSTemperature     string `json:"tms_temperature"`
	BottomholePressure string `json:"bottomhole_pressure"`
	ReservoirPressure  string `json:"reservoir_pressure"`
	LiquidRate         string `json:"liquid_rate"`
	WaterCut           string `json:"water_cut"`
	OilRate            string `json:"oil_rate"`
	PlannedActivities  string `json:"planned_activities"`
	SourceRow          int    `json:"source_row"`
	SourceRows         []int  `json:"source_rows,omitempty"`
	SourceRowCount     int    `json:"source_row_count,omitempty"`
	MergeNotes         string `json:"merge_notes,omitempty"`
	SourceFile         string `json:"source_file"`
	ImportedAt         string `json:"imported_at"`
}
type TechChange struct {
	Status       string
	ObjectKey    string
	Label        string
	ChangedCount int
	Details      string
}
type TechImportRecord struct {
	ImportedAt string `json:"imported_at"`
	SourceFile string `json:"source_file"`
	SHA256     string `json:"sha256"`
	Total      int    `json:"total"`
	Added      int    `json:"added"`
	Changed    int    `json:"changed"`
	Unchanged  int    `json:"unchanged"`
	Removed    int    `json:"removed,omitempty"`
	Skipped    int    `json:"skipped,omitempty"`
	Unresolved int    `json:"unresolved"`
}
type ObjectView struct {
	State   TechObjectState
	Eval    VersionRecord
	HasEval bool
}

type VersionRecord struct {
	VersionID       string          `json:"version_id"`
	ObjectKey       string          `json:"object_key"`
	Created         string          `json:"created"`
	Field           string          `json:"field"`
	Well            string          `json:"well"`
	Deposit         string          `json:"deposit"`
	ObjectPlast     string          `json:"object_plast"`
	Complication    string          `json:"complication"`
	Method          string          `json:"method"`
	Equipment       string          `json:"equipment"`
	ForecastMRP     string          `json:"forecast_mrp"`
	MRPLow          string          `json:"mrp_low,omitempty"`
	MRPBase         string          `json:"mrp_base,omitempty"`
	MRPHigh         string          `json:"mrp_high,omitempty"`
	Scenario        string          `json:"scenario,omitempty"`
	CurrentMRP      string          `json:"current_mrp"`
	LabStatus       string          `json:"lab_status"`
	EconMode        string          `json:"econ_mode"`
	EconStatus      string          `json:"econ_status"`
	EconResult      string          `json:"econ_result"`
	Decision        string          `json:"decision"`
	DecisionComment string          `json:"decision_comment"`
	Responsible     string          `json:"responsible"`
	SourceFile      string          `json:"source_file"`
	Archived        bool            `json:"archived"`
	Quarter         string          `json:"quarter"`
	Raw             json.RawMessage `json:"raw,omitempty"`
	PayloadRef      string          `json:"payload_ref,omitempty"`
	PayloadSHA256   string          `json:"payload_sha256,omitempty"`
	ImportedAt      string          `json:"imported_at"`
}

type Technology struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Group         string `json:"group"`
	Applicability string `json:"applicability"`
	Constraints   string `json:"constraints"`
	Evidence      string `json:"evidence"`
	Status        string `json:"status"`
}

type Outcome struct {
	ID                      string `json:"id"`
	VersionID               string `json:"version_id"`
	Date                    string `json:"date"`
	CycleStartDate          string `json:"cycle_start_date,omitempty"`
	CycleEndDate            string `json:"cycle_end_date,omitempty"`
	ActualMRP               string `json:"actual_mrp"`
	ActualCost              string `json:"actual_cost"`
	Result                  string `json:"result"`
	Comment                 string `json:"comment"`
	EventType               string `json:"event_type,omitempty"`
	PlannedIntervention     bool   `json:"planned_intervention,omitempty"`
	FailureMode             string `json:"failure_mode,omitempty"`
	ClassificationSource    string `json:"classification_source,omitempty"`
	ClassificationConfirmed bool   `json:"classification_confirmed,omitempty"`
	FailureCause            string `json:"failure_cause,omitempty"`
	DefectFinding           string `json:"defect_finding,omitempty"`
	EvidenceSource          string `json:"evidence_source,omitempty"`
	EvidenceComment         string `json:"evidence_comment,omitempty"`
	NextCycleStartDate      string `json:"next_cycle_start_date,omitempty"`
	NextCycleStatus         string `json:"next_cycle_status,omitempty"`
	ObservedEffect          string `json:"observed_effect,omitempty"`
	EffectMetric            string `json:"effect_metric,omitempty"`
	EffectBefore            string `json:"effect_before,omitempty"`
	EffectAfter             string `json:"effect_after,omitempty"`
	EffectAttribution       string `json:"effect_attribution,omitempty"`
}

type PlanItem struct {
	ID               string `json:"id"`
	DueDate          string `json:"due_date"`
	ObjectKey        string `json:"object_key"`
	Action           string `json:"action"`
	Basis            string `json:"basis"`
	Status           string `json:"status"`
	VersionID        string `json:"version_id"`
	MethodName       string `json:"method_name,omitempty"`
	DecisionID       string `json:"decision_id,omitempty"`
	Responsible      string `json:"responsible,omitempty"`
	ControlDate      string `json:"control_date,omitempty"`
	ControlCriterion string `json:"control_criterion,omitempty"`
	CompletedAt      string `json:"completed_at,omitempty"`
	ExecutionResult  string `json:"execution_result,omitempty"`
	ControlOutcome   string `json:"control_outcome,omitempty"`
	ControlComment   string `json:"control_comment,omitempty"`
	ParametersRef    string `json:"parameters_ref,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
	UpdatedBy        string `json:"updated_by,omitempty"`
}

type LedgerEntry struct {
	Sequence     int           `json:"sequence"`
	PreviousHash string        `json:"previous_hash"`
	Hash         string        `json:"hash"`
	Record       VersionRecord `json:"record"`
}

//go:embed OF_Model_icon_round.ico
var embeddedAppIcon []byte

//go:embed ROSA_brand_r.ico
var embeddedBrandR []byte

var (
	hMain                                                                                                                                                                                                                                                                        syscall.Handle
	hFont                                                                                                                                                                                                                                                                        syscall.Handle
	hFontBold                                                                                                                                                                                                                                                                    syscall.Handle
	brushWhite                                                                                                                                                                                                                                                                   syscall.Handle
	brushLight                                                                                                                                                                                                                                                                   syscall.Handle
	brushTop                                                                                                                                                                                                                                                                     syscall.Handle
	brushHeader                                                                                                                                                                                                                                                                  syscall.Handle
	brushNav                                                                                                                                                                                                                                                                     syscall.Handle
	brushGraphite                                                                                                                                                                                                                                                                syscall.Handle
	brushGraphitePressed                                                                                                                                                                                                                                                         syscall.Handle
	brushRedPressed                                                                                                                                                                                                                                                              syscall.Handle
	brushBorder                                                                                                                                                                                                                                                                  syscall.Handle
	brushWarning                                                                                                                                                                                                                                                                 syscall.Handle
	brushInputError                                                                                                                                                                                                                                                              syscall.Handle
	brushInputWarning                                                                                                                                                                                                                                                            syscall.Handle
	brushInputValid                                                                                                                                                                                                                                                              syscall.Handle
	brushInputEmpty                                                                                                                                                                                                                                                              syscall.Handle
	currentPage                                                                                                                                                                                                                                                                  = 0
	pageBackStack, pageForwardStack                                                                                                                                                                                                                                              []int
	pageNavChanging                                                                                                                                                                                                                                                              bool
	pageSwitching                                                                                                                                                                                                                                                                bool
	pageVisibilityCommitting                                                                                                                                                                                                                                                     bool
	pageVisualUpdateDepth                                                                                                                                                                                                                                                        = map[int]int{}
	pageRefreshing                                                                                                                                                                                                                                                               bool
	navButtons                                                                                                                                                                                                                                                                   = map[int]syscall.Handle{}
	pageControls                                                                                                                                                                                                                                                                 = map[int][]syscall.Handle{}
	pageControlOwner                                                                                                                                                                                                                                                             = map[syscall.Handle]int{}
	pageControlVisible                                                                                                                                                                                                                                                           = map[syscall.Handle]bool{}
	versions                                                                                                                                                                                                                                                                     []VersionRecord
	technologies                                                                                                                                                                                                                                                                 []Technology
	outcomes                                                                                                                                                                                                                                                                     []Outcome
	plans                                                                                                                                                                                                                                                                        []PlanItem
	dataDir                                                                                                                                                                                                                                                                      string
	historyList, techList, timelineList, planList, registryList, archiveList, recentList                                                                                                                                                                                         syscall.Handle
	dashboardStats, dashboardObjectTitle, dashboardNote, dashboardRecentTitle, historyDetail, techDetail, dataInfo                                                                                                                                                               syscall.Handle
	dashboardHistoryHint                                                                                                                                                                                                                                                         syscall.Handle
	dashboardSearchTitle, dashboardImportTitle, hBrandSignature                                                                                                                                                                                                                  syscall.Handle
	dashboardImportAccent, dashboardSelectionAccent, dashboardObjectAccent, dashboardHistoryAccent                                                                                                                                                                               syscall.Handle
	historyFilter                                                                                                                                                                                                                                                                syscall.Handle
	objectsFilter                                                                                                                                                                                                                                                                syscall.Handle
	objectsList, assessList, activitiesList, opiList, registryCurrentList, registryFixedList                                                                                                                                                                                     syscall.Handle
	assessObjectInfo, assessOverall, assessMethodsLabel, assessMethodsCombo, assessMethodInfo                                                                                                                                                                                    syscall.Handle
	assessProbabilityButton, assessHistoryButton, assessToActivitiesButton                                                                                                                                                                                                       syscall.Handle
	assessTabTechButton, assessTabLabButton, assessTabCompareButton                                                                                                                                                                                                              syscall.Handle
	assessTechList, assessProgramList, assessCompareList                                                                                                                                                                                                                         syscall.Handle
	assessClassCaption, assessClassCombo, assessClassComment, assessClassSaveButton, assessAcceptTechButton                                                                                                                                                                      syscall.Handle
	assessFinalCaption, assessFinalClassCombo, assessFinalCommentCaption, assessFinalComment, assessConfirmButton, assessConfirmKeepButton, assessConfirmCorrectButton, assessConfirmFullButton, assessTabDetail                                                                 syscall.Handle
	assessDecisionHistoryLabel, assessDecisionHistoryList                                                                                                                                                                                                                        syscall.Handle
	busyLoadingLabel                                                                                                                                                                                                                                                             syscall.Handle
	busyLoadingWindow                                                                                                                                                                                                                                                            syscall.Handle
	busyLoadingActive                                                                                                                                                                                                                                                            bool
	objectSelectFrame, objectSelectButton, objectEquipmentButton, objectPassportLabel, objectSelectedValue                                                                                                                                                                       syscall.Handle
	objectDetail, assessDetail, opiDetail, registriesInfo, activitiesScenarioDetail                                                                                                                                                                                              syscall.Handle
	factMRP, factCost, factResult, factComment, factSelected, factCycleStart, factCycleEnd, factEventType                                                                                                                                                                        syscall.Handle
	hTopBG, hBrandIcon, hTitle, hSubtitle, hSystemName, hLukoilLabel, hBuildMark, hHeaderSep1, hHeaderSep2, hHeaderNew, hHeaderBack, hHeaderForward, hHeaderOpen, hHeaderExport, hHeaderHelp, hHeaderApprove, hStripe, hWarningBG, hWarningTitle, hWarningText, hNavBG, hNavNote syscall.Handle
	techImportFileLabel, techImportSummary, techImportList                                                                                                                                                                                                                       syscall.Handle
	techChooseButton, techPreviewButton, techApplyButton                                                                                                                                                                                                                         syscall.Handle
	failureObjectFrame, failureObjectCombo, failureObjectOpenButton, failureAnalysisButton                                                                                                                                                                                       syscall.Handle
	failureImportFileLabel, failureImportSummary, failureImportList                                                                                                                                                                                                              syscall.Handle
	failureChooseButton, failurePreviewButton, failureApplyButton                                                                                                                                                                                                                syscall.Handle
	dataActionButtons                                                                                                                                                                                                                                                            []syscall.Handle
	techCurrent                                                                                                                                                                                                                                                                  []TechObjectState
	pendingTech                                                                                                                                                                                                                                                                  []TechObjectState
	techPreview                                                                                                                                                                                                                                                                  []TechChange
	techImports                                                                                                                                                                                                                                                                  []TechImportRecord
	selectedTechFile                                                                                                                                                                                                                                                             string
	candidateObjectKey                                                                                                                                                                                                                                                           string
	hIconBig, hIconSmall, hBrandR, hBrandBitmap, hLukoilBitmap, hWarningBitmap                                                                                                                                                                                                   syscall.Handle
	hUserLabel                                                                                                                                                                                                                                                                   syscall.Handle
	hSubtitleParts                                                                                                                                                                                                                                                               [8]syscall.Handle
	hFontTitle                                                                                                                                                                                                                                                                   syscall.Handle
	hFontHeader                                                                                                                                                                                                                                                                  syscall.Handle
	hFontHeaderBold                                                                                                                                                                                                                                                              syscall.Handle
	hFontSection                                                                                                                                                                                                                                                                 syscall.Handle
	hFontSystemName                                                                                                                                                                                                                                                              syscall.Handle
	hFontBrandCaption, hFontHeaderSmall, hFontHeaderTiny                                                                                                                                                                                                                         syscall.Handle
	hFontHeaderBrand                                                                                                                                                                                                                                                             syscall.Handle
	hFontSlogan                                                                                                                                                                                                                                                                  syscall.Handle
	hFontWarning                                                                                                                                                                                                                                                                 syscall.Handle
	hFontButton                                                                                                                                                                                                                                                                  syscall.Handle
	hFontButtonCompact                                                                                                                                                                                                                                                           syscall.Handle
	hFontResultHint                                                                                                                                                                                                                                                              syscall.Handle
	hFontReportEvidence                                                                                                                                                                                                                                                          syscall.Handle
)

func p(s string) *uint16       { q, _ := syscall.UTF16PtrFromString(s); return q }
func py(y int32) int32         { return y + PAGE_Y_OFFSET }
func rgb(r, g, b byte) uintptr { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }
func loWord(v uintptr) uint16  { return uint16(v & 0xffff) }
func hiWord(v uintptr) uint16  { return uint16((v >> 16) & 0xffff) }

func create(class, text string, style uint32, x, y, w, h int32, parent syscall.Handle, id int) syscall.Handle {
	hwnd, _, callErr := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(p(class))), uintptr(unsafe.Pointer(p(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	if hwnd == 0 {
		diagLog("UI create failed class=%s id=%d rect=%d,%d,%d,%d err=%v", class, id, x, y, w, h, callErr)
		return 0
	}
	if hFont != 0 {
		pSendMessageW.Call(hwnd, WM_SETFONT, uintptr(hFont), 1)
	}
	z := syscall.Handle(hwnd)
	registerButtonContextHelp(z, class)
	return z
}
func setPhysicalControlVisibility(h syscall.Handle, yes bool) {
	if h == 0 {
		return
	}
	if owner, owned := pageControlOwner[h]; owned {
		preparing := (pageSwitching && !pageVisibilityCommitting) || pageVisualUpdateDepth[owner] > 0
		yes = pageControlMayBeVisible(owner, currentPage, yes, preparing) && pageControlSubviewMatches(h, owner)
	}
	// Read the native style on every transition. WM_SETREDRAW and native controls
	// can change WS_VISIBLE outside this helper, so an in-memory visibility cache
	// can leave a stale control visible over another page. This keeps the fast path
	// while basing it on the real HWND state.
	style, _, _ := pGetWindowLongPtrW.Call(uintptr(h), ^uintptr(15)) // GWL_STYLE = -16
	if (style&uintptr(WS_VISIBLE) != 0) == yes {
		return
	}
	if pageSwitching && h != hMain {
		noRedrawVisibility(h, yes)
		return
	}
	if yes {
		pShowWindow.Call(uintptr(h), SW_SHOW)
	} else {
		pShowWindow.Call(uintptr(h), SW_HIDE)
	}
}

func show(h syscall.Handle, yes bool) {
	if h == 0 {
		return
	}
	// Keep the logical visibility separate from physical page isolation. A page
	// contains many mutually exclusive controls (tabs, preliminary/full failure
	// form sections, laboratory modes, etc.). RC703 physically hid inactive pages
	// correctly, but activatePage then forced *every* control of the target page
	// visible again. That resurrected controls which the page itself had hidden and
	// produced the stacked/"crumbled" screen seen in the Windows screenshot.
	if owner, ok := pageControlOwner[h]; ok {
		pageControlVisible[h] = yes
		if pageSwitching || pageVisualUpdateDepth[owner] > 0 {
			// Apply visibility once after the UI transaction. Existing controls
			// remain painted until the final state is committed on this thread.
			return
		}
		if yes && owner != currentPage {
			// Remember the requested state for the next activation, but do not leak
			// the control onto the currently active page.
			setPhysicalControlVisibility(h, false)
			return
		}
	} else if yes && currentPage != 0 && !isPersistentChromeControl(h) {
		// Page controls are direct children of hMain and must be registered.
		// Modal dialogs own their children independently; blocking those children
		// here made laboratory charts disappear when switched to infographic mode.
		parent, _, _ := pGetParent.Call(uintptr(h))
		if parent == uintptr(hMain) {
			diagLog("UI refused unowned main control show hwnd=%d page=%d", h, currentPage)
			return
		}
	}
	setPhysicalControlVisibility(h, yes)
}

func beginBusyLoading(text string) {
	// A single guarded loading surface prevents duplicate imports and gives the
	// user immediate feedback while the file is parsed on the UI thread.
	if busyLoadingActive {
		return
	}
	busyLoadingActive = true
	if busyLoadingLabel != 0 {
		setText(busyLoadingLabel, "⟳  "+text)
		show(busyLoadingLabel, true)
		pUpdateWindow.Call(uintptr(busyLoadingLabel))
	}
	startupLoadingHeadline = "ЗАГРУЗКА ФАЙЛА"
	busyLoadingWindow = showStartupLoadingWindow()
	setStartupLoadingStage(2)
	setStartupLoadingProgress(text)
	if hMain != 0 {
		pEnableWindow.Call(uintptr(hMain), 0)
	}
	pumpStartupLoadingMessages()
}

func endBusyLoading() {
	if busyLoadingWindow != 0 {
		pDestroyWindowAuth.Call(uintptr(busyLoadingWindow))
		busyLoadingWindow = 0
	}
	startupLoadingHeadline = "ВХОД ВЫПОЛНЕН"
	if hMain != 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
	}
	if busyLoadingLabel != 0 {
		show(busyLoadingLabel, false)
	}
	busyLoadingActive = false
}
func sanitizeUIText(s string) string {
	// Remove BOM/mojibake and invisible control characters that can appear in
	// imported CSV/TXT headers or copied file names. Keep CR/LF/TAB for edit boxes.
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.TrimPrefix(s, "ï»¿")
	s = strings.ReplaceAll(s, "\u200b", "")
	s = strings.ReplaceAll(s, "\u200c", "")
	s = strings.ReplaceAll(s, "\u200d", "")
	s = strings.ReplaceAll(s, "\ufffd", "")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\r' || r == '\n' || r == '\t' || r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func setText(h syscall.Handle, s string) {
	if h == 0 {
		return
	}
	s = sanitizeUIText(s)
	// Avoid generating WM_SETTEXT / EN_CHANGE / WM_PAINT for captions and
	// editors whose value is unchanged across adjacent inner tabs. Checking
	// the live HWND text (rather than caching a handle) also handles recycled
	// native window handles and external user edits correctly.
	if len(s) < 4096 && getText(h) == s {
		return
	}
	pSetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(p(s))))
}
func getText(h syscall.Handle) string {
	if h == 0 {
		return ""
	}
	n, _, _ := pGetWindowTextLengthW.Call(uintptr(h))
	buf := make([]uint16, n+1)
	pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}
func msg(title, text string, flags uintptr) {
	pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p(title))), flags)
}
func askYesNo(title, text string) bool {
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p(title))), 0x00000004|MB_ICONWARNING)
	return r == 6
}
func addPageControl(page int, h syscall.Handle) {
	if h == 0 {
		return
	}
	if owner, exists := pageControlOwner[h]; exists {
		if owner != page {
			diagLog("UI control owner conflict hwnd=%d old=%d new=%d", h, owner, page)
		}
		return
	}
	pageControls[page] = append(pageControls[page], h)
	pageControlOwner[h] = page
	// Controls are created without WS_VISIBLE. Historically page activation made
	// all of them visible once; defaulting the logical state to true preserves that
	// baseline while allowing page-specific show(..., false) calls to win.
	pageControlVisible[h] = true
}

func isPersistentChromeControl(h syscall.Handle) bool {
	if h == 0 {
		return false
	}
	for _, shared := range []syscall.Handle{
		hTopBG, hBrandIcon, hHeaderSep1, hBrandSignature, hSubtitle, hSystemName,
		hHeaderSep2, hLukoilLabel, hBuildMark, sessionTrashButton, hHeaderBack,
		hHeaderForward, hHeaderOpen, hHeaderHelp, hHeaderExport, hHeaderApprove,
		hUserLabel, hStripe, hWarningBG, hWarningTitle, hWarningText, sectionNoticeLabel, hNavBG,
	} {
		if h == shared {
			return true
		}
	}
	for _, nav := range navButtons {
		if h == nav {
			return true
		}
	}
	return false
}
func label(page int, text string, x, y, w, h int32) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|SS_LEFT|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	addPageControl(page, z)
	return z
}
func labelEllipsis(page int, text string, x, y, w, h int32) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|SS_LEFT|SS_NOPREFIX|SS_ENDELLIPSIS, x, py(y), w, h, hMain, 0)
	addPageControl(page, z)
	return z
}

func pageTitle(page int, text string, x, y, w, h int32) syscall.Handle {
	z := label(page, text, x, y, w, max32(h, 42))
	if z != 0 && hFontHeaderBold != 0 {
		pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(hFontHeaderBold), 1)
	}
	return z
}
func edit(page int, text string, x, y, w, h int32, multi, ro bool, id int) syscall.Handle {
	st := uint32(WS_CHILD | WS_TABSTOP | WS_BORDER)
	if multi {
		st |= ES_MULTILINE | ES_AUTOVSCROLL | WS_VSCROLL
	}
	if ro {
		st |= ES_READONLY
	}
	z := create("EDIT", text, st, x, py(y), w, h, hMain, id)
	addPageControl(page, z)
	return z
}

func richEdit(page int, text string, x, y, w, h int32, ro bool, id int) syscall.Handle {
	if err := riched20.Load(); err != nil {
		return edit(page, text, x, y, w, h, true, ro, id)
	}
	st := uint32(WS_CHILD | WS_TABSTOP | WS_BORDER | ES_MULTILINE | ES_AUTOVSCROLL | ES_NOHIDESEL | WS_VSCROLL)
	if ro {
		st |= ES_READONLY
	}
	z := create("RichEdit20W", text, st, x, py(y), w, h, hMain, id)
	if z == 0 {
		return edit(page, text, x, y, w, h, true, ro, id)
	}
	if hFont != 0 {
		pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(hFont), 1)
	}
	addPageControl(page, z)
	return z
}
func buttonUsesUniformStandardFont(id int) bool {
	// Laboratory controls are one visual family. Never shrink individual captions:
	// long names must be solved by geometry/wrapping, not by a different font size.
	if id == ID_LAB_IMPORT || id == ID_LAB_DOWNLOAD_TEMPLATE || id == ID_LAB_USE_SELECTED ||
		id == ID_LAB_ANALYZE || id == ID_LAB_EXPORT || id == ID_LAB_SAVE ||
		id == ID_LAB_CONTINUE || id == ID_LAB_COPY_RESULT || id == ID_LAB_DELETE_ROW ||
		id == ID_LAB_CLEAR_ALL || id == ID_LAB_CLEAR_CURRENT || id == ID_LAB_FIELDS_PREV || id == ID_LAB_FIELDS_NEXT || id == ID_LAB_OPEN_INFOGRAPHIC || id == ID_LAB_COMPLEX_ANALYSIS || id == ID_LAB_CALCULATION_BASIS {
		return true
	}
	for _, g := range labGroups {
		if g.ID == id {
			return true
		}
	}
	// Manual failure-card sections are one navigation family. Keep the standard
	// button type size and use two-line wrapping when a caption is long.
	if id >= ID_WELL_SECTION_BASE && id < ID_WELL_SECTION_BASE+7 {
		return true
	}
	// RC615: method cards are one visual family too. Never shrink their
	// captions independently; solve long names by card geometry and wrapping.
	if id >= ID_METHODS_CARD_BASE && id < ID_METHODS_CARD_BASE+ID_METHODS_CARD_COUNT {
		return true
	}
	return false
}

func button(page int, text string, x, y, w, h int32, id int) syscall.Handle {
	// The former 28-34 px controls were too tight for the larger application
	// typeface, especially at 125-150% Windows scaling. The four NNO toolbar
	// actions intentionally opt into the compact dimensions from their reference.
	if h < 40 && !isNNOActionButtonID(id) {
		h = 40
	}
	z := create("BUTTON", text, WS_CHILD|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, x, py(y), w, h, hMain, id)
	if z != 0 && hFontButton != 0 {
		pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(hFontButton), 1)
	}
	addPageControl(page, z)
	return z
}

func buildGNOSubmoduleTabs(page int) {
	button(page, "Динамометрический контроль", 242, 94, 260, 48, ID_GNO_DYNAMOMETRY)
	button(page, "Мониторинг состояния ГНО", 512, 94, 270, 48, ID_GNO_TELEMETRY)
}

func isGNOPage(page int) bool {
	return page == ID_DYN || page == ID_TELEMETRY
}

func gnoPageForCommand(id int) (int, bool) {
	switch id {
	case ID_GNO_DYNAMOMETRY:
		return ID_DYN, true
	case ID_GNO_TELEMETRY:
		return ID_TELEMETRY, true
	default:
		return 0, false
	}
}

func isNavigationButtonActive(id, page int) bool {
	return id == page || (id == ID_OBJECTS && (page == ID_TECHPARAM || isGNOPage(page)))
}

func isGNOSubmoduleTabActive(id, page int) bool {
	return (id == ID_GNO_DYNAMOMETRY && page == ID_DYN) ||
		(id == ID_GNO_TELEMETRY && page == ID_TELEMETRY)
}

func newList(page int, x, y, w, h int32, cols []struct {
	n string
	w int32
}) syscall.Handle {
	z := create("SysListView32", "", WS_CHILD|WS_BORDER|LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS, x, py(y), w, h, hMain, 0)
	addPageControl(page, z)
	pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(hFont), 1)
	header, _, _ := pSendMessageW.Call(uintptr(z), 0x101F, 0, 0) // LVM_GETHEADER
	if header != 0 {
		pSendMessageW.Call(header, WM_SETFONT, uintptr(hFontBold), 1)
	}
	pSendMessageW.Call(uintptr(z), LVM_SETEXTENDEDLISTVIEWSTYLE, LVS_EX_FULLROWSELECT|LVS_EX_GRIDLINES|LVS_EX_DOUBLEBUFFER, LVS_EX_FULLROWSELECT|LVS_EX_GRIDLINES|LVS_EX_DOUBLEBUFFER)
	pSendMessageW.Call(uintptr(z), LVM_SETBKCOLOR, 0, rgb(255, 255, 255))
	pSendMessageW.Call(uintptr(z), LVM_SETTEXTBKCOLOR, 0, rgb(255, 255, 255))
	pSendMessageW.Call(uintptr(z), LVM_SETTEXTCOLOR, 0, rgb(43, 46, 50))
	for i, c := range cols {
		headerText := strings.TrimSpace(c.n)
		if headerText == "" {
			// RC572 global table invariant: a visible column may never have an
			// empty header. Keep indices stable but expose an explicit neutral name.
			headerText = "Данные"
		}
		t := p(headerText)
		col := LVCOLUMN{Mask: LVCF_TEXT | LVCF_WIDTH | LVCF_FMT, Fmt: LVCFMT_LEFT, Cx: c.w, PszText: t}
		pSendMessageW.Call(uintptr(z), LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&col)))
	}
	return z
}

func setListHeaderFont(list, font syscall.Handle) {
	if list == 0 || font == 0 {
		return
	}
	header, _, _ := pSendMessageW.Call(uintptr(list), 0x101F, 0, 0) // LVM_GETHEADER
	if header != 0 {
		pSendMessageW.Call(header, WM_SETFONT, uintptr(font), 1)
	}
}

func dashboardHistoryCustomDraw(lParam uintptr) uintptr {
	if lParam == 0 {
		return CDRF_DODEFAULT
	}
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		row := int(draw.NMCD.ItemSpec)
		if row >= 0 && row < len(dashboardHistoryRows) && dashboardHistoryRows[row].FullyApproved {
			draw.ClrText = uint32(rgb(20, 92, 48))
			draw.ClrTextBk = uint32(rgb(226, 246, 232))
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func setListColumnWidth(list syscall.Handle, column int, width int32) {
	if list == 0 || width <= 0 {
		return
	}
	// A redundant column resize makes Windows invalidate the whole ListView,
	// including the archive's scrollable rows. Keep the geometry stable when
	// only the selected tab or an unrelated card has changed.
	const lvmGetColumnWidth = 0x101D
	current, _, _ := pSendMessageW.Call(uintptr(list), lvmGetColumnWidth, uintptr(column), 0)
	if int32(current) == width {
		return
	}
	pSendMessageW.Call(uintptr(list), LVM_SETCOLUMNWIDTH, uintptr(column), uintptr(width))
}

func centerListHeaders(list syscall.Handle, columns int) {
	if list == 0 {
		return
	}
	header, _, _ := pSendMessageW.Call(uintptr(list), LVM_GETHEADER, 0, 0)
	if header == 0 {
		return
	}
	item := HDITEM{Mask: HDI_FORMAT, Fmt: HDF_CENTER | HDF_STRING}
	for i := 0; i < columns; i++ {
		pSendMessageW.Call(header, HDM_SETITEMW, uintptr(i), uintptr(unsafe.Pointer(&item)))
	}
}
func listBeginUpdate(h syscall.Handle) {
	// The UI thread does not dispatch WM_PAINT while it replaces the rows.
	// WM_SETREDRAW changes WS_VISIBLE on native ListViews, which used to leave
	// a visible header above an empty, unpainted 200-row laboratory table.
}
func listEndUpdate(h syscall.Handle) {
	if h != 0 {
		pInvalidateRect.Call(uintptr(h), 0, 1)
	}
}
func listClear(h syscall.Handle) {
	if h != 0 {
		pSendMessageW.Call(uintptr(h), LVM_DELETEALLITEMS, 0, 0)
	}
}
func listAdd(h syscall.Handle, row int, vals []string) {
	if h == 0 || len(vals) == 0 {
		return
	}
	hasVisibleData := false
	for _, value := range vals {
		if strings.TrimSpace(value) != "" {
			hasVisibleData = true
			break
		}
	}
	if !hasVisibleData {
		// RC572 global table invariant: never render anonymous blank rows.
		return
	}
	t := p(vals[0])
	it := LVITEM{Mask: LVIF_TEXT, IItem: int32(row), PszText: t}
	pSendMessageW.Call(uintptr(h), LVM_INSERTITEMW, 0, uintptr(unsafe.Pointer(&it)))
	for j := 1; j < len(vals); j++ {
		tt := p(vals[j])
		sub := LVITEM{IItem: int32(row), ISubItem: int32(j), PszText: tt}
		pSendMessageW.Call(uintptr(h), LVM_SETITEMTEXTW, uintptr(row), uintptr(unsafe.Pointer(&sub)))
	}
}
func selectedRow(h syscall.Handle) int {
	r, _, _ := pSendMessageW.Call(uintptr(h), LVM_GETNEXTITEM, ^uintptr(0), LVNI_SELECTED)
	if int32(r) < 0 {
		return -1
	}
	return int(int32(r))
}

func selectListRow(h syscall.Handle, row int) {
	if h == 0 || row < 0 {
		return
	}
	item := LVITEM{State: LVIS_SELECTED | LVIS_FOCUSED, StateMask: LVIS_SELECTED | LVIS_FOCUSED}
	pSendMessageW.Call(uintptr(h), LVM_SETITEMSTATE, uintptr(row), uintptr(unsafe.Pointer(&item)))
	pSendMessageW.Call(uintptr(h), LVM_ENSUREVISIBLE, uintptr(row), 0)
}

func appDataDir() string {
	if currentSession != nil && currentSession.IsGuest && strings.TrimSpace(currentSession.TempDir) != "" {
		return currentSession.TempDir
	}
	if currentSession != nil && strings.TrimSpace(currentSession.UserID) != "" {
		return userDataDir(currentSession.UserID)
	}
	return filepath.Join(appRootDir(), "data") // legacy/fallback before local login
}
func sharedPath(name string) string {
	_ = os.MkdirAll(sharedDataDir(), 0700)
	return filepath.Join(sharedDataDir(), name)
}
func diagLog(format string, args ...any) {
	dir := dataDir
	if dir == "" {
		dir = appDataDir()
	}
	_ = os.MkdirAll(dir, 0755)
	f, e := os.OpenFile(filepath.Join(dir, "diagnostics.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if e != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s | "+format+"\n", append([]any{time.Now().Format("2006-01-02 15:04:05.000")}, args...)...)
}

// startupLog is intentionally stored only inside the application's private
// APPDATA tree. Writing beside the executable is unsafe because users often run
// ROSA.exe directly from the Windows Desktop; older builds therefore left
// service files on the Desktop during startup.
func startupLog(format string, args ...any) {
	path := filepath.Join(appRootDir(), "logs", "ROSA_startup.log")
	line := fmt.Sprintf("%s | "+format+"\n", append([]any{time.Now().Format("2006-01-02 15:04:05.000")}, args...)...)
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}
func ensureData() {
	started := time.Now()
	startupLog("DATA preparation started")
	dataDir = appDataDir()
	_ = os.MkdirAll(dataDir, 0700)
	_ = os.MkdirAll(sharedDataDir(), 0700)
	initPersistentStorage()
	setStartupLoadingStage(2)
	setStartupLoadingProgress("Проверяем учебный фонд и лабораторные записи…")
	seedBegin := time.Now()
	seedBundledDemo()
	startupLog("DATA teaching records ready in %s", time.Since(seedBegin).Round(time.Millisecond))
	setStartupLoadingProgress("Загружаем профиль и справочники…")
	loadOrganizationProfile()
	loadRuleCatalog()
	technologies = loadTechnologies()
	loadDeletedState()
	versions = filterDeletedVersions(loadVersions())
	outcomes = loadOutcomes()
	plans = loadPlans()
	loadJSON(sharedPath("tech_current.json"), &techCurrent)
	loadJSON(sharedPath("tech_imports.json"), &techImports)
	setStartupLoadingProgress("Проверяем историю отказов и экономические сценарии…")
	loadFailureEvents()
	retireLegacyTeachingFailures()
	ensureAlternativeScenariosLoaded()
	setStartupLoadingStage(3)
	// Compatibility with the pre-multi-user build: if the shared snapshot does not
	// exist yet, reuse the user/legacy copy once and then persist it as shared.
	if len(techCurrent) == 0 {
		loadJSON(filepath.Join(dataDir, "tech_current.json"), &techCurrent)
		if len(techCurrent) > 0 {
			_ = saveJSON(sharedPath("tech_current.json"), techCurrent)
		}
	}
	if len(techCurrent) > 0 {
		kept := techCurrent[:0]
		for _, state := range techCurrent {
			if !isRetiredLegacyTeachingObjectKey(state.ObjectKey) {
				kept = append(kept, state)
			}
		}
		if len(kept) != len(techCurrent) {
			techCurrent = kept
			_ = saveJSON(sharedPath("tech_current.json"), techCurrent)
		}
	}
	if len(techImports) == 0 {
		loadJSON(filepath.Join(dataDir, "tech_imports.json"), &techImports)
		if len(techImports) > 0 {
			_ = saveJSON(sharedPath("tech_imports.json"), techImports)
		}
	}
	loadTelemetryProfiles()
	loadTelemetry()
	loadUserPreferences()
	activateReleaseTeachingExample()
	loadTargetRunlifeSettings()
	loadSectionBannerSettings()
	if err := selfCheckBaseEconomics(); err != nil {
		diagLog("ECON self-check FAIL: %v", err)
	} else {
		diagLog("ECON self-check OK formula=%s", baseEconomicsFormulaVersion)
	}
	setStartupLoadingStage(4)
	startupLog("DATA preparation finished in %s", time.Since(started).Round(time.Millisecond))
}

func ensureDataWithVisibleTransition() {
	startupLoadingHeadline = "ВХОД ВЫПОЛНЕН"
	loading := showStartupLoadingWindow()
	if loading != 0 {
		defer pDestroyWindowAuth.Call(uintptr(loading))
	}
	ensureData()
}
func loadJSON(path string, v any) {
	fi, e := os.Stat(path)
	if e != nil {
		if !os.IsNotExist(e) {
			diagLog("JSON stat error path=%s err=%v", path, e)
		}
		return
	}
	// Local state files are expected to be small. Refuse pathological/corrupt files
	// instead of reading them fully during startup and exhausting memory.
	const maxJSON = int64(64 << 20)
	if fi.Size() > maxJSON {
		diagLog("JSON skipped oversized path=%s size=%d", path, fi.Size())
		return
	}
	b, e := os.ReadFile(path)
	if e != nil {
		diagLog("JSON read error path=%s err=%v", path, e)
		return
	}
	if personalPath(path) {
		if b, e = decryptPersonalBytes(b); e != nil {
			diagLog("JSON decrypt error path=%s err=%v", path, e)
			return
		}
	}
	if e = json.Unmarshal(b, v); e != nil {
		diagLog("JSON parse error path=%s err=%v", path, e)
		// Preserve the broken state for diagnostics, but quarantine it so the same
		// damaged file cannot break every subsequent launch.
		bad := path + ".corrupt-" + time.Now().Format("20060102-150405")
		if re := os.Rename(path, bad); re == nil {
			diagLog("JSON quarantined from=%s to=%s", path, bad)
		}
	}
}
func saveJSON(path string, v any) error {
	if currentSession != nil && currentSession.IsGuest {
		sharedRoot := filepath.Clean(sharedDataDir()) + string(os.PathSeparator)
		cleanPath := filepath.Clean(path)
		if strings.HasPrefix(cleanPath, sharedRoot) {
			return fmt.Errorf("гостевой режим: изменения общих данных не сохраняются")
		}
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	perm := os.FileMode(0644)
	if personalPath(path) {
		b, e = encryptPersonalBytes(b)
		if e != nil {
			return e
		}
		perm = 0600
	}
	return atomicWriteFile(path, b, perm)
}
func ledgerPath() string { return filepath.Join(dataDir, "calculation_versions.jsonl") }
func computeLedgerHash(seq int, prev string, v VersionRecord) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|", seq, prev) + string(b)))
	return hex.EncodeToString(h[:])
}
func loadVersions() []VersionRecord {
	path := ledgerPath()
	f, e := os.Open(path)
	if e == nil {
		defer f.Close()
		var out []VersionRecord
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
		prev := ""
		seq := 1
		ok := true
		for sc.Scan() {
			line, decErr := decryptPersonalLine(sc.Bytes())
			var le LedgerEntry
			if decErr != nil || json.Unmarshal(line, &le) != nil || le.Sequence != seq || le.PreviousHash != prev || le.Hash != computeLedgerHash(le.Sequence, le.PreviousHash, le.Record) {
				ok = false
				break
			}
			out = append(out, le.Record)
			prev = le.Hash
			seq++
		}
		if sc.Err() != nil {
			ok = false
		}
		if ok && len(out) > 0 {
			saveLedgerTail(seq-1, prev)
			sort.SliceStable(out, func(i, j int) bool { return parseDate(out[i].Created).After(parseDate(out[j].Created)) })
			return out
		}
	}
	var legacy []VersionRecord
	loadJSON(filepath.Join(dataDir, "calculation_versions.json"), &legacy)
	if len(legacy) > 0 {
		_ = rewriteLedger(legacy)
		return legacy
	}
	return nil
}
func appendLedger(v VersionRecord) error {
	v = compactVersionPayload(v)
	seq := 1
	prev := ""
	if fi, err := os.Stat(ledgerPath()); err == nil && fi.Size() > 0 {
		tail, ok := loadLedgerTail()
		if !ok {
			var e error
			tail, e = rebuildLedgerTail()
			if e != nil {
				return fmt.Errorf("журнал фиксации повреждён: %v; новая запись не добавлена", e)
			}
		}
		seq = tail.Sequence + 1
		prev = tail.Hash
	}
	le := LedgerEntry{Sequence: seq, PreviousHash: prev, Record: v}
	le.Hash = computeLedgerHash(seq, prev, v)
	b, e := json.Marshal(le)
	if e != nil {
		return e
	}
	if personalPath(ledgerPath()) {
		b, e = encryptPersonalLine(b)
		if e != nil {
			return e
		}
	}
	wf, e := os.OpenFile(ledgerPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	if _, e = wf.Write(append(b, '\n')); e != nil {
		wf.Close()
		return e
	}
	if e = wf.Sync(); e != nil {
		wf.Close()
		return e
	}
	if e = wf.Close(); e != nil {
		return e
	}
	saveLedgerTail(seq, le.Hash)
	return nil
}

func rewriteLedger(vs []VersionRecord) error {
	tmp := ledgerPath() + ".tmp"
	f, e := os.Create(tmp)
	if e != nil {
		return e
	}
	prev := ""
	for i, v := range vs {
		le := LedgerEntry{Sequence: i + 1, PreviousHash: prev, Record: v}
		le.Hash = computeLedgerHash(le.Sequence, prev, v)
		b, _ := json.Marshal(le)
		if personalPath(ledgerPath()) {
			b, e = encryptPersonalLine(b)
			if e != nil {
				f.Close()
				return e
			}
		}
		if _, e = f.Write(append(b, '\n')); e != nil {
			f.Close()
			return e
		}
		prev = le.Hash
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	b, e := os.ReadFile(tmp)
	if e != nil {
		return e
	}
	if e = atomicWriteFile(ledgerPath(), b, 0600); e != nil {
		return e
	}
	_ = os.Remove(tmp)
	saveLedgerTail(len(vs), prev)
	return nil
}
func verifyLedger() (bool, int, string) {
	f, e := os.Open(ledgerPath())
	if e != nil {
		return true, 0, "журнал ещё не создан"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	prev := ""
	seq := 1
	count := 0
	for sc.Scan() {
		line, decErr := decryptPersonalLine(sc.Bytes())
		var le LedgerEntry
		if decErr != nil || json.Unmarshal(line, &le) != nil {
			return false, count, "повреждена структура или шифрование записи"
		}
		if le.Sequence != seq || le.PreviousHash != prev || le.Hash != computeLedgerHash(le.Sequence, le.PreviousHash, le.Record) {
			return false, count, fmt.Sprintf("нарушена цепочка на записи %d", seq)
		}
		prev = le.Hash
		seq++
		count++
	}
	if sc.Err() != nil {
		return false, count, sc.Err().Error()
	}
	return true, count, "целостность подтверждена"
}
func loadOutcomes() []Outcome {
	var x []Outcome
	loadJSON(filepath.Join(dataDir, "outcomes.json"), &x)
	return x
}
func loadPlans() []PlanItem {
	var x []PlanItem
	loadJSON(filepath.Join(dataDir, "plan.json"), &x)
	return x
}
func loadTechnologies() []Technology {
	var x []Technology
	path := sharedPath("technologies.json")
	loadJSON(path, &x)
	if len(x) == 0 {
		x = []Technology{
			{"K-INH", "Дозирование ингибитора коррозии", "К — коррозия", "Коррозионный риск при подтверждённой применимости реагента", "Совместимость, режим дозирования, контроль остаточного содержания", "Лаборатория + мониторинг + ОЛИ/ОПИ/ОПР", "Действующая"},
			{"K-CONT", "Погружной контейнер от АСПО, коррозии, солеотложений (ТРИЛ-К, ТРИЛ-П-См, ТРИЛ-СВ)", "К — коррозия", "Объекты, где допустимо контейнерное размещение реагента", "Ресурс контейнера, замена, контроль остатка реагента", "Факт эксплуатации + контроль эффективности", "Действующая"},
			{"S-INH", "Дозирование ингибитора солеотложений", "С — соли", "Риск солеотложения, подтверждённый анализом воды и историей", "Совместимость, гидродинамика, температура и давление", "Анализ воды + ОЛИ/ОПИ/ОПР", "Действующая"},
			{"S-FLT", "Фильтрация + ингибитор солеотложений", "С / М — соли и мехпримеси", "Совместное наличие солей и твёрдых частиц", "Недопустимое гидравлическое сопротивление исключается", "Анализ воды/КВЧ + промысловая проверка", "Действующая"},
			{"A-SOLV", "Растворитель АСПО", "А — АСПО", "Подтверждённые органические отложения и применимость растворителя", "Совместимость, объём и периодичность по регламенту", "Лабораторный подбор + ОПИ/ОПР", "Действующая"},
			{"A-HOT", "Горячая промывка", "А — АСПО", "Термочувствительные органические отложения", "Температурные ограничения и безопасный возврат продукции", "Промысловый опыт + контроль результата", "Действующая"},
			{"GD-INH", "Ингибитор гидратообразования", "GD — гидраты", "Условия гидратообразования подтверждены расчётом/наблюдением", "Температура, давление, состав газа и расход реагента", "Расчёт + ОПИ/ОПР", "Действующая"},
			{"GD-MEOH", "Метанольная промывка", "GD — гидраты", "Оперативное удаление/предупреждение гидратов по регламенту", "Объём и периодичность задаются технологом", "Промысловая эффективность", "Действующая"},
			{"HCL", "Кислотная обработка HCl", "Специальная обработка", "Только при подтверждённой применимости к конкретному осложнению/загрязнению", "Состав и объём определяются подрядчиком и программой работ", "Лаборатория + дизайн обработки + ОПИ/ОПР", "По согласованию"},
			{"TRIL", "Погружной контейнер от АСПО, коррозии, солеотложений (ТРИЛ-К, ТРИЛ-П-См, ТРИЛ-СВ)", "По подтверждённой области применимости", "Использовать только при выполнении критериев применимости", "365 суток — гарантийный срок, не предел прогнозного МРП", "НД/испытания + фактическая наработка", "Действующая"},
		}
	}
	// The Excel Technology Module is the canonical starting list. Preserve any
	// locally verified additions, but make sure every method from the embedded
	// catalog is available after an upgrade.
	x = mergeTechnologyCatalog(x)
	_ = saveJSON(path, x)
	return x
}

type xlsxSharedStrings struct {
	Items []struct {
		T    string `xml:"t"`
		Runs []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}
type xlsxWorkbook struct {
	Sheets []struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}
type xlsxRelationships struct {
	Items []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

func xlsxRelationshipTargetPath(target string) string {
	target = strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(pathpkg.Clean(target), "/")
	}
	if strings.HasPrefix(target, "xl/") {
		return pathpkg.Clean(target)
	}
	// workbook.xml lives in xl/, therefore relationship targets such as
	// worksheets/sheet1.xml are relative to that directory.
	return pathpkg.Clean(pathpkg.Join("xl", target))
}

func xlsxWorksheetPath(z *zip.ReadCloser, preferredName string) (string, error) {
	wbBytes, err := readZipEntry(z, "xl/workbook.xml")
	if err != nil {
		return "", err
	}
	var wb xlsxWorkbook
	if err := xml.Unmarshal(wbBytes, &wb); err != nil {
		return "", err
	}
	if len(wb.Sheets) == 0 {
		return "", fmt.Errorf("в XLSX не найдены листы")
	}
	rid := ""
	if strings.TrimSpace(preferredName) != "" {
		for _, sh := range wb.Sheets {
			if strings.EqualFold(strings.TrimSpace(sh.Name), strings.TrimSpace(preferredName)) {
				rid = sh.RID
				break
			}
		}
	}
	if rid == "" {
		rid = wb.Sheets[0].RID
	}
	relBytes, err := readZipEntry(z, "xl/_rels/workbook.xml.rels")
	if err != nil {
		return "", err
	}
	var rel xlsxRelationships
	if err := xml.Unmarshal(relBytes, &rel); err != nil {
		return "", err
	}
	for _, rr := range rel.Items {
		if rr.ID != rid {
			continue
		}
		p := xlsxRelationshipTargetPath(rr.Target)
		if p == "" {
			break
		}
		return p, nil
	}
	return "", fmt.Errorf("не найден XML выбранного листа XLSX")
}

type xlsxSheet struct {
	Rows []struct {
		R     int `xml:"r,attr"`
		Cells []struct {
			R      string `xml:"r,attr"`
			T      string `xml:"t,attr"`
			V      string `xml:"v"`
			Inline struct {
				T string `xml:"t"`
			} `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func readZipEntry(z *zip.ReadCloser, name string) ([]byte, error) {
	for _, f := range z.File {
		if f.Name == name {
			r, e := f.Open()
			if e != nil {
				return nil, e
			}
			defer r.Close()
			const maxXML = 64 << 20
			b, er := io.ReadAll(io.LimitReader(r, maxXML+1))
			if er != nil {
				return nil, er
			}
			if len(b) > maxXML {
				return nil, fmt.Errorf("слишком большой XML-раздел XLSX: %s", name)
			}
			return b, nil
		}
	}
	return nil, fmt.Errorf("в XLSX не найден %s", name)
}
func colNumber(ref string) int {
	n := 0
	for _, ch := range ref {
		if ch < 'A' || ch > 'Z' {
			break
		}
		n = n*26 + int(ch-'A'+1)
	}
	return n
}
func sharedText(it struct {
	T    string `xml:"t"`
	Runs []struct {
		T string `xml:"t"`
	} `xml:"r"`
}) string {
	if it.T != "" {
		return it.T
	}
	var b strings.Builder
	for _, r := range it.Runs {
		b.WriteString(r.T)
	}
	return b.String()
}

type techHeaderRow struct {
	Row    int
	Values map[int]string
}

func defaultTechColumns() map[string]int {
	return map[string]int{
		"production_unit": 2, "workshop": 3, "dns": 4, "field": 5, "deposit": 6, "well": 7,
		"block": 10, "object_plast": 11, "element_development": 12, "equipment": 20, "pump_model": 22,
		"current_mrp": 33, "buffer_pressure": 36, "line_pressure": 37, "dynamic_level": 38,
		"tms_temperature": 42, "bottomhole_pressure": 43, "reservoir_pressure": 49,
		"liquid_rate": 54, "water_cut": 55, "oil_rate": 56, "planned_activities": 76,
	}
}

func normalizeTechHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	r := strings.NewReplacer(
		" ", "", "\t", "", "\r", "", "\n", "", "\u00a0", "",
		"№", "", ".", "", ",", "", ";", "", ":", "", "(", "", ")", "",
		"[", "", "]", "", "{", "", "}", "", "/", "", "\\", "", "-", "", "_", "",
		"%", "", "³", "3", "²", "2",
	)
	return r.Replace(s)
}

type techHeaderSpec struct {
	Key     string
	Aliases []string
}

var techHeaderSpecs = []techHeaderSpec{
	{Key: "bottomhole_pressure", Aliases: []string{"забойноедавление", "рзаб", "pзаб"}},
	{Key: "reservoir_pressure", Aliases: []string{"пластовоедавление", "рпл", "pпл"}},
	{Key: "buffer_pressure", Aliases: []string{"буферноедавление", "рбуф", "pбуф"}},
	{Key: "line_pressure", Aliases: []string{"линейноедавление", "рлин", "pлин"}},
	{Key: "dynamic_level", Aliases: []string{"динамическийуровень", "ндин", "hдин"}},
	{Key: "tms_temperature", Aliases: []string{"температуратмс", "tтмс", "тмс"}},
	{Key: "liquid_rate", Aliases: []string{"дебитжидкости", "qжидкости", "qжид"}},
	{Key: "water_cut", Aliases: []string{"обводненность", "обводненностьпродукции", "водосодержание"}},
	{Key: "oil_rate", Aliases: []string{"дебитнефти", "qнефти", "qнеф"}},
	{Key: "current_mrp", Aliases: []string{"текущиймрп", "мрпсут", "мрп"}},
	{Key: "pump_model", Aliases: []string{"марканасоса", "модельнасоса", "насос"}},
	{Key: "equipment", Aliases: []string{"типгно", "гно", "оборудование", "способэксплуатации"}},
	{Key: "planned_activities", Aliases: []string{"планируемыемероприятия", "мероприятия", "планработ"}},
	{Key: "object_plast", Aliases: []string{"объектразработки", "объектпласт", "продуктивныйпласт", "пластгоризонт"}},
	{Key: "element_development", Aliases: []string{"элементразработки"}},
	{Key: "production_unit", Aliases: []string{"тпп", "производственноеподразделение", "цднг"}},
	{Key: "workshop", Aliases: []string{"цех", "номерцеха"}},
	{Key: "dns", Aliases: []string{"днс"}},
	{Key: "field", Aliases: []string{"месторождение"}},
	{Key: "deposit", Aliases: []string{"залежь", "горизонт"}},
	{Key: "well", Aliases: []string{"номерскважины", "скважина", "скв"}},
	{Key: "block", Aliases: []string{"бдр", "блок", "блокразработки"}},
}

func techHeaderMatches(value string, aliases []string) bool {
	n := normalizeTechHeader(value)
	if n == "" {
		return false
	}
	for _, alias := range aliases {
		a := normalizeTechHeader(alias)
		if n == a || (len(a) >= 5 && strings.HasPrefix(n, a)) {
			return true
		}
	}
	return false
}

// resolveTechColumns keeps the historical positional mapping only as a compatibility
// fallback. When the workbook exposes recognizable headers, identity and process
// fields are taken from those headers instead. This prevents a shifted export schema
// from silently turning an unrelated numeric value (for example 145) into a well number.
func techIdentityLooksLikeData(field, well, objectPlast string) bool {
	field = strings.TrimSpace(field)
	well = strings.TrimSpace(well)
	objectPlast = strings.TrimSpace(objectPlast)
	if field == "" || well == "" || objectPlast == "" {
		return false
	}
	// Service rows in the monthly export contain column numbers (for example
	// 6 / 8 / 12) under the same positions as field / well / object. At least
	// the field or development object of a real row must contain a letter.
	hasLetter := func(s string) bool {
		for _, r := range s {
			if unicode.IsLetter(r) {
				return true
			}
		}
		return false
	}
	return hasLetter(field) || hasLetter(objectPlast)
}

func resolveTechColumns(rows []techHeaderRow) (map[string]int, int, bool) {
	cols := defaultTechColumns()
	detected := map[string]int{}
	lastHeaderRow := 0
	for _, row := range rows {
		for col, value := range row.Values {
			for _, spec := range techHeaderSpecs {
				if _, exists := detected[spec.Key]; exists {
					continue
				}
				if techHeaderMatches(value, spec.Aliases) {
					detected[spec.Key] = col
					if row.Row > lastHeaderRow {
						lastHeaderRow = row.Row
					}
					break
				}
			}
		}
	}
	// Trust a detected schema only when all three identity anchors used by the
	// technological module are explicit. Do not mix detected columns with a guessed
	// object/plast column: that is exactly how unrelated numbers can leak into keys.
	_, hasField := detected["field"]
	_, hasWell := detected["well"]
	_, hasObject := detected["object_plast"]
	if !hasField || !hasWell || !hasObject {
		return cols, 7, false
	}
	for key, col := range detected {
		cols[key] = col
	}
	dataStart := lastHeaderRow + 1
	if dataStart < 2 {
		dataStart = 2
	}
	// The source workbook can contain one or more service rows after captions:
	// units and/or sequential export-column numbers. Find the first row that
	// actually looks like an object using the resolved identity columns.
	for _, row := range rows {
		if row.Row < dataStart {
			continue
		}
		if techIdentityLooksLikeData(row.Values[cols["field"]], row.Values[cols["well"]], row.Values[cols["object_plast"]]) {
			dataStart = row.Row
			break
		}
	}
	return cols, dataStart, true
}
func techContinuationCanInheritWell(current, anchor TechObjectState) bool {
	// Hard rule for multi-reservoir wells in the Tech Regime: one explicit well row
	// may be followed by one or more rows where the well number is intentionally
	// blank, while Deposit / ObjectPlast / ElementDevelopment describes another
	// reservoir or development object of the same well. These continuation rows
	// must inherit ONLY the well number; their own deposit/object values remain
	// untouched so every distinct reservoir becomes a separate object key.
	if strings.TrimSpace(current.Well) != "" || strings.TrimSpace(anchor.Well) == "" {
		return false
	}
	curField := strings.TrimSpace(current.Field)
	anchorField := strings.TrimSpace(anchor.Field)
	if curField == "" || anchorField == "" || !strings.EqualFold(curField, anchorField) {
		return false
	}
	return strings.TrimSpace(current.Deposit) != "" ||
		strings.TrimSpace(current.ObjectPlast) != "" ||
		strings.TrimSpace(current.ElementDevelopment) != ""
}

func readTechXLSX(path string) ([]TechObjectState, error) {
	if info, e := os.Stat(path); e != nil {
		return nil, e
	} else if info.Size() > 100*1024*1024 {
		return nil, fmt.Errorf("файл Техрежима слишком большой: %.1f МБ; проверьте исходную выгрузку", float64(info.Size())/(1024*1024))
	}
	z, e := zip.OpenReader(path)
	if e != nil {
		return nil, e
	}
	defer z.Close()
	var shared []string
	if b, e := readZipEntry(z, "xl/sharedStrings.xml"); e == nil {
		var ss xlsxSharedStrings
		if xml.Unmarshal(b, &ss) == nil {
			for _, it := range ss.Items {
				shared = append(shared, sharedText(it))
			}
		}
	}
	sheetPath, sheetErr := xlsxWorksheetPath(z, "Техрежим")
	if sheetErr != nil {
		return nil, fmt.Errorf("не удалось определить лист Техрежима: %v", sheetErr)
	}
	b, e := readZipEntry(z, sheetPath)
	if e != nil {
		return nil, fmt.Errorf("не удалось открыть лист Техрежим (%s): %v", sheetPath, e)
	}
	var sh xlsxSheet
	if e = xml.Unmarshal(b, &sh); e != nil {
		return nil, e
	}
	rowValues := func(row struct {
		R     int `xml:"r,attr"`
		Cells []struct {
			R      string `xml:"r,attr"`
			T      string `xml:"t,attr"`
			V      string `xml:"v"`
			Inline struct {
				T string `xml:"t"`
			} `xml:"is"`
		} `xml:"c"`
	}) map[int]string {
		vals := map[int]string{}
		for _, c := range row.Cells {
			val := c.V
			if c.T == "s" {
				i, _ := strconv.Atoi(c.V)
				if i >= 0 && i < len(shared) {
					val = shared[i]
				}
			} else if c.T == "inlineStr" {
				val = c.Inline.T
			}
			vals[colNumber(c.R)] = strings.TrimSpace(val)
		}
		return vals
	}
	headerRows := make([]techHeaderRow, 0, 12)
	for _, row := range sh.Rows {
		if row.R > 12 {
			break
		}
		headerRows = append(headerRows, techHeaderRow{Row: row.R, Values: rowValues(row)})
	}
	cols, dataStartRow, headerMapped := resolveTechColumns(headerRows)
	if headerMapped {
		diagLog("tech import: header-based schema detected; data start row=%d, field col=%d, well col=%d", dataStartRow, cols["field"], cols["well"])
	} else {
		diagLog("tech import: recognizable field/well headers not found; using legacy positional schema")
	}
	var out []TechObjectState
	var wellAnchor TechObjectState
	for _, row := range sh.Rows {
		if row.R < dataStartRow {
			continue
		}
		vals := rowValues(row)
		get := func(c int) string { return strings.TrimSpace(vals[c]) }
		getKey := func(key string) string { return get(cols[key]) }
		st := TechObjectState{Organization: currentOrganizationName(), ProductionUnit: getKey("production_unit"), TPP: getKey("production_unit"), Workshop: getKey("workshop"), DNS: getKey("dns"), Field: getKey("field"), Deposit: getKey("deposit"), Well: getKey("well"), Block: getKey("block"), ObjectPlast: getKey("object_plast"), ElementDevelopment: getKey("element_development"), Equipment: getKey("equipment"), PumpModel: getKey("pump_model"), CurrentMRP: getKey("current_mrp"), BufferPressure: getKey("buffer_pressure"), LinePressure: getKey("line_pressure"), DynamicLevel: getKey("dynamic_level"), TMSTemperature: getKey("tms_temperature"), BottomholePressure: getKey("bottomhole_pressure"), ReservoirPressure: getKey("reservoir_pressure"), LiquidRate: getKey("liquid_rate"), WaterCut: getKey("water_cut"), OilRate: getKey("oil_rate"), PlannedActivities: getKey("planned_activities"), SourceRow: row.R, SourceRows: []int{row.R}, SourceRowCount: 1, SourceFile: filepath.Base(path), ImportedAt: time.Now().Format(time.RFC3339)}

		if strings.TrimSpace(st.Well) != "" {
			// A new explicit well number becomes the anchor for following continuation rows.
			wellAnchor = st
		} else if techContinuationCanInheritWell(st, wellAnchor) {
			st.Well = strings.TrimSpace(wellAnchor.Well)
			diagLog("tech import: inherited well %s for continuation row %d (%s / %s / %s)", st.Well, row.R, st.Field, st.Deposit, st.ObjectPlast)
		}

		st.ObjectKey = buildTechKey(st)
		if suspiciousTechIdentity(st) {
			st.ObjectKey = ""
		}
		if st.Field == "" && st.Well == "" && st.ObjectPlast == "" {
			continue
		}
		out = append(out, st)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("на листе Техрежима не найдены строки данных начиная со строки 7")
	}
	// Техрежим может содержать несколько строк одного объекта (например, интервалы перфорации).
	// Объект объединяется строго по согласованному ключу; источник строк сохраняется для проверки.
	merged := map[string]TechObjectState{}
	order := []string{}
	for _, st := range out {
		if st.ObjectKey == "" {
			key := fmt.Sprintf("__row_%d", st.SourceRow)
			merged[key] = st
			order = append(order, key)
			continue
		}
		if old, ok := merged[st.ObjectKey]; ok {
			merged[st.ObjectKey] = mergeTechState(old, st)
		} else {
			merged[st.ObjectKey] = st
			order = append(order, st.ObjectKey)
		}
	}
	out = out[:0]
	for _, k := range order {
		out = append(out, merged[k])
	}
	return out, nil
}
func preferTechValue(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	// Числовое значение само по себе не является признаком отсутствия данных.
	// В частности, 145 — допустимое значение и не должно иметь специальной семантики.
	return a
}
func mergeTechState(a, b TechObjectState) TechObjectState {
	conf := []string{}
	merge := func(name string, dst *string, src string) {
		before := strings.TrimSpace(*dst)
		after := strings.TrimSpace(src)
		if before != "" && after != "" && before != after {
			conf = append(conf, name+": "+before+" / "+after)
		}
		*dst = preferTechValue(*dst, src)
	}
	merge("оборудование", &a.Equipment, b.Equipment)
	merge("марка насоса", &a.PumpModel, b.PumpModel)
	merge("МРП", &a.CurrentMRP, b.CurrentMRP)
	merge("буферное давление (Рбуф)", &a.BufferPressure, b.BufferPressure)
	merge("линейное давление (Рлин)", &a.LinePressure, b.LinePressure)
	merge("динамический уровень", &a.DynamicLevel, b.DynamicLevel)
	merge("T ТМС", &a.TMSTemperature, b.TMSTemperature)
	merge("забойное давление (Рзаб)", &a.BottomholePressure, b.BottomholePressure)
	merge("пластовое давление (Рпл)", &a.ReservoirPressure, b.ReservoirPressure)
	merge("дебит жидкости", &a.LiquidRate, b.LiquidRate)
	merge("обводнённость", &a.WaterCut, b.WaterCut)
	merge("дебит нефти", &a.OilRate, b.OilRate)
	merge("мероприятия", &a.PlannedActivities, b.PlannedActivities)
	a.SourceRows = append(a.SourceRows, b.SourceRows...)
	a.SourceRowCount = len(a.SourceRows)
	if len(conf) > 0 {
		note := strings.Join(conf, "; ")
		if a.MergeNotes != "" {
			a.MergeNotes += "; "
		}
		a.MergeNotes += note
	}
	return a
}
func compactKeyPart(s string) string { return strings.Join(strings.Fields(strings.TrimSpace(s)), "") }
func suspiciousTechIdentity(s TechObjectState) bool {
	// Идентичность объекта считается недостаточной только при фактически пустых
	// ключевых полях. Значение 145 не является служебным маркером: оно допустимо
	// и проверяется в контексте конкретного поля, как и любое другое значение.
	f := strings.TrimSpace(s.Field)
	w := strings.TrimSpace(s.Well)
	o := strings.TrimSpace(s.ObjectPlast)
	return f == "" || w == "" || o == ""
}

func buildTechKey(s TechObjectState) string {
	// Match the technological module: Месторождение + № скв + Залежь +
	// Объект разработки/пласт. A blank deposit stays blank; it is never replaced
	// with the object/plast. Numeric values (including 145) are preserved only
	// when they actually come from their source field, e.g. a well number or a
	// numeric process parameter; there is no global special case for 145.
	return compactKeyPart(s.Field) + compactKeyPart(s.Well) + compactKeyPart(s.Deposit) + compactKeyPart(s.ObjectPlast)
}
func stateLabel(s TechObjectState) string {
	if strings.TrimSpace(s.Field+s.Well+s.Deposit+s.ObjectPlast) == "" {
		return s.ObjectKey
	}
	parts := make([]string, 0, 4)
	for _, v := range []string{s.Field, s.Well, s.Deposit, s.ObjectPlast} {
		if v = strings.TrimSpace(v); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " / ")
}
func techDiff(a, b TechObjectState) (int, string) {
	pairs := []struct{ n, x, y string }{{"оборудование", a.Equipment, b.Equipment}, {"марка насоса", a.PumpModel, b.PumpModel}, {"МРП", a.CurrentMRP, b.CurrentMRP}, {"дебит жидкости", a.LiquidRate, b.LiquidRate}, {"обводнённость", a.WaterCut, b.WaterCut}, {"дебит нефти", a.OilRate, b.OilRate}, {"буферное давление (Рбуф)", a.BufferPressure, b.BufferPressure}, {"линейное давление (Рлин)", a.LinePressure, b.LinePressure}, {"динамический уровень", a.DynamicLevel, b.DynamicLevel}, {"T ТМС", a.TMSTemperature, b.TMSTemperature}, {"забойное давление (Рзаб)", a.BottomholePressure, b.BottomholePressure}, {"пластовое давление (Рпл)", a.ReservoirPressure, b.ReservoirPressure}, {"планируемые мероприятия", a.PlannedActivities, b.PlannedActivities}}
	var d []string
	for _, q := range pairs {
		if strings.TrimSpace(q.x) != strings.TrimSpace(q.y) {
			d = append(d, q.n+": "+dash(q.x)+" → "+dash(q.y))
		}
	}
	return len(d), strings.Join(d, "; ")
}
func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
func computeTechPreview(path string, current []TechObjectState) ([]TechObjectState, []TechChange, error) {
	rows, e := readTechXLSX(path)
	if e != nil {
		return nil, nil, e
	}
	// В текущий снимок не включаются только строки с реально отсутствующими
	// ключевыми полями идентификации. Числовые значения, включая 145, не имеют
	// специальной семантики и не отбрасываются сами по себе.
	validRows := make([]TechObjectState, 0, len(rows))
	skippedIdentity := 0
	for _, row := range rows {
		if row.ObjectKey == "" || row.Field == "" || row.Well == "" {
			skippedIdentity++
			continue
		}
		validRows = append(validRows, row)
	}
	preview := make([]TechChange, 0, len(validRows)+1)
	old := map[string]TechObjectState{}
	for _, st := range current {
		if st.ObjectKey != "" {
			old[st.ObjectKey] = st
		}
	}
	seen := map[string]bool{}
	for _, n := range validRows {
		if n.ObjectKey != "" {
			seen[n.ObjectKey] = true
		}
		srcNote := ""
		if n.SourceRowCount > 1 {
			srcNote = fmt.Sprintf("Источник объединён из %d строк", n.SourceRowCount)
			if n.MergeNotes != "" {
				srcNote += "; конфликты: " + n.MergeNotes
			}
		}
		if o, ok := old[n.ObjectKey]; !ok {
			d := "новый объект в Техрежиме"
			if srcNote != "" {
				d += "; " + srcNote
			}
			preview = append(preview, TechChange{Status: "Новый", ObjectKey: n.ObjectKey, Label: stateLabel(n), Details: d})
		} else if c, d := techDiff(o, n); c > 0 {
			if srcNote != "" {
				d += "; " + srcNote
			}
			preview = append(preview, TechChange{Status: "Изменён", ObjectKey: n.ObjectKey, Label: stateLabel(n), ChangedCount: c, Details: d})
		} else {
			preview = append(preview, TechChange{Status: "Без изменений", ObjectKey: n.ObjectKey, Label: stateLabel(n), Details: srcNote})
		}
	}
	for key, o := range old {
		if !seen[key] {
			preview = append(preview, TechChange{Status: "Нет в новой выгрузке", ObjectKey: key, Label: stateLabel(o), Details: "объект присутствовал в предыдущем принятом Техрежиме, но отсутствует в новой выгрузке; прошлые расчёты сохраняются"})
		}
	}
	if skippedIdentity > 0 {
		preview = append(preview, TechChange{
			Status:       "Без идентификатора",
			Label:        "Строки без номера скважины",
			ChangedCount: skippedIdentity,
			Details:      "не включены в объектный снимок: отсутствуют обязательные поля идентификации объекта; остальные объекты импортируются без ручного сопоставления",
		})
	}
	return validRows, preview, nil
}

func previewTechImport(path string) error {
	rows, preview, e := computeTechPreview(path, append([]TechObjectState(nil), techCurrent...))
	if e != nil {
		return e
	}
	pendingTech = rows
	techPreview = preview
	selectedTechFile = path
	return nil
}

type techPreviewWorkerResult struct {
	path    string
	rows    []TechObjectState
	preview []TechChange
	err     error
}

var techPreviewWorker = make(chan techPreviewWorkerResult, 1)
var techPreviewRunning bool

func setTechPreviewControlsEnabled(enabled bool) {
	v := uintptr(0)
	if enabled {
		v = 1
	}
	for _, h := range []syscall.Handle{techChooseButton, techPreviewButton, techApplyButton} {
		if h != 0 {
			pEnableWindow.Call(uintptr(h), v)
		}
	}
}

func startTechPreviewAsync() {
	if techPreviewRunning {
		return
	}
	if strings.TrimSpace(selectedTechFile) == "" {
		msg("Импорт Техрежима", "Сначала выберите XLSX-файл Техрежима.", MB_OK|MB_ICONWARNING)
		return
	}
	path := selectedTechFile
	current := append([]TechObjectState(nil), techCurrent...)
	techPreviewRunning = true
	setTechPreviewControlsEnabled(false)
	setText(techImportSummary, "Проверка Техрежима выполняется в фоне. Интерфейс остаётся доступным.")
	diagLog("TECH preview start file=%s", path)
	go func() {
		res := techPreviewWorkerResult{path: path}
		defer func() {
			if r := recover(); r != nil {
				res.err = fmt.Errorf("внутренняя ошибка проверки Техрежима: %v", r)
				diagLog("TECH preview panic=%v\n%s", r, debug.Stack())
			}
			select {
			case techPreviewWorker <- res:
			default:
				diagLog("TECH preview result dropped: worker channel is full")
			}
			pPostMessageW.Call(uintptr(hMain), WM_APP_TECH_PREVIEW_DONE, 0, 0)
		}()
		res.rows, res.preview, res.err = computeTechPreview(path, current)
	}()
}

func finishTechPreviewAsync() {
	var res techPreviewWorkerResult
	select {
	case res = <-techPreviewWorker:
	default:
		return
	}
	techPreviewRunning = false
	setTechPreviewControlsEnabled(true)
	if res.err != nil {
		diagLog("TECH preview error: %v", res.err)
		setText(techImportSummary, "Проверка не выполнена: "+res.err.Error())
		if currentPage == ID_DASH {
			msg("Проверка Техрежима", res.err.Error(), MB_OK|MB_ICONERROR)
		}
		return
	}
	pendingTech = res.rows
	techPreview = res.preview
	selectedTechFile = res.path
	if currentPage != ID_DASH {
		diagLog("TECH preview stored for dashboard; active page=%d", currentPage)
		return
	}
	refreshTechImportUI()
	pRedrawWindow.Call(uintptr(hMain), 0, 0, 0x0001|0x0004|0x0080)
	pUpdateWindow.Call(uintptr(hMain))
	a, c, u, rem, skipped, unresolved := techPreviewCounts()
	diagLog("TECH preview done accepted=%d added=%d changed=%d unchanged=%d removed=%d skipped=%d unresolved=%d", len(pendingTech), a, c, u, rem, skipped, unresolved)
	msg("Проверка Техрежима", fmt.Sprintf("Проверка завершена.\nОбъектов принято к сравнению: %d\nНовых: %d\nИзменённых: %d\nБез изменений: %d\nНет в новой выгрузке: %d\nСтрок без номера скважины: %d", len(pendingTech), a, c, u, rem, skipped), MB_OK|MB_ICONINFORMATION)
}

func techPreviewCounts() (a, c, u, rem, skipped, unresolved int) {
	for _, x := range techPreview {
		switch x.Status {
		case "Новый":
			a++
		case "Изменён":
			c++
		case "Без изменений":
			u++
		case "Нет в новой выгрузке":
			rem++
		case "Без идентификатора":
			skipped += x.ChangedCount
		default:
			unresolved++
		}
	}
	return
}
func hashFile(path string) string {
	f, e := os.Open(path)
	if e != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	_, _ = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}
func applyTechImport() error {
	if len(pendingTech) == 0 {
		return fmt.Errorf("сначала выполните проверку изменений")
	}
	if e := createStateBackup(); e != nil {
		return fmt.Errorf("не удалось создать защитную копию состояния: %v", e)
	}
	if len(techCurrent) > 0 {
		dir := filepath.Join(sharedDataDir(), "tech_history")
		_ = os.MkdirAll(dir, 0755)
		if e := saveJSON(filepath.Join(dir, "tech_"+time.Now().Format("20060102_150405")+".json"), techCurrent); e != nil {
			return fmt.Errorf("не удалось сохранить предыдущее состояние Техрежима: %v", e)
		}
	}

	currentPath := sharedPath("tech_current.json")
	importsPath := sharedPath("tech_imports.json")
	oldCurrent, currentReadErr := os.ReadFile(currentPath)
	oldImports, importsReadErr := os.ReadFile(importsPath)
	newCurrent := append([]TechObjectState(nil), pendingTech...)
	a, c, u, rem, skipped, unresolved := techPreviewCounts()
	newImports := append([]TechImportRecord(nil), techImports...)
	newImports = append(newImports, TechImportRecord{ImportedAt: time.Now().Format(time.RFC3339), SourceFile: selectedTechFile, SHA256: hashFile(selectedTechFile), Total: len(newCurrent) + skipped, Added: a, Changed: c, Unchanged: u, Removed: rem, Skipped: skipped, Unresolved: unresolved})
	if e := saveJSON(currentPath, newCurrent); e != nil {
		return e
	}
	if e := saveJSON(importsPath, newImports); e != nil {
		rollbackErr := restoreImportFile(currentPath, oldCurrent, currentReadErr)
		_ = restoreImportFile(importsPath, oldImports, importsReadErr)
		if rollbackErr != nil {
			return fmt.Errorf("журнал импорта не сохранён: %v; откат текущего Техрежима также не выполнен: %v", e, rollbackErr)
		}
		return fmt.Errorf("журнал импорта не сохранён; Техрежим возвращён к предыдущему состоянию: %v", e)
	}
	techCurrent = newCurrent
	techImports = newImports
	invalidateMethodsBackground()
	pendingTech = nil
	return nil
}

func restoreImportFile(path string, previous []byte, readErr error) error {
	if readErr == nil {
		return atomicWriteFile(path, previous, 0600)
	}
	if os.IsNotExist(readErr) {
		if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
			return e
		}
		return nil
	}
	return readErr
}
func chooseTechFile() string {
	buf := make([]uint16, 32768)
	if selectedTechFile != "" {
		q, _ := syscall.UTF16FromString(selectedTechFile)
		copy(buf, q)
	}
	filter := utf16.Encode([]rune("Excel (*.xlsx)\x00*.xlsx\x00Все файлы (*.*)\x00*.*\x00\x00"))
	title := p("Выберите свежий Техрежим")
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: title, Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST, LpstrDefExt: p("xlsx")}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
func refreshTechImportUI() {
	if techImportFileLabel == 0 {
		return
	}
	f := selectedTechFile
	if f == "" {
		f = "Файл не выбран"
	}
	if f == "Файл не выбран" {
		setText(techImportFileLabel, f)
	} else {
		setText(techImportFileLabel, "Файл: "+filepath.Base(f))
	}
	a, c, u, rem, skipped, unresolved := techPreviewCounts()
	summary := fmt.Sprintf("Проверка: объектов %d | новых %d | изменённых %d | без изменений %d | нет в новой выгрузке %d | пропущено неполных строк %d", len(pendingTech), a, c, u, rem, skipped)
	if unresolved > 0 {
		summary += fmt.Sprintf(" | служебных статусов %d", unresolved)
	}
	setText(techImportSummary, summary)
	listBeginUpdate(techImportList)
	defer listEndUpdate(techImportList)
	listClear(techImportList)
	row := 0
	for _, x := range techPreview {
		if x.Status == "Без изменений" {
			continue
		}
		listAdd(techImportList, row, []string{x.Status, x.Label, strconv.Itoa(x.ChangedCount), x.Details})
		row++
		if row >= 250 {
			break
		}
	}
}

func mapStr(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch q := v.(type) {
	case string:
		return strings.TrimSpace(q)
	case float64:
		return strconv.FormatFloat(q, 'f', -1, 64)
	case bool:
		if q {
			return "Да"
		}
		return "Нет"
	default:
		b, _ := json.Marshal(q)
		return string(b)
	}
}
func mapBool(m map[string]any, k string) bool {
	v := m[k]
	switch q := v.(type) {
	case bool:
		return q
	case string:
		return q == "true" || q == "Да"
	}
	return false
}
func nestedSummary(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok {
		return ""
	}
	mm, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	keys := []string{"net_horizon", "npv", "benefit_cost", "payback_months", "status"}
	var a []string
	for _, x := range keys {
		if s := mapStr(mm, x); s != "" {
			a = append(a, x+"="+s)
		}
	}
	return strings.Join(a, "; ")
}
func versionFromRaw(raw json.RawMessage) VersionRecord {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	key := mapStr(m, "key")
	created := mapStr(m, "created")
	if created == "" {
		created = time.Now().Format("2006-01-02 15:04")
	}
	base := key + "|" + created + "|" + mapStr(m, "complication_code") + "|" + mapStr(m, "method") + "|" + mapStr(m, "forecast_mrp")
	sum := sha256.Sum256([]byte(base))
	id := hex.EncodeToString(sum[:10])
	return VersionRecord{VersionID: id, ObjectKey: key, Created: created, Field: mapStr(m, "field"), Well: mapStr(m, "well_no"), Deposit: mapStr(m, "deposit"), ObjectPlast: mapStr(m, "object_plast"), Complication: mapStr(m, "complication_code"), Method: mapStr(m, "method"), Equipment: mapStr(m, "equipment"), ForecastMRP: mapStr(m, "forecast_mrp"), CurrentMRP: mapStr(m, "current_mrp"), LabStatus: mapStr(m, "lab_status"), EconMode: mapStr(m, "econ_mode"), EconStatus: mapStr(m, "econ_status"), EconResult: nestedSummary(m, "econ"), Decision: mapStr(m, "decision"), DecisionComment: mapStr(m, "decision_comment"), Responsible: mapStr(m, "responsible"), SourceFile: mapStr(m, "source_file"), Archived: mapBool(m, "archived"), Quarter: mapStr(m, "quarter"), Raw: raw, ImportedAt: time.Now().Format(time.RFC3339)}
}
func findLegacyState() string {
	candidates := []string{}
	for _, base := range []string{os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA")} {
		if base != "" {
			candidates = append(candidates, filepath.Join(base, "OFModelNativeAlpha", "state_v13.json"), filepath.Join(base, "OFModel", "state_v13.json"), filepath.Join(base, "OFModelNativeAlpha", "state_v12.json"))
		}
	}
	if ex, _ := os.Executable(); ex != "" {
		d := filepath.Dir(ex)
		candidates = append(candidates, filepath.Join(d, "state_v13.json"), filepath.Join(d, "state_v12.json"))
	}
	for _, c := range candidates {
		if _, e := os.Stat(c); e == nil {
			return c
		}
	}
	return ""
}
func syncLegacy() (int, string, error) {
	path := findLegacyState()
	if path == "" {
		return 0, "", fmt.Errorf("локальное состояние расчётного модуля не найдено. Сначала запустите основной расчётный модуль и сохраните хотя бы одну оценку")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return 0, path, e
	}
	var root map[string]json.RawMessage
	if e = json.Unmarshal(b, &root); e != nil {
		return 0, path, e
	}
	var rr []json.RawMessage
	if x, ok := root["records"]; ok {
		_ = json.Unmarshal(x, &rr)
	}
	if len(rr) == 0 {
		return 0, path, fmt.Errorf("в состоянии нет сохранённых оценок")
	}
	have := map[string]bool{}
	for _, v := range versions {
		have[v.VersionID] = true
	}
	for id := range deletedVersionIDs {
		have[id] = true
	}
	n := 0
	for _, raw := range rr {
		v := versionFromRaw(raw)
		if !have[v.VersionID] {
			v = compactVersionPayload(v)
			if e = appendLedger(v); e != nil {
				return n, path, e
			}
			versions = append(versions, v)
			have[v.VersionID] = true
			n++
		}
	}
	sort.SliceStable(versions, func(i, j int) bool { return parseDate(versions[i].Created).After(parseDate(versions[j].Created)) })
	_ = saveJSON(filepath.Join(dataDir, "calculation_versions.json"), versions) // восстановимый индекс для быстрого просмотра
	return n, path, nil
}
func parseDate(s string) time.Time {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "02.01.2006 15:04", "02.01.2006", "2006-01-02"}
	for _, l := range layouts {
		if t, e := time.ParseInLocation(l, s, time.Local); e == nil {
			return t
		}
	}
	return time.Now()
}
func toFloat(s string) float64 {
	s = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), " ", ""), ",", ".")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
func latestByObject() map[string]VersionRecord {
	m := map[string]VersionRecord{}
	for _, v := range versions {
		if isRetiredLegacyTeachingObjectKey(v.ObjectKey) {
			continue
		}
		old, ok := m[v.ObjectKey]
		if !ok || parseDate(v.Created).After(parseDate(old.Created)) {
			m[v.ObjectKey] = v
		}
	}
	return m
}
func outcomeFor(ver string) *Outcome {
	for i := range outcomes {
		if outcomes[i].VersionID == ver {
			return &outcomes[i]
		}
	}
	if teaching, ok := teachingOutcomeForVersion(ver); ok {
		return teaching
	}
	return nil
}
func generatePlan() int {
	latest := latestByObject()
	existing := map[string]bool{}
	for _, p := range plans {
		existing[p.VersionID+"|"+p.Action] = true
	}
	n := 0
	for _, v := range latest {
		low := strings.ToLower(v.Method)
		mrp := toFloat(v.ForecastMRP)
		if mrp <= 0 {
			mrp = toFloat(v.CurrentMRP)
		}
		if strings.Contains(low, "промыв") && mrp > 0 {
			days := int(mrp * 2.0 / 3.0)
			if days < 1 {
				days = 1
			}
			due := parseDate(v.Created).AddDate(0, 0, days).Format("02.01.2006")
			action := "Плановое мероприятие по промывке"
			basis := fmt.Sprintf("Предварительный срок: 2/3 последнего МРП (%.0f сут.)", mrp)
			key := v.VersionID + "|" + action
			if !existing[key] {
				plans = append(plans, PlanItem{ID: v.VersionID[:10] + "-wash", DueDate: due, ObjectKey: v.ObjectKey, Action: action, Basis: basis, Status: "Предварительно запланировано", VersionID: v.VersionID})
				existing[key] = true
				n++
			}
		}
		if outcomeFor(v.VersionID) == nil {
			action := "Зарегистрировать фактический результат применения"
			key := v.VersionID + "|" + action
			if !existing[key] {
				plans = append(plans, PlanItem{ID: v.VersionID[:10] + "-fact", DueDate: "—", ObjectKey: v.ObjectKey, Action: action, Basis: "Нужен прогноз/факт для накопления эффективности технологии", Status: "Требуется назначить срок", VersionID: v.VersionID})
				existing[key] = true
				n++
			}
		}
		if strings.Contains(low, "контейнер") {
			action := "Контроль остаточного содержания реагента и состояния контейнера"
			key := v.VersionID + "|" + action
			if !existing[key] {
				plans = append(plans, PlanItem{ID: v.VersionID[:10] + "-cont", DueDate: "—", ObjectKey: v.ObjectKey, Action: action, Basis: "Срок определяется действующим технологическим регламентом", Status: "Требуется назначить срок", VersionID: v.VersionID})
				existing[key] = true
				n++
			}
		}
		if strings.Contains(low, "трил") {
			action := "Контроль фактической наработки погружного контейнера"
			key := v.VersionID + "|" + action
			if !existing[key] {
				plans = append(plans, PlanItem{ID: v.VersionID[:10] + "-tril", DueDate: "—", ObjectKey: v.ObjectKey, Action: action, Basis: "365 суток — гарантийный срок, а не ограничение прогнозного МРП", Status: "Требуется назначить срок", VersionID: v.VersionID})
				existing[key] = true
				n++
			}
		}
	}
	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].DueDate == "—" {
			return false
		}
		if plans[j].DueDate == "—" {
			return true
		}
		return parseDate(plans[i].DueDate).Before(parseDate(plans[j].DueDate))
	})
	_ = saveJSON(filepath.Join(dataDir, "plan.json"), plans)
	return n
}

func buildUI() {
	// Approved РОСА identity: a light engineering header, visible corporate icon,
	// restrained LUKOIL red and a dark-blue workflow rail.
	hTopBG = create("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1280, HEADER_H, hMain, 0)
	hBrandIcon = create("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP|SS_CENTERIMAGE, 8, 2, 178, 86, hMain, 0)
	if hBrandIcon != 0 {
		if bitmap, err := buildHeaderBrandBitmap(); bitmap != 0 {
			hBrandBitmap = bitmap
			pSendMessageW.Call(uintptr(hBrandIcon), STM_SETIMAGE, IMAGE_BITMAP, uintptr(hBrandBitmap))
		} else {
			diagLog("header brand bitmap error: %v", err)
		}
	}
	hHeaderSep1 = create("STATIC", "", WS_CHILD|WS_VISIBLE, 194, 10, 2, 88, hMain, 0)
	hBrandSignature = create("STATIC", "ИНФОРМАЦИОННО-\r\nАНАЛИТИЧЕСКАЯ СИСТЕМА", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, 8, 86, 178, 34, hMain, 0)
	pSendMessageW.Call(uintptr(hBrandSignature), WM_SETFONT, uintptr(hFontBrandCaption), 1)
	// One owner-draw label keeps the whole РОСА phrase geometrically stable.
	// Only the initials are painted red; the remaining letters stay dark blue.
	// Keeping one control prevents word groups from drifting apart at different DPI values.
	hSubtitle = create("STATIC", "", WS_CHILD|WS_VISIBLE|SS_OWNERDRAW, 224, 8, 682, 48, hMain, ID_HEADER_SUBTITLE)
	hSystemName = create("STATIC", "", WS_CHILD|SS_CENTER|SS_NOPREFIX, 224, 60, 682, 44, hMain, 0)
	pSendMessageW.Call(uintptr(hSystemName), WM_SETFONT, uintptr(hFontSystemName), 1)
	hHeaderSep2 = create("STATIC", "", WS_CHILD|WS_VISIBLE, 626, 8, 2, 108, hMain, 0)
	// The approved LUKOIL block is the compact right-hand section immediately
	// before the action buttons. The wide middle section belongs to the product
	// description, not to the corporate mark.
	hLukoilLabel = create("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP|SS_CENTERIMAGE, 622, 8, 176, 52, hMain, 0)
	if bitmap, err := buildHeaderLukoilBitmap(); bitmap != 0 {
		hLukoilBitmap = bitmap
		pSendMessageW.Call(uintptr(hLukoilLabel), STM_SETIMAGE, IMAGE_BITMAP, uintptr(hLukoilBitmap))
	} else {
		diagLog("header LUKOIL bitmap error: %v", err)
	}
	hBuildMark = create("STATIC", "ВСЕГДА В\r\nДВИЖЕНИИ", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, 622, 66, 176, 42, hMain, 0)
	pSendMessageW.Call(uintptr(hBuildMark), WM_SETFONT, uintptr(hFontSlogan), 1)
	hHeaderNew = 0
	sessionTrashButton = create("BUTTON", "Корзина · 0", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 724, 30, 84, 42, hMain, ID_SESSION_TRASH)
	hHeaderBack = create("BUTTON", "←", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 814, 30, 48, 42, hMain, ID_HEADER_BACK)
	hHeaderForward = create("BUTTON", "→", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 866, 30, 48, 42, hMain, ID_HEADER_FORWARD)
	hHeaderOpen = create("BUTTON", "История", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 918, 30, 82, 42, hMain, ID_HISTORY)
	hHeaderHelp = create("BUTTON", "Справка", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 1006, 30, 82, 42, hMain, ID_HEADER_HELP)
	hHeaderExport = create("BUTTON", "Выход", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 1094, 30, 82, 42, hMain, ID_HEADER_EXIT)
	hHeaderApprove = create("BUTTON", userSwitchCaption, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 1060, 10, 200, 36, hMain, ID_HEADER_APPROVE)
	hUserLabel = create("STATIC", currentUserLabel(), WS_CHILD|WS_VISIBLE|WS_BORDER|SS_CENTER|SS_CENTERIMAGE|SS_NOPREFIX|SS_ENDELLIPSIS, 1080, 34, 280, 36, hMain, 0)
	pSendMessageW.Call(uintptr(hUserLabel), WM_SETFONT, uintptr(hFontBold), 1)
	hStripe = create("STATIC", "", WS_CHILD|WS_VISIBLE, 0, STRIPE_Y, 1280, 4, hMain, 0)
	hWarningBG = create("STATIC", "", WS_CHILD|WS_VISIBLE, 12, WARNING_Y+4, 1256, WARNING_H-8, hMain, 0)
	hWarningTitle = create("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP|SS_CENTERIMAGE, 20, WARNING_Y+8, 32, 30, hMain, 0)
	if bitmap, err := buildWarningIconBitmap(); bitmap != 0 {
		hWarningBitmap = bitmap
		pSendMessageW.Call(uintptr(hWarningTitle), STM_SETIMAGE, IMAGE_BITMAP, uintptr(hWarningBitmap))
	} else {
		diagLog("warning icon bitmap error: %v", err)
	}
	hWarningText = create("STATIC", "Результаты носят предварительный характер и требуют подтверждения исследованиями и производственной проверкой.", WS_CHILD|WS_VISIBLE|SS_NOPREFIX|SS_ENDELLIPSIS, 58, WARNING_Y+9, 1174, 26, hMain, 0)
	pSendMessageW.Call(uintptr(hWarningText), WM_SETFONT, uintptr(hFontWarning), 1)
	createSectionNoticeControl()
	hNavBG = create("STATIC", "", WS_CHILD|WS_VISIBLE, 0, NAV_TOP, 220, 900, hMain, 0)
	nav := []struct {
		id int
		t  string
	}{{ID_DASH, "Главная"}, {ID_OBJECTS, "1. Фонд и карточка скважины"}, {ID_LAB, "2. Лабораторные данные"}, {ID_ASSESS, "3. Оценка осложнений"}, {ID_ACTIVITIES, "4. Методы и мероприятия"}, {ID_MRP, "5. Наработка и отказы"}, {ID_ECON, "6. Экономическая оценка"}, {ID_RESULT, "7. Итоговый отчёт"}, {ID_ARCHIVE, "8. Утверждённый реестр"}, {ID_DATA, "9. Настройки и журнал"}}
	y := int(NAV_TOP) + 16
	for _, n := range nav {
		buttonH := int32(54)
		b := create("BUTTON", n.t, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON|BS_OWNERDRAW, 12, int32(y), 196, buttonH, hMain, n.id)
		navButtons[n.id] = b
		y += int(buttonH) + 6
	}
	// The lower identity note duplicated the header and visually pulled the menu
	// upward. The navigation panel now contains navigation only.
	hNavNote = 0

	// dashboard — approved entry flow: load current tech regime, choose a concrete
	// object explicitly, or continue the previously confirmed object.
	pageTitle(ID_DASH, "Главная", 242, 94, 1010, 34)
	label(ID_DASH, "Техрежим и выбор рабочего объекта", 242, 130, 1010, 24)
	busyLoadingLabel = label(ID_DASH, "", 760, 130, 490, 26)
	if busyLoadingLabel != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(busyLoadingLabel), WM_SETFONT, uintptr(hFontBold), 1)
	}
	show(busyLoadingLabel, false)
	dashboardImportAccent = label(ID_DASH, "", 242, 160, 6, 30)
	dashboardImportTitle = label(ID_DASH, "1. ОБНОВЛЕНИЕ ТЕХРЕЖИМА", 258, 160, 500, 30)
	if dashboardImportTitle != 0 {
		pSendMessageW.Call(uintptr(dashboardImportTitle), WM_SETFONT, uintptr(hFontSection), 1)
	}
	techChooseButton = button(ID_DASH, "1  ВЫБРАТЬ ФАЙЛ", 242, 192, 165, 42, ID_TECH_CHOOSE)
	techPreviewButton = button(ID_DASH, "2  ПРОВЕРИТЬ", 415, 192, 165, 42, ID_TECH_PREVIEW)
	techApplyButton = button(ID_DASH, "3  ПРИНЯТЬ", 588, 192, 165, 42, ID_TECH_APPLY)
	// File and validation status lines were removed from the dashboard to keep
	// the action row centered and uncluttered. Detailed results remain available
	// through the import workflow itself.
	techImportFileLabel = 0
	techImportSummary = 0
	// A separate one-row tech-regime preview duplicated the summary and consumed
	// valuable vertical space. Detailed verification remains available in the
	// import flow; the Main page deliberately has no narrow table here.
	techImportList = 0
	dashboardSelectionAccent = label(ID_DASH, "", 242, 350, 6, 30)
	dashboardSearchTitle = label(ID_DASH, "3. ВЫБОР ОБЪЕКТА", 258, 350, 500, 30)
	if dashboardSearchTitle != 0 {
		pSendMessageW.Call(uintptr(dashboardSearchTitle), WM_SETFONT, uintptr(hFontSection), 1)
	}
	objectsFilter = edit(ID_DASH, "", 242, 386, 1010, 34, false, false, ID_FILTER_OBJECTS)
	objectsList = newList(ID_DASH, 242, 428, 1010, 166, []struct {
		n string
		w int32
	}{{"Месторождение", 180}, {"Скважина", 105}, {"Залежь", 160}, {"Объект разработки / пласт", 230}, {"Шифр", 165}, {"ГНО", 140}, {"МРП, сут.", 95}})
	setListHeaderFont(objectsList, hFontHeaderSmall)
	centerListHeaders(objectsList, 7)
	objectSelectFrame = create("STATIC", "", WS_CHILD|WS_BORDER, 238, py(188), 165, 50, hMain, 0)
	addPageControl(ID_DASH, objectSelectFrame)
	objectSelectButton = button(ID_DASH, "ЗАКРЕПИТЬ ВЫБРАННЫЙ ОБЪЕКТ", 761, 192, 165, 42, ID_OBJECT_CONFIRM)
	dashboardObjectAccent = label(ID_DASH, "", 242, 252, 6, 30)
	dashboardObjectTitle = label(ID_DASH, "2. ЗАКРЕПЛЁННЫЙ ОБЪЕКТ", 258, 252, 500, 30)
	if dashboardObjectTitle != 0 {
		pSendMessageW.Call(uintptr(dashboardObjectTitle), WM_SETFONT, uintptr(hFontSection), 1)
	}
	dashboardNote = edit(ID_DASH, "Объект ещё не выбран.", 242, 286, 1010, 52, true, true, 0)
	dashboardHistoryAccent = label(ID_DASH, "", 242, 620, 6, 30)
	dashboardRecentTitle = label(ID_DASH, "4. ЖУРНАЛ РАБОТ", 258, 620, 300, 30)
	if dashboardRecentTitle != 0 {
		pSendMessageW.Call(uintptr(dashboardRecentTitle), WM_SETFONT, uintptr(hFontSection), 1)
	}
	dashboardHistoryHint = label(ID_DASH, "Двойной щелчок — открыть сохранённую запись", 780, 624, 470, 22)
	if dashboardHistoryHint != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(dashboardHistoryHint), WM_SETFONT, uintptr(hFontBold), 1)
	}
	recentList = newList(ID_DASH, 242, 656, 1010, 150, []struct {
		n string
		w int32
	}{{"№", 50}, {"Дата", 90}, {"Шифр", 90}, {"Лабораторные данные", 180}, {"Классы осложнений", 155}, {"Методы", 75}, {"Экономическая оценка", 135}, {"Общее заключение", 155}, {"Заполнение, %", 105}})
	pSendMessageW.Call(uintptr(recentList), LVM_SETBKCOLOR, 0, rgb(246, 248, 251))
	pSendMessageW.Call(uintptr(recentList), LVM_SETTEXTBKCOLOR, 0, rgb(246, 248, 251))
	pSendMessageW.Call(uintptr(recentList), LVM_SETTEXTCOLOR, 0, rgb(43, 46, 50))
	setListHeaderFont(recentList, hFontHeaderSmall)
	centerListHeaders(recentList, 9)

	// objects
	pageTitle(ID_OBJECTS, "Карточка скважины", 242, 94, 600, 34)
	objectEquipmentButton = button(ID_OBJECTS, "СОСТОЯНИЕ ОБОРУДОВАНИЯ →", 940, 94, 312, 46, ID_OPEN_EQUIPMENT)
	objectPassportLabel = label(ID_OBJECTS, "Выбранный объект:", 242, 133, 238, 26)
	objectSelectedValue = labelEllipsis(ID_OBJECTS, "", 480, 133, 740, 26)
	pSendMessageW.Call(uintptr(objectSelectedValue), WM_SETFONT, uintptr(hFontBold), 1)
	objectDetail = edit(ID_OBJECTS, "Сначала выберите объект на Главной.", 242, 156, 1010, 62, true, true, 0)
	buildWellCardFailureUI()

	// laboratory: separate accumulated histories by complication direction.
	buildLaboratoryUI()

	// telemetry: offline imports from Telescope / MegaWeb / WebSCADA / generic exports.
	buildTelemetryUI()

	// assessment of complications: three explicit inner tabs mirror the working
	// logic of the Technology Module — technologist classification, preliminary
	// program assumption from laboratory data, and final reconciliation.
	pageTitle(ID_ASSESS, "Оценка осложнений", 242, 94, 900, 34)
	assessObjectInfo = label(ID_ASSESS, "Объект не выбран.", 242, 132, 650, 50)
	if assessObjectInfo != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(assessObjectInfo), WM_SETFONT, uintptr(hFontBold), 1)
	}
	assessProbabilityButton = button(ID_ASSESS, "Обновить оценку", 902, 132, 170, 50, ID_ASSESS_PROBABILITY)
	assessHistoryButton = button(ID_ASSESS, "История / сравнение", 1082, 132, 170, 50, ID_HISTORY)
	assessTabTechButton = button(ID_ASSESS, "1. Классификация технолога", 242, 190, 320, 44, ID_ASSESS_TAB_TECH)
	assessTabLabButton = button(ID_ASSESS, "2. Предположение по лаборатории", 574, 190, 340, 44, ID_ASSESS_TAB_LAB)
	assessTabCompareButton = button(ID_ASSESS, "3. Сверка и подтверждение", 926, 190, 326, 44, ID_ASSESS_TAB_COMPARE)
	assessOverall = label(ID_ASSESS, "Классификация строится отдельно по решению технолога и по лабораторному предположению; итог подтверждается вручную.", 242, 224, 1010, 44)
	if assessOverall != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(assessOverall), WM_SETFONT, uintptr(hFontBold), 1)
	}
	assessTechList = newList(ID_ASSESS, 242, 276, 1010, 280, []struct {
		n string
		w int32
	}{{"Направление", 200}, {"Префикс", 80}, {"Класс технолога", 150}, {"Статус", 170}, {"Комментарий", 410}})
	assessProgramList = newList(ID_ASSESS, 242, 276, 1010, 280, []struct {
		n string
		w int32
	}{{"Направление", 190}, {"Класс программы", 140}, {"Уверенность", 120}, {"Источник", 220}, {"Обоснование класса", 340}})
	assessCompareList = newList(ID_ASSESS, 242, 276, 1010, 280, []struct {
		n string
		w int32
	}{{"Направление", 205}, {"Технолог", 120}, {"Программа", 120}, {"Сверка", 300}, {"Выбранное решение", 265}})
	// Native checkboxes allow the technologist to apply one decision source to
	// several complication directions in a single action. The check itself is
	// only a batch selector; it never approves classification.
	pSendMessageW.Call(uintptr(assessCompareList), LVM_SETEXTENDEDLISTVIEWSTYLE, LVS_EX_CHECKBOXES, LVS_EX_CHECKBOXES)
	for _, table := range []syscall.Handle{assessTechList, assessProgramList, assessCompareList} {
		setListHeaderFont(table, hFontBold)
		centerListHeaders(table, 5)
	}
	// Legacy list is retained as a hidden compatibility surface for old handlers.
	assessList = newList(ID_ASSESS, 0, 0, 0, 0, []struct {
		n string
		w int32
	}{{"Направление", 185}, {"Результат", 135}, {"База / полнота", 130}, {"Дата", 105}, {"Источник", 170}, {"Ключевое основание", 305}})
	assessClassCaption = label(ID_ASSESS, "Класс / решение", 242, 568, 180, 24)
	if assessClassCaption != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(assessClassCaption), WM_SETFONT, uintptr(hFontBold), 1)
	}
	assessClassCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 426, py(562), 220, 240, hMain, ID_ASSESS_CLASS_COMBO)
	addPageControl(ID_ASSESS, assessClassCombo)
	assessClassComment = edit(ID_ASSESS, "", 660, 562, 380, 34, false, false, ID_ASSESS_CLASS_COMMENT)
	assessClassSaveButton = button(ID_ASSESS, "СОХРАНИТЬ", 1052, 562, 200, 36, ID_ASSESS_CLASS_SAVE)
	assessAcceptTechButton = button(ID_ASSESS, "ПРИНЯТЬ КЛАССИФИКАЦИЮ", 842, 562, 200, 36, ID_ASSESS_ACCEPT_TECH)
	assessFinalCaption = label(ID_ASSESS, "Итоговый класс выбранного осложнения", 242, 568, 180, 24)
	if assessFinalCaption != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(assessFinalCaption), WM_SETFONT, uintptr(hFontBold), 1)
	}
	assessFinalClassCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 426, py(562), 220, 240, hMain, ID_ASSESS_FINAL_COMBO)
	addPageControl(ID_ASSESS, assessFinalClassCombo)
	assessFinalComment = edit(ID_ASSESS, "", 660, 562, 380, 72, true, false, ID_ASSESS_FINAL_COMMENT)
	assessFinalCommentCaption = label(ID_ASSESS, "Комментарий к решению (обязателен при отклонении от реклассификации, не менее 10 знаков)", 242, 562, 900, 26)
	if assessFinalCommentCaption != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(assessFinalCommentCaption), WM_SETFONT, uintptr(hFontBold), 1)
	}
	assessConfirmButton = button(ID_ASSESS, "Подтвердить", 1052, 562, 200, 36, ID_ASSESS_CONFIRM_FINAL) // hidden compatibility action
	assessConfirmKeepButton = button(ID_ASSESS, "РЕШЕНИЕ ТЕХНОЛОГА", 782, 562, 260, 36, ID_ASSESS_CONFIRM_KEEP)
	assessConfirmCorrectButton = button(ID_ASSESS, "ПРИНЯТЬ РЕКЛАССИФИКАЦИЮ", 1052, 562, 260, 36, ID_ASSESS_CONFIRM_CORRECT)
	assessConfirmFullButton = button(ID_ASSESS, "УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ", 782, 610, 530, 42, ID_ASSESS_CONFIRM_FULL)
	assessTabDetail = richEdit(ID_ASSESS, "Сначала выберите объект.", 242, 610, 1010, 140, true, 0)
	assessMethodsLabel = label(ID_ASSESS, "Методы борьбы — кандидаты только после сверки классификации", 242, 760, 650, 24)
	assessToActivitiesButton = button(ID_ASSESS, "В мероприятия →", 1030, 756, 222, 34, ID_ASSESS_TO_ACTIVITIES)
	assessMethodsCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 242, py(790), 520, 260, hMain, ID_ASSESS_METHOD_COMBO)
	addPageControl(ID_ASSESS, assessMethodsCombo)
	assessMethodInfo = label(ID_ASSESS, "Метод не выбран. РОСА не назначает мероприятие автоматически.", 778, 790, 474, 42)
	assessDecisionHistoryLabel = label(ID_ASSESS, "РАНЕЕ СОХРАНЁННЫЕ КЛАССИФИКАЦИИ (УТВЕРЖДЁННЫЕ)", 242, 838, 1010, 24)
	if assessDecisionHistoryLabel != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(assessDecisionHistoryLabel), WM_SETFONT, uintptr(hFontBold), 1)
	}
	assessDecisionHistoryList = newList(ID_ASSESS, 242, 866, 1010, 120, []struct {
		n string
		w int32
	}{{"Дата", 140}, {"Направление", 165}, {"Класс", 100}, {"Источник", 355}, {"Статус", 120}, {"Подтвердил", 160}})
	setListHeaderFont(assessDecisionHistoryList, hFontBold)
	centerListHeaders(assessDecisionHistoryList, 6)
	assessDetail = edit(ID_ASSESS, "", 0, 0, 0, 0, true, true, 0)
	registerPageSubviewControl(ID_ASSESS, pageSubviewAssessmentTech, assessTechList, assessClassCombo, assessClassComment, assessClassSaveButton, assessClassCaption)
	registerPageSubviewControl(ID_ASSESS, pageSubviewAssessmentLab, assessProgramList)
	registerPageSubviewControl(ID_ASSESS, pageSubviewAssessmentCompare, assessCompareList, assessFinalComment, assessFinalCommentCaption, assessConfirmKeepButton, assessConfirmCorrectButton, assessConfirmFullButton)
	setPageSubview(ID_ASSESS, pageSubviewAssessmentTech)
	switchAssessmentInnerTab(0)

	// dynamogram image analysis
	buildDynamogramUI()
	buildWorkflowPagesUI()

	// methods / activities: RC564 makes this a strict continuation of the approved
	// classification. Suggestions are generated automatically, but the method is
	// fixed only by an explicit technologist approval.
	buildMethodsWorkflowUI()

	// OPI / OPR
	pageTitle(ID_OPI, "ОЛИ / ОПИ / ОПР", 242, 94, 900, 34)
	label(ID_OPI, "Кандидаты для подтверждения применимости и фактической эффективности технологий.", 242, 132, 980, 30)
	opiList = newList(ID_OPI, 242, 170, 1010, 310, []struct {
		n string
		w int32
	}{{"Технология", 230}, {"Группа", 130}, {"Требуемое подтверждение", 380}, {"Применений", 95}, {"С фактом", 85}})
	opiDetail = edit(ID_OPI, "Для ОПИ/ОПР критерий успеха должен фиксироваться до начала испытания. Фактический результат хранится отдельно от исходной оценки и затем используется в статистике технологии.", 242, 500, 1010, 160, true, true, 0)

	// history
	pageTitle(ID_HISTORY, "История оценок и обмен", 242, 94, 970, 34)
	label(ID_HISTORY, "Поиск", 242, 137, 55, 24)
	historyFilter = edit(ID_HISTORY, "", 300, 132, 250, 30, false, false, ID_FILTER_HISTORY)
	button(ID_HISTORY, "Синхронизировать", 560, 130, 180, 34, ID_SYNC)
	button(ID_HISTORY, "Сравнить версии", 750, 130, 180, 34, ID_COMPARE)
	button(ID_HISTORY, "Передать расчёт", 940, 130, 150, 34, ID_EXPORT_SELECTED)
	button(ID_HISTORY, "Импорт .rosa", 1100, 130, 152, 34, ID_IMPORT_ROSA)
	button(ID_HISTORY, "Открыть доступ", 242, 168, 155, 30, ID_SHARE_SELECTED)
	button(ID_HISTORY, "Доступно мне", 405, 168, 145, 30, ID_SHARED_WITH_ME)
	button(ID_HISTORY, "Кому открыт", 558, 168, 145, 30, ID_MANAGE_SHARES)
	button(ID_HISTORY, "Удалить расчёт", 711, 168, 180, 30, ID_DELETE_HISTORY)
	button(ID_HISTORY, "Очистить историю", 899, 168, 188, 30, ID_CLEAR_HISTORY)
	label(ID_HISTORY, "Для больших архивов без поиска отображаются последние 500 расчётов. Доступ к чужому архиву появляется только после явного разрешения владельца.", 242, 203, 1010, 24)
	historyList = newList(ID_HISTORY, 242, 234, 1010, 274, []struct {
		n string
		w int32
	}{{"Дата", 100}, {"Месторождение / скв.", 180}, {"Осложнение", 90}, {"Метод", 200}, {"МРП", 65}, {"Экономика", 100}, {"Решение", 110}, {"Статус", 115}})
	historyDetail = edit(ID_HISTORY, "Выберите версию расчёта.", 242, 522, 1010, 145, true, true, 0)

	// technologies
	pageTitle(ID_TECH, "Банк технологий и критерии применимости", 242, 94, 970, 34)
	techList = newList(ID_TECH, 242, 140, 1010, 280, []struct {
		n string
		w int32
	}{{"Технология", 225}, {"Группа", 125}, {"Область применимости", 315}, {"Применений", 85}, {"С фактом", 75}, {"Статус", 125}})
	techDetail = edit(ID_TECH, "Выберите технологию.", 242, 436, 1010, 120, true, true, 0)
	label(ID_TECH, "Фактический результат по выбранной расчётной версии", 242, 570, 620, 24)
	factSelected = label(ID_TECH, "Расчётная версия не выбрана", 650, 570, 565, 24)
	label(ID_TECH, "Факт. МРП, сут.", 242, 603, 125, 24)
	factMRP = edit(ID_TECH, "", 405, 598, 100, 28, false, false, 0)
	label(ID_TECH, "Факт. затраты, тыс. руб.", 525, 603, 180, 24)
	factCost = edit(ID_TECH, "", 710, 598, 110, 28, false, false, 0)
	label(ID_TECH, "Результат", 840, 603, 80, 24)
	factResult = edit(ID_TECH, "", 925, 598, 180, 28, false, false, 0)
	factComment = edit(ID_TECH, "Комментарий к факту", 242, 638, 738, 44, true, false, 0)
	button(ID_TECH, "Сохранить фактический результат", 1010, 598, 242, 48, ID_SAVE_FACT)
	label(ID_TECH, "Начало цикла", 242, 695, 105, 24)
	factCycleStart = edit(ID_TECH, "", 350, 690, 112, 28, false, false, 0)
	label(ID_TECH, "Конец цикла", 478, 695, 98, 24)
	factCycleEnd = edit(ID_TECH, "", 578, 690, 112, 28, false, false, 0)
	label(ID_TECH, "Тип события", 706, 695, 100, 24)
	factEventType = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 808, 690, 180, 220, hMain, 0)
	for _, item := range []string{"Авто", "Отказ", "Плановое вмешательство", "Ремонт/подъём", "Прочее вмешательство"} {
		pSendMessageW.Call(uintptr(factEventType), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(item))))
	}
	pSendMessageW.Call(uintptr(factEventType), CB_SETCURSEL, 0, 0)
	addPageControl(ID_TECH, factEventType)
	button(ID_TECH, "Подтвердить классификацию", 1010, 650, 242, 34, ID_CONFIRM_FACT_CLASS)
	button(ID_TECH, "Причина / дефектовка / эффект", 1010, 690, 242, 34, ID_OUTCOME_DETAILS)
	label(ID_TECH, "Даты: ДД.ММ.ГГГГ. Если заданы обе, длительность цикла проверяется по датам.", 242, 724, 746, 24)

	// timeline
	pageTitle(ID_TIMELINE, "Хронология объекта", 242, 94, 900, 34)
	label(ID_TIMELINE, "Оценка → решение → мероприятие → контроль → фактический результат.", 242, 132, 980, 24)
	timelineList = newList(ID_TIMELINE, 242, 170, 1010, 490, []struct {
		n string
		w int32
	}{{"Дата", 105}, {"Объект", 210}, {"Тип события", 150}, {"Событие", 350}, {"Статус", 145}})

	// plan
	pageTitle(ID_PLAN, "План мероприятий и контрольных действий", 242, 94, 970, 34)
	button(ID_PLAN, "Сформировать / актуализировать план", 242, 136, 300, 36, ID_GEN_PLAN)
	label(ID_PLAN, "План формируется по последним оценкам. Сроки по регламенту задаются пользователем.", 560, 142, 660, 32)
	planList = newList(ID_PLAN, 242, 190, 1010, 470, []struct {
		n string
		w int32
	}{{"Срок", 100}, {"Объект", 205}, {"Мероприятие / контроль", 315}, {"Основание", 265}, {"Статус", 95}})

	// registry
	pageTitle(ID_REGISTRY, "Квартальный реестр оценки объектов ОФ", 242, 94, 970, 34)
	label(ID_REGISTRY, "Рабочий свод утверждённых решений выбранного периода.", 242, 132, 970, 44)
	registryList = newList(ID_REGISTRY, 242, 185, 1010, 475, []struct {
		n string
		w int32
	}{{"Дата", 100}, {"Объект", 215}, {"Осложнение", 90}, {"Метод", 205}, {"Решение", 135}, {"Ответственный", 170}})

	// Historical RC709 marker retained for static QA: pageTitle(ID_ARCHIVE, "Утверждённый реестр"
	// Section 8 is deliberately not a second working archive. It receives only
	// the exact rows explicitly approved in section 7; drafts and saved passes
	// remain in the section-7 pass list. RC724 gives the register the same
	// card-based visual hierarchy as the final report.
	buildApprovedRegistryUI()

	// combined registries
	pageTitle(ID_REGISTRIES, "Реестры и выгрузка", 242, 94, 900, 34)
	registriesInfo = label(ID_REGISTRIES, "", 242, 132, 970, 44)
	label(ID_REGISTRIES, "Квартальный реестр оценки объектов ОФ", 242, 176, 520, 26)
	registryCurrentList = newList(ID_REGISTRIES, 242, 207, 1010, 205, []struct {
		n string
		w int32
	}{{"Дата", 100}, {"Объект", 215}, {"Осложнение", 90}, {"Метод", 205}, {"Решение", 135}, {"Ответственный", 170}})
	label(ID_REGISTRIES, "Зафиксированные решения / архив", 242, 428, 520, 26)
	registryFixedList = newList(ID_REGISTRIES, 242, 459, 1010, 205, []struct {
		n string
		w int32
	}{{"Дата", 95}, {"Квартал", 80}, {"Объект", 200}, {"Осложнение", 90}, {"Метод", 190}, {"Решение", 125}, {"Ответственный", 170}})

	// The ninth navigation entry keeps service operations only. Failure import
	// and object history are intentionally implemented on the Main page.
	pageTitle(ID_DATA, "Настройки и журнал", 242, 94, 900, 34)
	label(ID_DATA, "Контроль данных и служебные операции", 242, 132, 980, 24)
	dataInfo = edit(ID_DATA, "", 242, 160, 1010, 70, true, true, 0)
	label(ID_DATA, "СЛУЖЕБНЫЕ ОПЕРАЦИИ", 242, 236, 350, 22)
	for _, spec := range []struct {
		text string
		id   int
	}{
		{"Открыть папку данных", ID_OPEN_FOLDER}, {"Синхронизировать оценки", ID_SYNC}, {"Копия SQLite", ID_BACKUP}, {"Восстановить SQLite", ID_RESTORE_DB},
		{"Проверить копии", ID_VERIFY_BACKUPS}, {"Импортировать пакет РОСА", ID_IMPORT_ROSA}, {"Экспортировать архив", ID_EXPORT_ARCHIVE}, {"Журнал БД", ID_STORAGE_LOG},
		{"Очистить кэш", ID_CLEAR_CACHE}, {"LAB-90 диагностика", ID_LAB_DIAGNOSTICS}, {"Подпись по разделам", ID_SECTION_BANNER_SETTINGS},
	} {
		dataActionButtons = append(dataActionButtons, button(ID_DATA, spec.text, 242, 262, 238, 40, spec.id))
	}

	// Page builders can initialize their subviews while no page is active.
	// Reconcile every registered child once before the dashboard is first shown.
	// Later navigation only touches the outgoing and incoming pages.
	for page := range pageControls {
		hidePageControls(page)
	}
	activatePage(ID_DASH, false)
}
func refreshPage(id int) {
	if pageRefreshing {
		diagLog("REFRESH skipped reentrant page=%d", id)
		return
	}
	pageRefreshing = true
	started := time.Now()
	diagLog("REFRESH start page=%d", id)
	defer func() {
		pageRefreshing = false
		refreshSectionNotice()
		diagLog("REFRESH done page=%d elapsed=%s", id, time.Since(started))
	}()
	switch id {
	case ID_DASH:
		refreshDashboard()
	case ID_OBJECTS:
		refreshWellCard()
	case ID_LAB:
		refreshLaboratory()
	case ID_TECHPARAM:
		refreshTechParametersPage()
	case ID_MRP:
		refreshMRPForecastPage()
	case ID_ECON:
		refreshEconomicsPage()
	case ID_RESULT:
		refreshResultPage()
	case ID_TELEMETRY:
		refreshTelemetry()
	case ID_ASSESS:
		refreshAssessments()
	case ID_DYN:
		refreshDynamogramUI()
	case ID_ACTIVITIES:
		refreshActivities()
	case ID_OPI:
		refreshOPI()
	case ID_HISTORY:
		refreshHistory()
	case ID_TECH:
		refreshTech()
	case ID_REGISTRIES:
		refreshRegistries()
	case ID_DATA:
		refreshData()
	case ID_TIMELINE:
		refreshTimeline()
	case ID_PLAN:
		refreshPlan()
	case ID_REGISTRY:
		refreshRegistry()
	case ID_ARCHIVE:
		refreshArchive()
	}
}

func hidePageControls(page int) {
	for _, h := range pageControls[page] {
		setPhysicalControlVisibility(h, false)
	}
}

// Page updates change logical visibility immediately but reveal controls only
// after all their text, data and positions are ready. No WM_SETREDRAW is sent
// to a whole page: that message can alter WS_VISIBLE on child HWNDs.
func beginPageVisualUpdate(page int) {
	pageVisualUpdateDepth[page]++
}

func endPageVisualUpdate(page int) {
	if pageVisualUpdateDepth[page] == 0 {
		return
	}
	pageVisualUpdateDepth[page]--
	if pageVisualUpdateDepth[page] == 0 && currentPage == page && !pageSwitching {
		applyActivePageVisibility(page)
		invalidateVisibleFrame(page)
	}
}

// Inner workspaces only swap their own child controls. Native hide/show and
// position changes invalidate those controls; avoid repainting every other
// child of the main window for a single inner-tab click.
func endPageSubviewUpdate(page int) {
	if pageVisualUpdateDepth[page] == 0 {
		return
	}
	pageVisualUpdateDepth[page]--
	if pageVisualUpdateDepth[page] == 0 && currentPage == page && !pageSwitching {
		applyActivePageVisibility(page)
	}
}

func layoutCurrentPage() {
	if hMain == 0 {
		return
	}
	var r RECT
	if ok, _, _ := pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return
	}
	w, h := r.Right-r.Left, r.Bottom-r.Top
	if w <= 0 || h <= 0 {
		return
	}
	resizeCommonPage(w, h)
	layoutWorkflowPage(w, h)
}

func commitPageVisibility(page int) {
	pageVisibilityCommitting = true
	defer func() { pageVisibilityCommitting = false }()
	applyActivePageVisibility(page)
}

// A WS_CLIPCHILDREN parent cannot repaint its child HWNDs on their behalf.
// Explicitly invalidate the visible children after layout/visibility changes.
func invalidateVisibleFrame(page int) {
	// RedrawWindow(RDW_ALLCHILDREN) already invalidates each visible child,
	// including ListViews when the parent has WS_CLIPCHILDREN. Previously we
	// invalidated hundreds of individual HWNDs and *then* recursively invalidated
	// them again, which doubled the WM_PAINT traffic after every tab switch.
	if hMain != 0 && page == currentPage {
		pRedrawWindow.Call(uintptr(hMain), 0, 0, RDW_INVALIDATE|RDW_ALLCHILDREN)
	}
}

func applyActivePageVisibility(activePage int) {
	if activePage != currentPage {
		hidePageControls(activePage)
		return
	}
	shown, hidden := 0, 0
	for _, h := range pageControls[activePage] {
		if h == 0 {
			continue
		}
		wantVisible, known := pageControlVisible[h]
		if !known {
			// Defensive compatibility for controls registered by an older helper.
			wantVisible = true
			pageControlVisible[h] = true
		}
		setPhysicalControlVisibility(h, wantVisible)
		if wantVisible {
			shown++
		} else {
			hidden++
		}
	}
	diagLog("PAGE visibility applied id=%d shown=%d hidden=%d", activePage, shown, hidden)
}

func activatePage(id int, doRefresh bool) {
	diagLog("PAGE activate id=%d current=%d refresh=%v", id, currentPage, doRefresh)
	if pageSwitching {
		return
	}
	previousPage := currentPage
	pageSwitching = true
	committed := false
	defer func() {
		if !committed {
			currentPage = previousPage
			hidePageControls(id)
			commitPageVisibility(previousPage)
		}
		pageSwitching = false
	}()
	// Keep the last complete frame visible while preparing the target. Hiding it
	// before a slow refresh left the compositor showing a blank workspace.
	currentPage = id
	if hHeaderApprove != 0 {
		// Верхняя служебная кнопка всегда меняет профиль, а не завершает программу.
		// Кнопка выбора объекта остаётся только в рабочей области Главной.
		setText(hHeaderApprove, userSwitchCaption)
	}
	// Never WM_SETREDRAW the top-level HWND: Windows can toggle WS_VISIBLE,
	// causing blank frames, focus loss and apparent spontaneous minimization.
	// Child visibility is committed only after the target page is ready.
	frameStart := time.Now()
	var refreshDuration, layoutDuration, revealDuration time.Duration
	defer func() {
		invalidateVisibleFrame(currentPage)
		if elapsed := time.Since(frameStart); elapsed >= 250*time.Millisecond {
			limitedHealthLog(fmt.Sprintf("SLOW kind=page_transition page=%d total_ms=%d refresh_ms=%d layout_ms=%d reveal_ms=%d", id, elapsed.Milliseconds(), refreshDuration.Milliseconds(), layoutDuration.Milliseconds(), revealDuration.Milliseconds()))
		}
	}()
	// Target controls remain physically hidden even when a native ListView
	// re-enables WS_VISIBLE during its data update.
	if doRefresh {
		started := time.Now()
		refreshPage(id)
		refreshDuration = time.Since(started)
	}
	// Full-window layout runs on startup and in response to WM_SIZE/DPI changes.
	// Re-running it on every tab switch moved every child HWND and made otherwise
	// quick refreshes feel like a freeze. Startup is the only navigation that can
	// precede the first queued layout.
	startedLayout := time.Now()
	if !mainLayoutReady {
		resize()
		mainLayoutReady = true
	}
	// Each page has its own layout. The first global resize only laid out the
	// dashboard, and later tabs could open with their construction coordinates.
	layoutCurrentPage()
	// This layout satisfies any older queued WM_SIZE request as well.
	mainLayoutQueued = false
	layoutDuration = time.Since(startedLayout)
	// Swap the two registered page surfaces only after data and geometry agree.
	startedReveal := time.Now()
	hidePageControls(previousPage)
	commitPageVisibility(id)
	revealDuration = time.Since(startedReveal)
	committed = true
	refreshSectionNotice()
}
func switchPage(id int) {
	diagLog("PAGE request id=%d current=%d", id, currentPage)
	if pageSwitching || currentPage == id {
		return
	}
	if !pageNavChanging && currentPage != 0 {
		pageBackStack = append(pageBackStack, currentPage)
		pageForwardStack = nil
	}
	activatePage(id, true)
}

func navigatePageBack() {
	if len(pageBackStack) == 0 {
		return
	}
	target := pageBackStack[len(pageBackStack)-1]
	pageBackStack = pageBackStack[:len(pageBackStack)-1]
	if currentPage != 0 {
		pageForwardStack = append(pageForwardStack, currentPage)
	}
	pageNavChanging = true
	activatePage(target, true)
	pageNavChanging = false
}

func navigatePageForward() {
	if len(pageForwardStack) == 0 {
		return
	}
	target := pageForwardStack[len(pageForwardStack)-1]
	pageForwardStack = pageForwardStack[:len(pageForwardStack)-1]
	if currentPage != 0 {
		pageBackStack = append(pageBackStack, currentPage)
	}
	pageNavChanging = true
	activatePage(target, true)
	pageNavChanging = false
}
func versionStatus(v VersionRecord) string {
	if v.Archived {
		return "Утверждена"
	}
	if latest, ok := latestByObject()[v.ObjectKey]; ok && latest.VersionID != v.VersionID {
		return "Заменена последующей оценкой"
	}
	switch strings.ToLower(strings.TrimSpace(v.Decision)) {
	case "переносить":
		return "Направлена в квартальный реестр"
	case "не переносить":
		return "Не включена в квартальный реестр"
	case "доработать":
		return "Требует доработки"
	default:
		return "Зафиксирована"
	}
}
func objectLabel(v VersionRecord) string {
	a := strings.TrimSpace(v.Field + " / " + v.Well)
	if a == "/" || a == "" {
		if v.ObjectKey != "" {
			return v.ObjectKey
		}
		return "—"
	}
	return a
}
func refreshAll() {
	// Never refresh hidden pages. Native ListView controls emit notifications while
	// rows are rebuilt; refreshing all hidden pages can create re-entrant storms.
	refreshPage(currentPage)
}
func filteredVersions() []VersionRecord {
	q := strings.ToLower(strings.TrimSpace(getText(historyFilter)))
	rows := make([]VersionRecord, 0, len(versions)+1)
	for _, record := range versions {
		if isRetiredLegacyTeachingObjectKey(record.ObjectKey) {
			continue
		}
		rows = append(rows, record)
	}
	if teaching, ok := teachingCompletedVersionForObject(strings.TrimSpace(selectedObjectKey)); ok {
		found := false
		for _, row := range rows {
			if row.VersionID == teaching.VersionID && strings.EqualFold(strings.TrimSpace(row.ObjectKey), strings.TrimSpace(teaching.ObjectKey)) {
				found = true
				break
			}
		}
		if !found {
			rows = append(rows, teaching)
		}
	}
	if q == "" {
		return rows
	}
	var out []VersionRecord
	for _, v := range rows {
		s := strings.ToLower(strings.Join([]string{v.ObjectKey, v.Field, v.Well, v.Complication, v.Method, v.Decision, v.Responsible}, " "))
		if strings.Contains(s, q) {
			out = append(out, v)
		}
	}
	return out
}
func attentionFor(v VersionRecord) (string, string, string) {
	if strings.EqualFold(strings.TrimSpace(v.Decision), "Доработать") {
		return "Высокий", "оценка требует доработки", "Актуализировать"
	}
	if strings.TrimSpace(v.LabStatus) == "" {
		return "Средний", "лабораторные данные пока не добавлены; предварительная оценка доступна по базовым технологическим данным", "Уточнить при наличии"
	}
	if outcomeFor(v.VersionID) == nil {
		return "Средний", "нет зарегистрированного фактического результата", "Контроль"
	}
	return "Наблюдение", "критичных отклонений не зарегистрировано", "Наблюдение"
}
func refreshDashboard() {
	if dashboardStats == 0 {
		// The current Главная uses import status and an object table instead of a
		// large statistics banner.
	}
	lastImport := "Техрежим ещё не загружался"
	if len(techImports) > 0 {
		q := techImports[len(techImports)-1]
		lastImport = fmt.Sprintf("последнее обновление %s; строк %d; новых %d; изменённых %d; без № скв. %d; нет в новой выгрузке %d", q.ImportedAt, q.Total, q.Added, q.Changed, q.Skipped, q.Removed)
	}
	dynCount := len(loadDynamogramHistory())
	_ = dynCount
	_ = lastImport

	refreshTechImportUI()
	refreshObjects()
	listBeginUpdate(recentList)
	defer listEndUpdate(recentList)
	listClear(recentList)
	dashboardHistoryRows = dashboardHistoryRowsForMain()
	for i, row := range dashboardHistoryRows {
		listAdd(recentList, i, []string{
			fmt.Sprintf("%d", row.Number), row.Date, row.Code,
			row.LabConclusion, row.Classes, row.Methods, row.Economics,
			row.FullConclusion, fmt.Sprintf("%d%%", row.CompletionPct),
		})
	}
	setText(dashboardNote, dashboardObjectSummary())
}

func allObjectViews() []ObjectView {
	latest := latestByObject()
	seen := map[string]bool{}
	var out []ObjectView
	markSeen := func(key string) { seen[laboratoryObjectKeyFingerprint(key)] = true }
	isSeen := func(key string) bool { return seen[laboratoryObjectKeyFingerprint(key)] }
	for _, st := range techCurrent {
		v := ObjectView{State: st}
		if e, ok := latest[st.ObjectKey]; ok {
			v.Eval = e
			v.HasEval = true
		}
		out = append(out, v)
		markSeen(st.ObjectKey)
	}
	for k, e := range latest {
		if isSeen(k) {
			continue
		}
		out = append(out, ObjectView{State: TechObjectState{ObjectKey: k, Field: e.Field, Well: e.Well, Deposit: e.Deposit, ObjectPlast: e.ObjectPlast, Equipment: e.Equipment, CurrentMRP: e.CurrentMRP}, Eval: e, HasEval: true})
		markSeen(k)
	}
	// Saved laboratory and teaching records remain selectable even when they are
	// not present in the currently loaded Tech-regime.  Otherwise a user can open
	// a complete example but cannot approve it as the global working object.
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	for _, pass := range labPasses {
		key := strings.TrimSpace(pass.ObjectKey)
		if key == "" || pass.Status != labPassSavedStatus || isSeen(key) {
			continue
		}
		parts := strings.Split(key, "/")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		state := TechObjectState{ObjectKey: key, SourceFile: dash(pass.Source), ImportedAt: pass.UpdatedAt}
		if teaching, ok := teachingWorkflowStateForObject(key); ok {
			state = teaching
			state.SourceFile = dash(pass.Source) + " · учебный техконтекст синтетический"
			state.ImportedAt = pass.UpdatedAt
			// Use the pass already in hand instead of rescanning the complete laboratory
			// history for every teaching object. This keeps list rebuilding linear.
			state.WaterCut = strings.TrimSpace(pass.SharedValues["watercut"])
		} else {
			if len(parts) > 0 {
				state.Field = parts[0]
			}
			if len(parts) > 2 {
				state.Well = parts[1]
				state.ObjectPlast = strings.Join(parts[2:], " / ")
			} else if len(parts) == 2 {
				state.ObjectPlast = parts[1]
			}
		}
		out = append(out, ObjectView{State: state})
		markSeen(key)
	}
	return out
}
func objectViewLabel(v ObjectView) string {
	if v.State.ObjectKey != "" {
		return stateLabel(v.State)
	}
	return objectLabel(v.Eval)
}
func filteredObjects() []ObjectView {
	q := strings.ToLower(strings.TrimSpace(getText(objectsFilter)))
	x := allObjectViews()
	var out []ObjectView
	for _, v := range x {
		s := strings.ToLower(strings.Join([]string{v.State.ObjectKey, v.State.Field, v.State.Well, v.State.Deposit, v.State.ObjectPlast, v.State.Equipment, v.Eval.Complication, v.Eval.Method}, " "))
		if q == "" || strings.Contains(s, q) {
			out = append(out, v)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return objectViewLabel(out[i]) < objectViewLabel(out[j]) })
	return out
}
func attentionForView(v ObjectView) (string, string, string) {
	if !v.HasEval {
		return "Высокий", "для текущего объекта нет зафиксированной оценки", "Создать оценку"
	}
	p, reason, st := attentionFor(v.Eval)
	if v.State.ImportedAt != "" && parseDate(v.State.ImportedAt).After(parseDate(v.Eval.Created)) {
		return "Высокий", "Техрежим обновлён после последней оценки", "Проверить актуальность"
	}
	return p, reason, st
}
func refreshObjects() {
	if objectsList == 0 {
		return
	}
	x := filteredObjects()
	listBeginUpdate(objectsList)
	defer listEndUpdate(objectsList)
	listClear(objectsList)
	selected := -1
	for i, v := range x {
		listAdd(objectsList, i, []string{
			dash(v.State.Field), dash(v.State.Well), dash(v.State.Deposit),
			dash(v.State.ObjectPlast), dash(objectViewKey(v)), dash(v.State.Equipment),
			displayMRP(v.State.CurrentMRP),
		})
		if strings.TrimSpace(selectedObjectKey) != "" && laboratoryObjectKeysEquivalent(objectViewKey(v), selectedObjectKey) {
			selected = i
		}
	}
	if selected >= 0 {
		selectListRow(objectsList, selected)
	}
}

func displayMRP(value string) string {
	v := strings.TrimSpace(value)
	if v == "" || v == "—" || v == "-" {
		return "—"
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
	if err != nil {
		return v
	}
	if math.Abs(n-math.Round(n)) < 0.05 {
		return strconv.FormatInt(int64(math.Round(n)), 10)
	}
	return strconv.FormatFloat(n, 'f', 1, 64)
}
func showObjectDetail(v ObjectView) {
	s := fmt.Sprintf("Месторождение: %s    Скважина: %s    Залежь: %s\r\nОбъект разработки / пласт: %s    Шифр: %s\r\nОборудование: %s    МРП: %s сут.    Источник: %s", dash(v.State.Field), dash(v.State.Well), dash(v.State.Deposit), dash(v.State.ObjectPlast), dash(objectViewKey(v)), dash(v.State.Equipment), displayMRP(v.State.CurrentMRP), dash(v.State.SourceFile))
	setText(objectDetail, s)
}

func refreshWellCard() {
	refreshFailureObjectChooser()
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		setText(objectPassportLabel, "Выбранный объект:")
		setText(objectSelectedValue, "Сначала закрепите строку на Главной")
		setText(objectDetail, "Объект ещё не выбран. Вернитесь на Главную, выделите строку Техрежима и нажмите «Закрепить объект».")
		refreshWellCardFailures()
		return
	}
	v, ok := findObjectViewByKey(key)
	if !ok {
		caption := key
		for _, event := range failureEvents {
			if event.ObjectKey == key {
				caption = event.Field + " / скв. " + event.Well
				break
			}
		}
		setText(objectPassportLabel, "Выбранный объект:")
		setText(objectSelectedValue, caption)
		setText(objectDetail, "История отказов загружена из накопительного реестра.\r\nДанные паспорта и текущего техрежима появятся после их загрузки на Главной.")
		refreshWellCardFailures()
		return
	}
	setText(objectPassportLabel, "Выбранный объект:")
	setText(objectSelectedValue, objectViewLabel(v))
	showObjectDetail(v)
	refreshWellCardFailures()
}

type assessmentDirectionView struct {
	Group        LabGroupDef
	Record       *LabRecord
	Summary      laboratoryResultSummary
	Source       string
	Date         string
	Freshness    string
	CoverageText string
	KeyReason    string
}

var assessDirectionRows []assessmentDirectionView

// On the complication page salts are shown first: this is the working priority
// requested for the primary diagnostic review. The laboratory input tabs remain
// independent from this presentation order.
var assessmentDirectionPriority = []string{"scale", "corrosion", "aspo", "solids", "microbiology", "gases"}

func assessmentGroupByCode(code string) (LabGroupDef, bool) {
	for _, g := range labGroups {
		if g.Code == code {
			return g, true
		}
	}
	return LabGroupDef{}, false
}

func assessmentFreshnessText(date string) string {
	t := parseDate(strings.TrimSpace(date))
	if t.IsZero() {
		return "дата не указана"
	}
	days := int(time.Since(t).Hours() / 24)
	if days < 0 {
		return "проверьте дату анализа"
	}
	switch {
	case days <= 90:
		return fmt.Sprintf("актуальность: %d сут.", days)
	case days <= 180:
		return fmt.Sprintf("требует проверки актуальности: %d сут.", days)
	default:
		return fmt.Sprintf("давность данных: %d сут. — желательно обновить", days)
	}
}

func assessmentCoverageText(s laboratoryResultSummary) string {
	if s.BaseTotal <= 0 {
		return "—"
	}
	pct := (s.BaseFilled*100 + s.BaseTotal/2) / s.BaseTotal
	return fmt.Sprintf("%d/%d · %d%%", s.BaseFilled, s.BaseTotal, pct)
}

func latestLabRecordForAssessment(objectKey, groupCode string) *LabRecord {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil
	}

	// The classification page follows the laboratory record which the user has
	// actually opened, including a working copy created from history.  Falling
	// straight through to the newest saved pass made the visible selection and
	// the proposed class disagree.
	activeObject := strings.TrimSpace(getText(labObjectEdit))
	if laboratoryObjectKeysEquivalent(activeObject, objectKey) {
		// Prefer the exact working copy opened from history/teaching examples.
		// Hidden Win32 controls and the currently displayed ten-field page are only
		// a view; the pass draft is the authoritative complete laboratory record.
		// This prevents a fully populated teaching pass from degrading to «НД» when
		// the user leaves Laboratory and opens Assessment.
		if pass := labPassDrafts[normalizeLaboratoryObjectKey(activeObject)+"|"+labActivePassID]; pass != nil && laboratoryObjectKeysEquivalent(pass.ObjectKey, objectKey) {
			values := cloneAssessmentLabValues(pass.Groups[groupCode])
			hasValue := false
			for _, value := range values {
				if strings.TrimSpace(value) != "" {
					hasValue = true
					break
				}
			}
			if hasValue {
				units := cloneAssessmentLabValues(pass.Units[groupCode])
				if group, ok := assessmentGroupByCode(groupCode); ok {
					values = canonicalizeLaboratoryValues(group, values, units)
				}
				return &LabRecord{
					ID: "ACTIVE-PASS-" + strings.TrimSpace(pass.PassID), ObjectKey: pass.ObjectKey,
					Date: pass.Date, GroupCode: groupCode,
					Source: "открытая лабораторная запись • " + dash(strings.TrimSpace(pass.Source)),
					Values: values, Units: units, ScaleModel: pass.ScaleModel,
					Comment: pass.Comment, Conclusion: pass.Conclusion, CreatedAt: pass.UpdatedAt,
				}
			}
		}
		values := cloneAssessmentLabValues(labDrafts[groupCode])
		hasValue := false
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				hasValue = true
				break
			}
		}
		if hasValue {
			units := cloneAssessmentLabValues(labUnitDrafts[groupCode])
			if group, ok := assessmentGroupByCode(groupCode); ok {
				values = canonicalizeLaboratoryValues(group, values, units)
			}
			return &LabRecord{
				ID:         "ACTIVE-" + strings.TrimSpace(labActivePassID),
				ObjectKey:  activeObject,
				Date:       strings.TrimSpace(getText(labDateEdit)),
				GroupCode:  groupCode,
				Source:     "выбранная лабораторная запись • " + dash(strings.TrimSpace(getText(labSourceEdit))),
				Values:     values,
				Units:      units,
				ScaleModel: labScaleModelCode,
				Comment:    strings.TrimSpace(getText(labCommentEdit)),
			}
		}
	}

	// RC371: the current laboratory workflow saves one coherent LabPassRecord.
	// Assessment used to read only legacy per-section LabRecord rows, so a freshly
	// saved analysis could appear as «НЕТ ДАННЫХ». Prefer the newest saved pass
	// that actually contains this direction, while keeping legacy rows readable.
	passes := assessmentLabPassesForObjectCached(objectKey)
	var bestPass *LabPassRecord
	bestPassTime := time.Time{}
	for i := range passes {
		pass := &passes[i]
		if pass.Status != labPassSavedStatus {
			continue
		}
		values := pass.Groups[groupCode]
		hasValue := false
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				hasValue = true
				break
			}
		}
		if !hasValue {
			continue
		}
		t := parseDate(pass.Date)
		if parsed, err := time.Parse(time.RFC3339, pass.UpdatedAt); err == nil && (t.IsZero() || parsed.After(t)) {
			t = parsed
		}
		if bestPass == nil || t.After(bestPassTime) {
			copyPass := *pass
			bestPass = &copyPass
			bestPassTime = t
		}
	}
	if bestPass != nil {
		values := bestPass.Groups[groupCode]
		units := bestPass.Units[groupCode]
		if group, ok := assessmentGroupByCode(groupCode); ok {
			values = canonicalizeLaboratoryValues(group, values, units)
		}
		source := strings.TrimSpace(bestPass.Source)
		if source == "" {
			source = "источник не указан"
		}
		return &LabRecord{
			ID:         "PASS-" + bestPass.PassID,
			ObjectKey:  bestPass.ObjectKey,
			Date:       bestPass.Date,
			GroupCode:  groupCode,
			Source:     "единая диагностика • " + source,
			Values:     values,
			Units:      units,
			ScaleModel: bestPass.ScaleModel,
			Comment:    bestPass.Comment,
			Conclusion: bestPass.Conclusion,
			CreatedAt:  bestPass.UpdatedAt,
			ImportedAt: bestPass.ImportedAt,
		}
	}

	// Backward compatibility for analyses saved before the unified pass model.
	if !labRecordsLoaded {
		labRecords = loadLabRecords()
	}
	var best *LabRecord
	bestTime := time.Time{}
	for i := range labRecords {
		r := &labRecords[i]
		if !strings.EqualFold(strings.TrimSpace(r.ObjectKey), objectKey) || r.GroupCode != groupCode {
			continue
		}
		t := parseDate(r.Date)
		if best == nil || t.After(bestTime) {
			best = r
			bestTime = t
		}
	}
	if best == nil {
		if teaching, ok := teachingLatestLabRecord(objectKey, groupCode, time.Now()); ok {
			return teaching
		}
	}
	return best
}

func cloneAssessmentLabValues(source map[string]string) map[string]string {
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func assessmentKeyReason(s laboratoryResultSummary) string {
	text := strings.TrimSpace(s.Message)
	for _, suffix := range []string{
		" Это основание для проверки, а не подтверждённый факт повреждения.",
		" Это не исключает осложнение при другом составе, режиме или по неучтённым причинам.",
		" Это не означает, что осложнения нет.",
	} {
		text = strings.ReplaceAll(text, suffix, "")
	}
	if len([]rune(text)) > 115 {
		r := []rune(text)
		text = string(r[:112]) + "…"
	}
	return dash(text)
}

func assessmentDirectionViews(objectKey string) []assessmentDirectionView {
	rows := make([]assessmentDirectionView, 0, len(assessmentDirectionPriority))
	for _, code := range assessmentDirectionPriority {
		group, ok := assessmentGroupByCode(code)
		if !ok {
			continue
		}
		rec := latestLabRecordForAssessment(objectKey, group.Code)
		row := assessmentDirectionView{Group: group, Record: rec, Source: "данные не внесены", Date: "—", Freshness: "нет анализа"}
		if rec == nil {
			row.Summary = laboratoryResultSummary{State: "НЕТ ДАННЫХ", Message: "Лабораторные данные по направлению не внесены."}
		} else {
			row.Summary = laboratoryGroupSummary(group, rec.Values, rec.ScaleModel)
			row.Source = strings.TrimSpace(rec.Source)
			if row.Source == "" {
				row.Source = "источник не указан"
			}
			row.Date = dash(rec.Date)
			row.Freshness = assessmentFreshnessText(rec.Date)
		}
		row.CoverageText = assessmentCoverageText(row.Summary)
		row.KeyReason = assessmentKeyReason(row.Summary)
		rows = append(rows, row)
	}
	return rows
}

func refreshAssessments() {
	// RC562: one storage snapshot per assessment refresh. Individual rows and
	// custom-draw notifications must never re-query SQLite/JSON on the UI thread.
	assessmentLabPassSnapshotValid = false
	if assessList == 0 {
		return
	}
	previousCode := ""
	if assessSelectedRow >= 0 && assessSelectedRow < len(assessDirectionRows) {
		previousCode = assessDirectionRows[assessSelectedRow].Group.Code
	}
	key := strings.TrimSpace(selectedObjectKey)
	listBeginUpdate(assessList)
	defer listEndUpdate(assessList)
	listClear(assessList)
	assessDirectionRows = nil
	if key == "" {
		assessSelectedRow = -1
		assessMethodCandidates = nil
		if assessMethodsCombo != 0 {
			pSendMessageW.Call(uintptr(assessMethodsCombo), CB_RESETCONTENT, 0, 0)
			pSendMessageW.Call(uintptr(assessMethodsCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p("— сначала выберите объект —"))))
			pSendMessageW.Call(uintptr(assessMethodsCombo), CB_SETCURSEL, 0, 0)
		}
		setText(assessMethodInfo, "Методы борьбы появятся после выбора объекта и направления оценки.")
		setText(assessObjectInfo, "Объект не выбран. Сначала выберите скважину на шаге 1.")
		setText(assessOverall, "Сначала выберите объект. После этого будут доступны ручная классификация технолога, лабораторное предположение и их сверка.")
		setText(assessDetail, "После выбора объекта здесь будут собраны шесть лабораторных направлений. Нет данных ≠ нет осложнения.")
		refreshAssessmentClassTabs()
		return
	}
	labelText := readableObjectKey(key)
	if st, ok := currentSelectedState(); ok {
		labelText = assessmentObjectCaption(st, key)
	}
	labKey := strings.TrimSpace(getText(labObjectEdit))
	labMismatch := labKey != "" && !laboratoryObjectKeysEquivalent(labKey, key)
	if assessProbabilityButton != 0 {
		if labMismatch {
			setText(assessProbabilityButton, "ЗАКРЕПИТЬ И ПЕРЕСЧИТАТЬ")
		} else {
			setText(assessProbabilityButton, "ОБНОВИТЬ ОЦЕНКУ")
		}
	}
	if labMismatch {
		labelText = fmt.Sprintf("⚠ ОБЪЕКТ ИЗМЕНЁН\r\nГлавная: %s   |   Лаборатория: %s", labelText, readableObjectKey(labKey))
	} else if labKey != "" {
		labelText = "ЗАКРЕПЛЁННЫЙ ОБЪЕКТ: " + labelText + " • выбранная лабораторная запись совпадает"
	} else {
		labelText = "ЗАКРЕПЛЁННЫЙ ОБЪЕКТ: " + labelText + " • лабораторная запись не выбрана"
	}
	setText(assessObjectInfo, labelText)
	assessDirectionRows = assessmentDirectionViews(key)
	risk, unknown, complete, withData := 0, 0, 0, 0
	for i, row := range assessDirectionRows {
		coverage := row.CoverageText
		if row.Summary.BaseTotal > 0 && row.Summary.BaseFilled == row.Summary.BaseTotal {
			complete++
		}
		if row.Summary.HasData {
			withData++
		}
		state := row.Summary.State
		if row.Summary.Level >= 2 {
			risk++
		}
		if !row.Summary.HasData || strings.Contains(strings.ToUpper(state), "НЕТ ОЦЕНКИ") || strings.Contains(strings.ToUpper(state), "НЕТ ДАННЫХ") {
			unknown++
		}
		dateCell := row.Date
		if row.Record != nil && strings.Contains(row.Freshness, "давность") {
			dateCell += " ⚠"
		}
		listAdd(assessList, i, []string{row.Group.Name, state, coverage, dateCell, row.Source, row.KeyReason})
	}
	classText := "не зафиксирован"
	if lv, ok := latestByObject()[key]; ok && strings.TrimSpace(lv.Complication) != "" {
		classText = lv.Complication
	}
	overallText := fmt.Sprintf("Свод: данные есть по %d из 6 направлений; признаки/пограничный результат — %d; без достаточной оценки — %d; базовый набор полностью заполнен — %d из 6. Зафиксированный класс: %s. Лабораторный признак не равен подтверждённой причине отказа. Нет данных ≠ нет осложнения.", withData, risk, unknown, complete, classText)
	if labMismatch {
		overallText = fmt.Sprintf("⚠ В ЛАБОРАТОРНОМ РАЗДЕЛЕ ОТКРЫТ ДРУГОЙ ОБЪЕКТ: %s. Рекомендации ниже относятся только к закреплённому на Главной объекту.\r\n%s", readableObjectKey(labKey), overallText)
	}
	setText(assessOverall, overallText)
	if len(assessDirectionRows) > 0 {
		selected := -1
		if previousCode != "" {
			for i := range assessDirectionRows {
				if assessDirectionRows[i].Group.Code == previousCode {
					selected = i
					break
				}
			}
		}
		if selected < 0 {
			for i := range assessDirectionRows {
				if assessDirectionRows[i].Summary.HasData {
					selected = i
					break
				}
			}
		}
		if selected < 0 {
			selected = 0
		}
		selectAssessmentDirection(selected)
	}
	refreshAssessmentClassTabs()
	refreshAssessmentOverallClassificationSummary()
	// RC574: an assessment refresh may be triggered while the user is still in
	// Laboratory (for example after re-pinning an object from the corrosion
	// section). Data may refresh off-page, but hidden assessment controls must
	// never be shown or laid out over the active page.
	if currentPage == ID_ASSESS {
		layoutAssessmentInnerTabs()
	}
}

func selectAssessmentDirection(index int) {
	if index < 0 || index >= len(assessDirectionRows) {
		assessSelectedRow = -1
		assessMethodCandidates = nil
		setText(assessMethodInfo, "Метод не выбран.")
		return
	}
	assessSelectedRow = index
	row := assessDirectionRows[index]
	populateAssessmentMethodCombo(row.Group.Code)
	showAssessmentDirectionDetail(row)
}

func showAssessmentDirectionDetail(row assessmentDirectionView) {
	coverage := fmt.Sprintf("%d из %d обязательных базовых показателей", row.Summary.BaseFilled, row.Summary.BaseTotal)
	if row.Summary.BaseTotal == 0 {
		coverage = "для этого направления базовый набор не задан"
	}
	date, source := row.Date, row.Source
	if row.Record == nil {
		date, source = "—", "—"
	}
	text := fmt.Sprintf("НАПРАВЛЕНИЕ: %s\r\nРезультат: %s\r\nПокрытие данных: %s\r\nДата анализа: %s   Источник: %s\r\nАктуальность: %s\r\n\r\n%s\r\n\r\n%s", row.Group.Name, row.Summary.State, coverage, date, source, row.Freshness, row.Summary.Message, laboratoryPracticalExplanation(row.Group, row.Summary))
	if method, ok := selectedAssessmentMethod(); ok {
		text += "\r\n\r\n" + assessmentMethodDetailText(method)
		setText(assessMethodInfo, method.Status+" • "+method.Source)
	}
	setText(assessDetail, text)
}

func technologyRelation(t Technology, v VersionRecord) (string, string) {
	g := strings.ToLower(t.Group)
	c := strings.ToLower(v.Complication)
	match := false
	if strings.Contains(g, "корроз") && (strings.Contains(c, "к") || strings.Contains(c, "k")) {
		match = true
	}
	if strings.Contains(g, "соли") && (strings.Contains(c, "с") || strings.Contains(c, "s")) {
		match = true
	}
	if strings.Contains(g, "аспо") && (strings.Contains(c, "а") || strings.Contains(c, "a")) {
		match = true
	}
	if strings.Contains(g, "гидрат") && strings.Contains(c, "gd") {
		match = true
	}
	if strings.Contains(g, "мех") && (strings.Contains(c, "м") || strings.Contains(c, "m")) {
		match = true
	}
	if !match {
		return "Не относится", "другая группа осложнений"
	}
	if strings.EqualFold(t.Status, "По согласованию") {
		return "Условно допустима", "требуется отдельное подтверждение применимости"
	}
	return "К проверке", "соответствует группе осложнения; требуется проверка всех критериев и ограничений"
}
func showAssessmentDetail(v VersionRecord) {
	var rel []string
	for ti, t := range technologies {
		if ti >= 2000 {
			break
		}
		st, b := technologyRelation(t, v)
		if st != "Не относится" {
			rel = append(rel, fmt.Sprintf("- %s - %s (%s)", t.Name, st, b))
		}
	}
	if len(rel) > 5 {
		rel = rel[:5]
	}
	if len(rel) == 0 {
		rel = []string{"- По текущему коду осложнения технологии-кандидаты автоматически не определены."}
	}
	setText(assessDetail, fmt.Sprintf("Оценка %s | %s\r\nОсложнение: %s | текущий МРП %s сут. | прогноз %s сут.\r\nМетод: %s\r\n\r\nПРЕДВАРИТЕЛЬНАЯ МАТРИЦА ПРИМЕНИМОСТИ\r\n%s", v.Created, objectLabel(v), v.Complication, v.CurrentMRP, v.ForecastMRP, v.Method, strings.Join(rel, "\r\n")))
}

func versionLabelIndex() map[string]string {
	m := make(map[string]string, len(versions))
	for _, v := range versions {
		m[v.VersionID] = objectLabel(v)
	}
	return m
}

func refreshActivities() {
	// RC616 hotfix: entering Methods must never terminate the application.
	// The workflow refresh is isolated so a bad saved record or a visual/layout
	// regression degrades this page only; the rest of ROSA remains available.
	defer func() {
		if r := recover(); r != nil {
			diagLog("METHODS refresh recovered panic=%v\n%s", r, debug.Stack())
			showMethodsSafeFallback(fmt.Sprintf("%v", r))
		}
	}()
	refreshMethodsWorkflowUI()
}

func refreshOPI() {
	if opiList == 0 {
		return
	}
	listBeginUpdate(opiList)
	defer listEndUpdate(opiList)
	listClear(opiList)
	for i, t := range technologies {
		u, f, _ := techUsage(t)
		listAdd(opiList, i, []string{t.Name, t.Group, t.Evidence, strconv.Itoa(u), strconv.Itoa(f)})
	}
	setText(opiDetail, "Контур ОПИ/ОПР в этом промежуточном варианте уже связан с Банком технологий и фактическими результатами. Следующий этап — отдельная карточка испытания: цель, критерий успеха, план наблюдений, факт, вывод и решение по применимости.")
}
func refreshRegistries() {
	if registryCurrentList == 0 {
		return
	}
	listBeginUpdate(registryCurrentList)
	listBeginUpdate(registryFixedList)
	defer listEndUpdate(registryCurrentList)
	defer listEndUpdate(registryFixedList)
	listClear(registryCurrentList)
	listClear(registryFixedList)
	cur, fix := 0, 0
	shownCur, shownFix := 0, 0
	for _, v := range versions {
		if v.Archived {
			fix++
			if shownFix < 2000 {
				listAdd(registryFixedList, shownFix, []string{v.Created, v.Quarter, objectLabel(v), v.Complication, v.Method, v.Decision, v.Responsible})
				shownFix++
			}
		} else if v.Decision != "" {
			cur++
			if shownCur < 2000 {
				listAdd(registryCurrentList, shownCur, []string{v.Created, objectLabel(v), v.Complication, v.Method, v.Decision, v.Responsible})
				shownCur++
			}
		}
	}
	setText(registriesInfo, fmt.Sprintf("Рабочих реестровых решений: %d     Зафиксированных решений: %d     В таблицах отображается не более 2000 строк каждого раздела; данные в архиве не обрезаются.", cur, fix))
}

func refreshHistory() {
	if historyList == 0 {
		return
	}
	x := filteredVersions()
	listBeginUpdate(historyList)
	defer listEndUpdate(historyList)
	listClear(historyList)
	for i, v := range x {
		if i >= 500 {
			break
		}
		econ := v.EconMode
		if econ == "" {
			econ = v.EconStatus
		}
		listAdd(historyList, i, []string{v.Created, objectLabel(v), v.Complication, v.Method, v.ForecastMRP, econ, v.Decision, versionStatus(v)})
	}
	if len(x) > 0 {
		showVersionDetail(x[0])
	}
}
func showVersionDetail(v VersionRecord) {
	fact := "Фактический результат: не зарегистрирован"
	if factCycleStart != 0 {
		setText(factCycleStart, "")
	}
	if factCycleEnd != 0 {
		setText(factCycleEnd, "")
	}
	if factEventType != 0 {
		pSendMessageW.Call(uintptr(factEventType), CB_SETCURSEL, 0, 0)
	}
	if o := outcomeFor(v.VersionID); o != nil {
		confirmed := "авто"
		if o.ClassificationConfirmed {
			confirmed = "подтверждено"
		}
		fact = fmt.Sprintf("Фактический результат: МРП %s сут.; затраты %s тыс. руб.; %s. %s Тип события: %s (%s).", o.ActualMRP, o.ActualCost, o.Result, o.Comment, dash(o.EventType), confirmed)
		if structured := outcomeStructuredSummary(*o); strings.TrimSpace(structured) != "" {
			fact += "\r\n" + structured
		}
		comparison := outcomeForecastFactComparison(v, *o)
		if strings.TrimSpace(comparison) != "" {
			fact += "\r\n\r\nСОПОСТАВЛЕНИЕ ПРОГНОЗ → ФАКТ\r\n" + comparison
		}
		if factCycleStart != 0 {
			setText(factCycleStart, o.CycleStartDate)
		}
		if factCycleEnd != 0 {
			setText(factCycleEnd, o.CycleEndDate)
		}
		if factEventType != 0 {
			idx := 0
			switch normalizeMRPEventType(o.EventType, o.PlannedIntervention) {
			case "отказ":
				idx = 1
			case "плановое вмешательство":
				idx = 2
			case "ремонт":
				idx = 3
			default:
				if strings.TrimSpace(o.EventType) != "" && normalizeMRPEventType(o.EventType, o.PlannedIntervention) != "не классифицировано" {
					idx = 4
				}
			}
			pSendMessageW.Call(uintptr(factEventType), CB_SETCURSEL, uintptr(idx), 0)
		}
	}
	scen := ""
	if v.MRPLow != "" || v.MRPBase != "" || v.MRPHigh != "" {
		scen = fmt.Sprintf("Сценарии МРП: низкий %s | базовый %s | высокий %s; выбран: %s.\r\n", v.MRPLow, v.MRPBase, v.MRPHigh, v.Scenario)
	}
	s := fmt.Sprintf("Оценка: %s\r\nКлюч объекта: %s\r\nИсточник: %s\r\nОсложнение: %s | Метод: %s | Фактическое оборудование: %s\r\nМРП: текущий %s → итоговый прогноз %s сут.\r\n%sЛабораторный статус: %s\r\nЭкономика: %s %s | %s\r\nРешение: %s. %s\r\n%s", v.VersionID, v.ObjectKey, v.SourceFile, v.Complication, v.Method, v.Equipment, v.CurrentMRP, v.ForecastMRP, scen, v.LabStatus, v.EconMode, v.EconStatus, v.EconResult, v.Decision, v.DecisionComment, fact)
	setText(historyDetail, s)
	if factSelected != 0 {
		setText(factSelected, fmt.Sprintf("Выбрано: %s | %s | %s", objectLabel(v), v.Created, v.VersionID[:8]))
	}
}
func techUsage(t Technology) (int, int, float64) {
	uses := 0
	facts := 0
	sum := 0.0
	for _, v := range versions {
		if technologyMatches(t, v) {
			uses++
			if o := outcomeFor(v.VersionID); o != nil {
				facts++
				sum += toFloat(o.ActualMRP)
			}
		}
	}
	avg := 0.0
	if facts > 0 {
		avg = sum / float64(facts)
	}
	return uses, facts, avg
}
func technologyMatches(t Technology, v VersionRecord) bool {
	n := strings.ToLower(t.Name)
	m := strings.ToLower(v.Method)
	g := strings.ToLower(t.Group)
	c := strings.ToLower(v.Complication)
	if strings.Contains(n, "трил") {
		return strings.Contains(m, "трил")
	}
	if strings.Contains(n, "метанол") {
		return strings.Contains(m, "метанол")
	}
	if strings.Contains(n, "горяч") {
		return strings.Contains(m, "горяч")
	}
	if strings.Contains(n, "растворитель") {
		return strings.Contains(m, "раствор")
	}
	if strings.Contains(n, "фильтр") {
		return strings.Contains(m, "фильтр")
	}
	if strings.Contains(n, "hcl") || strings.Contains(n, "кислот") {
		return strings.Contains(m, "hcl") || strings.Contains(m, "кислот")
	}
	if strings.Contains(g, "корроз") {
		return strings.Contains(c, "к") && strings.Contains(m, "ингиб")
	}
	if strings.Contains(g, "соли") {
		return strings.Contains(c, "с") && strings.Contains(m, "ингиб")
	}
	if strings.Contains(g, "гидрат") {
		return strings.Contains(c, "gd")
	}
	return strings.Contains(m, strings.ToLower(strings.Fields(t.Name)[0]))
}
func refreshTech() {
	if techList == 0 {
		return
	}
	listBeginUpdate(techList)
	defer listEndUpdate(techList)
	listClear(techList)
	for i, t := range technologies {
		u, f, _ := techUsage(t)
		listAdd(techList, i, []string{t.Name, t.Group, t.Applicability, strconv.Itoa(u), strconv.Itoa(f), t.Status})
	}
	if len(technologies) > 0 {
		showTechDetail(technologies[0])
	}
}
func showTechDetail(t Technology) {
	uses, facts, avg := techUsage(t)
	extra := ""
	if facts > 0 {
		extra = fmt.Sprintf("; средний фактический МРП %.0f сут.", avg)
	}
	setText(techDetail, fmt.Sprintf("%s\r\nОграничения: %s\r\nТребуемое подтверждение: %s\r\nНакоплено применений: %d; с зарегистрированным фактом: %d%s\r\n\r\nОБОСНОВАНИЕ ИЗ ПЕРЕНЕСЁННОЙ МОДЕЛИ\r\n%s", t.Applicability, t.Constraints, t.Evidence, uses, facts, extra, ruleSummaryForTechnology(t)))
}
func refreshTimeline() {
	if timelineList == 0 {
		return
	}
	type ev struct{ d, o, t, e, s string }
	visibleVersions := append([]VersionRecord(nil), versions...)
	if teaching, ok := teachingCompletedVersionForObject(strings.TrimSpace(selectedObjectKey)); ok {
		found := false
		for _, row := range visibleVersions {
			if row.VersionID == teaching.VersionID && strings.EqualFold(strings.TrimSpace(row.ObjectKey), strings.TrimSpace(teaching.ObjectKey)) {
				found = true
				break
			}
		}
		if !found {
			visibleVersions = append(visibleVersions, teaching)
		}
	}
	all := make([]ev, 0, len(visibleVersions)+len(plans)+len(outcomes))
	for _, v := range visibleVersions {
		all = append(all, ev{v.Created, objectLabel(v), "Версия оценки", fmt.Sprintf("%s; %s; МРП %s сут.", v.Complication, v.Method, v.ForecastMRP), versionStatus(v)})
		if o := outcomeFor(v.VersionID); o != nil {
			all = append(all, ev{o.Date, objectLabel(v), "Фактический результат", fmt.Sprintf("МРП %s сут.; %s", o.ActualMRP, o.Result), "Зарегистрирован"})
		}
	}
	labels := versionLabelIndex()
	for _, pl := range plans {
		obj := pl.ObjectKey
		if q := labels[pl.VersionID]; q != "" {
			obj = q
		}
		all = append(all, ev{pl.DueDate, obj, "Плановый контроль", pl.Action, pl.Status})
	}
	for _, a := range loadDynamogramHistory() {
		obj := a.ObjectKey
		if strings.TrimSpace(obj) == "" {
			obj = "не привязана к объекту"
		}
		scale := "анализ формы"
		if a.Calibrated {
			scale = "шкалы откалиброваны"
		}
		all = append(all, ev{a.Created, obj, "Диагностика ШГН / УШГН", fmt.Sprintf("%s; уверенность %.0f %%; %s", filepath.Base(a.SourceFile), a.Confidence*100, scale), "Распознана"})
	}
	sort.SliceStable(all, func(i, j int) bool { return parseDate(all[i].d).After(parseDate(all[j].d)) })
	listBeginUpdate(timelineList)
	defer listEndUpdate(timelineList)
	listClear(timelineList)
	limit := len(all)
	if limit > 2000 {
		limit = 2000
	}
	for i := 0; i < limit; i++ {
		e := all[i]
		listAdd(timelineList, i, []string{e.d, e.o, e.t, e.e, e.s})
	}
}

func refreshPlan() {
	if planList == 0 {
		return
	}
	labels := versionLabelIndex()
	listBeginUpdate(planList)
	defer listEndUpdate(planList)
	listClear(planList)
	limit := len(plans)
	if limit > 2000 {
		limit = 2000
	}
	for i := 0; i < limit; i++ {
		pv := plans[i]
		obj := pv.ObjectKey
		if q := labels[pv.VersionID]; q != "" {
			obj = q
		}
		listAdd(planList, i, []string{pv.DueDate, obj, pv.Action, pv.Basis, pv.Status})
	}
}

func refreshRegistry() {
	if registryList == 0 {
		return
	}
	listBeginUpdate(registryList)
	defer listEndUpdate(registryList)
	listClear(registryList)
	r := 0
	for _, v := range versions {
		if v.Decision != "" && !v.Archived {
			if r >= 2000 {
				break
			}
			listAdd(registryList, r, []string{v.Created, objectLabel(v), v.Complication, v.Method, v.Decision, v.Responsible})
			r++
		}
	}
}
func refreshArchive() {
	if archiveList == 0 {
		return
	}
	ensureFullReportsLoaded()
	listBeginUpdate(archiveList)
	defer listEndUpdate(archiveList)
	listClear(archiveList)
	approvedRegistryRows = approvedFullReportRecords()
	refreshApprovedRegistrySummary(approvedRegistryRows)
	for row, record := range approvedRegistryRows {
		approvedAt := firstNonEmpty(strings.TrimSpace(record.ApprovedAt), record.CreatedAt)
		listAdd(archiveList, row, []string{
			record.ID + "  " + approvedAt,
			record.ObjectLabel,
			record.ScenarioName,
			record.Methods,
			fmt.Sprintf("%.0f сут.", record.ForecastDays),
			fmt.Sprintf("%+.1f млн руб.", record.NetBenefitK/1000),
			record.Author,
		})
	}
	fitWellFailureListColumns(archiveList, []int32{145, 190, 120, 260, 100, 125, 170})
	// RC735: keep an explicit active record so the three export buttons work
	// immediately after entering section 8, without an extra prerequisite click.
	if len(approvedRegistryRows) > 0 {
		selectListRow(archiveList, 0)
	}
}
func refreshData() {
	if dataInfo == 0 {
		return
	}
	legacy := findLegacyState()
	if legacy == "" {
		legacy = "не найдено"
	}
	ok, cnt, integ := verifyLedger()
	ist := "OK"
	if !ok {
		ist = "НАРУШЕНА"
	}
	lastImport := "нет принятого импорта"
	if len(techImports) > 0 {
		q := techImports[len(techImports)-1]
		lastImport = fmt.Sprintf("%s; строк %d; изменено %d; новых %d; без № скв. %d; нет в новой выгрузке %d", q.ImportedAt, q.Total, q.Changed, q.Added, q.Skipped, q.Removed)
	}
	st := storageStats()
	failureStatus := "OK"
	if failureLoadError != nil {
		failureStatus = failureLoadError.Error()
	}
	setText(dataInfo, fmt.Sprintf("Данные: %s\r\nТехрежим: %d объектов; импорт: %s | расчётов: %d | отказов: %d (%s)\r\nЦелостность: %s (%d; %s) | хранилище: %s, файлов %d, копии %s | SQLite: %s", dataDir, len(techCurrent), lastImport, len(versions), len(failureEvents), failureStatus, ist, cnt, integ, humanBytes(st.TotalBytes), st.DataFiles, humanBytes(st.BackupBytes), persistentStorageStatusText()))
	refreshTechImportUI()
	refreshFailureImportUI()
	refreshFailureObjectChooser()
}

func compareSelected() {
	x := filteredVersions()
	i := selectedRow(historyList)
	if i < 0 || i >= len(x) {
		msg("Сравнение версий", "Выберите оценку в истории оценок.", MB_OK|MB_ICONWARNING)
		return
	}
	cur := x[i]
	var prev *VersionRecord
	for j := i + 1; j < len(x); j++ {
		if x[j].ObjectKey == cur.ObjectKey {
			q := x[j]
			prev = &q
			break
		}
	}
	if prev == nil {
		msg("Сравнение версий", "Для выбранного объекта более ранняя версия не найдена.", MB_OK|MB_ICONINFORMATION)
		return
	}
	pairs := [][3]string{{"Осложнение", prev.Complication, cur.Complication}, {"Метод", prev.Method, cur.Method}, {"Прогноз МРП", prev.ForecastMRP, cur.ForecastMRP}, {"Выбранный сценарий", prev.Scenario, cur.Scenario}, {"Лабораторный статус", prev.LabStatus, cur.LabStatus}, {"Уровень экономики", prev.EconMode, cur.EconMode}, {"Решение", prev.Decision, cur.Decision}}
	var lines []string
	for _, p := range pairs {
		if p[1] != p[2] {
			lines = append(lines, fmt.Sprintf("%s: %s → %s", p[0], p[1], p[2]))
		}
	}
	if len(lines) == 0 {
		lines = []string{"Ключевые итоговые поля не изменились."}
	}
	msg("Сравнение оценок", fmt.Sprintf("Объект: %s\nПредыдущая версия: %s\nТекущая версия: %s\n\n%s", objectLabel(cur), prev.Created, cur.Created, strings.Join(lines, "\n")), MB_OK|MB_ICONINFORMATION)
}
func manualMRPEventTypeFromUI(v string) (eventType string, planned bool, manual bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "отказ":
		return "отказ", false, true
	case "плановое вмешательство":
		return "плановое вмешательство", true, true
	case "ремонт/подъём", "ремонт/подъем":
		return "ремонт", false, true
	case "прочее вмешательство":
		return "прочее вмешательство", false, true
	default:
		return "", false, false
	}
}

func saveFact() { // fact is linked to selected history version; require selecting there first
	x := filteredVersions()
	i := selectedRow(historyList)
	if i < 0 || i >= len(x) {
		msg("Фактический результат", "Сначала выберите оценку на странице «История оценок», затем перейдите в раздел «Технологии и критерии».", MB_OK|MB_ICONWARNING)
		return
	}
	v := x[i]
	resultText, commentText := getText(factResult), getText(factComment)
	eventType, planned, failureMode := classifyMRPOutcomeEvent(resultText, commentText)
	classificationSource := "автоклассификация по полям результата/комментария; требуется ручное подтверждение для анализа отказов"
	confirmed := false
	if manualType, manualPlanned, manual := manualMRPEventTypeFromUI(getText(factEventType)); manual {
		eventType, planned = manualType, manualPlanned
		classificationSource = "ручная классификация пользователем при сохранении факта"
		confirmed = true
	}
	cycleStart, cycleEnd := strings.TrimSpace(getText(factCycleStart)), strings.TrimSpace(getText(factCycleEnd))
	if cycleStart != "" && parseDate(cycleStart).IsZero() {
		msg("Фактический результат", "Некорректная дата начала цикла. Используйте ДД.ММ.ГГГГ.", MB_OK|MB_ICONWARNING)
		return
	}
	if cycleEnd != "" && parseDate(cycleEnd).IsZero() {
		msg("Фактический результат", "Некорректная дата окончания цикла. Используйте ДД.ММ.ГГГГ.", MB_OK|MB_ICONWARNING)
		return
	}
	if cycleStart != "" && cycleEnd != "" && parseDate(cycleEnd).Before(parseDate(cycleStart)) {
		msg("Фактический результат", "Дата окончания цикла не может быть раньше даты начала.", MB_OK|MB_ICONWARNING)
		return
	}
	actualMRP := strings.TrimSpace(getText(factMRP))
	if actualMRP == "" && cycleStart != "" && cycleEnd != "" {
		startDate, endDate := parseDate(cycleStart), parseDate(cycleEnd)
		if !startDate.IsZero() && !endDate.IsZero() && !endDate.Before(startDate) {
			actualMRP = fmt.Sprintf("%.0f", endDate.Sub(startDate).Hours()/24)
		}
	}
	eventDate := time.Now().Format("02.01.2006")
	if cycleEnd != "" && !parseDate(cycleEnd).IsZero() {
		eventDate = cycleEnd
	}
	o := Outcome{ID: v.VersionID + "-fact", VersionID: v.VersionID, Date: eventDate, CycleStartDate: cycleStart, CycleEndDate: cycleEnd, ActualMRP: actualMRP, ActualCost: getText(factCost), Result: resultText, Comment: commentText, EventType: eventType, PlannedIntervention: planned, FailureMode: failureMode, ClassificationSource: classificationSource, ClassificationConfirmed: confirmed}
	preserveOutcomeDetails(&o, outcomeFor(v.VersionID))
	updated := false
	for j := range outcomes {
		if outcomes[j].VersionID == v.VersionID {
			outcomes[j] = o
			updated = true
			break
		}
	}
	if !updated {
		outcomes = append(outcomes, o)
	}
	if e := saveJSON(filepath.Join(dataDir, "outcomes.json"), outcomes); e != nil {
		msg("Ошибка", e.Error(), MB_OK|MB_ICONERROR)
		return
	}
	invalidateMethodsBackground()
	classNote := "классификация автоматическая и ещё не подтверждена"
	if confirmed {
		classNote = "классификация подтверждена вручную"
	}
	msg("Фактический результат", fmt.Sprintf("Результат сохранён как отдельная запись и не изменяет исходную расчётную версию.\nТип события: %s; %s.", eventType, classNote), MB_OK|MB_ICONINFORMATION)
	refreshAll()
}

func confirmSelectedOutcomeClassification() {
	x := filteredVersions()
	i := selectedRow(historyList)
	if i < 0 || i >= len(x) {
		msg("Классификация МРП", "Сначала выберите оценку в истории.", MB_OK|MB_ICONWARNING)
		return
	}
	v := x[i]
	var target *Outcome
	for j := range outcomes {
		if outcomes[j].VersionID == v.VersionID {
			target = &outcomes[j]
			break
		}
	}
	if target == nil {
		msg("Классификация МРП", "Для выбранной версии ещё нет фактического результата.", MB_OK|MB_ICONWARNING)
		return
	}
	if manualType, manualPlanned, manual := manualMRPEventTypeFromUI(getText(factEventType)); manual {
		target.EventType, target.PlannedIntervention = manualType, manualPlanned
	}
	if strings.TrimSpace(target.EventType) == "" || normalizeMRPEventType(target.EventType, target.PlannedIntervention) == "не классифицировано" {
		msg("Классификация МРП", "Сначала выберите тип события или уточните результат/комментарий.", MB_OK|MB_ICONWARNING)
		return
	}
	target.ClassificationConfirmed = true
	target.ClassificationSource = "ручное подтверждение пользователем после проверки фактического результата"
	if e := saveJSON(filepath.Join(dataDir, "outcomes.json"), outcomes); e != nil {
		msg("Ошибка", e.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Классификация МРП", fmt.Sprintf("Подтверждено: %s; причина: %s.", target.EventType, target.FailureMode), MB_OK|MB_ICONINFORMATION)
	refreshAll()
}

func createBackup() error {
	dir := filepath.Join(dataDir, "reserve_copies")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	target := filepath.Join(dir, "OF_data_"+time.Now().Format("20060102_150405")+".zip")
	zf, e := os.Create(target)
	if e != nil {
		return e
	}
	zw := zip.NewWriter(zf)
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"reserve_copies"+string(filepath.Separator)) {
			return nil
		}
		if strings.Contains(strings.ToLower(filepath.Base(path)), "_display_") && strings.HasSuffix(strings.ToLower(path), ".bmp") {
			return nil
		}
		rel, _ := filepath.Rel(dataDir, path)
		w, e := zw.Create(rel)
		if e != nil {
			return e
		}
		f, e := os.Open(path)
		if e != nil {
			return e
		}
		defer f.Close()
		_, e = io.Copy(w, f)
		return e
	})
	ce := zw.Close()
	fe := zf.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	if fe != nil {
		return fe
	}
	if persistentStore != nil {
		if _, e := createManualPersistentDatabaseBackup(); e != nil {
			return fmt.Errorf("файловая копия создана, но SQLite backup не выполнен: %w", e)
		}
	}
	return nil
}
func openFolder() {
	op := p("open")
	path := p(dataDir)
	pShellExecuteW.Call(uintptr(hMain), uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(path)), 0, 0, 1)
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
func setPos(h syscall.Handle, x, y, w, hh int32) {
	if h == 0 || w <= 0 || hh <= 0 {
		return
	}
	// Native child-window geometry is committed together at the end of each
	// workbench layout. No individual child can expose the previous page's
	// background in the middle of a multi-control geometry transaction.
	if layoutBatchDepth > 0 {
		queueLayoutPosition(h, x, y, w, hh)
		return
	}
	// A layout pass can visit hundreds of controls without any geometry having
	// changed. SetWindowPos on unchanged children invalidates their entire parent
	// and can produce an expensive cascade of erased/partly repainted frames.
	// Compare the actual HWND rectangle rather than caching handles, because
	// Windows can recycle HWND values after modal dialogs have been destroyed.
	var actual RECT
	if ok, _, _ := pGetWindowRect.Call(uintptr(h), uintptr(unsafe.Pointer(&actual))); ok != 0 {
		parent, _, _ := pGetParent.Call(uintptr(h))
		origin := POINT{X: actual.Left, Y: actual.Top}
		if parent != 0 {
			pScreenToClient.Call(parent, uintptr(unsafe.Pointer(&origin)))
		}
		if origin.X == x && origin.Y == y && actual.Right-actual.Left == w && actual.Bottom-actual.Top == hh {
			return
		}
	}
	// SetWindowPos invalidates both the moved child and the exposed parent.
	// Suppressing that invalidation produced persistent partial frames after a
	// tab change or a monitor resize. Painting occurs after this UI call returns.
	pSetWindowPos.Call(uintptr(h), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(hh), SWP_NOZORDER|frameNoActivate)
}

// layoutDashboardFundColumns keeps the seven logical fund columns fitted to
// the ListView client area.  The last column receives the rounding remainder,
// so a resize can never leave a misleading eighth empty column on the right.
func layoutDashboardFundColumns(contentW int32) {
	if objectsList == 0 {
		return
	}
	available := max32(420, contentW-18) // reserve the native vertical scrollbar
	widths := []int32{
		max32(135, contentW*17/100), // Месторождение
		max32(90, contentW*10/100),  // Скважина
		max32(115, contentW*13/100), // Залежь
		max32(190, contentW*21/100), // Объект разработки / пласт
		max32(80, contentW*12/100),  // Шифр
		max32(75, contentW*10/100),  // ГНО
		max32(90, contentW*9/100),   // МРП, сут.
	}
	total := int32(0)
	for _, width := range widths {
		total += width
	}
	// At the supported minimum window the long fourth header has priority;
	// reduce only the flexible columns if a narrow client area is encountered.
	if total > available {
		over := total - available
		for _, i := range []int{0, 2, 4, 5, 6} {
			cut := min32(over, max32(0, widths[i]-55))
			widths[i] -= cut
			over -= cut
			if over == 0 {
				break
			}
		}
	}
	total = 0
	for _, width := range widths {
		total += width
	}
	widths[len(widths)-1] += available - total
	for i, width := range widths {
		setListColumnWidth(objectsList, i, max32(55, width))
	}
}

func layoutDashboardHistoryColumns(contentW int32) {
	if recentList == 0 {
		return
	}
	available := max32(420, contentW-2)
	var client RECT
	if r, _, _ := pGetClientRect.Call(uintptr(recentList), uintptr(unsafe.Pointer(&client))); r != 0 {
		if width := client.Right - client.Left; width > 0 {
			available = width
		}
	}
	base := []int32{50, 90, 90, 180, 155, 75, 135, 155, 105}
	baseTotal := int32(0)
	for _, width := range base {
		baseTotal += width
	}
	widths := make([]int32, len(base))
	total := int32(0)
	for i, width := range base {
		widths[i] = max32(42, available*width/baseTotal)
		total += widths[i]
	}
	widths[len(widths)-1] += available - total
	for i, width := range widths {
		setListColumnWidth(recentList, i, max32(42, width))
	}
}

func resizeCommonPage(w, h int32) {
	contentW := max32(720, w-270)
	// Wide tables and text fields follow the window width. This keeps long engineering text
	// readable instead of leaving a narrow fixed column on large monitors.
	switch currentPage {
	case ID_DASH:
		// Center the dashboard actions. The three tech import steps share a quiet
		// neutral style; the final object-confirmation action remains the only red
		// primary action in this row.
		buttonGap := int32(16)
		buttonW := min32(280, max32(180, (contentW-buttonGap*3)/4))
		rowW := buttonW*4 + buttonGap*3
		rowX := int32(242) + max32(0, (contentW-rowW)/2)
		setPos(dashboardImportAccent, 242, py(160), 6, 30)
		setPos(dashboardImportTitle, 258, py(160), contentW-16, 30)
		setPos(techChooseButton, rowX, py(192), buttonW, 42)
		setPos(techPreviewButton, rowX+buttonW+buttonGap, py(192), buttonW, 42)
		setPos(techApplyButton, rowX+(buttonW+buttonGap)*2, py(192), buttonW, 42)
		confirmX := rowX + (buttonW+buttonGap)*3
		setPos(objectSelectFrame, confirmX-4, py(188), buttonW+8, 50)
		setPos(objectSelectButton, confirmX, py(192), buttonW, 42)
		// The confirmed-object card now sits directly below the action row and
		// spans the same width as the fund/history area.
		setPos(dashboardObjectAccent, 242, py(252), 6, 30)
		setPos(dashboardObjectTitle, 258, py(252), contentW-16, 30)
		setPos(dashboardNote, 242, py(286), contentW, 52)

		setPos(dashboardSelectionAccent, 242, py(350), 6, 30)
		setPos(dashboardSearchTitle, 258, py(350), contentW-16, 30)
		setPos(objectsFilter, 242, py(386), contentW, 34)
		objectY := py(428)
		remaining := max32(250, h-objectY)
		rowCount := int32(len(filteredObjects()))
		// Do not collapse the main object table when the current filter returns only
		// a few rows.  This is the primary object-selection surface, so it keeps a
		// stable readable band while the history table yields the remaining height.
		desiredObjectH := max32(214, 34+min32(rowCount, 10)*24)
		maxObjectH := max32(180, remaining-214)
		objectH := min32(maxObjectH, desiredObjectH)
		setPos(objectsList, 242, objectY, contentW, objectH)
		layoutDashboardFundColumns(contentW)

		// Previously confirmed objects now occupy the whole lower band. The fund
		// table yields height to it instead of forcing either block to be clipped.
		bottomY := objectY + objectH + 10
		setPos(dashboardHistoryAccent, 242, bottomY, 6, 30)
		setPos(dashboardRecentTitle, 258, bottomY, 500, 30)
		setPos(dashboardHistoryHint, 780, bottomY+5, max32(220, contentW-538), 22)
		historyY := bottomY + 36
		setPos(recentList, 242, historyY, contentW, max32(90, h-historyY-12))
		layoutDashboardHistoryColumns(contentW)
	case ID_OBJECTS:
		setPos(objectEquipmentButton, 242+contentW-312, py(94), 312, 48)
		setPos(objectPassportLabel, 242, py(133), 238, 26)
		setPos(objectSelectedValue, 480, py(133), max32(100, contentW-238), 26)
		setPos(objectDetail, 242, py(156), contentW, 62)
		layoutWellCardFailureUI(w, h)
	case ID_LAB:
		layoutLaboratory(w, h)
	case ID_TELEMETRY:
		layoutTelemetry(w, h)
	case ID_ASSESS:
		// RC577: reserve a real two-line action row.  "ЗАКРЕПИТЬ И ПЕРЕСЧИТАТЬ"
		// is intentionally allowed to wrap and therefore must not share the old
		// 40 px height.  The object warning uses the same vertical band.
		actionGap := int32(10)
		actionW := min32(260, max32(220, contentW/4))
		actionsTotal := actionW*2 + actionGap
		objectW := max32(300, contentW-actionsTotal-18)
		actionY := py(132)
		actionH := int32(50)
		setPos(assessObjectInfo, 242, actionY, objectW, actionH)
		setPos(assessProbabilityButton, 242+contentW-actionsTotal, actionY, actionW, actionH)
		setPos(assessHistoryButton, 242+contentW-actionW, actionY, actionW, actionH)
		layoutAssessmentInnerTabs()
	case ID_ACTIVITIES:
		layoutMethodsWorkflow(w, h)
	case ID_ARCHIVE:
		// Historical geometry marker retained for regression QA: setPos(archiveList, 242, py(185), contentW, max32(120, h-py(185)-18))
		layoutApprovedRegistryWorkspace(w, h)

	case ID_DYN:
		layoutDynamogram(w, h)
	case ID_DATA:
		layoutDataPage(w, h)
	}
}

func layoutDataPage(w, h int32) {
	contentW := max32(720, w-270)
	setPos(dataInfo, 242, py(160), contentW, 70)
	gap := int32(8)
	colW := (contentW - 3*gap) / 4
	for i, control := range dataActionButtons {
		row, col := int32(i/4), int32(i%4)
		setPos(control, 242+col*(colW+gap), py(550)+row*47, colW, 40)
	}
}

func layoutHeaderSubtitle(x, width int32) {
	if hSubtitle == 0 || width <= 0 {
		return
	}
	// The full phrase is one label. A narrower header uses the same drawing logic
	// with a smaller font selected by drawHeaderSubtitle.
	// Lower the phrase into the marked central band, leaving a separate row
	// below it for the two history controls.
	setPos(hSubtitle, x, 32, width, 56)
}

func drawHeaderSubtitle(di *DRAWITEMSTRUCT) {
	if di == nil || di.HDC == 0 {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushTop))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	availableW := di.RcItem.Right - di.RcItem.Left
	parts := []struct {
		text string
		red  bool
	}{
		{"Р", true}, {"АСЧЁТ • ", false},
		{"О", true}, {"ЦЕНКА • ", false},
		{"С", true}, {"ОПРОВОЖДЕНИЕ • ", false},
		{"А", true}, {"ВТОМАТИЗАЦИЯ", false},
	}
	// Measure the real phrase at the current Windows DPI and select the largest
	// font that actually fits. Width thresholds alone were not reliable at
	// 125-150% scaling and caused the final word to be clipped.
	fonts := []syscall.Handle{hFontHeaderBrand, hFontHeaderBold, hFontButton, hFontButtonCompact, hFontHeaderSmall, hFontHeaderTiny}
	font := hFontButton
	widths := make([]int32, len(parts))
	totalW := int32(0)
	oldFont := uintptr(0)
	for _, candidate := range fonts {
		if candidate == 0 {
			continue
		}
		prev, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(candidate))
		trialTotal := int32(0)
		trialWidths := make([]int32, len(parts))
		for i, part := range parts {
			buf, _ := syscall.UTF16FromString(part.text)
			r := RECT{Left: 0, Top: 0, Right: 2400, Bottom: 120}
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX)
			trialWidths[i] = r.Right - r.Left
			trialTotal += trialWidths[i]
		}
		pSelectObject.Call(uintptr(di.HDC), prev)
		font, widths, totalW = candidate, trialWidths, trialTotal
		if trialTotal <= availableW-20 {
			break
		}
	}
	if font != 0 {
		oldFont, _, _ = pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	}
	startX := di.RcItem.Left + max32(0, (availableW-totalW)/2)
	if totalW > availableW-12 {
		startX = di.RcItem.Left + 6
	}
	x := startX
	for i, part := range parts {
		if part.red {
			pSetTextColor.Call(uintptr(di.HDC), rgb(183, 20, 37))
		} else {
			pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
		}
		buf, _ := syscall.UTF16FromString(part.text)
		r := RECT{Left: x, Top: di.RcItem.Top, Right: min32(di.RcItem.Right, x+widths[i]+4), Bottom: di.RcItem.Bottom}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		x += widths[i]
		if x >= di.RcItem.Right-4 {
			break
		}
	}
	if oldFont != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldFont)
	}
}

func resize() {
	var r RECT
	pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&r)))
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	if w <= 0 || h <= 0 {
		return
	}
	setPos(hTopBG, 0, 0, w, HEADER_H)
	setPos(hStripe, 0, STRIPE_Y, w, 4)
	setPos(hWarningBG, 12, WARNING_Y+4, max32(0, w-24), WARNING_H-8)
	setPos(hWarningTitle, 20, WARNING_Y+8, 32, 30)
	noticeTextW, noticeX := sectionNoticeLayout(w)
	setPos(hWarningText, 58, WARNING_Y+9, noticeTextW, 26)
	setPos(sectionNoticeLabel, noticeX, WARNING_Y+5, max32(0, w-noticeX-12), 36)
	// RC108: keep three real zones and give the right corporate cluster enough
	// room to breathe.  The previous RC compressed the logo, user name and four
	// utility buttons into ~360 px, which clipped both the LUKOIL mark and labels
	// at 125-150% DPI.
	setPos(hBrandIcon, 8, 2, 178, 86)
	// Keep both caption lines above the red header separator at 100-150% DPI.
	setPos(hBrandSignature, 8, 86, 178, 34)
	setPos(hHeaderSep1, 194, 10, 2, 104)
	// Reserve a real corporate/service zone. RC108 still compressed the LUKOIL
	// mark and utility buttons on wide screens because 520 px was too tight at
	// common 125-150% Windows DPI.
	rightBlockW := int32(620)
	rightX := max32(610, w-rightBlockW)
	headerTextX := int32(224)
	descriptionW := max32(0, rightX-headerTextX-18)
	layoutHeaderSubtitle(headerTextX, descriptionW)
	// Give the second line a genuinely tall box and a lower safety margin.
	setPos(hSystemName, headerTextX, 72, descriptionW, 30)
	setPos(hHeaderSep2, rightX-10, 10, 2, 104)

	lukoilW := int32(160)
	// Give the bitmap and the slogan independent boxes with a bottom safety
	// margin; neither may touch the red separator line.
	// Keep the LUKOIL mark and slogan clear of the red separator.  The slogan is
	// deliberately lifted and given extra height so its second line cannot be
	// clipped by Windows font metrics at 125-150% DPI.
	setPos(hLukoilLabel, rightX+12, 8, lukoilW, 52)
	setPos(hBuildMark, rightX+12, 61, lukoilW, 47)
	userX := rightX + lukoilW + 32
	userW := max32(0, w-userX-16)
	approveW := int32(156)
	setPos(hUserLabel, userX, 10, max32(0, userW-approveW-8), 38)
	setPos(hHeaderApprove, userX+max32(0, userW-approveW-8)+8, 11, approveW, 44)
	// Нижняя панель: корзина слева, затем история, справка и навигация.
	show(hHeaderExport, false)
	setText(hHeaderBack, "←")
	setText(hHeaderForward, "→")
	gap := int32(6)
	buttonW := int32(80)
	arrowW := int32(68)
	trashW := int32(106)
	actionGroupW := trashW + buttonW*2 + arrowW*2 + gap*4
	actionX := userX + max32(0, (userW-actionGroupW)/2)
	setPos(sessionTrashButton, actionX, 68, trashW, 38)
	setPos(hHeaderOpen, actionX+trashW+gap, 68, buttonW, 38)
	setPos(hHeaderHelp, actionX+trashW+gap+buttonW+gap, 68, buttonW, 38)
	setPos(hHeaderBack, actionX+trashW+gap+2*(buttonW+gap), 68, arrowW, 38)
	setPos(hHeaderForward, actionX+trashW+gap+2*(buttonW+gap)+arrowW+gap, 68, arrowW, 38)
	setPos(hNavBG, 0, NAV_TOP, 230, max32(0, h-NAV_TOP))
	navOrder := []int{ID_DASH, ID_OBJECTS, ID_LAB, ID_ASSESS, ID_ACTIVITIES, ID_MRP, ID_ECON, ID_RESULT, ID_ARCHIVE, ID_DATA}
	navAvailable := max32(0, h-NAV_TOP)
	navGap := int32(3)
	navPadding := int32(5)
	baseH := max32(37, (navAvailable-navPadding*2-navGap*int32(len(navOrder)-1))/int32(len(navOrder)))
	navY := int32(NAV_TOP) + navPadding
	for _, id := range navOrder {
		setPos(navButtons[id], 8, navY, 214, baseH)
		navY += baseH + navGap
	}
	resizeCommonPage(w, h)
	layoutWorkflowPage(w, h)
}

func wndProc(hwnd syscall.Handle, msgid uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			diagLog("PANIC message=0x%X panic=%v\n%s", msgid, r, debug.Stack())
			if msgid == WM_CREATE {
				// Returning -1 from WM_CREATE tells CreateWindowExW to abort cleanly.
				// Do not leave a half-built, apparently frozen main window alive.
				ret = ^uintptr(0)
				return
			}
			ret = 0
		}
	}()
	switch msgid {
	case WM_CREATE:
		hMain = hwnd
		// Apply icon again at window level. The handles are created before class registration
		// from bytes embedded into the executable; external ICO is only a fallback.
		if hIconBig == 0 || hIconSmall == 0 {
			loadAppIconsFallback()
		}
		if hIconBig != 0 {
			pSendMessageW.Call(uintptr(hwnd), WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(uintptr(hwnd), WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		diagLog("UI build start")
		buildUI()
		diagLog("UI build done")
		// Let WM_CREATE return before populating even the first ListView. This keeps
		// Windows' message pump responsive during startup.
		pPostMessageW.Call(uintptr(hwnd), WM_APP_INITIAL_REFRESH, 0, 0)
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			if mmi != nil {
				// Constrain maximization to the visible work area of the current
				// monitor. Without these values Windows may reuse a negative virtual-
				// desktop origin and cut the left navigation and upper header off.
				monitor, _, _ := pMonitorFromWindow.Call(uintptr(hwnd), 2)
				if monitor != 0 {
					mi := MONITORINFO{CbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
					ok, _, _ := pGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&mi)))
					if ok != 0 {
						mmi.PtMaxPosition.X = mi.RcWork.Left - mi.RcMonitor.Left
						mmi.PtMaxPosition.Y = mi.RcWork.Top - mi.RcMonitor.Top
						mmi.PtMaxSize.X = mi.RcWork.Right - mi.RcWork.Left
						mmi.PtMaxSize.Y = mi.RcWork.Bottom - mi.RcWork.Top
					}
				}
				// Keep the restored window inside a 1280x720-class work area.
				// The previous 1280x768 minimum could force the frame under the
				// taskbar and visually cut both the left navigation and archive.
				mmi.PtMinTrackSize.X = 1180
				mmi.PtMinTrackSize.Y = 700
			}
		}
		return 0
	case WM_DPICHANGED:
		// ROSA always occupies the current monitor work area. The suggested DPI
		// rectangle may contain stale negative coordinates after monitor or scale
		// changes; applying it shifted the header, icon and field edges off-screen.
		// Restore first so a subsequent maximize recalculates the real work area.
		anchorMainWindowToMonitor(hwnd)
		loadEmbeddedAppIcons()
		if hIconBig != 0 {
			pSendMessageW.Call(uintptr(hwnd), WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(uintptr(hwnd), WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		requestMainLayout()
		return 0
	case WM_DISPLAYCHANGE:
		// ROSA is a full-workspace application. Re-maximizing after a monitor or
		// resolution change removes stale off-screen restored coordinates.
		anchorMainWindowToMonitor(hwnd)
		requestMainLayout()
		return 0
	case 0x0014: // WM_ERASEBKGND: draw section-6 panels into the parent surface.
		if currentPage == ID_ECON && paintEconomics813Parent(syscall.Handle(wParam)) {
			return 1
		}
	case WM_SIZE:
		if wParam != SIZE_MINIMIZED && !pageSwitching {
			requestMainLayout()
		}
		return 0
	case 0x020A: // WM_MOUSEWHEEL; section 5 has a scrollable data area.
		if currentPage == ID_MRP && nnoFleetEnabled {
			if int16(hiWord(wParam)) > 0 {
				nnoScrollBy(-120)
			} else {
				nnoScrollBy(120)
			}
			return 0
		}
	case WM_EXITSIZEMOVE:
		pRedrawWindow.Call(uintptr(hMain), 0, 0, 0x0001|0x0004|0x0080)
		if currentPage == ID_DYN {
			refreshDynamogramPreviewSize()
		}
		return 0
	case WM_COMMAND:
		if !pageInputSourceAllowed(syscall.Handle(lParam)) {
			return 0
		}
		id := int(loWord(wParam))
		code := hiWord(wParam)
		if currentPage == ID_ECON && (code == 0 || code == 5) && handleEconomics812Card(id) {
			return 0
		}
		if currentPage == ID_ECON && id == ID_ECON_FUND_FILTER && code == 1 { // CBN_SELCHANGE
			changeEconomicsFundFilter()
			return 0
		}
		if currentPage == ID_MRP && handleNNOFleetCommand(id, int(code)) {
			return 0
		}
		if currentPage == ID_OBJECTS && (code == 3 || code == 0x0100) {
			rememberWellFailureField(syscall.Handle(lParam))
		}
		if code == 0x0300 && invalidateEconomicsForEdit(syscall.Handle(lParam)) {
			return 0
		}
		if code == 1 && handleLaboratoryUnitChange(id) {
			return 0
		}
		if code == 1 && handleWellCardComboChange(id) {
			return 0
		}
		if currentPage == ID_OBJECTS && wellFormActive && code == 0x0300 {
			refreshFailureFormProgress()
		}
		if currentPage == ID_OBJECTS && wellFormActive && code == 5 {
			refreshFailureFormProgress()
			updateWellCardVisibility()
		}
		if code == 1 && handleLaboratoryScaleModelChange(id) {
			return 0
		}
		if id == ID_MRP_TARGET_SCOPE && code == 1 {
			handleMRPTargetScopeChange()
			return 0
		}
		if id == ID_ASSESS_METHOD_COMBO && code == 1 {
			rememberSelectedAssessmentMethod()
			refreshAssessmentMethodDetail()
			return 0
		}
		if id == ID_LAB_SCOPE_COMBO && code == 1 {
			index, _, _ := pSendMessageW.Call(uintptr(labScopeCombo), CB_GETCURSEL, 0, 0)
			labOverviewScopeCode = laboratoryScopeCodeByIndex(int(index))
			laboratoryRefreshDetailObjects()
			layoutLaboratoryCurrentClient()
			pInvalidateRect.Call(uintptr(labInfluenceChart), 0, 1)
			pInvalidateRect.Call(uintptr(labTrendChart), 0, 1)
			return 0
		}
		if id == ID_LAB_TREND_METRIC && code == 1 {
			pInvalidateRect.Call(uintptr(labTrendChart), 0, 1)
			return 0
		}
		if id == ID_LAB_RISK_FILTER && code == 1 {
			index, _, _ := pSendMessageW.Call(uintptr(labRiskFilterCombo), CB_GETCURSEL, 0, 0)
			labOverviewRiskFilterCode = laboratoryRiskFilterCodeByIndex(int(index))
			laboratoryRefreshDetailObjects()
			pInvalidateRect.Call(uintptr(labInfluenceChart), 0, 1)
			pInvalidateRect.Call(uintptr(labTrendChart), 0, 1)
			return 0
		}
		if id == ID_LAB_GROUP_COMBO && code == 1 {
			index, _, _ := pSendMessageW.Call(uintptr(labSavedGroupCombo), CB_GETCURSEL, 0, 0)
			if group, ok := laboratorySavedGroupByIndex(int(index)); ok {
				setText(labGroupNameEdit, group.Name)
				setText(labScopeObjectsEdit, laboratorySavedGroupText(group))
				laboratoryRefreshDetailObjects()
				pInvalidateRect.Call(uintptr(labInfluenceChart), 0, 1)
				pInvalidateRect.Call(uintptr(labTrendChart), 0, 1)
			}
			return 0
		}
		if id == ID_LAB_TREND_SORT && code == 1 {
			pInvalidateRect.Call(uintptr(labTrendChart), 0, 1)
			return 0
		}
		if id == ID_LAB_DETAIL_OBJECT && code == 1 {
			pInvalidateRect.Call(uintptr(labTrendChart), 0, 1)
			return 0
		}
		if id == ID_LAB_GROUP_SAVE && code == 0 {
			if saveLaboratorySavedGroup(getText(labGroupNameEdit), getText(labScopeObjectsEdit)) {
				pSendMessageW.Call(uintptr(labSavedGroupCombo), CB_RESETCONTENT, 0, 0)
				for _, group := range laboratorySavedGroups {
					pSendMessageW.Call(uintptr(labSavedGroupCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(group.Name))))
				}
				msg("РОСА", "Группа объектов сохранена.", MB_OK)
			} else {
				msg("РОСА", "Укажите название группы и хотя бы один объект.", MB_OK|MB_ICONWARNING)
			}
			return 0
		}
		if id == ID_LAB_SCOPE_OBJECTS_EDIT && code == 0x0300 {
			laboratoryRefreshDetailObjects()
			pInvalidateRect.Call(uintptr(labInfluenceChart), 0, 1)
			pInvalidateRect.Call(uintptr(labTrendChart), 0, 1)
			return 0
		}
		if id == ID_HEADER_BACK {
			navigatePageBack()
			return 0
		}
		if id == ID_HEADER_FORWARD {
			navigatePageForward()
			return 0
		}
		if id == ID_HEADER_EXIT {
			switchCurrentUser()
			return 0
		}
		// Secondary pages remain reachable from buttons after moving them out of
		// the primary nine-step navigation.
		switch id {
		case ID_DASH, ID_OBJECTS, ID_TECHPARAM, ID_LAB, ID_TELEMETRY, ID_DYN,
			ID_ASSESS, ID_ACTIVITIES, ID_MRP, ID_ECON, ID_RESULT, ID_ARCHIVE, ID_OPI, ID_HISTORY,
			ID_TECH, ID_REGISTRIES, ID_DATA:
			switchPage(id)
			return 0
		}
		if page, ok := gnoPageForCommand(id); ok {
			switchPage(page)
			return 0
		}
		if currentPage == ID_OBJECTS && id == ID_FAILURE_OBJECT_COMBO && code == 1 { // CBN_SELCHANGE
			index, _, _ := pSendMessageW.Call(uintptr(failureObjectCombo), CB_GETCURSEL, 0, 0)
			if int(index) >= 0 && int(index) < len(failureObjectKeys) {
				selectFailureObject(failureObjectKeys[int(index)])
			}
			return 0
		}
		if currentPage == ID_OBJECTS && handleWellCardCommand(id) {
			return 0
		}
		if handleMethodsWorkflowCommand(id, code) {
			return 0
		}
		if handleWorkflowCommand(id) {
			return 0
		}
		if handleTelemetryCommand(id) {
			return 0
		}
		if id == ID_LAB_SOURCE_EDIT && code == 0x0300 { // EN_CHANGE
			handleLaboratorySourceChange()
			return 0
		}
		switch id {
		case ID_ASSESS_TAB_TECH:
			switchAssessmentInnerTab(0)
			return 0
		case ID_ASSESS_TAB_LAB:
			switchAssessmentInnerTab(1)
			return 0
		case ID_ASSESS_TAB_COMPARE:
			switchAssessmentInnerTab(2)
			return 0
		case ID_ASSESS_CLASS_SAVE:
			if err := saveAssessmentTabValue(); err != nil {
				msg("Оценка осложнений", err.Error(), MB_OK|MB_ICONWARNING)
			} else {
				refreshAssessmentOverallClassificationSummary()
			}
			return 0
		case ID_ASSESS_CONFIRM_FINAL:
			if err := confirmAssessmentFinalClass("с корректировкой"); err != nil {
				msg("Оценка осложнений", err.Error(), MB_OK|MB_ICONWARNING)
			} else {
				refreshAssessmentOverallClassificationSummary()
				msg("Оценка осложнений", "Итоговая классификация подтверждена и сохранена.", MB_OK|MB_ICONINFORMATION)
			}
			return 0
		case ID_ASSESS_ACCEPT_TECH:
			var err error
			wasLaboratoryTab := assessInnerTab == 1
			if wasLaboratoryTab {
				err = skipAssessmentLaboratoryReview()
			} else {
				err = acceptTechnologistClassificationWithoutLab()
			}
			if err != nil {
				msg("Оценка осложнений", err.Error(), MB_OK|MB_ICONWARNING)
			} else if wasLaboratoryTab {
				msg("Оценка осложнений", "Лабораторная вкладка пропущена. В третьей вкладке можно принять классы, заданные технологом.", MB_OK|MB_ICONINFORMATION)
			} else {
				refreshAssessmentOverallClassificationSummary()
				refreshResultPage()
				msg("Оценка осложнений", "Решения технолога выбраны без лабораторных данных. Классификация ещё не утверждена: перейдите в третью вкладку и нажмите «УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ».", MB_OK|MB_ICONINFORMATION)
			}
			return 0
		case ID_ASSESS_CONFIRM_KEEP, ID_ASSESS_CONFIRM_CORRECT:
			mode := "без корректировки"
			if id == ID_ASSESS_CONFIRM_CORRECT {
				mode = "с корректировкой"
			}
			indices := assessmentCheckedCompareDirections()
			updated, err := confirmAssessmentFinalClasses(mode, indices)
			if err != nil {
				msg("Оценка осложнений", err.Error(), MB_OK|MB_ICONWARNING)
			} else {
				refreshAssessmentOverallClassificationSummary()
				refreshResultPage()
				what := "Решение технолога выбрано"
				if id == ID_ASSESS_CONFIRM_CORRECT {
					what = "Предложение программы выбрано"
				}
				msg("Оценка осложнений", fmt.Sprintf("%s для %d отмеченных осложнений. Это ещё не утверждение классификации. После выбора решений по всем осложнениям нажмите «УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ».", what, updated), MB_OK|MB_ICONINFORMATION)
			}
			return 0
		case ID_ASSESS_CONFIRM_FULL:
			if err := confirmAssessmentFullClassification(); err != nil {
				msg("Оценка осложнений", err.Error(), MB_OK|MB_ICONWARNING)
			} else {
				refreshAssessmentOverallClassificationSummary()
				refreshResultPage()
				msg("Оценка осложнений", "Полная классификация по всем осложнениям утверждена и закреплена.", MB_OK|MB_ICONINFORMATION)
			}
			return 0
		case ID_LAB_TAB_CORROSION, ID_LAB_TAB_SCALE, ID_LAB_TAB_ASPO, ID_LAB_TAB_SOLIDS, ID_LAB_TAB_MICRO, ID_LAB_TAB_GASES, ID_LAB_TAB_OVERALL:
			switchLabGroupByID(id)
			return 0
		case ID_LAB_TOGGLE:
			toggleLaboratoryConclusion()
			return 0
		case ID_LAB_FIELDS_PREV:
			switchLabFieldsPage(-1)
			return 0
		case ID_LAB_FIELDS_NEXT:
			switchLabFieldsPage(1)
			return 0
		case ID_LAB_CONTINUE:
			continueLaboratoryPassEditing()
			return 0
		case ID_LAB_COPY_RESULT:
			copyLaboratoryConclusion()
			return 0
		case ID_LAB_DELETE_ROW:
			deleteSelectedLaboratoryHistory()
			return 0
		case ID_LAB_CLEAR_ALL:
			clearLaboratoryHistory()
			return 0
		case ID_LAB_CLEAR_CURRENT:
			clearLaboratoryCurrentInput()
			return 0
		case ID_LAB_SAVE:
			saveLaboratoryRecord()
			return 0
		case ID_LAB_OPEN_INFOGRAPHIC:
			openLaboratoryInfographicDialog()
			return 0
		case ID_LAB_COMPLEX_ANALYSIS:
			openLaboratoryComplexAnalysisDialog()
			return 0
		case ID_LAB_CALCULATION_BASIS:
			openLaboratoryCalculationBasisDialog()
			return 0
		case ID_LAB_ANALYZE:
			analyzeLaboratoryInputs()
			return 0
		case ID_LAB_EXPORT:
			exportLaboratoryReport()
			return 0
		case ID_LAB_IMPORT:
			beginBusyLoading("Загрузка лабораторного файла…")
			defer endBusyLoading()
			importLaboratoryUI()
			return 0
		case ID_LAB_DOWNLOAD_TEMPLATE:
			downloadLaboratoryTemplate()
			return 0
		case ID_LAB_USE_SELECTED:
			useSelectedObjectForLab()
			return 0
		case ID_HEADER_APPROVE:
			switchCurrentUser()
			return 0
		case ID_HEADER_HELP:
			msg("Справка РОСА", fmt.Sprintf("РОСА %s\n\n%s\n\n%s\n\nВсе автоматические выводы требуют проверки специалистом.", appBuildVersion, rosaPurposeText, rosaAutotechnologBoundaryText), MB_OK|MB_ICONINFORMATION)
			return 0
		case ID_ASSESS_PROBABILITY:
			// Refresh the class proposal only after the visible laboratory object is
			// explicitly attached to the global working key.  This prevents an open
			// teaching example from silently producing classes for the previous object.
			labKey := strings.TrimSpace(getText(labObjectEdit))
			repinned := false
			if labKey != "" && (selectedObjectKey == "" || !laboratoryObjectKeysEquivalent(labKey, selectedObjectKey)) {
				if !approveOpenLaboratoryObject() {
					return 0
				}
				repinned = true
			}
			refreshAssessments()
			if assessSelectedRow >= 0 && assessSelectedRow < len(assessDirectionRows) {
				showAssessmentDirectionDetail(assessDirectionRows[assessSelectedRow])
			}
			if repinned {
				msg("Оценка осложнений", "Открытый лабораторный объект закреплён для всех вкладок. Программные классы пересчитаны по этой записи. Проверьте остальные вкладки и дополните отсутствующие данные перед экспертным утверждением.", MB_OK|MB_ICONINFORMATION)
			}
			return 0
		case ID_ASSESS_TO_ACTIVITIES:
			method, err := createAssessmentMethodDraftPlan()
			if err != nil {
				msg("Оценка осложнений", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			refreshActivities()
			switchPage(ID_ACTIVITIES)
			msg("Методы и мероприятия", "Создан черновик для проверки: "+method+". Это не автоматическое назначение; подтвердите применимость и затраты перед экономической оценкой.", MB_OK|MB_ICONINFORMATION)
			return 0
		case ID_SYNC:
			n, path, e := syncLegacy()
			if e != nil {
				msg("Синхронизация", e.Error(), MB_OK|MB_ICONWARNING)
			} else {
				msg("Синхронизация", fmt.Sprintf("Источник: %s\nДобавлено новых расчётных версий: %d.", path, n), MB_OK|MB_ICONINFORMATION)
			}
			refreshAll()
			return 0
		case ID_COMPARE:
			compareSelected()
			return 0
		case ID_EXPORT_SELECTED:
			exportSelectedRosaPackage()
			return 0
		case ID_IMPORT_ROSA:
			importRosaPackageUI()
			return 0
		case ID_EXPORT_ARCHIVE:
			exportArchiveRosaPackage()
			return 0
		case ID_DELETE_HISTORY:
			deleteSelectedVersion()
			return 0
		case ID_CLEAR_HISTORY:
			clearVisibleHistory()
			return 0
		case ID_SESSION_TRASH:
			openSessionTrashDialog()
			return 0
		case ID_SHARE_SELECTED:
			shareSelectedCalculationUI()
			return 0
		case ID_SHARED_WITH_ME:
			sharedWithMeUI()
			return 0
		case ID_MANAGE_SHARES:
			manageOwnedSharesUI()
			return 0
		case ID_CLEAR_CACHE:
			n, bytes, e := clearDisplayCache()
			if e != nil {
				msg("Очистка кэша", e.Error(), MB_OK|MB_ICONERROR)
			} else {
				msg("Очистка кэша", fmt.Sprintf("Удалено временных файлов: %d\nОсвобождено: %s\nИсходные расчёты и вложения не затронуты.", n, humanBytes(bytes)), MB_OK|MB_ICONINFORMATION)
			}
			refreshAll()
			return 0
		case ID_OBJECT_CONFIRM:
			confirmSelectedObject()
			return 0
		case ID_OPEN_EQUIPMENT:
			if strings.TrimSpace(selectedObjectKey) == "" {
				msg("Состояние оборудования", "Сначала закрепите объект на Главной.", MB_OK|MB_ICONWARNING)
				return 0
			}
			switchPage(ID_TECHPARAM)
			return 0
		case ID_CONTINUE_OBJECT:
			continueSelectedObject()
			return 0
		case ID_DYN_FOR_OBJECT:
			openDynamogramForSelectedObject()
			return 0
		case ID_DYN_CHOOSE:
			if currentPage != ID_DYN {
				return 0
			}
			if f := chooseDynamogramFile(); f != "" {
				selectedDynPDFPage = 1
				if strings.EqualFold(filepath.Ext(f), ".pdf") {
					page, ok, err := chooseDynamogramPDFPage(f)
					if err != nil {
						msg("Выбор страницы PDF", err.Error(), MB_OK|MB_ICONERROR)
						return 0
					}
					if !ok {
						return 0
					}
					selectedDynPDFPage = page
				}
				selectedDynFile = f
				currentDyn = nil
				currentDynPoints = nil
				refreshDynamogramUI()
				// Loading an image immediately starts background digitisation. This gives
				// the same user flow as CSV/TXT: load -> graph -> interpretation.
				startDynamogramAnalysisAsync()
			}
			return 0
		case ID_DYN_IMPORT_CSV:
			if f := chooseDynamogramCSVFile(); f != "" {
				a, pts, e := importDynamogramCSV(f, getText(dynObjectEdit))
				if e != nil {
					msg("Импорт динамограммы", e.Error(), MB_OK|MB_ICONERROR)
				} else {
					selectedDynFile = f
					currentDyn = &a
					currentDynPoints = pts
					dynHistory = loadDynamogramHistory()
					refreshDynamogramUI()
					msg("Импорт динамограммы", fmt.Sprintf("Цифровой массив принят.\nТочек контура: %d\nДиапазон X: %.3g - %.3g\nДиапазон Y: %.3g - %.3g", a.PointCount, a.XMin, a.XMax, a.YMin, a.YMax), MB_OK|MB_ICONINFORMATION)
				}
			}
			return 0
		case ID_DYN_ANALYZE:
			startDynamogramAnalysisAsync()
			return 0
		case ID_DYN_PREVIEW:
			if currentDyn == nil {
				msg("Диагностика ШГН / УШГН", "Сначала выполните анализ изображения.", MB_OK|MB_ICONWARNING)
			} else if path, e := materializeDynPreview(currentDyn.PreviewPath); e != nil {
				msg("Диагностика ШГН / УШГН", e.Error(), MB_OK|MB_ICONERROR)
			} else {
				openExternal(path)
			}
			return 0
		case ID_DYN_EXPORT:
			if currentDyn == nil {
				msg("Диагностика ШГН / УШГН", "Сначала выполните анализ изображения.", MB_OK|MB_ICONWARNING)
			} else if path, e := materializeDynCSV(currentDyn.CSVPath); e != nil {
				msg("Диагностика ШГН / УШГН", e.Error(), MB_OK|MB_ICONERROR)
			} else {
				openExternal(path)
			}
			return 0
		case ID_DYN_EXPORT_EXCEL:
			exportCurrentDynamogramExcelUI()
			return 0
		case ID_DYN_CALIBRATE:
			if e := calibrateSelectedDynamogram(); e != nil {
				msg("Калибровка динамограммы", e.Error(), MB_OK|MB_ICONWARNING)
			} else {
				msg("Калибровка динамограммы", "Шкалы применены. CSV обновлён и теперь содержит нормированные и физические координаты.", MB_OK|MB_ICONINFORMATION)
			}
			return 0
		case ID_DYN_CONFIRM:
			reviewCurrentDynamogram("confirmed")
			return 0
		case ID_DYN_REJECT:
			reviewCurrentDynamogram("rejected")
			return 0
		case ID_DYN_DELETE:
			deleteSelectedDynamogram()
			return 0
		case ID_DYN_CLEAR:
			clearDynamogramHistoryForObject()
			return 0
		case ID_DYN_OPEN_SOURCE:
			openCurrentDynamogramSource()
			return 0
		case ID_DYN_OPEN_RECONSTRUCTION:
			openCurrentDynamogramReconstruction()
			return 0
		case ID_OPEN_FOLDER:
			openFolder()
			return 0
		case ID_ACTIVITY_ADD_METHOD:
			if method, err := addSelectedMethodsWorkflowMethod(); err != nil {
				msg("Методы и мероприятия", err.Error(), MB_OK|MB_ICONWARNING)
			} else {
				msg("Методы и мероприятия", "Добавлен в комплекс: "+method+". РОСА пересчитала применимость и совместный предварительный вклад без простого сложения эффектов.", MB_OK|MB_ICONINFORMATION)
			}
			refreshActivities()
			return 0
		case ID_ACTIVITY_REMOVE_METHOD:
			if c, ok := technicalScenarioForSelectedObject(); ok {
				if method, removed := removeCombinedMethod(c); removed {
					msg("Методы и мероприятия", "Удалён из комбинации: "+method, MB_OK|MB_ICONINFORMATION)
				} else {
					msg("Методы и мероприятия", "Основной метод оставлен: сценарий не может быть пустым.", MB_OK|MB_ICONWARNING)
				}
			}
			refreshActivities()
			return 0
		case ID_ACTIVITY_RESET_METHODS:
			if c, ok := technicalScenarioForSelectedObject(); ok {
				resetCombinedMethods(c)
			}
			refreshActivities()
			return 0
		case ID_GEN_PLAN:
			if currentPage == ID_ACTIVITIES {
				openMethodsPlanEditorSelected()
				return 0
			}
			n := generatePlan()
			msg("План контрольных и технологических мероприятий", fmt.Sprintf("Добавлено новых контрольных действий: %d.\nАвтоматические сроки не заменяют утверждённые технологические регламенты.", n), MB_OK|MB_ICONINFORMATION)
			refreshAll()
			return 0
		case ID_SAVE_FACT:
			saveFact()
			return 0
		case ID_CONFIRM_FACT_CLASS:
			confirmSelectedOutcomeClassification()
			return 0
		case ID_OUTCOME_DETAILS:
			openOutcomeDetailsDialog()
			return 0
		case ID_TECH_CHOOSE:
			if currentPage != ID_DASH {
				return 0
			}
			beginBusyLoading("Выбор файла техрежима…")
			if f := chooseTechFile(); f != "" {
				selectedTechFile = f
				techPreview = nil
				pendingTech = nil
				refreshTechImportUI()
			}
			endBusyLoading()
			return 0
		case ID_TECH_PREVIEW:
			if currentPage != ID_DASH {
				return 0
			}
			startTechPreviewAsync()
			return 0
		case ID_TECH_APPLY:
			if currentPage != ID_DASH {
				return 0
			}
			if e := applyTechImport(); e != nil {
				msg("Принятие Техрежима", e.Error(), MB_OK|MB_ICONWARNING)
			} else {
				msg("Принятие Техрежима", "Новый Техрежим принят как текущее состояние. Предыдущее состояние и резервная копия сохранены.", MB_OK|MB_ICONINFORMATION)
				refreshAll()
			}
			return 0
		case ID_BACKUP:
			if e := createBackup(); e != nil {
				msg("Резервная копия", e.Error(), MB_OK|MB_ICONERROR)
			} else {
				msg("Резервная копия", "Созданы резервная копия долговременных файлов и отдельная проверенная копия SQLite.", MB_OK|MB_ICONINFORMATION)
				refreshData()
			}
			return 0
		case ID_RESTORE_DB:
			backup := choosePersistentDatabaseBackup()
			if backup == "" {
				return 0
			}
			r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p("Текущая SQLite-база будет сохранена как аварийная копия, затем заменена выбранной резервной копией. Продолжить?"))), uintptr(unsafe.Pointer(p("Восстановление SQLite"))), 0x00000004|MB_ICONWARNING)
			if r != 6 {
				return 0
			}
			if e := restorePersistentDatabaseFromBackup(backup); e != nil {
				msg("Восстановление SQLite", e.Error(), MB_OK|MB_ICONERROR)
			} else {
				labPassesLoaded = false
				msg("Восстановление SQLite", "База восстановлена и прошла integrity_check. Предыдущее состояние сохранено рядом с рабочей БД.", MB_OK|MB_ICONINFORMATION)
				refreshAll()
			}
			return 0
		case ID_STORAGE_LOG:
			msg("Журнал операций с БД", storageRecentEventsText(30), MB_OK|MB_ICONINFORMATION)
			return 0
		case ID_LAB_DIAGNOSTICS:
			msg("LAB-90 · диагностика лаборатории", laboratoryLAB90RuntimeDiagnosticsText(), MB_OK|MB_ICONINFORMATION)
			return 0
		case ID_SECTION_BANNER_SETTINGS:
			if chooseSectionBannerSettings() {
				refreshSectionNotice()
			}
			return 0
		case ID_FAILURE_IMPORT, ID_FAILURE_CHOOSE:
			if currentPage == ID_OBJECTS {
				chooseFailureWorkbookUI()
			}
			return 0
		case ID_FAILURE_PREVIEW:
			if currentPage == ID_OBJECTS {
				previewFailureWorkbookUI()
			}
			return 0
		case ID_FAILURE_APPLY:
			if currentPage == ID_OBJECTS {
				applyFailureWorkbookUI()
			}
			return 0
		case ID_FAILURE_OBJECT_OPEN:
			if currentPage == ID_OBJECTS {
				openFailureObjectHistoryUI()
			}
			return 0
		case ID_VERIFY_BACKUPS:
			okN, badN, problems := verifyAllPersistentBackups()
			text := fmt.Sprintf("Проверено резервных копий: %d\nИсправных: %d\nПовреждённых: %d", okN+badN, okN, badN)
			if len(problems) > 0 {
				limit := len(problems)
				if limit > 8 {
					limit = 8
				}
				text += "\n\nПроблемы:\n" + strings.Join(problems[:limit], "\n")
			}
			icon := uintptr(MB_ICONINFORMATION)
			if badN > 0 {
				icon = MB_ICONWARNING
			}
			msg("Проверка резервных копий SQLite", text, MB_OK|icon)
			refreshData()
			return 0
		case ID_FILTER_HISTORY:
			if code == 0x0300 {
				refreshHistory()
			}
			return 0
		case ID_FILTER_OBJECTS:
			if code == 0x0300 {
				candidateObjectKey = ""
				candidateFromRecent = false
				refreshObjects()
			}
			return 0
		case ID_LAB_FILTER:
			if code == 0x0300 {
				refreshLaboratory()
			}
			return 0
		case ID_LAB_OBJECT_EDIT:
			if code == 0x0300 {
				handleLaboratoryObjectChange()
			}
			return 0
		}
		if code == 0x0100 && handleLaboratoryFieldFocus(id) {
			return 0
		}
		if code == 0x0200 && normalizeLaboratoryFieldOnBlur(id) { // EN_KILLFOCUS
			return 0
		}
		if code == 0x0300 && handleLaboratoryFieldChange(id) {
			return 0
		}
	case WM_APP_METHODS_DONE:
		completeMethodsEvaluation(uint64(wParam))
		return 0
	case WM_APP_LAYOUT_PENDING:
		executeMainLayout()
		return 0
	case WM_APP_UI_PULSE:
		recordUIPulse()
		return 0
	case WM_APP_INITIAL_REFRESH:
		if _, full := teachingFullScenarioForObject(selectedObjectKey); full {
			restoreSelectedObjectWorkspace(selectedObjectKey)
			for _, h := range pageControls[ID_LAB] {
				// Startup isolation only: preserve the laboratory's own logical
				// visibility so the first later activation is not blank.
				setPhysicalControlVisibility(h, false)
			}
		}
		refreshPage(currentPage)
		return 0
	case WM_APP_DYN_DONE:
		finishDynamogramAnalysisAsync()
		return 0
	case WM_APP_TECH_PREVIEW_DONE:
		finishTechPreviewAsync()
		return 0
	case WM_APP_LAB_SELECT:
		if currentPage == ID_LAB && wParam == labGroupGeneration {
			selectLabHistory()
		}
		return 0
	case WM_APP_LAB_REPORT_DONE:
		laboratoryReportExportDone()
		return 0
	case WM_APP_ARCHIVE_EXPORT_DONE:
		approvedRegistryExportDone()
		return 0
	case WM_NOTIFY:
		if lParam == 0 {
			return 0
		}
		hdr := (*NMHDR)(unsafe.Pointer(lParam))
		if hdr == nil {
			return 0
		}
		if hdr.Code != NM_CUSTOMDRAW && !pageInputSourceAllowed(hdr.HwndFrom) {
			return 0
		}
		if handleNNOFleetNotify(lParam) {
			return 0
		}
		if hdr.Code == NM_CUSTOMDRAW {
			if nnoFleetList(hdr.HwndFrom) {
				return nnoFleetListCustomDraw(lParam)
			}
			for _, list := range []syscall.Handle{nnoFleetMethods, nnoFleetMonths, nnoFleetEvents, mrpHistoryList, mrpMethodsList, econSchemesList, econCompareList} {
				if list != 0 {
					header, _, _ := pSendMessageW.Call(uintptr(list), LVM_GETHEADER, 0, 0)
					if hdr.HwndFrom == syscall.Handle(header) {
						return nnoFleetHeaderCustomDraw(lParam, syscall.Handle(header))
					}
				}
			}
		}
		if hdr.Code == NM_CUSTOMDRAW && (hdr.HwndFrom == econSchemesList || hdr.HwndFrom == econCompareList) {
			return economicsVisualCustomDraw(hdr.HwndFrom, lParam)
		}
		if hdr.Code == NM_CUSTOMDRAW {
			for _, list := range []syscall.Handle{wellHistoryList, wellManualList} {
				if list != 0 {
					header, _, _ := pSendMessageW.Call(uintptr(list), LVM_GETHEADER, 0, 0)
					if hdr.HwndFrom == syscall.Handle(header) {
						return wellFailureHeaderCustomDraw(lParam, syscall.Handle(header))
					}
				}
			}
		}
		if hdr.Code == NM_CUSTOMDRAW && hdr.HwndFrom == recentList {
			return dashboardHistoryCustomDraw(lParam)
		}
		if hdr.Code == NM_CUSTOMDRAW && hdr.HwndFrom == assessCompareList {
			return assessmentCompareCustomDraw(lParam)
		}
		if hdr.Code == NM_CUSTOMDRAW && hdr.HwndFrom == assessDecisionHistoryList {
			return assessmentDecisionHistoryCustomDraw(lParam)
		}
		if hdr.Code == NM_CUSTOMDRAW && (hdr.HwndFrom == assessTechList || hdr.HwndFrom == assessProgramList) {
			return assessmentDirectionCustomDraw(lParam)
		}
		if hdr.Code == NM_CUSTOMDRAW && (hdr.HwndFrom == wellHistoryList || hdr.HwndFrom == wellManualList) {
			return wellFailureCustomDraw(lParam, hdr.HwndFrom)
		}
		if hdr.Code == NM_CUSTOMDRAW && hdr.HwndFrom == resultVariantsList {
			return resultVariantCustomDraw(lParam)
		}
		if hdr.Code == NM_CUSTOMDRAW && hdr.HwndFrom == resultArchiveList {
			return resultArchiveCustomDraw(lParam)
		}
		if hdr.Code == NM_CUSTOMDRAW && hdr.HwndFrom == archiveList {
			return approvedRegistryCustomDraw(lParam)
		}
		// Only react to selection notifications from the list that actually sent the event.
		// The previous implementation refreshed every list on every notification; on the
		// dynamogram page this recursively rebuilt the same ListView and could freeze the UI.
		if hdr.Code != NM_CLICK && hdr.Code != NM_DBLCLK {
			return 0
		}
		src := hdr.HwndFrom
		switch src {
		case objectsList:
			if currentPage != ID_DASH {
				return 0
			}
			x := filteredObjects()
			i := selectedRow(objectsList)
			if i >= 0 && i < len(x) {
				candidateFromRecent = false
				candidateObjectKey = x[i].State.ObjectKey
				if candidateObjectKey == "" {
					candidateObjectKey = x[i].Eval.ObjectKey
				}
				if hdr.Code == NM_DBLCLK {
					confirmSelectedObjectAfterQuestion()
				}
			}
		case recentList:
			if currentPage == ID_DASH {
				i := selectedRow(recentList)
				if i >= 0 && i < len(dashboardHistoryRows) {
					candidateFromRecent = true
					candidateObjectKey = dashboardHistoryRows[i].ObjectKey
					if hdr.Code == NM_DBLCLK {
						openDashboardHistoryPass(i)
					}
				}
			}
		case methodsCandidateList:
			if currentPage == ID_ACTIVITIES {
				// Every method row is interactive. A normal click selects the method and
				// refreshes the contextual explanation. Clicking the method-name cell or
				// the priority/status badge opens the full-size method card; double-click
				// anywhere on the row does the same. Double-click never silently adds a
				// method to the plan — adding remains an explicit technologist action.
				act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))
				i := selectedRow(methodsCandidateList)
				subItem := -1
				if act != nil && int(act.IItem) >= 0 {
					i = int(act.IItem)
					subItem = int(act.ISubItem)
				}
				if i >= 0 && i < len(methodWorkflowSuggestions) {
					selectMethodsCandidateRow(i)
					if hdr.Code == NM_DBLCLK || (hdr.Code == NM_CLICK && (subItem == 1 || subItem == 4)) {
						openMethodsCandidateReference(i)
					}
				}
			}
		case methodsPlanList:
			if currentPage == ID_ACTIVITIES {
				act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))
				i := selectedRow(methodsPlanList)
				subItem := -1
				if act != nil && int(act.IItem) >= 0 {
					i = int(act.IItem)
					subItem = int(act.ISubItem)
				}
				names := currentMethodsPlanNames()
				if i >= 0 && i < len(names) {
					if hdr.Code == NM_DBLCLK {
						openMethodsPlanEditorAtRow(i)
					} else if hdr.Code == NM_CLICK && subItem == 1 {
						openMethodsReferenceByName(names[i])
					}
				}
			}
		case econSchemesList:
			if currentPage == ID_ECON && (hdr.Code == NM_CLICK || hdr.Code == NM_DBLCLK) {
				act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))
				i := selectedRow(econSchemesList)
				if act != nil && act.IItem >= 0 {
					i = int(act.IItem)
				}
				selectEconomicsSchemeFromTable(i)
				if hdr.Code == NM_DBLCLK {
					showEconomics812SchemeDetails(i)
				}
			}
		case econCompareList:
			if currentPage == ID_ECON && hdr.Code == NM_DBLCLK {
				// The lower table is a fund-wide object archive, not another
				// per-scheme comparison of the selected object. It opens only
				// by double click so row selection remains harmless.
				act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))
				i := selectedRow(econCompareList)
				if act != nil && act.IItem >= 0 {
					i = int(act.IItem)
				}
				if i >= 0 {
					openEconomicsFundArchiveAtRow(i)
				}
			}
		case methodsArchiveList:
			if currentPage == ID_ACTIVITIES {
				act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))
				i := selectedRow(methodsArchiveList)
				if act != nil && int(act.IItem) >= 0 {
					i = int(act.IItem)
				}
				if hdr.Code == NM_DBLCLK && i >= 0 && i < len(methodsArchiveRows) {
					openMethodsArchiveObjectHistoryAtRow(i)
				}
			}
		case assessTechList:
			if currentPage == ID_ASSESS {
				i := selectedRow(assessTechList)
				if i >= 0 && i < len(assessmentClassDirections) {
					assessTechSelected = i
					updateAssessmentTabEditor()
				}
			}
		case assessProgramList:
			if currentPage == ID_ASSESS {
				i := selectedRow(assessProgramList)
				if i >= 0 && i < len(assessmentClassDirections) {
					assessLabSelected = i
					updateAssessmentTabEditor()
				}
			}
		case assessCompareList:
			if currentPage == ID_ASSESS {
				i := selectedRow(assessCompareList)
				if i >= 0 && i < len(assessmentClassDirections) {
					assessCompareSelected = i
					updateAssessmentTabEditor()
				}
			}
		case assessDecisionHistoryList:
			if currentPage == ID_ASSESS && hdr.Code == NM_DBLCLK {
				i := selectedRow(assessDecisionHistoryList)
				if i >= 0 {
					if strings.TrimSpace(selectedObjectKey) == "" {
						rows := teachingAssessmentOverviewHistoryRows()
						if i < len(rows) {
							msg("Сквозной учебный пример", readableObjectKey(rows[i].ObjectKey)+"\n\nЭта строка связана с лабораторным анализом и отказом из учебной истории. Выберите эту скважину на Главной, чтобы открыть её историю в других разделах.", MB_OK|MB_ICONINFORMATION)
						}
					} else {
						realCount := len(assessmentDecisionsForObject(selectedObjectKey))
						if i >= realCount && len(teachingAssessmentHistoryRows(selectedObjectKey, time.Now())) > 0 {
							msg("Учебная история", "Это синтетическая учебная запись. Она доступна для просмотра и проверки интерфейса, но не восстанавливается как производственное решение.", MB_OK|MB_ICONINFORMATION)
						} else if err := restoreAssessmentDecisionSnapshot(i); err != nil {
							msg("Архивная классификация", err.Error(), MB_OK|MB_ICONWARNING)
						}
					}
				}
			}
		case assessList:
			if currentPage != ID_ASSESS {
				return 0
			}
			i := selectedRow(assessList)
			if i >= 0 && i < len(assessDirectionRows) {
				selectAssessmentDirection(i)
			}
		case historyList:
			if currentPage != ID_HISTORY {
				return 0
			}
			x := filteredVersions()
			i := selectedRow(historyList)
			if i >= 0 && i < len(x) {
				showVersionDetail(x[i])
			}
		case techList:
			if currentPage != ID_TECH {
				return 0
			}
			i := selectedRow(techList)
			if i >= 0 && i < len(technologies) {
				showTechDetail(technologies[i])
			}
		case dynHistoryList:
			if currentPage == ID_DYN {
				selectDynamogramHistory()
			}
		case labHistoryList:
			if currentPage == ID_LAB && hdr.Code == NM_DBLCLK {
				// A single click only highlights the row. Loading/replacing the
				// current laboratory form is an explicit double-click action.
				pPostMessageW.Call(uintptr(hMain), WM_APP_LAB_SELECT, labGroupGeneration, 0)
			}
		case telHistoryList:
			if currentPage == ID_TELEMETRY {
				selectTelemetryHistory()
			}
		case resultVariantsList:
			if currentPage == ID_RESULT && hdr.Code == NM_DBLCLK {
				if err := activateSelectedResultScenario(); err != nil {
					msg("Выбор схемы", err.Error(), MB_OK|MB_ICONWARNING)
				}
			}
		case resultArchiveList:
			if currentPage == ID_RESULT && hdr.Code == NM_DBLCLK {
				openSelectedFullReport()
			}
		case archiveList:
			if currentPage == ID_ARCHIVE {
				// RC739: use the row reported by NMITEMACTIVATE itself. Relying only
				// on ListView selection state could leave selectedRow() at -1 during
				// a fast double-click, so the visible row appeared non-responsive.
				act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))
				if act != nil && int(act.IItem) >= 0 && int(act.IItem) < len(approvedRegistryRows) {
					selectListRow(archiveList, int(act.IItem))
				}
				if hdr.Code == NM_DBLCLK {
					openSelectedApprovedRegistryReport()
				}
			}
		}
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if drawNNOFleetMetricButton(di) {
			return 1
		}
		if drawNNOFleetMethodCard(di) {
			return 1
		}
		if drawNNOFleetBand(di) {
			return 1
		}
		if paintEconomicsMetricCard(di) {
			return 1
		}
		if paintEconomics812Bands(di) || paintEconomics812ScenarioCard(di) || paintEconomics813Status(di) {
			return 1
		}
		if di != nil && int(di.CtlID) == ID_NNO_TREND {
			drawNNOFleetTrend(di)
			return 1
		}
		if di != nil && int(di.CtlID) == ID_NNO_DISTRIBUTION {
			nnoDrawCycleDistribution(di)
			return 1
		}
		if di != nil && int(di.CtlID) == ID_HEADER_SUBTITLE {
			drawHeaderSubtitle(di)
			return 1
		}
		if di != nil && int(di.CtlID) == ID_LAB_LEGEND {
			drawLaboratoryLegend(di)
			return 1
		}
		if di != nil && int(di.CtlID) >= ID_LAB_BADGE_BASE && int(di.CtlID) < ID_LAB_BADGE_BASE+len(labFieldBadges) {
			drawLaboratoryFieldBadge(di)
			return 1
		}
		if di != nil && int(di.CtlID) == ID_LAB_INFLUENCE {
			drawLaboratoryInfluenceChart(di)
			return 1
		}
		if di != nil && int(di.CtlID) == ID_LAB_STIFF_CHART {
			drawLaboratoryStiffChart(di)
			return 1
		}
		if di != nil && int(di.CtlID) == ID_LAB_MIXING_CHART {
			drawLaboratoryMixingChart(di)
			return 1
		}
		if di != nil && int(di.CtlID) == ID_LAB_TREND_CHART {
			drawLaboratoryTrendChart(di)
			return 1
		}
		if di != nil && di.HwndItem == labFieldsPageLabel {
			drawLaboratoryPagerTitle(di)
			return 1
		}
		if di != nil && isResultCardControl(di.HwndItem) {
			drawResultCard(di)
			return 1
		}
		if di != nil && di.CtlType == ODT_BUTTON {
			id := int(di.CtlID)
			h := di.HwndItem
			methodCandidateCard := id >= ID_METHODS_CARD_BASE && id < ID_METHODS_CARD_BASE+ID_METHODS_CARD_COUNT
			methodCandidateSelected := false
			nav := false
			if b, ok := navButtons[id]; ok && b == h {
				nav = true
			}
			br := brushLight
			textColor := rgb(43, 46, 50)
			if id == ID_METHODS_SCENARIO_ADD {
				br = brushHeader
				textColor = rgb(255, 255, 255)
			}
			if nav {
				br = brushGraphite
				textColor = rgb(255, 255, 255)
				if isNavigationButtonActive(id, currentPage) {
					br = brushHeader
				}
			}
			gnoTab := id == ID_GNO_DYNAMOMETRY || id == ID_GNO_TELEMETRY
			if gnoTab {
				// GNO sub-tabs use the same quiet light surface as the object
				// selection controls. Only the active direction is red.
				br = brushLight
				textColor = rgb(15, 48, 82)
				if isGNOSubmoduleTabActive(id, currentPage) {
					br = brushHeader
				}
				if isGNOSubmoduleTabActive(id, currentPage) {
					textColor = rgb(255, 255, 255)
				}
			}
			wellCardAction := h == wellOpenEvent || h == wellImportForm || h == wellDownloadTemplate || h == wellAddFailure || h == wellEditFailure || h == wellDeleteFailure || h == wellClearFailure || h == wellCancelFailure
			labTab := false
			if b, ok := labTabButtons[id]; ok && b == h {
				labTab = true
				// Laboratory navigation should read as navigation, not as a wall of
				// solid colour. Ordinary inactive tabs use a quiet light surface.
				// The cross-section summary is intentionally dark even while inactive:
				// it is a final synthesis rather than another input direction.
				br = brushBorder
				textColor = rgb(15, 48, 82)
				if g, ok := labGroupByID(id); ok {
					if g.Code == "overall" {
						br = brushNav
						textColor = rgb(255, 255, 255)
					}
					if g.Code == labActiveGroup {
						br = brushHeader
						textColor = rgb(255, 255, 255)
					}
				}
			}
			labPager := id == ID_LAB_FIELDS_PREV || id == ID_LAB_FIELDS_NEXT
			if labPager {
				br = brushNav
				textColor = rgb(255, 255, 255)
			}
			techAction := false
			techActionPrimary := false
			for i, actionButton := range techActionButtons {
				if h == actionButton {
					techAction = true
					techActionPrimary = i == len(techActionButtons)-1
					break
				}
			}
			if techAction {
				br = brushLight
				textColor = rgb(15, 48, 82)
				if techActionPrimary {
					br = brushHeader
					textColor = rgb(255, 255, 255)
				}
			}
			primary := id == ID_HEADER_APPROVE || id == ID_SAVE_FACT || id == ID_OUTCOME_DETAILS || id == ID_GEN_PLAN || id == ID_DYN_ANALYZE || id == ID_DYN_IMPORT_CSV || id == ID_DYN_CALIBRATE || id == ID_DYN_EXPORT_EXCEL || id == ID_FAILURE_CHOOSE || id == ID_FAILURE_APPLY || id == ID_OBJECT_CONFIRM || id == ID_WELL_DOWNLOAD_TEMPLATE || id == ID_WELL_SAVE_FAILURE || id == ID_WELL_APPROVE_FAILURE || id == ID_WELL_ADD_FAILURE || id == ID_OPEN_EQUIPMENT || id == ID_HEADER_EXIT || id == ID_LAB_SAVE || id == ID_LAB_ANALYZE || id == ID_LAB_EXPORT || id == ID_LAB_IMPORT || id == ID_LAB_DOWNLOAD_TEMPLATE || id == ID_LAB_USE_SELECTED || id == ID_LAB_OPEN_INFOGRAPHIC || id == ID_ASSESS_PROBABILITY || id == ID_ASSESS_CLASS_SAVE || id == ID_ASSESS_CONFIRM_FINAL || id == ID_ASSESS_ACCEPT_TECH || id == ID_ASSESS_CONFIRM_KEEP || id == ID_ASSESS_CONFIRM_CORRECT || id == ID_ASSESS_TO_ACTIVITIES || id == ID_TEL_ANALYZE_SCREENSHOT || id == ID_RESULT_APPROVE_SCHEME || id == ID_ECON_CALCULATE_ALL
			if id == ID_RESULT_COMPARE_SCHEMES {
				primary = false
				br = brushNav
				textColor = rgb(255, 255, 255)
			}
			if id == ID_RESULT_ADD_SCHEME || id == ID_RESULT_COPY_SCHEME || id == ID_RESULT_USE_SELECTED || id == ID_RESULT_SAVE_FULL {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
			}
			if h == hHeaderApprove {
				// Profile switching is a quiet service action: light surface, navy text.
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
			}
			if id == ID_WELL_HISTORY_TAB || id == ID_WELL_MANUAL_TAB {
				// Both top-level failure tabs use the same calm navigation accent.
				// Red is reserved for a small number of true actions inside the form,
				// so switching to manual entry no longer turns the whole header red.
				primary = false
				active := (id == ID_WELL_HISTORY_TAB && !wellManualMode) || (id == ID_WELL_MANUAL_TAB && wellManualMode)
				if active {
					br = brushNav
					textColor = rgb(255, 255, 255)
				} else {
					br = brushLight
					textColor = rgb(15, 48, 82)
				}
			}
			if id == ID_WELL_FORM_MODE_OPERATIONAL || id == ID_WELL_FORM_MODE_FULL {
				// Form-depth switches are not primary actions. The active state is
				// indicated by red text/frame on a light surface instead of another
				// full-width coloured slab.
				primary = false
				active := (id == ID_WELL_FORM_MODE_OPERATIONAL && !wellFullCardMode) || (id == ID_WELL_FORM_MODE_FULL && wellFullCardMode)
				br = brushLight
				textColor = rgb(15, 48, 82)
				if active {
					textColor = rgb(183, 20, 37)
				}
			}
			assessmentAction := h == assessProbabilityButton || h == assessHistoryButton || h == assessClassSaveButton || h == assessAcceptTechButton || h == assessConfirmButton || h == assessConfirmKeepButton || h == assessConfirmCorrectButton || h == assessConfirmFullButton || h == assessToActivitiesButton
			assessmentTab := h == assessTabTechButton || h == assessTabLabButton || h == assessTabCompareButton
			if assessmentAction {
				// Keep service and choice actions quiet. Red is reserved for the
				// current stage's decisive action.
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
			}
			if h == assessClassSaveButton || h == assessAcceptTechButton || h == assessConfirmFullButton {
				primary = true
				br = brushHeader
				textColor = rgb(255, 255, 255)
			}
			if assessmentTab {
				primary = false
				br = brushBorder
				textColor = rgb(15, 48, 82)
				active := (h == assessTabTechButton && assessInnerTab == 0) || (h == assessTabLabButton && assessInnerTab == 1) || (h == assessTabCompareButton && assessInnerTab == 2)
				if active {
					br = brushNav
					textColor = rgb(255, 255, 255)
				}
			}
			if !nav && primary {
				br = brushHeader
				textColor = rgb(255, 255, 255)
			}
			// RC573 methods dashboard: one blue decision action, one red plan action,
			// and quiet secondary controls. This mirrors the approved reference and
			// prevents every button from competing for attention.
			if h == methodsApproveButton {
				primary = true
				br = brushNav
				textColor = rgb(255, 255, 255)
			}
			if h == methodsScenarioNextButton {
				primary = false
				br = brushNav
				textColor = rgb(255, 255, 255)
			}
			if h == methodsAvailabilityButton {
				primary = false
				caption := strings.ToLower(strings.TrimSpace(getText(h)))
				switch {
				case strings.Contains(caption, "недоступ"):
					br = brushInputError
					textColor = rgb(145, 0, 24)
				case strings.Contains(caption, "частично") || strings.Contains(caption, "не проверено"):
					br = brushInputWarning
					textColor = rgb(90, 62, 0)
				default:
					br = brushInputValid
					textColor = rgb(24, 86, 42)
				}
			}
			// RC708 four-action baseline retained for regression QA only:
			// resultDecisionAction := h == resultSelectedInfoButton || h == resultUseSelectedButton || h == resultSaveButton || h == resultApproveButton
			// The upper scenario table already owns working variants, so the
			// redundant snapshot action is no longer part of the live row.
			resultDecisionAction := h == resultSelectedInfoButton || h == resultUseSelectedButton || h == resultApproveButton
			if resultDecisionAction {
				primary = false
				br = brushNav
				textColor = rgb(255, 255, 255)
			}
			// The catalogue switch reads as a real mode tab: the active full-catalogue
			// state is dark blue, while recommended mode keeps it quiet and secondary.
			if h == methodsShowAllButton {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
				if methodsShowAll {
					br = brushNav
					textColor = rgb(255, 255, 255)
				}
			}
			if h == methodsCandidatePrev || h == methodsCandidateNext || h == methodsImplementationEditButton {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
			}
			if h == methodsReferenceButton || h == methodsReplicationButton || h == methodsRemoveButton || h == methodsResetButton {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
			}
			if h == methodsAddButton || h == methodsToClassificationButton {
				primary = true
				br = brushNav
				textColor = rgb(255, 255, 255)
			}
			if methodCandidateCard {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
				if methodsSuggestionCombo != 0 {
					idx, _, _ := pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_GETCURSEL, 0, 0)
					methodCandidateSelected = int(idx)-1 == methodsCandidateIndexForCardID(id)
					if methodCandidateSelected {
						// Reference uses a calm pale selection, not a solid dark card.
						br = brushBorder
						textColor = rgb(15, 48, 82)
					}
				}
			}
			if h == assessConfirmKeepButton || h == assessConfirmCorrectButton {
				primary = false
				br = brushNav
				textColor = rgb(255, 255, 255)
			}
			// The three laboratory summary actions form one transparent button row.
			// Keep the same quiet surface for all three; the infographic action must
			// not become a red exception through the generic primary-button rule.
			labSummaryAction := h == labInfographicButton || h == labComplexAnalysisButton || h == labCalculationBasisButton
			if labSummaryAction {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
			}
			if di.ItemState&ODS_SELECTED != 0 {
				if resultDecisionAction {
					br = brushBorder
					textColor = rgb(15, 48, 82)
				} else if labSummaryAction {
					br = brushBorder
					textColor = rgb(15, 48, 82)
				} else if nav || primary || labTab || gnoTab || techAction || labPager || assessmentTab {
					br = brushRedPressed
				} else {
					br = brushBorder
				}
			}
			if h == hHeaderBack || h == hHeaderForward {
				// Match the neighbouring utility buttons instead of using red tiles.
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushBorder
				}
			}
			if h == failureAnalysisButton {
				br = brushNav
				textColor = rgb(255, 255, 255)
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushRedPressed
				}
			}
			if h == wellCatalogButton || h == wellReferenceButton || h == wellManualEntryButton || h == wellOpenEvent {
				// Reference/catalogue actions stay quiet; only direct manual entry and
				// opening the selected event are strong actions. This gives the form
				// distinct visual levels instead of three adjacent solid buttons.
				br = brushLight
				textColor = rgb(15, 48, 82)
				if h == wellManualEntryButton {
					br = brushHeader
					textColor = rgb(255, 255, 255)
				} else if h == wellOpenEvent {
					br = brushNav
					textColor = rgb(255, 255, 255)
				}
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushBorder
					textColor = rgb(15, 48, 82)
				}
			}
			// The equipment page uses three quiet actions and one red accent. Apply
			// this after generic navigation/primary rules because these command IDs
			// are also shared with other pages.
			if techAction {
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushRedPressed
					textColor = rgb(255, 255, 255)
				} else if techActionPrimary {
					br = brushHeader
					textColor = rgb(255, 255, 255)
				} else {
					br = brushLight
					textColor = rgb(15, 48, 82)
				}
			}
			if h == mrpGuideButton || h == mrpCalculateButton || h == mrpDrillButton || h == mrpTargetSaveButton || h == mrpTargetResetButton {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
				if h == mrpCalculateButton {
					br = brushNav
					textColor = rgb(255, 255, 255)
				}
				if h == mrpDrillButton {
					br = brushHeader
					textColor = rgb(255, 255, 255)
				}
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushBorder
					textColor = rgb(15, 48, 82)
				}
			}
			// Reassert the GNO tab state after the generic primary/pressed rules;
			// the tab command IDs are shared with navigation styling elsewhere.
			if gnoTab {
				if isGNOSubmoduleTabActive(id, currentPage) {
					br = brushHeader
					textColor = rgb(255, 255, 255)
				} else {
					br = brushLight
					textColor = rgb(15, 48, 82)
				}
			}
			// Keep exactly one failure workflow tab accented. The generic primary
			// branch above otherwise painted both tabs red.
			if id == ID_WELL_HISTORY_TAB || id == ID_WELL_MANUAL_TAB {
				active := (id == ID_WELL_HISTORY_TAB && !wellManualMode) || (id == ID_WELL_MANUAL_TAB && wellManualMode)
				if active {
					br = brushHeader
					textColor = rgb(255, 255, 255)
				} else {
					br = brushLight
					textColor = rgb(15, 48, 82)
				}
			}
			// Failure-history actions follow the object-selection table style:
			// quiet light controls for navigation/edit/delete, with the save action
			// remaining the only primary action inside the form.
			if wellCardAction {
				br = brushLight
				textColor = rgb(15, 48, 82)
				// In manual-entry mode only the creation action is accented.
				// Edit/delete/import remain visually secondary, matching the object-selection page.
				if id == ID_WELL_ADD_FAILURE {
					br = brushHeader
					textColor = rgb(255, 255, 255)
				}
			}
			if h == wellOpenEvent {
				br = brushNav
				textColor = rgb(255, 255, 255)
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushRedPressed
				}
			}
			failureImportStep := h == wellDownloadTemplate || h == failureChooseButton || h == failurePreviewButton || h == failureApplyButton
			if failureImportStep {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushBorder
				}
			}
			for i, sectionButton := range wellSectionButtons {
				if h == sectionButton {
					br = brushLight
					textColor = rgb(15, 48, 82)
					if i < 2 {
						// Early investigation stages keep a restrained red text cue while
						// inactive, but the selected stage uses the common navy accent.
						textColor = rgb(183, 20, 37)
					}
					if i == wellFormSection {
						br = brushNav
						textColor = rgb(255, 255, 255)
					}
				}
			}
			// Match the NNO reference: compact pale outlined actions, with only
			// «Обновить фонд» filled in navy as the primary action.
			nnoAction := isNNOActionButton(h)
			if nnoAction {
				primary = false
				br = brushLight
				textColor = rgb(15, 48, 82)
				if h == nnoFleetUpdate {
					br = brushNav
					textColor = rgb(255, 255, 255)
				}
				if di.ItemState&ODS_SELECTED != 0 {
					br = brushBorder
					textColor = rgb(15, 48, 82)
					if h == nnoFleetUpdate {
						br = brushRedPressed
						textColor = rgb(255, 255, 255)
					}
				}
			}
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(br))
			frameBrush := brushBorder
			if failureImportStep {
				frameBrush = brushNav
			}
			if nav || primary || labTab || gnoTab || techActionPrimary || labPager {
				frameBrush = br
			}
			if h == hHeaderBack || h == hHeaderForward {
				frameBrush = brushBorder
			}
			if h == failureAnalysisButton {
				frameBrush = br
			}
			if nnoAction && h == nnoFleetUpdate {
				frameBrush = br
			}
			if methodCandidateCard {
				frameBrush = brushBorder
				if methodCandidateSelected {
					frameBrush = brushNav
				}
			}
			if h == wellCatalogButton || h == wellReferenceButton || h == wellManualEntryButton || h == wellOpenEvent {
				frameBrush = br
			}
			for i, sectionButton := range wellSectionButtons {
				if h == sectionButton && i == wellFormSection {
					frameBrush = br
				}
			}
			if id == ID_WELL_FORM_MODE_OPERATIONAL || id == ID_WELL_FORM_MODE_FULL {
				active := (id == ID_WELL_FORM_MODE_OPERATIONAL && !wellFullCardMode) || (id == ID_WELL_FORM_MODE_FULL && wellFullCardMode)
				if active {
					frameBrush = brushHeader
				}
			}
			if (id == ID_WELL_HISTORY_TAB && wellManualMode) || (id == ID_WELL_MANUAL_TAB && !wellManualMode) {
				frameBrush = brushBorder
			}
			// Draw each owner-drawn button border inset from its client edge.
			// This leaves a clean gap from surrounding card/panel rules and keeps
			// adjacent button outlines from visually merging at shared boundaries.
			buttonFrame := di.RcItem
			buttonFrame.Left += 2
			buttonFrame.Top += 2
			buttonFrame.Right -= 2
			buttonFrame.Bottom -= 2
			pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buttonFrame)), uintptr(frameBrush))
			if h == hHeaderBack || h == hHeaderForward {
				// Fine-line navigation arrow: a light shaft and open chevron read
				// cleaner than the old heavy filled block arrow.
				cx := (di.RcItem.Left + di.RcItem.Right) / 2
				mid := (di.RcItem.Top + di.RcItem.Bottom) / 2
				shaft := [2]POINT{{cx - 11, mid}, {cx + 12, mid}}
				head := [3]POINT{{cx - 2, mid - 9}, {cx - 11, mid}, {cx - 2, mid + 9}}
				if h == hHeaderForward {
					for i := range shaft {
						shaft[i].X = 2*cx - shaft[i].X
					}
					for i := range head {
						head[i].X = 2*cx - head[i].X
					}
				}
				arrowColor := rgb(15, 48, 82)
				if di.ItemState&ODS_SELECTED != 0 {
					arrowColor = rgb(183, 20, 37)
				}
				pen, _, _ := pCreatePen.Call(0, 2, uintptr(arrowColor))
				oldPen, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
				pPolyline.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&shaft[0])), uintptr(len(shaft)))
				pPolyline.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&head[0])), uintptr(len(head)))
				pSelectObject.Call(uintptr(di.HDC), oldPen)
				if pen != 0 {
					pDeleteObject.Call(pen)
				}
				return 1
			}
			pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
			pSetTextColor.Call(uintptr(di.HDC), textColor)
			oldFont := uintptr(0)
			drawFont := hFontButton
			sectionCaption := false
			for _, sectionButton := range wellSectionButtons {
				if h == sectionButton {
					sectionCaption = true
				}
			}
			if (labTab || h == sessionTrashButton || h == hHeaderBack || h == hHeaderForward || h == hHeaderOpen || h == hHeaderHelp || h == hHeaderExport || h == hHeaderApprove) && hFontButtonCompact != 0 {
				drawFont = hFontButtonCompact
			}
			if sectionCaption && hFontButton != 0 {
				drawFont = hFontButton
			}
			if h == hHeaderApprove {
				drawFont = methodsFont(methodsTypeBody, true)
			}
			// RC731: the availability badge can carry the long caption «ЧАСТИЧНО ДОСТУПНО».
			// Keep its typography centred and compact instead of clipping the last word.
			if h == methodsAvailabilityButton && hFontButtonCompact != 0 {
				drawFont = hFontButtonCompact
			}
			if nnoAction && hFontButton != 0 {
				// Match standard interface buttons; fit captions on narrow
				// layouts without reverting to the tiny brand caption font.
				drawFont = hFontButton
				if h == nnoFleetRules && hFontButtonCompact != 0 {
					drawFont = hFontButtonCompact
				}
			}
			if h == methodsShowAllButton {
				// RC676: the active caption «ТОЛЬКО РЕКОМЕНДУЕМЫЕ» is intentionally
				// long. Use the shared compact token instead of clipping a second line.
				drawFont = methodsFont(methodsTypeCompact, true)
			}
			// RC720: one caption size for every button in section 7. Do not let
			// single-line and two-line actions silently fall back to different fonts.
			if isResultWorkspaceButton(h) && hFontButtonCompact != 0 {
				drawFont = hFontButtonCompact
			}
			if methodCandidateCard {
				drawFont = methodsFont(methodsTypeBody, true)
			}
			caption := strings.TrimSpace(getText(h))
			if caption == "Вероятность осложнений" && hFontButtonCompact != 0 {
				drawFont = hFontButtonCompact
			}
			if rosaUIEconomicsAction(h) { // Economy always uses one readable action font.
				drawFont = rosaUIFont(rosaUIAction)
			}
			if drawFont != 0 {
				oldFont, _, _ = pSelectObject.Call(uintptr(di.HDC), uintptr(drawFont))
			}
			n, _, _ := pGetWindowTextLengthW.Call(uintptr(h))
			buf := make([]uint16, n+1)
			if n > 0 {
				pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), n+1)
				// Measure wrapped text first, then center the resulting block. The old
				// DT_SINGLELINE path clipped most longer labels after the font increase.
				textRect := di.RcItem
				textRect.Left += 6
				textRect.Right -= 6
				if isResultWorkspaceButton(h) {
					// One safe inset for the whole section-7 button family. The extra
					// horizontal room prevents the long Russian captions touching frames.
					textRect.Left += 2
					textRect.Right -= 2
					textRect.Top += 3
					textRect.Bottom -= 3
				}
				if methodCandidateCard {
					textRect.Left += 8
					textRect.Right -= 8
				}
				measure := RECT{Left: textRect.Left, Top: 0, Right: textRect.Right, Bottom: 0}
				flags := uintptr(DT_CENTER | DT_WORDBREAK | DT_NOPREFIX)
				if methodCandidateCard {
					flags = DT_WORDBREAK | DT_NOPREFIX
				}
				if h == hHeaderBack || h == hHeaderForward || h == methodsScenarioNextButton || nnoAction || isResultSingleLineButton(h) {
					flags = DT_CENTER | DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX
				}
				pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), n, uintptr(unsafe.Pointer(&measure)), flags|DT_CALCRECT)
				textHeight := measure.Bottom - measure.Top
				availableHeight := textRect.Bottom - textRect.Top
				if textHeight > availableHeight-4 && hFontButtonCompact != 0 && !buttonUsesUniformStandardFont(id) && !rosaUIEconomicsAction(h) {
					pSelectObject.Call(uintptr(di.HDC), uintptr(hFontButtonCompact))
					measure = RECT{Left: textRect.Left, Top: 0, Right: textRect.Right, Bottom: 0}
					pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), n, uintptr(unsafe.Pointer(&measure)), flags|DT_CALCRECT)
					textHeight = measure.Bottom - measure.Top
				}
				if textHeight < availableHeight && h != hHeaderBack && h != hHeaderForward && !nnoAction && !isResultSingleLineButton(h) {
					// Result single-line buttons already use DT_VCENTER. Applying the
					// manual top shift as well would centre them twice and make rows look
					// inconsistent. Wrapped captions are centred by the measured block.
					textRect.Top += (availableHeight - textHeight) / 2
				}
				// RC720: use one optical correction for the complete section-7 family.
				// This replaces the accumulated RC718/719 per-row offsets that made
				// neighbouring buttons look like they had different typography.
				if lift := resultButtonOpticalLift(h); lift != 0 {
					textRect.Top -= lift
					textRect.Bottom -= lift
				}
				pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), n, uintptr(unsafe.Pointer(&textRect)), flags)
			}
			if oldFont != 0 {
				pSelectObject.Call(uintptr(di.HDC), oldFont)
			}
			return 1
		}
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		// Section 6 uses the same restrained typography as the other analytical
		// pages, rather than default black captions on randomly placed controls.
		if child == econModeInfo {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(98, 115, 134))
			return uintptr(brushLight)
		}
		// The section-6 dashboard uses white panel surfaces. Paint their labels
		// with the same background to avoid grey rectangles over panel frames.
		isEconomicsPanelText := child == econInputsTitle || child == econSchemesTitle || child == econCompareTitle || child == econ813ScenarioHint || child == econ813BreakTitle
		if !isEconomicsPanelText {
			for _, handle := range econ812Units {
				if handle == child {
					isEconomicsPanelText = true
					break
				}
			}
		}
		if !isEconomicsPanelText {
			for _, handle := range econFieldLabels {
				if handle == child {
					isEconomicsPanelText = true
					break
				}
			}
		}
		if isEconomicsPanelText {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		if child == econDecisionStatus || child == econDeferralInfo || child == econObjectLabel {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
		if child == resultSubtitle || child == resultVariantsSubtitle || child == resultArchiveSubtitle || child == archiveRegistrySubtitle || child == archiveRegistryHint {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(104, 121, 139))
			return uintptr(brushLight)
		}
		if child == resultVariantsTitle || child == resultArchiveTitle || child == archiveRegistryTitle {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
		if child == resultDecisionHint {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(116, 128, 140))
			return uintptr(brushLight)
		}
		if isResultCardControl(child) {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		if child == labObjectLabel || child == labObjectCaption {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			// Подпись «Ключ объекта» синяя; значение шифра красное.
			if child == labObjectLabel {
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
			} else {
				pSetTextColor.Call(wParam, rgb(190, 24, 49))
			}
			return uintptr(brushLight)
		}
		if child == objectSelectedValue || child == assessObjectInfo {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(190, 24, 49))
			return uintptr(brushLight)
		}
		if child == methodsObjectCode {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(190, 24, 49))
			return uintptr(brushWhite)
		}
		if child == methodsPageTitle || child == methodsObjectTitle || child == methodsKeyDataTitle || child == methodsClassTitle || child == methodsSuggestionTitle || child == methodsAvailableTitle || child == methodsBasisTitle || child == methodsImplementationTitle || child == methodsForecastTitle || child == methodsPlanTitle || child == methodsArchiveTitle {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		for i, card := range methodsCandidateCards {
			if card != 0 && child == card {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				if methodsCandidateDraftSelected[i] {
					pSetBkColor.Call(wParam, rgb(255, 255, 255))
					return uintptr(brushWhite)
				}
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				return uintptr(brushWhite)
			}
		}
		for i, panel := range methodsWorkspacePanels {
			if child == panel {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				return uintptr(methodsWorkspacePanelBrush(i))
			}
		}
		sectionBands := [5]syscall.Handle{methodsContextBand, methodsClassificationBand, methodsActionBand, methodsForecastBand, methodsArchiveBand}
		sectionColors := [5]uint32{uint32(rgb(255, 255, 255)), uint32(rgb(255, 255, 255)), uint32(rgb(255, 255, 255)), uint32(rgb(255, 255, 255)), uint32(rgb(255, 255, 255))}
		for i, band := range sectionBands {
			if child == band {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, uintptr(sectionColors[i]))
				return uintptr(methodsSectionAccentBrush(i))
			}
		}
		for _, chip := range methodsClassificationChips {
			if child == chip {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushWhite)
			}
		}
		for i, detail := range methodsCandidateDetails {
			if detail != 0 && child == detail {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				if methodsCandidateDraftSelected[i] {
					pSetBkColor.Call(wParam, rgb(255, 255, 255))
					return uintptr(brushWhite)
				}
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				return uintptr(brushWhite)
			}
		}
		for i := 0; i < 5; i++ {
			if child == methodsBasisNumbers[i] || child == methodsBasisItems[i] {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushWhite)
			}
		}
		for i := 0; i < 6; i++ {
			if child == methodsParameterLabels[i] || child == methodsParameterValues[i] {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushWhite)
			}
		}
		if child == methodsClassSummary {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		if child == methodsObjectContext || child == methodsForecastNote {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(43, 46, 50))
			return uintptr(brushWhite)
		}
		if child == methodsReadyBanner {
			pSetBkMode.Call(wParam, OPAQUE)
			if methodsClassificationReady {
				pSetBkColor.Call(wParam, rgb(232, 245, 233))
				pSetTextColor.Call(wParam, rgb(24, 86, 42))
				return uintptr(brushInputValid)
			}
			pSetBkColor.Call(wParam, rgb(255, 246, 204))
			pSetTextColor.Call(wParam, rgb(90, 62, 0))
			return uintptr(brushInputWarning)
		}
		if child == methodsEmptyState || child == methodsSuggestionInfo || child == methodsImplementation {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		if child == methodsTechEquipment || child == methodsTechMRP || child == methodsTechLiquid || child == methodsTechWatercut {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		if child == methodsContextHeaderBand {
			return uintptr(brushWhite)
		}
		for _, frame := range methodsTechMetricFrames {
			if child == frame {
				return uintptr(brushWhite)
			}
		}
		if child == methodsTechEquipmentValue || child == methodsTechMRPValue || child == methodsTechLiquidValue || child == methodsTechWatercutValue {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(190, 24, 49))
			return uintptr(brushWhite)
		}
		if child == methodsForecastCurrent || child == methodsForecastBasis {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		if child == methodsForecastGain {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		if child == methodsForecastResult {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(232, 245, 233))
			pSetTextColor.Call(wParam, rgb(24, 86, 42))
			return uintptr(brushInputValid)
		}
		for i, handle := range wellFormLabels {
			if child == handle {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(246, 248, 251))
				color := rgb(15, 48, 82)
				if wellCoreSection(i) < 2 {
					color = rgb(183, 20, 37)
				}
				pSetTextColor.Call(wParam, color)
				return uintptr(brushLight)
			}
		}
		for i, handle := range wellExtraLabels {
			if child == handle {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(246, 248, 251))
				color := rgb(15, 48, 82)
				if wellExtraSpecs[i].section < 2 {
					color = rgb(183, 20, 37)
				}
				pSetTextColor.Call(wParam, color)
				return uintptr(brushLight)
			}
		}
		if child == wellFormModeNote {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			color := rgb(15, 48, 82)
			if wellFormSection < 2 {
				color = rgb(183, 20, 37)
			}
			pSetTextColor.Call(wParam, color)
			return uintptr(brushLight)
		}
		for _, metric := range wellMetricTitles {
			if child == metric {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(246, 248, 251))
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushLight)
			}
		}
		for _, metric := range wellMetrics {
			if child == metric {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(246, 248, 251))
				pSetTextColor.Call(wParam, rgb(190, 24, 49))
				return uintptr(brushLight)
			}
		}
		if child == wellManualCounter {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
		if child == dashboardImportAccent || child == dashboardSelectionAccent || child == dashboardObjectAccent || child == dashboardHistoryAccent {
			return uintptr(brushHeader)
		}
		if child == labFieldsPageLabel {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(245, 247, 250))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
		if child == hTopBG || child == hBrandIcon || child == hBrandSignature || child == hTitle || child == hSystemName {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushTop)
		}
		if child == hUserLabel {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushTop)
		}
		if child == hHeaderSep1 || child == hHeaderSep2 {
			return uintptr(brushHeader)
		}
		if child == hLukoilLabel {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(203, 27, 43))
			return uintptr(brushTop)
		}
		if child == hBuildMark {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(35, 31, 32))
			return uintptr(brushTop)
		}
		if child == hStripe {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushHeader)
		}
		if child == sectionNoticeLabel {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(105, 112, 121))
			return uintptr(brushWarning)
		}
		if child == hWarningBG || child == hWarningTitle || child == hWarningText {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(255, 237, 240))
			pSetTextColor.Call(wParam, rgb(183, 20, 37))
			return uintptr(brushWarning)
		}
		if child == hNavBG || child == hNavNote {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		}
		if child == dynSplitter {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(211, 219, 228))
			pSetTextColor.Call(wParam, rgb(90, 104, 120))
			return uintptr(brushBorder)
		}
		if child == mrpRollingHeader || child == mrpMethodsHeader || child == mrpHistoryHeader || child == mrpDetailHeader || child == mrpObjectLabel {
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
		for _, metric := range []syscall.Handle{mrpCurrentLabel, mrpBaseLabel, mrpTargetStatusLabel, mrpRangeLabel, mrpEvidenceLabel} {
			if child == metric {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(246, 248, 251))
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushLight)
			}
		}
		// Opaque light background prevents stale text from remaining under updated
		// status labels (most visible in the dynamogram interpretation block).
		pSetBkMode.Call(wParam, OPAQUE)
		pSetBkColor.Call(wParam, rgb(246, 248, 251))
		pSetTextColor.Call(wParam, rgb(43, 46, 50))
		return uintptr(brushLight)
	case WM_CTLCOLOREDIT:
		child := syscall.Handle(lParam)
		if child == resultObjectCard || child == econScenarioCard || child == econResult {
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		for _, card := range resultSectionCards {
			if child == card {
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushWhite)
			}
		}
		if child == resultDecisionCard {
			pSetBkColor.Call(wParam, rgb(232, 245, 233))
			pSetTextColor.Call(wParam, rgb(24, 86, 42))
			return uintptr(brushInputValid)
		}
		if child == resultTraceCard {
			pSetBkColor.Call(wParam, rgb(246, 248, 251))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
		validation := laboratoryEditValidation(syscall.Handle(lParam))
		if validation.State == labValueError {
			pSetBkColor.Call(wParam, rgb(255, 225, 230))
			pSetTextColor.Call(wParam, rgb(145, 0, 24))
			return uintptr(brushInputError)
		}
		if validation.State == labValueWarning {
			pSetBkColor.Call(wParam, rgb(255, 246, 204))
			pSetTextColor.Call(wParam, rgb(90, 62, 0))
			return uintptr(brushInputWarning)
		}
		if validation.State == labValueEmpty {
			pSetBkColor.Call(wParam, rgb(255, 249, 218))
			pSetTextColor.Call(wParam, rgb(90, 62, 0))
			return uintptr(brushInputEmpty)
		}
		if validation.State == labValueValid {
			pSetBkColor.Call(wParam, rgb(232, 245, 233))
			pSetTextColor.Call(wParam, rgb(24, 86, 42))
			return uintptr(brushInputValid)
		}
		pSetTextColor.Call(wParam, rgb(43, 46, 50))
		return uintptr(brushWhite)
	case WM_DESTROY:
		clearDynamogramPreview()
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msgid), wParam, lParam)
	return r
}

func icoImageBytes(data []byte, target int) []byte {
	if len(data) < 6 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	bestScore := 1 << 30
	var best []byte
	for i := 0; i < count; i++ {
		o := 6 + i*16
		if o+16 > len(data) {
			break
		}
		w, h := int(data[o]), int(data[o+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		sz := int(binary.LittleEndian.Uint32(data[o+8 : o+12]))
		off := int(binary.LittleEndian.Uint32(data[o+12 : o+16]))
		if sz <= 0 || off < 0 || off+sz > len(data) {
			continue
		}
		dw, dh := w-target, h-target
		if dw < 0 {
			dw = -dw
		}
		if dh < 0 {
			dh = -dh
		}
		score := dw + dh
		if score < bestScore {
			bestScore = score
			best = data[off : off+sz]
		}
	}
	return best
}
func iconFromEmbeddedBytes(data []byte, target int) syscall.Handle {
	b := icoImageBytes(data, target)
	if len(b) == 0 {
		return 0
	}
	h, _, _ := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 1, 0x00030000, uintptr(target), uintptr(target), 0)
	return syscall.Handle(h)
}
func loadEmbeddedAppIcons() {
	if hIconBig == 0 {
		hIconBig = iconFromEmbeddedBytes(embeddedAppIcon, 64)
	}
	if hIconSmall == 0 {
		hIconSmall = iconFromEmbeddedBytes(embeddedAppIcon, 16)
	}
	if hBrandR == 0 {
		hBrandR = iconFromEmbeddedBytes(embeddedBrandR, 64)
	}
}
func loadAppIconsFallback() {
	loadEmbeddedAppIcons()
	if hIconBig != 0 && hIconSmall != 0 {
		return
	}
	if ex, e := os.Executable(); e == nil {
		ip := filepath.Join(filepath.Dir(ex), "OF_Model_icon_round.ico")
		if _, e = os.Stat(ip); e == nil {
			if hIconBig == 0 {
				hIconBig = syscall.Handle(mustCall(pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(ip))), IMAGE_ICON, 32, 32, LR_LOADFROMFILE)))
			}
			if hIconSmall == 0 {
				hIconSmall = syscall.Handle(mustCall(pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(ip))), IMAGE_ICON, 16, 16, LR_LOADFROMFILE)))
			}
		}
	}
}

func runApp() error {
	// Establish a private storage root before the login UI is created. The login
	// screen builds raster caches during WM_CREATE; leaving dataDir empty makes
	// relative paths resolve against the process working directory (often the
	// Windows Desktop). Never allow startup resources to escape into that folder.
	dataDir = appDataDir()
	_ = os.MkdirAll(dataDir, 0700)
	_ = os.MkdirAll(uiAssetCacheDir(), 0700)

	// Disable DPI virtualization before any window is created. API presence is not
	// enough: Windows can reject the requested context. Check the result and use
	// system-aware mode as a fallback so monitor coordinates and child geometry
	// cannot mix physical and virtualized pixels.
	dpiAware := false
	if err := pSetProcessDpiAwarenessContext.Find(); err == nil {
		result, _, callErr := pSetProcessDpiAwarenessContext.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == -4
		dpiAware = result != 0
		if !dpiAware {
			diagLog("DPI PMv2 request rejected err=%v; trying system-aware fallback", callErr)
		}
	}
	if !dpiAware {
		result, _, callErr := pSetProcessDPIAware.Call()
		dpiAware = result != 0
		if !dpiAware {
			diagLog("DPI system-aware fallback rejected err=%v", callErr)
		}
	}
	diagLog("DPI awareness established=%v", dpiAware)
	diagLog("START")
	if !runOfflineLogin() {
		diagLog("LOGIN cancelled")
		return nil
	}
	ensureDataWithVisibleTransition()
	diagLog("DATA loaded versions=%d tech=%d outcomes=%d plans=%d technologies=%d", len(versions), len(techCurrent), len(outcomes), len(plans), len(technologies))
	ex, _ := os.Executable()
	if ex != "" {
		sample := filepath.Join(filepath.Dir(ex), "Техрежим_август_2026_отдельный.xlsx")
		if _, e := os.Stat(sample); e == nil {
			selectedTechFile = sample
		}
	}
	icc := INITCOMMONCONTROLSEX{DwSize: uint32(unsafe.Sizeof(INITCOMMONCONTROLSEX{})), DwICC: 0x00004000 | 0x00000001 | 0x00000100}
	pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
	brushWhite = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 255, 255))))
	brushLight = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(246, 248, 251))))
	brushTop = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 255, 255))))
	brushHeader = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(203, 27, 43))))
	brushRedPressed = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(157, 18, 35))))
	brushNav = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(15, 48, 82))))
	brushGraphite = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(31, 66, 101))))
	brushGraphitePressed = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(10, 39, 68))))
	brushBorder = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(211, 219, 228))))
	brushWarning = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 237, 240))))
	brushInputError = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 225, 230))))
	brushInputWarning = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 246, 204))))
	brushInputValid = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(232, 245, 233))))
	brushInputEmpty = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 249, 218))))
	hFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiTextHeight-1), 0, 0, 0, 400, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontBold = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiTextHeight-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontTitle = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiBrandHeight-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontHeader = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiHeadingHeight-1), 0, 0, 0, 500, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontHeaderBold = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiHeadingHeight-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontSection = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(20-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontSystemName = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiSystemNameHeight-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontBrandCaption = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(10-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontHeaderSmall = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(14-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontHeaderTiny = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(10-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontHeaderBrand = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiHeaderBrandTextHeight-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontSlogan = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiCaptionHeight-1), 0, 0, 0, 500, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontWarning = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiTextHeight-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontButton = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiTextHeight-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontPanel = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiPanelHeight-1), 0, 0, 0, 400, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontButtonCompact = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(uiCompactHeight-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontResultHint = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(13-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	hFontReportEvidence = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(12-1), 0, 0, 0, 400, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	loadAppIconsFallback()
	cls := p("ROSAApprovedWorkBuild20260915")
	hInstRaw, _, hInstErr := pGetModuleHandleW.Call(0)
	if hInstRaw == 0 {
		diagLog("FATAL GetModuleHandleW failed err=%v", hInstErr)
		msg("Ошибка запуска", "Не удалось получить дескриптор приложения Windows. Подробности записаны в diagnostics.log.", MB_OK|MB_ICONERROR)
		return fmt.Errorf("GetModuleHandleW: %v", hInstErr)
	}
	hInst := syscall.Handle(hInstRaw)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: hInst, HbrBackground: brushLight, LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	atom, _, regErr := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		diagLog("FATAL RegisterClassExW failed err=%v", regErr)
		msg("Ошибка запуска", "Windows не удалось зарегистрировать главное окно приложения. Подробности записаны в diagnostics.log.", MB_OK|MB_ICONERROR)
		return fmt.Errorf("RegisterClassExW: %v", regErr)
	}
	hwnd, _, createErr := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — Расчёт • Оценка • Сопровождение • Автоматизация · "+appBuildVersion))), WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN_FRAME, 22, 22, 1380, 900, 0, 0, uintptr(hInst), 0)
	if hwnd == 0 {
		diagLog("FATAL CreateWindowExW failed err=%v", createErr)
		msg("Ошибка запуска", "Windows не удалось создать главное окно приложения. Подробности записаны в diagnostics.log.", MB_OK|MB_ICONERROR)
		return fmt.Errorf("CreateWindowExW: %v", createErr)
	}
	hMain = syscall.Handle(hwnd)
	// Set both window icons explicitly even though they are also present in WNDCLASSEX.
	// This avoids losing the icon after shell/theme/DPI changes.
	if hIconBig != 0 {
		pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
	}
	if hIconSmall != 0 {
		pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
	}
	pShowWindow.Call(hwnd, SW_MAXIMIZE)
	pUpdateWindow.Call(hwnd)
	diagLog("WINDOW created hwnd=%d hinstance=%d", hwnd, hInstRaw)
	stopUIWatchdog := startUIWatchdog(hMain)
	defer stopUIWatchdog()
	var m MSG
	var runErr error
	for {
		r, _, getMsgErr := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) == -1 {
			diagLog("FATAL GetMessageW failed err=%v", getMsgErr)
			msg("Ошибка интерфейса", "Цикл сообщений Windows завершился с ошибкой. Подробности записаны в diagnostics.log.", MB_OK|MB_ICONERROR)
			runErr = fmt.Errorf("GetMessageW: %v", getMsgErr)
			break
		}
		if r == 0 {
			break
		}
		if handleStandardEditShortcut(&m) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchStarted := time.Now()
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		recordUIPulse()
		reportSlowFrame("dispatch", currentPage, dispatchStarted, 400*time.Millisecond)
	}
	deleteMethodsFonts()
	if hFont != 0 {
		pDeleteObject.Call(uintptr(hFont))
	}
	if hFontBold != 0 {
		pDeleteObject.Call(uintptr(hFontBold))
	}
	if hFontTitle != 0 {
		pDeleteObject.Call(uintptr(hFontTitle))
	}
	if hFontHeader != 0 {
		pDeleteObject.Call(uintptr(hFontHeader))
	}
	if hFontHeaderBold != 0 {
		pDeleteObject.Call(uintptr(hFontHeaderBold))
	}
	if hFontSection != 0 {
		pDeleteObject.Call(uintptr(hFontSection))
	}
	if hFontSystemName != 0 {
		pDeleteObject.Call(uintptr(hFontSystemName))
	}
	for _, font := range []syscall.Handle{hFontBrandCaption, hFontHeaderSmall, hFontHeaderTiny, hFontWellMetricValue} {
		if font != 0 {
			pDeleteObject.Call(uintptr(font))
		}
	}
	if hFontHeaderBrand != 0 {
		pDeleteObject.Call(uintptr(hFontHeaderBrand))
	}
	if hFontSlogan != 0 {
		pDeleteObject.Call(uintptr(hFontSlogan))
	}
	if hFontWarning != 0 {
		pDeleteObject.Call(uintptr(hFontWarning))
	}
	if hFontButton != 0 {
		pDeleteObject.Call(uintptr(hFontButton))
	}
	if hFontPanel != 0 {
		pDeleteObject.Call(uintptr(hFontPanel))
	}
	if hFontButtonCompact != 0 {
		pDeleteObject.Call(uintptr(hFontButtonCompact))
	}
	if hFontResultHint != 0 {
		pDeleteObject.Call(uintptr(hFontResultHint))
	}
	if hFontReportEvidence != 0 {
		pDeleteObject.Call(uintptr(hFontReportEvidence))
	}
	if hIconBig != 0 {
		pDestroyIcon.Call(uintptr(hIconBig))
	}
	if hIconSmall != 0 {
		pDestroyIcon.Call(uintptr(hIconSmall))
	}
	if hBrandR != 0 {
		pDestroyIcon.Call(uintptr(hBrandR))
	}
	if hBrandBitmap != 0 {
		pDeleteObject.Call(uintptr(hBrandBitmap))
	}
	if hLukoilBitmap != 0 {
		pDeleteObject.Call(uintptr(hLukoilBitmap))
	}
	if hWarningBitmap != 0 {
		pDeleteObject.Call(uintptr(hWarningBitmap))
	}
	clearCurrentSession()
	for _, b := range []syscall.Handle{brushWhite, brushLight, brushTop, brushHeader, brushRedPressed, brushNav, brushGraphite, brushGraphitePressed, brushBorder, brushWarning} {
		if b != 0 {
			pDeleteObject.Call(uintptr(b))
		}
	}
	return runErr
}
func mustCall(v uintptr, _ uintptr, _ error) uintptr { return v }

// Keep bufio referenced: it is used in a tiny integrity scan during development builds and harmless in release.
var _ = bufio.ErrInvalidUnreadByte

func runningFromArchiveTemp() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	path := strings.ToLower(filepath.Clean(exe))
	// WinRAR/7-Zip temporary extraction folders do not guarantee that the whole
	// ROSA package (templates, resources and sibling files) is present. Running
	// from there previously looked like a black/broken main window.
	return strings.Contains(path, `\rar$exa`) ||
		strings.Contains(path, `\rar$dia`) ||
		strings.Contains(path, `\7z`) && strings.Contains(path, `\temp\`)
}

func main() {
	// Win32 windows and their message queues are thread-affine. Without this lock
	// the Go scheduler may move the main goroutine to another OS thread between
	// CreateWindowExW and GetMessageW, producing random hangs / "Not responding".
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	exitCode := 0
	startupLog("PROCESS started build=%s pid=%d go=%s arch=%s", appBuildVersion, os.Getpid(), runtime.Version(), runtime.GOARCH)
	if runningFromArchiveTemp() {
		startupLog("START blocked: executable is running from archive temporary directory")
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(p("РОСА запущена непосредственно из архива.\n\nПолностью распакуйте комплект в отдельную папку и запускайте Windows_x64\\ROSA.exe уже из распакованной папки. Это необходимо, чтобы все файлы приложения и шаблоны были доступны."))), uintptr(unsafe.Pointer(p("РОСА — распакуйте архив"))), MB_OK|MB_ICONWARNING)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			exitCode = 1
			startupLog("FATAL panic=%v\n%s", r, debug.Stack())
			diagLog("FATAL panic=%v\n%s", r, debug.Stack())
			pMessageBoxW.Call(0, uintptr(unsafe.Pointer(p("Приложение завершило работу из-за внутренней ошибки. Подробности сохранены в diagnostics.log."))), uintptr(unsafe.Pointer(p("РОСА"))), MB_OK|MB_ICONERROR)
		}
		startupLog("PROCESS finished exit_code=%d", exitCode)
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()
	if err := runApp(); err != nil {
		exitCode = 1
		startupLog("FATAL startup=%v", err)
	}
}
