//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type pageApprovalRecord struct {
	ObjectKey string `json:"object_key"`
	Page      int    `json:"page"`
	Section   string `json:"section"`
	Author    string `json:"author"`
	Approved  string `json:"approved"`
}

var pageApprovals map[string]pageApprovalRecord

func approvalSection() string {
	if currentPage == ID_LAB {
		if g, ok := labGroupByCode(labActiveGroup); ok {
			return "Лаборатория / " + g.Name
		}
	}
	if currentPage == ID_OBJECTS && wellManualMode {
		return "Отказы / ручной ввод"
	}
	if currentPage == ID_OBJECTS {
		return "Отказы / история"
	}
	return fmt.Sprintf("Вкладка %d", currentPage)
}

func approveCurrentPage() {
	if currentPage == ID_DASH {
		confirmSelectedObject()
		return
	}
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		msg("Утверждение вкладки", "Сначала закрепите объект на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	if currentPage == ID_LAB {
		if strings.TrimSpace(getText(labObjectEdit)) != key {
			msg("Утверждение вкладки", "Открыта запись другого объекта. Вернитесь к закреплённому объекту перед утверждением.", MB_OK|MB_ICONWARNING)
			return
		}
		saveLabDraft()
	}
	if pageApprovals == nil {
		pageApprovals = map[string]pageApprovalRecord{}
		loadJSON(sharedPath("page_approvals.json"), &pageApprovals)
	}
	section := approvalSection()
	record := pageApprovalRecord{ObjectKey: key, Page: currentPage, Section: section, Author: currentUserLabel(), Approved: time.Now().Format("02.01.2006 15:04")}
	pageApprovals[key+"|"+strconv.Itoa(currentPage)+"|"+section] = record
	if err := saveJSON(sharedPath("page_approvals.json"), pageApprovals); err != nil {
		msg("Утверждение вкладки", "Не удалось записать утверждение: "+err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	msg("Вкладка утверждена", fmt.Sprintf("%s\nОбъект: %s\n%s · %s", section, readableObjectKey(key), record.Author, record.Approved), MB_OK|MB_ICONINFORMATION)
}
