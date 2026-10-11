//go:build windows

package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// The distributed blank is the same workbook that the import parser accepts:
// sheet 1 contains the preliminary entry and sheet 2 contains the extended
// investigation. Keeping the approved workbook embedded makes the action work
// from the standalone EXE without relying on sibling files.
//
//go:embed РОСА_БЛАНК_РЕЕСТРА_ОТКАЗОВ_632-RC.xlsx
var failureBlankTemplateXLSX []byte

func chooseFailureTemplatePath(defaultName string) string {
	filter := utf16.Encode([]rune("Excel (*.xlsx)\x00*.xlsx\x00Все файлы (*.*)\x00*.*\x00\x00"))
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(defaultName)))
	of := OPENFILENAMEW{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain,
		LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)),
		LpstrTitle: p("Сохранить бланк реестра отказов"),
		Flags:      OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT, LpstrDefExt: p("xlsx"),
	}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	return path
}

func downloadFailureTemplate() {
	path := chooseFailureTemplatePath("РОСА_БЛАНК_РЕЕСТРА_ОТКАЗОВ.xlsx")
	if path == "" {
		return
	}
	if len(failureBlankTemplateXLSX) == 0 {
		msg("Бланк реестра отказов", "Встроенный бланк недоступен.", MB_OK|MB_ICONERROR)
		return
	}
	if err := os.WriteFile(path, failureBlankTemplateXLSX, 0o600); err != nil {
		msg("Бланк реестра отказов", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Бланк реестра отказов", "Актуальный бланк реестра отказов сохранён. Сначала заполните лист «Оперативный ввод», затем дополните «Историю отказов»: фонтанная добыча, оборудование, гидрозащита, ТМС и штанги расположены до заключения комиссии.", MB_OK|MB_ICONINFORMATION)
}
