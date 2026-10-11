//go:build windows

package main

import "syscall"

var methodsParameterLabels [6]syscall.Handle
var methodsParameterValues [6]syscall.Handle

func buildMethodsParameterPanel() {
	for i := 0; i < 6; i++ {
		methodsParameterLabels[i] = label(ID_ACTIVITIES, "", 1010, 502, 90, 25)
		methodsParameterValues[i] = create("EDIT", "", WS_CHILD|WS_BORDER|WS_TABSTOP|ES_READONLY|ES_MULTILINE, 1100, py(502), 150, 25, hMain, 0) // Wrapped full values, measured before display.
		addPageControl(ID_ACTIVITIES, methodsParameterValues[i])
		pSendMessageW.Call(uintptr(methodsParameterLabels[i]), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 0)
		pSendMessageW.Call(uintptr(methodsParameterValues[i]), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 0)
		show(methodsParameterLabels[i], false)
		show(methodsParameterValues[i], false)
	}
}

// Rows are sized by actual wrapped text. Never silently crop values to make
// a fixed six-row grid fit: retain the full scrollable summary when necessary.
func layoutMethodsParameterPanel(x, y, w, bodyH int32) {
	s, ok := methodsSelectedSuggestion()
	var fields []methodImplementationField
	if ok && !methodsBackgroundBusy {
		fields = methodImplementationEffectiveRecord(selectedObjectKey, s).Fields
	}
	labelW := min32(104, max32(74, w*25/100))
	var heights []int32
	size := methodsTypeBody
	if len(fields) > 0 && len(fields) <= 6 && w-labelW-4 >= 120 {
		for ; size >= methodsTypeCompact; size-- {
			labels, values := []int32{}, []int32{}
			for _, field := range fields {
				labels = append(labels, methodsMeasuredTextHeight(methodsParameterCaption(field.Label), labelW-8, size, false))
				values = append(values, methodsMeasuredTextHeight(field.Value, w-labelW-16, size, false))
			}
			if planned, fit := methodsParameterRowHeights(labels, values, bodyH); fit {
				heights = planned
				break
			}
		}
	}
	visible := len(heights) == len(fields) && len(fields) > 0
	show(methodsImplementation, !visible)
	rowY := y
	for i := 0; i < 6; i++ {
		shown := visible && i < len(fields)
		show(methodsParameterLabels[i], shown)
		show(methodsParameterValues[i], shown)
		if !shown {
			continue
		}
		rowH := heights[i]
		setPos(methodsParameterLabels[i], x, rowY+3, labelW-6, rowH-6)
		setPos(methodsParameterValues[i], x+labelW+4, rowY, w-labelW-4, rowH)
		pSendMessageW.Call(uintptr(methodsParameterLabels[i]), WM_SETFONT, uintptr(methodsFont(size, false)), 0)
		pSendMessageW.Call(uintptr(methodsParameterValues[i]), WM_SETFONT, uintptr(methodsFont(size, false)), 0)
		pSendMessageW.Call(uintptr(methodsParameterValues[i]), 0x00D3, 3, 4|(4<<16)) // EM_SETMARGINS
		setText(methodsParameterLabels[i], methodsParameterCaption(fields[i].Label))
		setText(methodsParameterValues[i], fields[i].Value)
		rowY += rowH + 4
	}
}

func methodsParameterRowHeights(labelHeights, valueHeights []int32, available int32) ([]int32, bool) {
	if len(labelHeights) == 0 || len(labelHeights) != len(valueHeights) || len(labelHeights) > 6 {
		return nil, false
	}
	rows := make([]int32, len(labelHeights))
	total := int32(0)
	for i := range rows {
		if labelHeights[i] <= 0 || valueHeights[i] <= 0 {
			return nil, false
		}
		rows[i] = max32(26, max32(labelHeights[i], valueHeights[i])+8)
		total += rows[i]
		if i > 0 {
			total += 4
		}
	}
	if total > available {
		return nil, false
	}
	return rows, true
}

func hideMethodsParameterPanel() {
	for i := 0; i < 6; i++ {
		show(methodsParameterLabels[i], false)
		show(methodsParameterValues[i], false)
	}
}

func methodsParameterCaption(label string) string {
	switch label {
	case "База дозирования":
		return "База"
	case "Концентрация / расход":
		return "Доза / расход"
	case "Тоннаж / концентрация":
		return "Доза / тоннаж"
	case "Растворитель / реагент":
		return "Реагент"
	case "Последовательность":
		return "Порядок"
	case "Время контакта":
		return "Контакт"
	case "Контроль результата":
		return "Контроль"
	case "Продолжительность":
		return "Длительность"
	case "Ограничения оборудования":
		return "Ограничения ГНО"
	case "Критерии прекращения":
		return "Критерий"
	case "Дополнительное условие":
		return "Условие"
	}
	return label
}
