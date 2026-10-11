//go:build windows

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	ID_DYN                     = 113
	ID_DYN_CHOOSE              = 215
	ID_DYN_ANALYZE             = 216
	ID_DYN_PREVIEW             = 217
	ID_DYN_CALIBRATE           = 218
	ID_DYN_EXPORT              = 219
	ID_DYN_FOR_OBJECT          = 220
	ID_DYN_IMPORT_CSV          = 221
	ID_DYN_CONFIRM             = 280
	ID_DYN_REJECT              = 281
	ID_DYN_DELETE              = 282
	ID_DYN_CLEAR               = 283
	ID_DYN_OPEN_SOURCE         = 298
	ID_DYN_OPEN_RECONSTRUCTION = 299
	ID_DYN_EXPORT_EXCEL        = 300
	SS_NOTIFY                  = 0x00000100
	WM_LBUTTONDOWN             = 0x0201
	WM_LBUTTONUP               = 0x0202
	WM_MOUSEMOVE               = 0x0200
)

var (
	dynFileLabel                syscall.Handle
	dynSummary                  syscall.Handle
	dynCapabilities             syscall.Handle
	dynDetail                   syscall.Handle
	dynObjectEdit               syscall.Handle
	dynXMin                     syscall.Handle
	dynXMax                     syscall.Handle
	dynYMin                     syscall.Handle
	dynYMax                     syscall.Handle
	dynXUnit                    syscall.Handle
	dynYUnit                    syscall.Handle
	dynHistoryList              syscall.Handle
	dynAnalyzeButton            syscall.Handle
	dynSourceImage              syscall.Handle
	dynSourceBitmap             syscall.Handle
	dynPreviewImage             syscall.Handle
	dynPreviewBitmap            syscall.Handle
	dynSourceTitle              syscall.Handle
	dynImageTitle               syscall.Handle
	dynTextTitle                syscall.Handle
	dynOpenSourceButton         syscall.Handle
	dynOpenReconstructionButton syscall.Handle
	dynConfirmButton            syscall.Handle
	dynRejectButton             syscall.Handle
	dynCalibrationTitle         syscall.Handle
	dynXMinLabel                syscall.Handle
	dynXMaxLabel                syscall.Handle
	dynYMinLabel                syscall.Handle
	dynYMaxLabel                syscall.Handle
	dynXUnitLabel               syscall.Handle
	dynYUnitLabel               syscall.Handle
	dynCalibrateButton          syscall.Handle
	dynHistoryTitle             syscall.Handle
	dynDeleteButton             syscall.Handle
	dynClearButton              syscall.Handle
	dynSplitter                 syscall.Handle
	dynSplitterOldProc          uintptr
	dynSplitterCallback         uintptr
	dynSplitRatio               = 0.43
	dynSplitDragging            bool
	dynLastMouseX               int32
	dynLastContentW             int32
	selectedDynFile             string
	selectedDynPDFPage          = 1
	currentDyn                  *DynamogramAnalysis
	currentDynPoints            []DynamogramPoint
	dynHistory                  []DynamogramAnalysis
)

var (
	pSetWindowLongPtrDyn = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcDyn   = user32.NewProc("CallWindowProcW")
	pSetCaptureDyn       = user32.NewProc("SetCapture")
	pReleaseCaptureDyn   = user32.NewProc("ReleaseCapture")
	pGetCursorPosDyn     = user32.NewProc("GetCursorPos")
)

func dynCursorX() int32 {
	var pt POINT
	if r, _, _ := pGetCursorPosDyn.Call(uintptr(unsafe.Pointer(&pt))); r != 0 {
		return pt.X
	}
	return 0
}

func dynSplitterWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_LBUTTONDOWN:
		dynSplitDragging = true
		dynLastMouseX = dynCursorX()
		pSetCaptureDyn.Call(uintptr(hwnd))
		return 0
	case WM_MOUSEMOVE:
		if dynSplitDragging && dynLastContentW > 0 {
			x := dynCursorX()
			delta := x - dynLastMouseX
			if delta != 0 {
				left := int32(float64(dynLastContentW)*dynSplitRatio) + delta
				left = max32(300, left)
				left = min32(dynLastContentW-340, left)
				if dynLastContentW > 0 {
					dynSplitRatio = float64(left) / float64(dynLastContentW)
				}
				dynLastMouseX = x
				var r RECT
				pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&r)))
				layoutDynamogram(r.Right-r.Left, r.Bottom-r.Top)
			}
			return 0
		}
	case WM_LBUTTONUP:
		if dynSplitDragging {
			dynSplitDragging = false
			pReleaseCaptureDyn.Call()
			refreshDynamogramPreviewSize()
			return 0
		}
	}
	if dynSplitterOldProc != 0 {
		r, _, _ := pCallWindowProcDyn.Call(dynSplitterOldProc, uintptr(hwnd), uintptr(msg), wParam, lParam)
		return r
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func initDynSplitter() {
	if dynSplitter == 0 || dynSplitterOldProc != 0 {
		return
	}
	if dynSplitterCallback == 0 {
		dynSplitterCallback = syscall.NewCallback(dynSplitterWndProc)
	}
	old, _, _ := pSetWindowLongPtrDyn.Call(uintptr(dynSplitter), ^uintptr(3), dynSplitterCallback) // GWLP_WNDPROC = -4
	dynSplitterOldProc = old
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func layoutDynamogram(w, h int32) {
	if dynPreviewImage == 0 || dynDetail == 0 {
		return
	}
	left := int32(242)
	contentW := max32(720, w-left-28)
	right := left + contentW
	dynLastContentW = contentW
	gap := int32(10)
	// Three independent panes: original image, reconstructed contour and interpretation.
	// The text pane is slightly wider because it must remain readable at 125/150% DPI.
	imageW := int32(float64(contentW) * 0.285)
	if imageW < 245 {
		imageW = 245
	}
	textW := contentW - 2*imageW - 2*gap
	if textW < 300 {
		textW = 300
		imageW = max32(210, (contentW-textW-2*gap)/2)
	}
	x1 := left
	x2 := x1 + imageW + gap
	x3 := x2 + imageW + gap
	paneY := py(478)
	paneH := int32(164)

	setPos(dynFileLabel, left, py(268), contentW, 22)
	setPos(dynCapabilities, left, py(330), contentW, 82)
	setPos(dynSummary, left, py(420), contentW, 26)
	openW := min32(132, max32(104, imageW/2))
	setPos(dynSourceTitle, x1, py(452), imageW-openW-6, 22)
	setPos(dynOpenSourceButton, x1+imageW-openW, py(446), openW, 28)
	setPos(dynImageTitle, x2, py(452), imageW-openW-6, 22)
	setPos(dynOpenReconstructionButton, x2+imageW-openW, py(446), openW, 28)
	setPos(dynTextTitle, x3, py(452), textW, 22)
	setPos(dynSourceImage, x1, paneY, imageW, paneH)
	setPos(dynPreviewImage, x2, paneY, imageW, paneH)
	setPos(dynDetail, x3, paneY, textW, paneH)

	calY := int32(650) + max32(0, paneH-164)
	setPos(dynConfirmButton, left, py(calY), 190, 42)
	setPos(dynRejectButton, left+200, py(calY), 232, 42)
	calX := left + 446
	setPos(dynCalibrationTitle, calX, py(calY), 270, 22)
	fieldY := calY + 40
	historyGap := int32(44)
	if contentW >= 900 {
		setPos(dynXMinLabel, calX, py(fieldY+5), 38, 22)
		setPos(dynXMin, calX+40, py(fieldY), 48, 30)
		setPos(dynXMaxLabel, calX+92, py(fieldY+5), 42, 22)
		setPos(dynXMax, calX+136, py(fieldY), 48, 30)
		setPos(dynXUnitLabel, calX+188, py(fieldY+5), 18, 22)
		setPos(dynXUnit, calX+208, py(fieldY), 40, 30)
		setPos(dynYMinLabel, calX+258, py(fieldY+5), 38, 22)
		setPos(dynYMin, calX+298, py(fieldY), 48, 30)
		setPos(dynYMaxLabel, calX+350, py(fieldY+5), 42, 22)
		setPos(dynYMax, calX+394, py(fieldY), 48, 30)
		setPos(dynYUnitLabel, calX+446, py(fieldY+5), 18, 22)
		setPos(dynYUnit, calX+466, py(fieldY), 40, 30)
		setPos(dynCalibrateButton, right-132, py(calY), 132, 34)
	} else {
		calX = left
		fieldY = calY + 50
		setPos(dynCalibrationTitle, calX, py(fieldY), 270, 22)
		fieldY += 28
		setPos(dynXMinLabel, calX, py(fieldY+5), 38, 22)
		setPos(dynXMin, calX+40, py(fieldY), 48, 30)
		setPos(dynXMaxLabel, calX+92, py(fieldY+5), 42, 22)
		setPos(dynXMax, calX+136, py(fieldY), 48, 30)
		setPos(dynXUnitLabel, calX+188, py(fieldY+5), 18, 22)
		setPos(dynXUnit, calX+208, py(fieldY), 40, 30)
		setPos(dynYMinLabel, calX+258, py(fieldY+5), 38, 22)
		setPos(dynYMin, calX+298, py(fieldY), 48, 30)
		setPos(dynYMaxLabel, calX+350, py(fieldY+5), 42, 22)
		setPos(dynYMax, calX+394, py(fieldY), 48, 30)
		setPos(dynYUnitLabel, calX+446, py(fieldY+5), 18, 22)
		setPos(dynYUnit, calX+466, py(fieldY), 40, 30)
		setPos(dynCalibrateButton, right-132, py(fieldY+36), 132, 38)
		historyGap = 84
	}
	historyY := fieldY + historyGap
	setPos(dynHistoryTitle, left, py(historyY+8), 230, 22)
	setPos(dynDeleteButton, right-474, py(historyY), 232, 38)
	setPos(dynClearButton, right-232, py(historyY), 232, 38)
	setPos(dynHistoryList, left, py(historyY+44), contentW, max32(70, h-py(historyY+44)-20))
}

func refreshDynamogramPreviewSize() {
	if currentDyn == nil {
		return
	}
	setDynamogramImages(currentDyn)
}

func buildDynamogramUI() {
	buildGNOSubmoduleTabs(ID_DYN)
	pageTitle(ID_DYN, "Состояние оборудования — динамометрический контроль", 242, 148, 980, 32)
	labelEllipsis(ID_DYN, "Источник: изображение динамограммы или цифровой массив CSV/TXT. Все операции выполняются локально.", 242, 182, 1010, 24)

	button(ID_DYN, "Загрузить изображение", 242, 214, 178, 48, ID_DYN_CHOOSE)
	button(ID_DYN, "Импорт CSV/TXT", 430, 214, 144, 48, ID_DYN_IMPORT_CSV)
	dynAnalyzeButton = button(ID_DYN, "Повторить анализ", 584, 214, 150, 48, ID_DYN_ANALYZE)
	button(ID_DYN, "Контроль изображения", 744, 214, 186, 48, ID_DYN_PREVIEW)
	button(ID_DYN, "Экспорт CSV", 940, 214, 140, 48, ID_DYN_EXPORT)
	button(ID_DYN, "Экспорт Excel", 1090, 214, 162, 48, ID_DYN_EXPORT_EXCEL)

	dynFileLabel = labelEllipsis(ID_DYN, "Источник не выбран", 242, 268, 1010, 22)
	label(ID_DYN, "Объект / ключ", 242, 298, 105, 24)
	dynObjectEdit = edit(ID_DYN, "", 352, 293, 350, 30, false, false, 0)
	labelEllipsis(ID_DYN, "При заданном ключе результат сохраняется в истории объекта.", 718, 298, 500, 24)

	dynCapabilities = edit(ID_DYN, dynamogramCapabilityText(nil), 242, 330, 1010, 82, true, true, 0)
	dynSummary = labelEllipsis(ID_DYN, "Выберите изображение или импортируйте CSV/TXT.", 242, 420, 1010, 26)
	dynSourceTitle = label(ID_DYN, "Исходное изображение", 242, 452, 260, 22)
	dynImageTitle = label(ID_DYN, "Перерисованный контур", 520, 452, 260, 22)
	dynTextTitle = label(ID_DYN, "Техническая интерпретация", 800, 452, 390, 22)
	dynOpenSourceButton = button(ID_DYN, "Увеличить", 380, 446, 122, 28, ID_DYN_OPEN_SOURCE)
	dynOpenReconstructionButton = button(ID_DYN, "Увеличить", 658, 446, 122, 28, ID_DYN_OPEN_RECONSTRUCTION)
	dynSourceImage = create("STATIC", "", WS_CHILD|WS_BORDER|SS_BITMAP|SS_CENTERIMAGE|SS_NOTIFY, 242, py(478), 280, 164, hMain, ID_DYN_OPEN_SOURCE)
	addPageControl(ID_DYN, dynSourceImage)
	dynPreviewImage = create("STATIC", "", WS_CHILD|WS_BORDER|SS_BITMAP|SS_CENTERIMAGE|SS_NOTIFY, 532, py(478), 280, 164, hMain, ID_DYN_OPEN_RECONSTRUCTION)
	addPageControl(ID_DYN, dynPreviewImage)
	dynDetail = edit(ID_DYN, "После загрузки здесь появятся наблюдения, ограничения метода и рекомендации.", 822, 478, 430, 164, true, true, 0)

	dynConfirmButton = button(ID_DYN, "Контур подтверждён", 242, 650, 190, 44, ID_DYN_CONFIRM)
	dynRejectButton = button(ID_DYN, "Контур распознан неверно", 442, 650, 232, 44, ID_DYN_REJECT)
	dynCalibrationTitle = label(ID_DYN, "Оси: X — ход; Y — нагрузка", 690, 654, 270, 22)
	dynXMinLabel = label(ID_DYN, "X min", 802, 654, 45, 22)
	dynXMin = edit(ID_DYN, "0", 848, 649, 56, 30, false, false, 0)
	dynXMaxLabel = label(ID_DYN, "X max", 909, 654, 48, 22)
	dynXMax = edit(ID_DYN, "", 958, 649, 56, 30, false, false, 0)
	dynXUnit = edit(ID_DYN, "м", 1020, 649, 46, 30, false, false, 0)
	dynXUnitLabel = label(ID_DYN, "ед.", 1068, 654, 28, 22)
	dynYMinLabel = label(ID_DYN, "Y min", 1072, 654, 45, 22)
	dynYMin = edit(ID_DYN, "0", 1118, 649, 56, 30, false, false, 0)
	dynYMaxLabel = label(ID_DYN, "Y max", 1180, 654, 48, 22)
	dynYMax = edit(ID_DYN, "", 1228, 649, 54, 30, false, false, 0)
	dynYUnitLabel = label(ID_DYN, "ед.", 1072, 686, 45, 22)
	dynYUnit = edit(ID_DYN, "кН", 1118, 681, 56, 30, false, false, 0)
	dynCalibrateButton = button(ID_DYN, "Применить шкалы", 1184, 681, 140, 40, ID_DYN_CALIBRATE)

	dynHistoryTitle = label(ID_DYN, "История динамограмм", 242, 712, 280, 22)
	dynDeleteButton = button(ID_DYN, "Удалить выбранную запись", 650, 704, 232, 42, ID_DYN_DELETE)
	dynClearButton = button(ID_DYN, "Очистить историю объекта", 894, 704, 232, 42, ID_DYN_CLEAR)
	dynHistoryList = newList(ID_DYN, 242, 748, 1010, 100, []struct {
		n string
		w int32
	}{
		{"Дата", 130}, {"Объект", 210}, {"Файл", 220}, {"Качество", 90}, {"Уверенность", 100}, {"Проверка", 110}, {"Режим", 160},
	})
}

func clearBitmapControl(control syscall.Handle, bitmap *syscall.Handle) {
	if control != 0 {
		pSendMessageW.Call(uintptr(control), STM_SETIMAGE, IMAGE_BITMAP, 0)
	}
	if bitmap != nil && *bitmap != 0 {
		pDeleteObject.Call(uintptr(*bitmap))
		*bitmap = 0
	}
}

func clearDynamogramPreview() {
	clearBitmapControl(dynSourceImage, &dynSourceBitmap)
	clearBitmapControl(dynPreviewImage, &dynPreviewBitmap)
}

func setBitmapPreview(control syscall.Handle, bitmap *syscall.Handle, imagePath string) {
	if control == 0 || strings.TrimSpace(imagePath) == "" {
		return
	}
	var cr RECT
	pGetClientRect.Call(uintptr(control), uintptr(unsafe.Pointer(&cr)))
	targetW := int(max32(120, cr.Right-cr.Left-6))
	targetH := int(max32(100, cr.Bottom-cr.Top-6))
	bmpPath, err := ensurePreviewBMP(imagePath, targetW, targetH)
	if err != nil {
		diagLog("DYN preview prepare error path=%s err=%v", imagePath, err)
		clearBitmapControl(control, bitmap)
		return
	}
	hb, _, callErr := pLoadImageW.Call(0, uintptr(unsafe.Pointer(p(bmpPath))), IMAGE_BITMAP, 0, 0, LR_LOADFROMFILE)
	if hb == 0 {
		diagLog("DYN preview LoadImage error path=%s err=%v", bmpPath, callErr)
		clearBitmapControl(control, bitmap)
		return
	}
	old, _, _ := pSendMessageW.Call(uintptr(control), STM_SETIMAGE, IMAGE_BITMAP, hb)
	oldHandle := syscall.Handle(0)
	if bitmap != nil {
		oldHandle = *bitmap
		*bitmap = syscall.Handle(hb)
	}
	if old != 0 && old != hb {
		pDeleteObject.Call(old)
	} else if oldHandle != 0 && uintptr(oldHandle) != hb {
		pDeleteObject.Call(uintptr(oldHandle))
	}
}

func setDynamogramImages(a *DynamogramAnalysis) {
	if a == nil {
		clearDynamogramPreview()
		return
	}
	isDigital := strings.Contains(strings.ToLower(a.DetectionMode), "цифров")
	if isDigital {
		setText(dynSourceTitle, "Исходный цифровой график")
		if pth, err := materializeDynPreview(a.PreviewPath); err == nil {
			setBitmapPreview(dynSourceImage, &dynSourceBitmap, pth)
		}
	} else if src, err := materializeDynSource(a); err == nil {
		if strings.EqualFold(filepath.Ext(a.SourceFile), ".pdf") {
			page := a.PDFPage
			if page < 1 {
				page = 1
			}
			setText(dynSourceTitle, fmt.Sprintf("Исходный PDF, страница %d", page))
			if rendered, renderErr := renderPDFPageToPNG(src, page); renderErr == nil {
				setBitmapPreview(dynSourceImage, &dynSourceBitmap, rendered)
			} else {
				diagLog("DYN PDF preview error id=%s err=%v", a.ID, renderErr)
				clearBitmapControl(dynSourceImage, &dynSourceBitmap)
			}
		} else {
			setText(dynSourceTitle, "Исходное изображение")
			setBitmapPreview(dynSourceImage, &dynSourceBitmap, src)
		}
	} else {
		diagLog("DYN source materialize error id=%s err=%v", a.ID, err)
		clearBitmapControl(dynSourceImage, &dynSourceBitmap)
	}
	reconstructed := a.ReconstructionPath
	if strings.TrimSpace(reconstructed) == "" {
		// Digital imports already use PreviewPath as a clean graph. Older image
		// analyses fall back to their control overlay until they are re-analysed.
		reconstructed = a.PreviewPath
	}
	if pth, err := materializeDynReconstruction(reconstructed); err == nil {
		setBitmapPreview(dynPreviewImage, &dynPreviewBitmap, pth)
	} else {
		diagLog("DYN reconstruction materialize error id=%s err=%v", a.ID, err)
		clearBitmapControl(dynPreviewImage, &dynPreviewBitmap)
	}
}

func setDynamogramPreview(previewPNG string) {
	if pth, err := materializeDynPreview(previewPNG); err == nil {
		setBitmapPreview(dynPreviewImage, &dynPreviewBitmap, pth)
	} else {
		diagLog("DYN preview materialize error path=%s err=%v", previewPNG, err)
		clearBitmapControl(dynPreviewImage, &dynPreviewBitmap)
	}
}

func chooseDynamogramFile() string {
	buf := make([]uint16, 32768)
	if selectedDynFile != "" {
		q, _ := syscall.UTF16FromString(selectedDynFile)
		copy(buf, q)
	}
	filter := utf16.Encode([]rune("Динамограмма: PDF или изображение (*.pdf;*.png;*.jpg;*.jpeg;*.bmp)\x00*.pdf;*.png;*.jpg;*.jpeg;*.bmp\x00PDF (*.pdf)\x00*.pdf\x00Изображения (*.png;*.jpg;*.jpeg;*.bmp)\x00*.png;*.jpg;*.jpeg;*.bmp\x00Все файлы (*.*)\x00*.*\x00\x00"))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: p("Выберите PDF или изображение динамограммы"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

var dynPDFPageDialogDone, dynPDFPageDialogAccepted bool
var dynPDFPageDialogCombo syscall.Handle
var dynPDFPageDialogCount int

const (
	ID_DYN_PDF_PAGE_ACCEPT = 330
	ID_DYN_PDF_PAGE_CANCEL = 331
)

func dynamogramPDFPageWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		modalCreate("STATIC", fmt.Sprintf("PDF содержит страниц: %d. Выберите страницу с динамограммой.", dynPDFPageDialogCount), WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 20, 490, 28, syscall.Handle(hwnd), 0)
		dynPDFPageDialogCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|labCBSDropList, 22, 58, 220, 260, syscall.Handle(hwnd), 0)
		for i := 1; i <= dynPDFPageDialogCount; i++ {
			pSendMessageW.Call(uintptr(dynPDFPageDialogCombo), labCBAddString, 0, uintptr(unsafe.Pointer(p(fmt.Sprintf("Страница %d", i)))))
		}
		pSendMessageW.Call(uintptr(dynPDFPageDialogCombo), labCBSetCurSel, 0, 0)
		modalCreate("BUTTON", "Открыть страницу", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 258, 57, 150, 38, syscall.Handle(hwnd), ID_DYN_PDF_PAGE_ACCEPT)
		modalCreate("BUTTON", "Отмена", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 416, 57, 100, 38, syscall.Handle(hwnd), ID_DYN_PDF_PAGE_CANCEL)
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case ID_DYN_PDF_PAGE_ACCEPT:
			selection, _, _ := pSendMessageW.Call(uintptr(dynPDFPageDialogCombo), labCBGetCurSel, 0, 0)
			selectedDynPDFPage = int(int32(selection)) + 1
			dynPDFPageDialogAccepted, dynPDFPageDialogDone = true, true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case ID_DYN_PDF_PAGE_CANCEL:
			dynPDFPageDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		dynPDFPageDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func chooseDynamogramPDFPage(path string) (int, bool, error) {
	count, err := pdfPageCount(path)
	if err != nil {
		return 0, false, err
	}
	if count == 1 {
		return 1, true, nil
	}
	dynPDFPageDialogCount = count
	dynPDFPageDialogDone, dynPDFPageDialogAccepted = false, false
	cls := p("ROSADynamogramPDFPageDialogV1")
	hInstRaw, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(dynamogramPDFPageWndProc), HInstance: syscall.Handle(hInstRaw), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — выбор страницы PDF"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 560, 155, uintptr(hMain), 0, hInstRaw, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return 0, false, fmt.Errorf("не удалось открыть окно выбора страницы PDF")
	}
	var message MSG
	for !dynPDFPageDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return selectedDynPDFPage, dynPDFPageDialogAccepted, nil
}

func chooseDynamogramCSVFile() string {
	buf := make([]uint16, 32768)
	filter := utf16.Encode([]rune("Цифровая динамограмма (*.csv;*.txt)\x00*.csv;*.txt\x00CSV (*.csv)\x00*.csv\x00TXT (*.txt)\x00*.txt\x00Все файлы (*.*)\x00*.*\x00\x00"))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: p("Выберите CSV/TXT с двумя числовыми колонками: ход и нагрузка"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func openExternal(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		msg("Открытие файла", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	pShellExecuteW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p("open"))), uintptr(unsafe.Pointer(p(path))), 0, 0, 1)
}

func openCurrentDynamogramSource() {
	if currentDyn == nil {
		msg("Диагностика ШГН / УШГН", "Сначала загрузите и обработайте динамограмму.", MB_OK|MB_ICONWARNING)
		return
	}
	var path string
	var err error
	if strings.Contains(strings.ToLower(currentDyn.DetectionMode), "цифров") {
		path, err = materializeDynPreview(currentDyn.PreviewPath)
	} else {
		path, err = materializeDynSource(currentDyn)
	}
	if err != nil {
		msg("Открытие исходной динамограммы", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	openExternal(path)
}

func openCurrentDynamogramReconstruction() {
	if currentDyn == nil {
		msg("Диагностика ШГН / УШГН", "Сначала загрузите и обработайте динамограмму.", MB_OK|MB_ICONWARNING)
		return
	}
	path := currentDyn.ReconstructionPath
	if strings.TrimSpace(path) == "" {
		path = currentDyn.PreviewPath
	}
	materialized, err := materializeDynReconstruction(path)
	if err != nil {
		msg("Открытие перерисованного контура", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	openExternal(materialized)
}

func parseDynFloat(h syscall.Handle, name string) (float64, error) {
	s := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(getText(h)), " ", ""), ",", ".")
	if s == "" {
		return 0, fmt.Errorf("не задано поле %s", name)
	}
	v, e := strconv.ParseFloat(s, 64)
	if e != nil {
		return 0, fmt.Errorf("некорректное значение %s", name)
	}
	return v, nil
}

type dynAnalysisWorkerResult struct {
	a   DynamogramAnalysis
	pts []DynamogramPoint
	err error
}

var dynAnalysisWorker = make(chan dynAnalysisWorkerResult, 1)
var dynAnalysisRunning bool

func startDynamogramAnalysisAsync() {
	if dynAnalysisRunning {
		return
	}
	if selectedDynFile == "" {
		msg("Анализ динамограммы", "Сначала загрузите PDF или изображение PNG/JPG/BMP.", MB_OK|MB_ICONWARNING)
		return
	}
	ext := strings.ToLower(filepath.Ext(selectedDynFile))
	if ext == ".csv" || ext == ".txt" {
		msg("Анализ динамограммы", "Выбран цифровой массив CSV/TXT: он уже импортирован и не требует анализа как изображение.", MB_OK|MB_ICONWARNING)
		return
	}
	path := selectedDynFile
	objectKey := getText(dynObjectEdit)
	pdfPage := selectedDynPDFPage
	dynAnalysisRunning = true
	diagLog("DYN analysis start file=%s", path)
	if dynAnalyzeButton != 0 {
		pEnableWindow.Call(uintptr(dynAnalyzeButton), 0)
	}
	setText(dynSummary, "Анализ изображения выполняется в фоне. Интерфейс остаётся доступным.")
	go func() {
		res := dynAnalysisWorkerResult{}
		defer func() {
			if r := recover(); r != nil {
				res.err = fmt.Errorf("внутренняя ошибка анализа изображения: %v", r)
				diagLog("DYN analysis panic=%v\n%s", r, debug.Stack())
			}
			select {
			case dynAnalysisWorker <- res:
			default:
				diagLog("DYN result dropped: worker channel is full")
			}
			pPostMessageW.Call(uintptr(hMain), WM_APP_DYN_DONE, 0, 0)
		}()
		res.a, res.pts, res.err = analyzeDynamogram(path, objectKey, pdfPage)
	}()
}

func finishDynamogramAnalysisAsync() {
	var r dynAnalysisWorkerResult
	select {
	case r = <-dynAnalysisWorker:
	default:
		return
	}
	dynAnalysisRunning = false
	if r.err != nil {
		diagLog("DYN analysis error: %v", r.err)
	} else {
		diagLog("DYN analysis done confidence=%.3f points=%d", r.a.Confidence, r.a.PointCount)
	}
	if dynAnalyzeButton != 0 {
		pEnableWindow.Call(uintptr(dynAnalyzeButton), 1)
	}
	if r.err != nil {
		setText(dynSummary, "Анализ не выполнен: "+r.err.Error())
		if currentPage == ID_DYN {
			msg("Анализ динамограммы", r.err.Error(), MB_OK|MB_ICONERROR)
		}
		return
	}
	currentDyn = &r.a
	currentDynPoints = r.pts
	dynHistory = loadDynamogramHistory()
	// Retain the result. The destination section will render it on activation.
	if currentPage != ID_DYN {
		return
	}
	refreshDynamogramUI()
	gate := "автоконтроль пройден"
	if r.a.QualityGate == dynQualityGateWarning {
		gate = "автоконтроль: есть предупреждения"
	} else if r.a.QualityGate == dynQualityGateBlocked {
		gate = "автоконтроль: подтверждение заблокировано"
	}
	msg("Анализ динамограммы", fmt.Sprintf("Кривая оцифрована.\nКачество: %s\nУверенность: %.0f %%\nТочек: %d\n%s\n\nПеред использованием результата откройте контрольное изображение и проверьте распознанный контур.", r.a.Quality, r.a.Confidence*100, r.a.PointCount, gate), MB_OK|MB_ICONINFORMATION)
}

func analyzeSelectedDynamogram() error {
	if selectedDynFile == "" {
		return fmt.Errorf("сначала загрузите PDF или изображение PNG/JPG/BMP")
	}
	ext := strings.ToLower(filepath.Ext(selectedDynFile))
	if ext == ".csv" || ext == ".txt" {
		return fmt.Errorf("выбран цифровой массив CSV/TXT: он уже импортирован и не требует анализа как изображение")
	}
	a, pts, e := analyzeDynamogram(selectedDynFile, getText(dynObjectEdit), selectedDynPDFPage)
	if e != nil {
		return e
	}
	currentDyn = &a
	currentDynPoints = pts
	dynHistory = loadDynamogramHistory()
	refreshDynamogramUI()
	return nil
}

func dynamogramCalibrationSummary(a *DynamogramAnalysis) string {
	if a == nil || !a.Calibrated {
		return "шкалы не подтверждены; показаны нормированные координаты"
	}
	return fmt.Sprintf("шкалы подтверждены: X %.6g–%.6g %s; Y %.6g–%.6g %s", a.XMin, a.XMax, a.XUnit, a.YMin, a.YMax, a.YUnit)
}

func calibrateSelectedDynamogram() error {
	if currentDyn == nil {
		return fmt.Errorf("сначала выполните анализ изображения")
	}
	xmin, e := parseDynFloat(dynXMin, "X min")
	if e != nil {
		return e
	}
	xmax, e := parseDynFloat(dynXMax, "X max")
	if e != nil {
		return e
	}
	ymin, e := parseDynFloat(dynYMin, "Y min")
	if e != nil {
		return e
	}
	ymax, e := parseDynFloat(dynYMax, "Y max")
	if e != nil {
		return e
	}
	if e = applyDynCalibration(currentDyn, currentDynPoints, xmin, xmax, ymin, ymax, getText(dynXUnit), getText(dynYUnit)); e != nil {
		return e
	}
	dynHistory = loadDynamogramHistory()
	refreshDynamogramUI()
	msg("Калибровка динамограммы", "Шкалы сохранены и перерисованный контур пересобран с осями и делениями.\n\n"+dynamogramCalibrationSummary(currentDyn), MB_OK|MB_ICONINFORMATION)
	return nil
}

func renderCurrentDynamogram() {
	if dynFileLabel == 0 {
		return
	}
	f := strings.TrimSpace(selectedDynFile)
	if f == "" {
		setText(dynFileLabel, "Источник: не выбран")
	} else {
		setText(dynFileLabel, "Источник: "+filepath.Base(f))
	}
	if currentDyn == nil {
		setText(dynCapabilities, dynamogramCapabilityText(nil))
		setText(dynSummary, "Результат ещё не сформирован.")
		setText(dynDetail, "После загрузки РОСА сначала оценивает пригодность исходника, затем показывает исходное изображение рядом с отдельно перерисованным контуром. Диагностический вывод не должен использоваться, если пользователь отклонил распознавание.")
		clearDynamogramPreview()
		return
	}
	a := currentDyn
	if a.MiddleSeparation == 0 && len(currentDynPoints) > 0 {
		populateDynamogramProfile(a, currentDynPoints)
	}
	setText(dynCapabilities, dynamogramCapabilityText(a))
	gate := "контроль пройден"
	if a.QualityGate == dynQualityGateWarning {
		gate = "есть предупреждения"
	} else if a.QualityGate == dynQualityGateBlocked {
		gate = "подтверждение заблокировано"
	} else if a.QualityGate == "" {
		gate = "метрика не рассчитана"
	}
	setText(dynSummary, fmt.Sprintf("Качество: %s; уверенность: %.0f %%; точек: %d; %s; проверка: %s; %s", a.Quality, a.Confidence*100, a.PointCount, gate, dynReviewLabel(a.ReviewStatus), dynamogramCalibrationSummary(a)))
	setText(dynDetail, dynamogramInterpretation(a))
	setDynamogramImages(a)
	if a.ObjectKey != "" && strings.TrimSpace(getText(dynObjectEdit)) == "" {
		setText(dynObjectEdit, a.ObjectKey)
	}
	if a.Calibrated {
		setText(dynXMin, fmt.Sprintf("%.6g", a.XMin))
		setText(dynXMax, fmt.Sprintf("%.6g", a.XMax))
		setText(dynYMin, fmt.Sprintf("%.6g", a.YMin))
		setText(dynYMax, fmt.Sprintf("%.6g", a.YMax))
		setText(dynXUnit, a.XUnit)
		setText(dynYUnit, a.YUnit)
	}
}

func dynReviewLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed":
		return "подтверждён"
	case "rejected":
		return "отклонён"
	case "digital":
		return "цифровой"
	default:
		return "не проверен"
	}
}

func dynamogramCapabilityText(a *DynamogramAnalysis) string {
	base := "ЧТО МОЖЕТ РОСА ПО PDF И ИЗОБРАЖЕНИЮ: выделить и оцифровать контур, оценить геометрию/замкнутость/асимметрию и показать отдельно перерисованный график. PDF с текстовым слоем дополнительно передаёт доступные реквизиты отчёта и шкалы. " +
		"АБСОЛЮТНЫЕ ЗНАЧЕНИЯ X/Y: только после подтверждённой калибровки шкал или из цифрового массива. " +
		"НЕ ОПРЕДЕЛЯЕТ ПО РИСУНКУ: дебит, глубину подвески, диаметр/тип насоса, давление (Р), частоту качаний и однозначную причину дефекта; эти сведения должны поступать из техрежима, телеметрии или ручного ввода. OCR шкал и подписей сейчас не используется."
	if a == nil {
		return base
	}
	var status string
	if strings.Contains(strings.ToLower(a.DetectionMode), "цифров") {
		status = " Источник цифровой: анализ изображения не требуется; проверьте смысл колонок и единицы измерения."
	} else if strings.EqualFold(a.ReviewStatus, "rejected") {
		status = " СТАТУС: пользователь отклонил распознанный контур — автоматическую интерпретацию использовать нельзя."
	} else if strings.EqualFold(a.ReviewStatus, "confirmed") {
		status = " СТАТУС: пользователь подтвердил соответствие перерисованного контура исходнику."
	} else if a.QualityGate == dynQualityGateBlocked {
		status = " СТАТУС: автоматический контроль обнаружил грубый дефект; подтверждение заблокировано. Повторите анализ или исправьте контур."
	} else if a.QualityGate == dynQualityGateWarning {
		status = " СТАТУС: автоматический контроль выявил отклонения; нужна особенно внимательная визуальная сверка."
	} else if a.Confidence < 0.62 {
		status = " СТАТУС: низкая уверенность распознавания; необходим более качественный исходник или цифровой массив."
	} else if !a.Calibrated {
		status = " СТАТУС: форма доступна для предварительной оценки, но физические шкалы не подтверждены."
	} else {
		status = " СТАТУС: контур оцифрован и шкалы заданы; всё равно требуется визуальная сверка исходника и перерисованного графика."
	}
	return base + status
}

func reviewCurrentDynamogram(status string) {
	if currentDyn == nil {
		msg("Динамограмма", "Сначала загрузите и проанализируйте динамограмму.", MB_OK|MB_ICONWARNING)
		return
	}
	if strings.Contains(strings.ToLower(currentDyn.DetectionMode), "цифров") {
		msg("Динамограмма", "Для цифрового массива графический контур строится непосредственно по числовым координатам; отдельное подтверждение распознавания изображения не требуется.", MB_OK|MB_ICONINFORMATION)
		return
	}
	if status != "confirmed" && status != "rejected" {
		return
	}
	if status == "confirmed" {
		gate, issues := dynamogramQualityGate(currentDyn)
		currentDyn.QualityGate, currentDyn.QualityIssues = gate, issues
		if gate == dynQualityGateBlocked {
			text := "Подтверждение заблокировано автоматическим контролем качества.\n\n"
			for _, issue := range issues {
				text += "• " + issue + "\n"
			}
			text += "\nОтклоните результат, повторите анализ на более качественном исходнике либо выполните ручную коррекцию после её добавления."
			msg("Динамограмма — грубый дефект контура", text, MB_OK|MB_ICONWARNING)
			return
		}
	}
	currentDyn.ReviewStatus = status
	if status == "confirmed" {
		currentDyn.ReviewNote = "Перерисованный контур визуально сопоставлен с исходным изображением и подтверждён пользователем."
		if currentDyn.Confidence >= 0.82 {
			currentDyn.Quality = "высокое"
		} else if currentDyn.Confidence >= 0.62 {
			currentDyn.Quality = "приемлемое"
		}
	} else {
		currentDyn.ReviewNote = "Пользователь отметил распознавание контура как некорректное; результат исключён из диагностической интерпретации."
		currentDyn.Quality = "отклонено"
	}
	hist := loadDynamogramHistory()
	for i := range hist {
		if hist[i].ID == currentDyn.ID {
			hist[i] = *currentDyn
			break
		}
	}
	if err := saveDynamogramHistory(hist); err != nil {
		msg("Динамограмма", "Не удалось сохранить результат визуальной проверки: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	dynHistory = hist
	refreshDynamogramUI()
}

func dynAssetUsedElsewhere(path, excludingID string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	for _, a := range dynHistory {
		if a.ID == excludingID {
			continue
		}
		if a.SourceCopy == path || a.PreviewPath == path || a.ReconstructionPath == path || a.CSVPath == path {
			return true
		}
	}
	return false
}

func removeDynAssetsIfUnused(a DynamogramAnalysis) {
	for _, path := range []string{a.SourceCopy, a.PreviewPath, a.ReconstructionPath, a.CSVPath} {
		if strings.TrimSpace(path) != "" && !dynAssetUsedElsewhere(path, a.ID) {
			_ = os.Remove(path)
		}
	}
}

func deleteSelectedDynamogram() {
	i := selectedRow(dynHistoryList)
	if i < 0 || i >= len(dynHistory) {
		msg("Динамограмма", "Выберите запись в истории.", MB_OK|MB_ICONWARNING)
		return
	}
	target := dynHistory[i]
	text := fmt.Sprintf("Удалить выбранную запись динамограммы?\n\n%s\n%s\n\nБудет удалён только этот сохранённый анализ и его неиспользуемые вложения.", target.ObjectKey, filepath.Base(target.SourceFile))
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Удаление динамограммы"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	out := make([]DynamogramAnalysis, 0, len(dynHistory)-1)
	for _, a := range dynHistory {
		if a.ID != target.ID {
			out = append(out, a)
		}
	}
	dynHistory = out
	if err := saveDynamogramHistory(dynHistory); err != nil {
		msg("Динамограмма", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	addSessionTrashItem("Динамограмма", target.ObjectKey+"\r\n"+filepath.Base(target.SourceFile), func() error {
		dynHistory = append(dynHistory, target)
		if err := saveDynamogramHistory(dynHistory); err != nil {
			return err
		}
		refreshDynamogramUI()
		return nil
	}, func() { removeDynAssetsIfUnused(target) })
	if currentDyn != nil && currentDyn.ID == target.ID {
		currentDyn = nil
		currentDynPoints = nil
		selectedDynFile = ""
	}
	refreshDynamogramUI()
}

func clearDynamogramHistoryForObject() {
	key := strings.TrimSpace(getText(dynObjectEdit))
	if key == "" && currentDyn != nil {
		key = strings.TrimSpace(currentDyn.ObjectKey)
	}
	if key == "" {
		msg("Динамограмма", "Укажите объект. Полная очистка истории всех объектов из этого экрана не выполняется.", MB_OK|MB_ICONWARNING)
		return
	}
	var targets []DynamogramAnalysis
	for _, a := range dynHistory {
		if strings.EqualFold(strings.TrimSpace(a.ObjectKey), key) {
			targets = append(targets, a)
		}
	}
	if len(targets) == 0 {
		msg("Динамограмма", "Для выбранного объекта нет сохранённых динамограмм.", MB_OK|MB_ICONINFORMATION)
		return
	}
	text := fmt.Sprintf("Очистить историю динамограмм объекта?\n\n%s\nЗаписей: %d\n\nТехрежим, лаборатория, телеметрия и расчётная история не затрагиваются.", key, len(targets))
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Очистка истории динамограмм"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	out := make([]DynamogramAnalysis, 0, len(dynHistory)-len(targets))
	for _, a := range dynHistory {
		if !strings.EqualFold(strings.TrimSpace(a.ObjectKey), key) {
			out = append(out, a)
		}
	}
	dynHistory = out
	if err := saveDynamogramHistory(dynHistory); err != nil {
		msg("Динамограмма", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	restoredTargets := append([]DynamogramAnalysis(nil), targets...)
	addSessionTrashItem(fmt.Sprintf("Динамограммы · %d записей", len(restoredTargets)), key, func() error {
		dynHistory = append(dynHistory, restoredTargets...)
		if err := saveDynamogramHistory(dynHistory); err != nil {
			return err
		}
		refreshDynamogramUI()
		return nil
	}, func() {
		for _, analysis := range restoredTargets {
			removeDynAssetsIfUnused(analysis)
		}
	})
	if currentDyn != nil && strings.EqualFold(strings.TrimSpace(currentDyn.ObjectKey), key) {
		currentDyn = nil
		currentDynPoints = nil
		selectedDynFile = ""
	}
	refreshDynamogramUI()
}

func refreshDynamogramUI() {
	if dynFileLabel == 0 {
		return
	}
	if dynHistory == nil {
		dynHistory = loadDynamogramHistory()
	}
	renderCurrentDynamogram()
	// Rebuild history only during an explicit page/data refresh. Selection itself must
	// never rebuild this ListView, otherwise LVN_ITEMCHANGED recursively re-enters the UI.
	listBeginUpdate(dynHistoryList)
	defer listEndUpdate(dynHistoryList)
	listClear(dynHistoryList)
	for i, a := range dynHistory {
		if i >= 60 {
			break
		}
		listAdd(dynHistoryList, i, []string{a.Created, a.ObjectKey, filepath.Base(a.SourceFile), a.Quality, fmt.Sprintf("%.0f %%", a.Confidence*100), dynReviewLabel(a.ReviewStatus), a.DetectionMode})
	}
}

func loadDynPointsFromCSV(a DynamogramAnalysis) ([]DynamogramPoint, error) {
	path, e := materializeDynCSV(a.CSVPath)
	if e != nil {
		return nil, e
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	r := csv.NewReader(f)
	_, e = r.Read()
	if e != nil {
		return nil, e
	}
	var out []DynamogramPoint
	for {
		rec, e := r.Read()
		if e != nil {
			break
		}
		if len(rec) < 3 {
			continue
		}
		xn, _ := strconv.ParseFloat(rec[0], 64)
		un, _ := strconv.ParseFloat(rec[1], 64)
		ln, _ := strconv.ParseFloat(rec[2], 64)
		p := DynamogramPoint{XNorm: xn, UpperNorm: un, LowerNorm: ln}
		if len(rec) >= 6 {
			p.X, _ = strconv.ParseFloat(rec[3], 64)
			p.Upper, _ = strconv.ParseFloat(rec[4], 64)
			p.Lower, _ = strconv.ParseFloat(rec[5], 64)
		}
		out = append(out, p)
	}
	return out, nil
}

func selectDynamogramHistory() {
	if dynHistoryList == 0 {
		return
	}
	i := selectedRow(dynHistoryList)
	if i < 0 || i >= len(dynHistory) {
		return
	}
	a := dynHistory[i]
	pts, e := loadDynPointsFromCSV(a)
	if e != nil {
		diagLog("DYN history read error id=%s err=%v", a.ID, e)
		setText(dynSummary, "Не удалось открыть сохранённый результат: "+e.Error())
		return
	}
	currentDyn = &a
	currentDynPoints = pts
	selectedDynFile = a.SourceFile
	selectedDynPDFPage = a.PDFPage
	if selectedDynPDFPage < 1 {
		selectedDynPDFPage = 1
	}
	// Only update the detail pane. Do not rebuild the history ListView from inside
	// its own selection notification.
	renderCurrentDynamogram()
}

func dynCountForObject(key string) int {
	key = strings.TrimSpace(key)
	if key == "" {
		return 0
	}
	if dynHistory == nil {
		dynHistory = loadDynamogramHistory()
	}
	n := 0
	for _, a := range dynHistory {
		if strings.EqualFold(strings.TrimSpace(a.ObjectKey), key) {
			n++
		}
	}
	return n
}

func openDynamogramForSelectedObject() {
	x := filteredObjects()
	i := selectedRow(objectsList)
	if i < 0 || i >= len(x) {
		if len(x) == 0 {
			msg("Диагностика ШГН / УШГН", "Объект не выбран.", MB_OK|MB_ICONWARNING)
			return
		}
		i = 0
	}
	key := strings.TrimSpace(x[i].State.ObjectKey)
	if key == "" {
		key = strings.TrimSpace(x[i].Eval.ObjectKey)
	}
	switchPage(ID_DYN)
	setText(dynObjectEdit, key)
}
