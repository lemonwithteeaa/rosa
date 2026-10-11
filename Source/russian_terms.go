package main

import "strings"

// Пользовательский интерфейс РОСА русифицируется полностью. Внутренние
// идентификаторы и названия статистических методов могут оставаться машинными,
// но в пользовательских текстах сначала показываются русские эквиваленты;
// английский термин допускается только после русского эквивалента в скобках.
func ruYesNo(v bool) string {
	if v {
		return "да"
	}
	return "нет"
}

func mrpUserModelName(version string) string {
	s := strings.TrimSpace(version)
	switch s {
	case "MRP-base-1.3-ru":
		return "базовая модель МРП 1.3"
	case "MRP-base-1.4-ru":
		return "базовая модель МРП 1.4"
	case "MRP-reliability-2.3-ru":
		return "модель безотказной работы МРП 2.3"
	case "":
		return "не указана"
	default:
		return s
	}
}

func economicsStatusRu(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "PASS":
		return "расчёт выполнен"
	case "FAIL — ПРОВЕРИТЬ ИСХОДНЫЕ ДАННЫЕ", "FAIL":
		return "расчёт не выполнен — проверить исходные данные"
	default:
		if strings.TrimSpace(status) == "" {
			return "не указан"
		}
		return status
	}
}
