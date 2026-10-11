//go:build windows

package main

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unsafe"
)

// RC723: every native BUTTON created by ROSA receives one common right-click
// help contract. A right click never executes the action; it only explains what
// the button does. This is intentionally implemented at BUTTON-window level so
// the rule covers every page, modal dialog and dynamically created button rather
// than a hand-maintained subset of screens.
var (
	buttonHelpCallWindowProcW = user32.NewProc("CallWindowProcW")
	buttonHelpGetDlgCtrlID    = user32.NewProc("GetDlgCtrlID")
	buttonHelpGetAncestor     = user32.NewProc("GetAncestor")
	buttonHelpOldProc         sync.Map // map[uintptr]uintptr
	buttonHelpWndProcCallback = syscall.NewCallback(buttonContextHelpWndProc)
)

const (
	wmNCDestroyButtonHelp = 0x0082
	wmRButtonUpButtonHelp = 0x0205
	gaRootButtonHelp      = 2
)

func registerButtonContextHelp(h syscall.Handle, className string) {
	if h == 0 || !strings.EqualFold(strings.TrimSpace(className), "BUTTON") {
		return
	}
	if _, exists := buttonHelpOldProc.Load(uintptr(h)); exists {
		return
	}
	old, _, _ := pSetWindowLongPtrW.Call(uintptr(h), ^uintptr(3), buttonHelpWndProcCallback) // GWLP_WNDPROC = -4
	if old != 0 {
		buttonHelpOldProc.Store(uintptr(h), old)
	}
}

func buttonContextHelpWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == wmRButtonUpButtonHelp || msg == WM_CONTEXTMENU {
		showButtonContextHelp(syscall.Handle(hwnd))
		return 0
	}

	oldAny, ok := buttonHelpOldProc.Load(hwnd)
	if !ok {
		r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	old := oldAny.(uintptr)
	r, _, _ := buttonHelpCallWindowProcW.Call(old, hwnd, uintptr(msg), wParam, lParam)
	if msg == wmNCDestroyButtonHelp {
		buttonHelpOldProc.Delete(hwnd)
	}
	return r
}

func showButtonContextHelp(h syscall.Handle) {
	if h == 0 {
		return
	}
	caption := strings.TrimSpace(getText(h))
	idRaw, _, _ := buttonHelpGetDlgCtrlID.Call(uintptr(h))
	explanation := buttonFunctionHelp(caption, int(int32(idRaw)))
	if explanation == "" {
		explanation = "Выполняет действие этой кнопки для текущего экрана и выбранных данных."
	}

	titleCaption := caption
	if titleCaption == "" {
		titleCaption = "кнопка"
	}
	if len([]rune(titleCaption)) > 48 {
		rr := []rune(titleCaption)
		titleCaption = string(rr[:48]) + "…"
	}
	title := "РОСА — назначение кнопки · " + titleCaption
	text := explanation + "\n\nЛевая кнопка мыши — выполнить действие. Правая кнопка — показать эту подсказку."

	owner, _, _ := buttonHelpGetAncestor.Call(uintptr(h), gaRootButtonHelp)
	if owner == 0 {
		owner = uintptr(hMain)
	}
	pMessageBoxW.Call(owner, uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p(title))), MB_OK|MB_ICONINFORMATION)
}

func normalizedButtonCaption(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r", " "))
	s = strings.Join(strings.Fields(s), " ")
	return strings.ToLower(s)
}

func captionLooksLikeSectionNavigation(caption string) bool {
	r := []rune(strings.TrimSpace(caption))
	if len(r) == 0 {
		return false
	}
	if unicode.IsDigit(r[0]) {
		return true
	}
	lower := strings.ToLower(caption)
	for _, marker := range []string{
		"главная", "фонд", "карточка", "показатели", "лаборатор", "оценка осложнений",
		"методы", "мероприят", "наработка", "экономика", "итоговый отч", "архив",
		"динамо", "мониторинг состояния гно", "история оценок", "технологии и критерии",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func buttonFunctionHelp(caption string, id int) string {
	c := normalizedButtonCaption(caption)
	if c == "" {
		return "Выполняет служебное действие текущего окна."
	}

	// Exact high-value actions where the generic verb is not descriptive enough.
	switch c {
	case "дублировать вариант":
		return "Создаёт копию выбранного варианта решения, чтобы изменить методы, прогноз наработки или экономический сценарий, не затрагивая исходный вариант."
	case "сравнить варианты":
		return "Открывает сравнение вариантов по методам, прогнозу наработки, CAPEX/OPEX, чистой выгоде и основанию выбора."
	case "карточка метода":
		return "Открывает подробную карточку метода, связанного с выбранной строкой сценария."
	case "расчёт эффекта", "расчет эффекта":
		return "Открывает расчёт ожидаемого эффекта для выбранного сценария и его исходных параметров."
	case "утвердить решение":
		return "Фиксирует выбранный вариант как утверждённое решение технолога и подготавливает его для попадания в утверждённый реестр."
	case "обновить сводку":
		return "Пересобирает сводку раздела по текущему закреплённому объекту и актуальным данным предыдущих этапов."
	case "открыть":
		return "Открывает выбранную запись, документ или подробности текущего элемента."
	case "pdf":
		return "Формирует выгрузку текущего результата в PDF."
	case "docx":
		return "Формирует выгрузку текущего результата в DOCX."
	case "xlsx":
		return "Формирует выгрузку текущего результата в XLSX."
	case "скачать pdf":
		return "Сохраняет выбранный утверждённый отчёт из раздела 8 в PDF с подробной расшифровкой разделов 1–7 и расчётным снимком на момент утверждения."
	case "скачать word":
		return "Сохраняет выбранный утверждённый отчёт из раздела 8 в Word (DOCX) с подробной расшифровкой разделов 1–7 и расчётами."
	case "скачать excel":
		return "Сохраняет выбранный утверждённый отчёт из раздела 8 в Excel (XLSX) с подробной расшифровкой разделов 1–7 и расчётами."
	case "история":
		return "Открывает историю ранее сохранённых оценок и решений."
	case "справка":
		return "Открывает справочную информацию по текущему разделу и работе с программой."
	case "выход":
		return "Завершает текущий пользовательский сеанс и возвращает к экрану выбора/входа пользователя."
	case "←":
		return "Возвращает к предыдущему экрану или предыдущему состоянию навигации."
	case "→":
		return "Переходит к следующему экрану или следующему состоянию навигации."
	case "+":
		return "Увеличивает масштаб текущего графика или представления."
	case "−", "-":
		return "Уменьшает масштаб текущего графика или представления."
	case "корзина · 0":
		return "Открывает корзину текущей сессии для просмотра и восстановления временно удалённых записей."
	case "войти →", "войти":
		return "Выполняет вход в выбранный профиль с указанными учётными данными."
	case "создать профиль":
		return "Открывает создание нового локального профиля пользователя РОСА."
	case "войти без авторизации":
		return "Открывает рабочее пространство в гостевом режиме без входа в персональный профиль."
	case "забыли пароль?":
		return "Открывает восстановление доступа к выбранному локальному профилю."
	case "показать":
		return "Временно показывает введённый пароль вместо маскирующих символов."
	case "запомнить пользователя":
		return "Сохраняет выбор пользователя для следующего запуска приложения."
	}

	if captionLooksLikeSectionNavigation(caption) {
		return fmt.Sprintf("Переходит в раздел «%s». Текущие данные других разделов при переходе не удаляются.", strings.TrimSpace(caption))
	}

	// Verb-based coverage for all current and future buttons. Keeping this logic
	// central means newly added tabs automatically receive meaningful help even
	// before a dedicated sentence is added for a new caption.
	switch {
	case strings.Contains(c, "выбрать все"):
		return "Выбирает все доступные элементы текущего списка для последующего действия."
	case strings.Contains(c, "очистить выбор"):
		return "Снимает текущее выделение элементов, не удаляя сами данные."
	case strings.Contains(c, "выбрать"):
		return "Открывает выбор подходящего элемента или фиксирует текущий выбор для дальнейшей работы."
	case strings.Contains(c, "подгруз") || strings.Contains(c, "загруз") || strings.Contains(c, "импорт"):
		return "Запускает загрузку данных из внешнего файла или подготовленного источника в текущий раздел."
	case strings.Contains(c, "перенести в программу"):
		return "Переносит проверенные данные из предварительного просмотра в рабочие данные РОСА."
	case strings.Contains(c, "применить"):
		return "Применяет выбранные параметры или вариант к текущему объекту/расчёту."
	case strings.Contains(c, "утверд") || strings.Contains(c, "принять"):
		return "Фиксирует текущее выбранное решение или данные как принятые технологом."
	case strings.Contains(c, "сохран"):
		return "Сохраняет текущие введённые или рассчитанные данные в соответствующий раздел/архив."
	case strings.Contains(c, "добав"):
		return "Добавляет новую запись или значение в текущий рабочий набор."
	case strings.Contains(c, "копиров") || strings.Contains(c, "создать мою копию"):
		return "Создаёт копию выбранной записи или данных для отдельной дальнейшей работы."
	case strings.Contains(c, "сравн"):
		return "Открывает сопоставление выбранных данных или версий по ключевым показателям."
	case strings.Contains(c, "обнов") || strings.Contains(c, "синхрониз"):
		return "Обновляет отображаемые данные и пересчитывает состояние текущего блока без изменения исходных записей вручную."
	case strings.Contains(c, "выполнить расч") || strings.Contains(c, "рассчит"):
		return "Запускает расчёт по текущим исходным данным и выбранным параметрам."
	case strings.Contains(c, "выгруз") || strings.Contains(c, "excel") || strings.Contains(c, "word"):
		return "Формирует внешний файл с данными или результатами текущего раздела."
	case strings.Contains(c, "открыть"):
		return "Открывает выбранный объект, документ, параметры или подробное представление."
	case strings.Contains(c, "просмотр") || strings.Contains(c, "просмотреть"):
		return "Открывает данные в режиме просмотра без обязательного изменения исходной записи."
	case strings.Contains(c, "закрыть доступ"):
		return "Отзывает ранее предоставленный доступ к выбранному расчёту."
	case strings.Contains(c, "открыть доступ"):
		return "Открывает настройку передачи выбранного расчёта другому пользователю."
	case strings.Contains(c, "закрыть"):
		return "Закрывает текущее окно и возвращает к предыдущему рабочему экрану."
	case strings.Contains(c, "отмена") || strings.Contains(c, "отменить"):
		return "Отменяет текущее действие и закрывает его без применения новых изменений."
	case strings.Contains(c, "удал") || strings.Contains(c, "очистить корзину"):
		return "Удаляет выбранную запись или очищает соответствующий набор после подтверждения, если оно предусмотрено."
	case strings.Contains(c, "восстанов"):
		return "Восстанавливает выбранную ранее удалённую или архивную запись для дальнейшей работы."
	case strings.Contains(c, "найти"):
		return "Выполняет поиск по заданным условиям в текущем наборе данных."
	case strings.Contains(c, "инфограф"):
		return "Открывает визуальное представление результатов диагностики и ключевых показателей."
	case strings.Contains(c, "график"):
		return "Открывает или формирует графическое представление текущих данных."
	case strings.Contains(c, "развернуть"):
		return "Разворачивает текущий блок или представление, чтобы показать больше данных."
	case strings.Contains(c, "свернуть"):
		return "Сворачивает текущий блок или представление, освобождая место на экране."
	case strings.Contains(c, "назад"):
		return "Возвращает к предыдущему шагу текущего рабочего процесса."
	case strings.Contains(c, "вперёд") || strings.Contains(c, "вперед"):
		return "Переходит к следующему шагу текущего рабочего процесса."
	case strings.Contains(c, "справочник") || strings.Contains(c, "источник") || strings.Contains(c, "как считается"):
		return "Открывает справочную или поясняющую информацию, связанную с текущим расчётом или методом."
	case strings.Contains(c, "линия тренда"):
		return "Добавляет или изменяет отображение линии тренда для выбранного графика."
	case strings.Contains(c, "все объекты"):
		return "Переключает анализ на отображение всех выбранных объектов."
	}

	return fmt.Sprintf("Выполняет действие «%s» для текущего экрана и выбранных данных.", strings.TrimSpace(caption))
}
