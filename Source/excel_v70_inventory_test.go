//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestExcelV70InventoryRC151UsesActualWorkbookSheetCount(t *testing.T) {
	b, err := os.ReadFile("reference/excel_v70_inventory/ИНВЕНТАРИЗАЦИЯ_ТЕХМОДУЛЯ_RC151.txt")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"Фактически листов в текущем файле: 19", "Лаборатория и ОПР", "Данные модели", "Восстановление"} {
		if !strings.Contains(text, want) {
			t.Fatalf("inventory missing %q", want)
		}
	}
}
