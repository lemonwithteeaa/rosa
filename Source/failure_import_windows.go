//go:build windows

package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	failureStatusNew       = "Новый"
	failureStatusUpdate    = "Обновление"
	failureStatusDuplicate = "Дубль"
	failureStatusKeyError  = "Ошибка ключа"
	failureStatusConflict  = "Требует проверки"
)

type FailureImportCandidate struct {
	Row             int
	Event           FailureEvent
	Status          string
	Details         string
	OperationalOnly bool
}

type failureImportRowRecord struct {
	Sheet     string `json:"sheet,omitempty"`
	Row       int    `json:"row"`
	ObjectKey string `json:"object_key,omitempty"`
	Status    string `json:"status"`
	Details   string `json:"details,omitempty"`
}

type failureImportRecord struct {
	When       string                   `json:"when"`
	User       string                   `json:"user,omitempty"`
	File       string                   `json:"file"`
	FileSHA256 string                   `json:"file_sha256"`
	Total      int                      `json:"total"`
	Counts     map[string]int           `json:"counts"`
	Applied    int                      `json:"applied"`
	Rows       []failureImportRowRecord `json:"rows,omitempty"`
}

type failureWorkbookColumns struct {
	department, productionUnit, field, well, objectPlast         int
	cycleStart, failureDate, repairEnd                           int
	operationMode, equipment, equipmentSize                      int
	equipmentSize2                                               int
	equipmentBrand, eventType, failedUnit                        int
	stopCause, symptom, preliminaryCause                         int
	primaryComplication, associated1, associated2                int
	tkrs1, tkrs2, damage, finalCause, refinedComplication        int
	methodBefore1, methodBefore2, methodAfter1, methodAfter2     int
	confirmation, investigationStatus, confidence                int
	pumpNominal, pumpMaker                                       int
	nktType, nktLength, nktYear, spo, nktCondition               int
	motorMaker, motorSize, motorYear, motorCondition             int
	cableSize, cableLength, cableCondition                       int
	extensionSize, extensionLength, extensionCondition           int
	hydroMaker, hydroSize, hydroYear, hydroCondition             int
	tmsMaker, tmsSize, tmsYear, tmsCondition                     int
	rodCondition, rodMaker, rod19, rod22, rod25, rodYear, rodSPO int
	commissionConclusion, commissionSource                       int
	source, comment                                              int
	fountainType, fountainDepth                                  int
	causeCode, sourceCause, sourceClass, complicationFactor      int
	inspection                                                   int
}

func failureWorkbookColumn(header map[int]string, aliases ...string) int {
	wanted := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		wanted[normalizeTechHeader(alias)] = true
	}
	for column, title := range header {
		if wanted[normalizeTechHeader(title)] {
			return column
		}
	}
	return 0
}

func failureWorkbookSchema(header map[int]string) failureWorkbookColumns {
	c := failureWorkbookColumns{
		department:           failureWorkbookColumn(header, "Подразделение"),
		productionUnit:       failureWorkbookColumn(header, "ЦДНГ", "Производственная единица"),
		field:                failureWorkbookColumn(header, "Месторождение", "Месторождение*"),
		well:                 failureWorkbookColumn(header, "№ скважины", "№ скважины*", "Скважина", "Номер скважины"),
		objectPlast:          failureWorkbookColumn(header, "Объект / пласт (если известен)", "Объект / пласт"),
		cycleStart:           failureWorkbookColumn(header, "Дата запуска цикла", "Дата пуска", "Дата пуска*"),
		failureDate:          failureWorkbookColumn(header, "Дата отказа", "Дата отказа / окончания", "Дата отказа / окончания*"),
		repairEnd:            failureWorkbookColumn(header, "Дата окончания ремонта"),
		operationMode:        failureWorkbookColumn(header, "Способ эксплуатации", "Способ эксплуатации*", "Способ эксплуатации / тип насоса*"),
		equipment:            failureWorkbookColumn(header, "Тип / компоновка ГНО", "Тип ГНО", "Компоновка ГНО"),
		equipmentSize:        failureWorkbookColumn(header, "Типоразмер ГНО 1", "Типоразмер ГНО 1*", "Типоразмер ГНО", "Типоразмер ГНО*"),
		equipmentSize2:       failureWorkbookColumn(header, "Типоразмер ГНО 2 (ОРД)"),
		equipmentBrand:       failureWorkbookColumn(header, "Марка ГНО", "Марка ГНО*"),
		eventType:            failureWorkbookColumn(header, "Тип события", "Тип события*"),
		failedUnit:           failureWorkbookColumn(header, "Отказавший узел", "Отказавший узел*"),
		stopCause:            failureWorkbookColumn(header, "Причина остановки / симптом", "Причина остановки", "Симптом", "Признак / предпосылка", "Признак / предпосылка*"),
		symptom:              failureWorkbookColumn(header, "Признак / предпосылка", "Признак / предпосылка*"),
		preliminaryCause:     failureWorkbookColumn(header, "Предварительная причина", "Предварительная причина*"),
		primaryComplication:  failureWorkbookColumn(header, "Основная категория осложнения", "Основная категория осложнения*"),
		associated1:          failureWorkbookColumn(header, "Сопутствующее осложнение 1"),
		associated2:          failureWorkbookColumn(header, "Сопутствующее осложнение 2"),
		tkrs1:                failureWorkbookColumn(header, "ТКРС 1", "[ДОП] ТКРС 1"),
		tkrs2:                failureWorkbookColumn(header, "ТКРС 2", "[ДОП] ТКРС 2"),
		damage:               failureWorkbookColumn(header, "Вид повреждения", "[ДОП] Вид повреждения"),
		finalCause:           failureWorkbookColumn(header, "Итоговая причина", "[ДОП] Итоговая причина"),
		refinedComplication:  failureWorkbookColumn(header, "Уточнённая категория осложнения", "[ДОП] Уточнённая категория осложнения"),
		methodBefore1:        failureWorkbookColumn(header, "Метод до отказа 1", "[ДОП] Метод до отказа 1", "Метод до отказа"),
		methodBefore2:        failureWorkbookColumn(header, "Метод до отказа 2", "[ДОП] Метод до отказа 2"),
		methodAfter1:         failureWorkbookColumn(header, "Метод после отказа 1", "[ДОП] Метод после отказа 1", "Метод после отказа"),
		methodAfter2:         failureWorkbookColumn(header, "Метод после отказа 2", "[ДОП] Метод после отказа 2"),
		confirmation:         failureWorkbookColumn(header, "Подтверждение", "[ДОП] Подтверждение"),
		investigationStatus:  failureWorkbookColumn(header, "Статус расследования", "[ДОП] Статус расследования"),
		confidence:           failureWorkbookColumn(header, "Достоверность", "[ДОП] Достоверность"),
		pumpNominal:          failureWorkbookColumn(header, "Условный диаметр / размер ШГН, мм", "Номинальная подача / размер"),
		pumpMaker:            failureWorkbookColumn(header, "Производитель насоса"),
		nktType:              failureWorkbookColumn(header, "тип НКТ", "Тип НКТ"),
		nktLength:            failureWorkbookColumn(header, "Длина НКТ, м"),
		nktYear:              failureWorkbookColumn(header, "год НКТ", "Год НКТ"),
		spo:                  failureWorkbookColumn(header, "СПО"),
		nktCondition:         failureWorkbookColumn(header, "состояние до спуска", "Состояние НКТ до спуска"),
		motorMaker:           failureWorkbookColumn(header, "Производитель двигателя"),
		motorSize:            failureWorkbookColumn(header, "Типоразмер двигателя (габарит), мм", "Типоразмер двигателя, мм"),
		motorYear:            failureWorkbookColumn(header, "год двигателя", "Год двигателя"),
		motorCondition:       failureWorkbookColumn(header, "состояние до спуска", "Состояние двигателя"),
		cableSize:            failureWorkbookColumn(header, "Типоразмер кабеля (сечение), мм²", "Типоразмер кабеля, мм²"),
		cableLength:          failureWorkbookColumn(header, "Длина кабеля, м"),
		cableCondition:       failureWorkbookColumn(header, "состояние до спуска", "Состояние кабеля"),
		extensionSize:        failureWorkbookColumn(header, "Типоразмер удлинителя (сечение), мм²", "Типоразмер удлинителя, мм²"),
		extensionLength:      failureWorkbookColumn(header, "Длина удлинителя, м"),
		extensionCondition:   failureWorkbookColumn(header, "состояние до спуска", "Состояние удлинителя"),
		hydroMaker:           failureWorkbookColumn(header, "Производитель гидрозащиты"),
		hydroSize:            failureWorkbookColumn(header, "Типоразмер гидрозащиты (габарит), мм", "Типоразмер гидрозащиты, мм"),
		hydroYear:            failureWorkbookColumn(header, "год гидрозащиты", "Год гидрозащиты"),
		hydroCondition:       failureWorkbookColumn(header, "состояние до спуска", "Состояние гидрозащиты"),
		tmsMaker:             failureWorkbookColumn(header, "Производитель ТМС"),
		tmsSize:              failureWorkbookColumn(header, "типоразмер ТМС", "Типоразмер ТМС"),
		tmsYear:              failureWorkbookColumn(header, "год ТМС", "Год ТМС"),
		rodMaker:             failureWorkbookColumn(header, "Производитель штанг"),
		commissionConclusion: failureWorkbookColumn(header, "Причина по заключению комиссии", "Заключение комиссии", "Заключение комиссии / акт", "[ДОП] Заключение комиссии / акт"),
		commissionSource:     failureWorkbookColumn(header, "Источник / документ", "[ДОП] Источник / документ"),
		source:               failureWorkbookColumn(header, "Источник / файл", "Источник"),
		comment:              failureWorkbookColumn(header, "Комментарий"),
		fountainType:         failureWorkbookColumn(header, "Тип насоса", "Тип насоса / способ эксплуатации", "Фонтанный способ / тип насоса"),
		fountainDepth:        failureWorkbookColumn(header, "Глубина спуска насоса, м", "Глубина насоса, м"),
		causeCode:            failureWorkbookColumn(header, "Код причины по исходному классификатору"),
		sourceCause:          failureWorkbookColumn(header, "Причина по исходному классификатору"),
		sourceClass:          failureWorkbookColumn(header, "Исходный класс", "Класс осложнения"),
		complicationFactor:   failureWorkbookColumn(header, "Осложняющий фактор указан"),
		inspection:           failureWorkbookColumn(header, "Обстоятельства отказа / осмотр / дефектовка", "Осмотр / дефектовка"),
	}
	// The current approved form intentionally repeats the caption «состояние до спуска»
	// in several equipment blocks. Resolve those positions from their unambiguous
	// left-hand anchor rather than from map iteration order.
	if c.nktType > 0 {
		c.nktCondition = c.nktType + 4
	}
	if c.motorMaker > 0 {
		c.motorCondition = c.motorMaker + 3
	}
	if c.cableSize > 0 {
		c.cableCondition = c.cableSize + 2
	}
	if c.extensionSize > 0 {
		c.extensionCondition = c.extensionSize + 2
	}
	if c.hydroMaker > 0 {
		c.hydroCondition = c.hydroMaker + 3
	}
	if c.tmsMaker > 0 {
		c.tmsCondition = c.tmsMaker + 3
	}
	// Rod captions are intentionally short in the approved form. Resolve the
	// seven-field block relative to its manufacturer column so repeated captions
	// such as «Год» and «СПО» cannot be mapped to the NKT block.
	if c.rodMaker == 0 {
		for column, title := range header {
			if normalizeTechHeader(title) == normalizeTechHeader("Производитель") && column > c.tmsMaker {
				c.rodMaker = column
				break
			}
		}
	}
	if c.rodMaker > 0 {
		c.rodCondition = c.rodMaker - 1
		c.rod19, c.rod22, c.rod25 = c.rodMaker+1, c.rodMaker+2, c.rodMaker+3
		c.rodYear, c.rodSPO = c.rodMaker+4, c.rodMaker+5
	}
	return c
}

func failureWorkbookValue(row map[int]string, column int) string {
	if column <= 0 {
		return ""
	}
	return failureNormalizeEntry(row[column])
}

func failureEventFromWorkbookRowWithSource(row map[int]string, columns failureWorkbookColumns, fileName, sheetName string, rowNumber int, importID, importedAt, importedBy string) FailureEvent {
	sourceDoc := failureWorkbookValue(row, columns.commissionSource)
	if sourceDoc == "" {
		sourceDoc = failureWorkbookValue(row, columns.source)
	}
	stopCause := failureWorkbookValue(row, columns.stopCause)
	if stopCause == "" {
		stopCause = failureWorkbookValue(row, columns.symptom)
	}
	preliminary := failureWorkbookValue(row, columns.preliminaryCause)
	if preliminary == "" {
		preliminary = failureWorkbookValue(row, columns.sourceCause)
	}
	primary := failureWorkbookValue(row, columns.primaryComplication)
	if primary == "" {
		primary = firstNonEmpty(failureWorkbookValue(row, columns.complicationFactor), failureWorkbookValue(row, columns.sourceClass))
	}
	equipment := failureWorkbookValue(row, columns.equipment)
	if equipment == "" {
		equipment = failureWorkbookValue(row, columns.operationMode)
	}
	event := FailureEvent{
		Department:              failureWorkbookValue(row, columns.department),
		ProductionUnit:          failureWorkbookValue(row, columns.productionUnit),
		Field:                   failureWorkbookValue(row, columns.field),
		Well:                    failureWorkbookValue(row, columns.well),
		ObjectPlast:             failureWorkbookValue(row, columns.objectPlast),
		CycleStartDate:          failureExcelDate(failureWorkbookValue(row, columns.cycleStart)),
		Date:                    failureExcelDate(failureWorkbookValue(row, columns.failureDate)),
		RepairEndDate:           failureExcelDate(failureWorkbookValue(row, columns.repairEnd)),
		EventType:               failureWorkbookValue(row, columns.eventType),
		OperationMode:           failureCanonicalOperationMode(failureWorkbookValue(row, columns.operationMode)),
		Equipment:               equipment,
		EquipmentSize:           failureWorkbookValue(row, columns.equipmentSize),
		EquipmentSize2:          failureWorkbookValue(row, columns.equipmentSize2),
		EquipmentSize3:          "",
		EquipmentSize4:          "",
		EquipmentBrand:          failureWorkbookValue(row, columns.equipmentBrand),
		FailedUnit:              failureWorkbookValue(row, columns.failedUnit),
		StopCause:               stopCause,
		PreliminaryCause:        preliminary,
		PrimaryComplication:     primary,
		AssociatedComplication1: failureWorkbookValue(row, columns.associated1),
		AssociatedComplication2: failureWorkbookValue(row, columns.associated2),
		TKRS1:                   failureWorkbookValue(row, columns.tkrs1),
		TKRS2:                   failureWorkbookValue(row, columns.tkrs2),
		DamageType:              failureWorkbookValue(row, columns.damage),
		FinalCause:              failureWorkbookValue(row, columns.finalCause),
		RefinedComplication:     failureWorkbookValue(row, columns.refinedComplication),
		MethodBeforeFailure:     failureWorkbookValue(row, columns.methodBefore1),
		MethodBeforeFailure2:    failureWorkbookValue(row, columns.methodBefore2),
		MethodAfterFailure:      failureWorkbookValue(row, columns.methodAfter1),
		MethodAfterFailure2:     failureWorkbookValue(row, columns.methodAfter2),
		Confirmation:            failureWorkbookValue(row, columns.confirmation),
		InvestigationStatus:     failureWorkbookValue(row, columns.investigationStatus),
		Confidence:              failureWorkbookValue(row, columns.confidence),
		CommissionConclusion:    failureWorkbookValue(row, columns.commissionConclusion),
		CommissionSource:        sourceDoc,
		SourceCauseCode:         failureWorkbookValue(row, columns.causeCode),
		SourceCause:             preliminary,
		NormalizedCause:         normalizeFailureCause(preliminary),
		SourceClass:             primary,
		NormalizedClass:         normalizeFailureClass(primary),
		ComplicationFactor:      primary,
		Inspection:              failureWorkbookValue(row, columns.inspection),
		Source:                  sourceDoc,
		Comment:                 failureWorkbookValue(row, columns.comment),
		FountainPumpType:        failureWorkbookValue(row, columns.fountainType),
		FountainPumpDepth:       failureWorkbookValue(row, columns.fountainDepth),
		Manual:                  false,
		SourceType:              "excel",
		SourceFile:              filepath.Base(fileName),
		SourceSheet:             sheetName,
		SourceRow:               rowNumber,
		SourceImportID:          importID,
		ImportedAt:              importedAt,
		ImportedBy:              importedBy,
		Extra:                   map[string]string{},
	}
	if event.EventType == "" {
		event.EventType = "Отказ"
	}
	if event.PreliminaryConclusion == "" && failureHasValue(event.PreliminaryCause) {
		event.PreliminaryConclusion = event.PreliminaryCause
	}
	if event.Inspection == "" {
		event.Inspection = event.DamageType
	}
	for key, value := range map[string]string{
		"AG": failureWorkbookValue(row, columns.nktType),
		"AH": failureWorkbookValue(row, columns.nktLength),
		"AI": failureWorkbookValue(row, columns.nktYear),
		"AJ": failureWorkbookValue(row, columns.spo),
		"AK": failureWorkbookValue(row, columns.nktCondition),
		"AL": event.FountainPumpType,
		"AM": failureWorkbookValue(row, columns.pumpNominal),
		"AN": failureWorkbookValue(row, columns.pumpMaker),
		"AO": event.FountainPumpDepth,
		"AP": failureWorkbookValue(row, columns.motorMaker),
		"AQ": failureWorkbookValue(row, columns.motorSize),
		"AR": failureWorkbookValue(row, columns.motorYear),
		"AS": failureWorkbookValue(row, columns.motorCondition),
		"AT": failureWorkbookValue(row, columns.cableSize),
		"AU": failureWorkbookValue(row, columns.cableLength),
		"AV": failureWorkbookValue(row, columns.cableCondition),
		"AW": failureWorkbookValue(row, columns.extensionSize),
		"AX": failureWorkbookValue(row, columns.extensionLength),
		"AY": failureWorkbookValue(row, columns.extensionCondition),
		"AZ": failureWorkbookValue(row, columns.hydroMaker),
		"BA": failureWorkbookValue(row, columns.hydroSize),
		"BB": failureWorkbookValue(row, columns.hydroYear),
		"BC": failureWorkbookValue(row, columns.hydroCondition),
		"BD": failureWorkbookValue(row, columns.tmsMaker),
		"BE": failureWorkbookValue(row, columns.tmsSize),
		"BF": failureWorkbookValue(row, columns.tmsYear),
		"BG": failureWorkbookValue(row, columns.tmsCondition),
		"BH": failureWorkbookValue(row, columns.rodCondition),
		"BI": failureWorkbookValue(row, columns.rodMaker),
		"BJ": failureWorkbookValue(row, columns.rod19),
		"BK": failureWorkbookValue(row, columns.rod22),
		"BL": failureWorkbookValue(row, columns.rod25),
		"BM": failureWorkbookValue(row, columns.rodYear),
		"BN": failureWorkbookValue(row, columns.rodSPO),
		"BO": event.CommissionConclusion,
		"BP": event.CommissionSource,
	} {
		if strings.TrimSpace(value) != "" {
			event.Extra[key] = value
		}
	}
	event.ID = failureNaturalID(event)
	refreshFailureDerived(&event)
	return event
}

func failureEventFromWorkbookRow(row map[int]string, columns failureWorkbookColumns, fileName string) FailureEvent {
	e := failureEventFromWorkbookRowWithSource(row, columns, fileName, "", 0, "", "", "")
	return e
}

func failureEventHasExtendedData(event FailureEvent) bool {
	refreshFailureDerived(&event)
	return event.HasFullData
}

var selectedFailureFile string
var pendingFailureImport []FailureImportCandidate
var failureObjectKeys []string
var failureObjectFilterActive bool
var failureImportError string

func failureBelongsToObject(event FailureEvent, key string) bool {
	if event.ObjectKey != "" {
		if event.ObjectKey == key {
			return true
		}
		// A history imported before its well appeared in the tech regime still
		// belongs to that well after the current regime is loaded.
		if event.ObjectKey != failureImportObjectKey(event) {
			return false
		}
	}
	if event.Manual || event.Field == "" || event.Well == "" {
		return false
	}
	keys := map[string]bool{}
	for _, st := range techCurrent {
		if !strings.EqualFold(strings.TrimSpace(st.Field), strings.TrimSpace(event.Field)) || !strings.EqualFold(strings.TrimSpace(st.Well), strings.TrimSpace(event.Well)) {
			continue
		}
		if event.ObjectPlast != "" && !strings.EqualFold(strings.TrimSpace(st.ObjectPlast), strings.TrimSpace(event.ObjectPlast)) {
			continue
		}
		keys[st.ObjectKey] = true
	}
	return len(keys) == 1 && keys[key]
}

func failureImportObjectKey(event FailureEvent) string {
	facts := strings.ToLower(strings.TrimSpace(event.Field)) + "|" + strings.ToLower(strings.TrimSpace(event.Well))
	sum := sha256.Sum256([]byte(facts))
	return "failure-well-" + hex.EncodeToString(sum[:12])
}

func failureObjectMatches(event FailureEvent) []string {
	var matches []string
	seen := map[string]bool{}
	for _, st := range techCurrent {
		if !strings.EqualFold(strings.TrimSpace(st.Field), strings.TrimSpace(event.Field)) ||
			!strings.EqualFold(strings.TrimSpace(st.Well), strings.TrimSpace(event.Well)) ||
			(event.ObjectPlast != "" && !strings.EqualFold(strings.TrimSpace(st.ObjectPlast), strings.TrimSpace(event.ObjectPlast))) {
			continue
		}
		if key := strings.TrimSpace(st.ObjectKey); key != "" && !seen[key] {
			matches = append(matches, key)
			seen[key] = true
		}
	}
	return matches
}

func failureExcelDate(value string) string {
	value = strings.TrimSpace(value)
	if date, err := failureDate(value); err == nil {
		return date.Format("02.01.2006")
	}
	serial, err := strconv.ParseFloat(value, 64)
	if err != nil || serial < 1 || serial > 2958465 {
		return value
	}
	return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(serial)).Format("02.01.2006")
}

func normalizeFailureClass(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	v = strings.NewReplacer("K", "К", "C", "С", "A", "А", "M", "М", "B", "В", "V", "В", "-", "", " ", "").Replace(v)
	if len([]rune(v)) == 2 && strings.Contains("КСАМВ", string([]rune(v)[0])) && strings.Contains("123", string([]rune(v)[1])) {
		return string([]rune(v)[0]) + "-" + string([]rune(v)[1])
	}
	return ""
}

func normalizeFailureCause(value string) string {
	// Only typography is normalized automatically. Engineering meaning is not
	// inferred from free text and the original wording is always retained.
	parts := strings.Split(value, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Join(strings.Fields(part), " ")
		if part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, " / ")
}

func failureFingerprint(event FailureEvent) string {
	facts := []string{
		event.Field,
		event.Well,
		event.ObjectPlast,
		event.CycleStartDate,
		event.Date,
		event.OperationMode,
		event.EventType,
		event.EquipmentSize,
	}
	for i := range facts {
		facts[i] = strings.ToLower(failureNormalizeEntry(facts[i]))
	}
	sum := sha256.Sum256([]byte(strings.Join(facts, "|")))
	return hex.EncodeToString(sum[:16])
}

func failureNaturalID(event FailureEvent) string {
	return "import-" + failureFingerprint(event)
}

func failureLegacyNaturalID(event FailureEvent) string {
	equipmentIdentity := strings.TrimSpace(event.Equipment)
	if strings.TrimSpace(event.OperationMode) != "" {
		equipmentIdentity = strings.Trim(strings.Join([]string{event.OperationMode, event.Equipment}, " / "), " /")
	}
	facts := []string{event.Field, event.Well, event.ObjectPlast, event.Date, equipmentIdentity, event.FailedUnit}
	for i := range facts {
		facts[i] = strings.ToLower(strings.TrimSpace(facts[i]))
	}
	sum := sha256.Sum256([]byte(strings.Join(facts, "|")))
	return "import-" + hex.EncodeToString(sum[:16])
}

func failureComparable(event FailureEvent) FailureEvent {
	event.UpdatedAt = ""
	event.CorrectedByUser = false
	event.FullCompletionPct = 0
	event.NoDataCount = 0
	event.HasFullData = false
	event.OperationalComplete = false
	event.OperationalOnly = false
	event.SourceImportID = ""
	event.ImportedAt = ""
	event.ImportedBy = ""
	event.SourceFile = ""
	event.SourceSheet = ""
	event.SourceRow = 0
	return event
}

func failureEventsEquivalent(a, b FailureEvent) bool {
	a = failureComparable(a)
	b = failureComparable(b)
	a.ObjectKey, b.ObjectKey = "", ""
	aa, _ := xml.Marshal(a)
	bb, _ := xml.Marshal(b)
	return string(aa) == string(bb)
}

func failureEventsEquivalentIgnoringReview(a, b FailureEvent) bool {
	a = failureComparable(a)
	b = failureComparable(b)
	aa, _ := xml.Marshal(a)
	bb, _ := xml.Marshal(b)
	return string(aa) == string(bb)
}

func failureExistingByCandidate(candidate FailureEvent) (FailureEvent, bool) {
	for _, old := range failureEvents {
		if old.ID == candidate.ID || old.ID == failureLegacyNaturalID(candidate) {
			return old, true
		}
		if !old.Manual && failureFingerprint(old) == failureFingerprint(candidate) {
			return old, true
		}
	}
	return FailureEvent{}, false
}

func mergeFailureExtraPreservingExisting(old, incoming map[string]string) (map[string]string, int, []string) {
	merged := make(map[string]string, len(old)+len(incoming))
	for k, v := range old {
		merged[k] = v
	}
	added := 0
	var conflicts []string
	for k, v := range incoming {
		v = failureNormalizeEntry(v)
		if v == "" {
			continue
		}
		cur := failureNormalizeEntry(merged[k])
		if !failureHasValue(cur) && !failureIsNoData(cur) {
			merged[k] = v
			added++
			continue
		}
		if cur != "" && !strings.EqualFold(cur, v) {
			conflicts = append(conflicts, k)
		}
	}
	return merged, added, conflicts
}

func mergeImportedFailure(old, incoming FailureEvent) (FailureEvent, int, []string) {
	merged := old
	added := 0
	var conflicts []string
	merge := func(name string, dst *string, src string) {
		src = failureNormalizeEntry(src)
		if src == "" {
			return
		}
		cur := failureNormalizeEntry(*dst)
		if !failureHasValue(cur) && !failureIsNoData(cur) {
			*dst = src
			added++
			return
		}
		if cur != "" && !strings.EqualFold(cur, src) {
			conflicts = append(conflicts, name)
		}
	}
	merge("Подразделение", &merged.Department, incoming.Department)
	merge("ЦДНГ", &merged.ProductionUnit, incoming.ProductionUnit)
	merge("Месторождение", &merged.Field, incoming.Field)
	merge("Скважина", &merged.Well, incoming.Well)
	merge("Объект / пласт", &merged.ObjectPlast, incoming.ObjectPlast)
	merge("Дата пуска", &merged.CycleStartDate, incoming.CycleStartDate)
	merge("Дата отказа", &merged.Date, incoming.Date)
	merge("Дата окончания ремонта", &merged.RepairEndDate, incoming.RepairEndDate)
	merge("Тип события", &merged.EventType, incoming.EventType)
	merge("Способ эксплуатации", &merged.OperationMode, incoming.OperationMode)
	merge("Тип / компоновка ГНО", &merged.Equipment, incoming.Equipment)
	merge("Типоразмер ГНО 1", &merged.EquipmentSize, incoming.EquipmentSize)
	merge("Типоразмер ГНО 2", &merged.EquipmentSize2, incoming.EquipmentSize2)
	// GNO 3/4 are obsolete in the approved ORD schema. Legacy values are
	// deliberately ignored instead of being surfaced or copied forward.
	merged.EquipmentSize3 = ""
	merged.EquipmentSize4 = ""
	merge("Марка ГНО", &merged.EquipmentBrand, incoming.EquipmentBrand)
	merge("Отказавший узел", &merged.FailedUnit, incoming.FailedUnit)
	merge("Признак / предпосылка", &merged.StopCause, incoming.StopCause)
	merge("Предварительная причина", &merged.PreliminaryCause, incoming.PreliminaryCause)
	merge("Основное осложнение", &merged.PrimaryComplication, incoming.PrimaryComplication)
	merge("Сопутствующее осложнение 1", &merged.AssociatedComplication1, incoming.AssociatedComplication1)
	merge("Сопутствующее осложнение 2", &merged.AssociatedComplication2, incoming.AssociatedComplication2)
	merge("ТКРС 1", &merged.TKRS1, incoming.TKRS1)
	merge("ТКРС 2", &merged.TKRS2, incoming.TKRS2)
	merge("Вид повреждения", &merged.DamageType, incoming.DamageType)
	merge("Итоговая причина", &merged.FinalCause, incoming.FinalCause)
	merge("Уточнённая категория осложнения", &merged.RefinedComplication, incoming.RefinedComplication)
	merge("Метод до отказа 1", &merged.MethodBeforeFailure, incoming.MethodBeforeFailure)
	merge("Метод до отказа 2", &merged.MethodBeforeFailure2, incoming.MethodBeforeFailure2)
	merge("Метод после отказа 1", &merged.MethodAfterFailure, incoming.MethodAfterFailure)
	merge("Метод после отказа 2", &merged.MethodAfterFailure2, incoming.MethodAfterFailure2)
	merge("Подтверждение", &merged.Confirmation, incoming.Confirmation)
	merge("Статус расследования", &merged.InvestigationStatus, incoming.InvestigationStatus)
	merge("Достоверность", &merged.Confidence, incoming.Confidence)
	merge("Заключение комиссии / акт", &merged.CommissionConclusion, incoming.CommissionConclusion)
	merge("Источник / документ", &merged.CommissionSource, incoming.CommissionSource)
	merge("Источник", &merged.Source, incoming.Source)
	merge("Комментарий", &merged.Comment, incoming.Comment)
	merge("Тип насоса", &merged.FountainPumpType, incoming.FountainPumpType)
	merge("Глубина спуска насоса, м", &merged.FountainPumpDepth, incoming.FountainPumpDepth)
	mergedExtra, extraAdded, extraConflicts := mergeFailureExtraPreservingExisting(old.Extra, incoming.Extra)
	merged.Extra = mergedExtra
	added += extraAdded
	conflicts = append(conflicts, extraConflicts...)

	// Provenance of the latest import is always refreshed, while factual/user
	// fields above are never silently overwritten.
	merged.SourceType = "excel"
	merged.SourceFile = incoming.SourceFile
	merged.SourceSheet = incoming.SourceSheet
	merged.SourceRow = incoming.SourceRow
	merged.SourceImportID = incoming.SourceImportID
	merged.ImportedAt = incoming.ImportedAt
	merged.ImportedBy = incoming.ImportedBy
	merged.ID = old.ID
	refreshFailureDerived(&merged)
	return merged, added, conflicts
}

func classifyFailureCandidates(rows []FailureImportCandidate) []FailureImportCandidate {
	seen := map[string]bool{}
	for i := range rows {
		candidate := &rows[i]
		if candidate.Status == failureStatusKeyError {
			continue
		}
		if candidate.Event.ObjectKey == "" && candidate.Status == "" {
			candidate.Status = failureStatusKeyError
			candidate.Details = "невозможно однозначно определить скважину"
			continue
		}
		fingerprint := failureFingerprint(candidate.Event)
		if seen[fingerprint] {
			candidate.Status = failureStatusDuplicate
			candidate.Details = "повтор события внутри выбранного файла"
			continue
		}
		seen[fingerprint] = true
		old, ok := failureExistingByCandidate(candidate.Event)
		if !ok {
			candidate.Status = failureStatusNew
			candidate.Details = "будет добавлено"
			continue
		}
		merged, added, conflicts := mergeImportedFailure(old, candidate.Event)
		candidate.Event = merged
		candidate.Event.ID = old.ID
		candidate.OperationalOnly = merged.OperationalOnly
		if len(conflicts) > 0 {
			candidate.Status = failureStatusConflict
			preview := conflicts
			if len(preview) > 3 {
				preview = preview[:3]
			}
			candidate.Details = fmt.Sprintf("расхождения: %s", strings.Join(preview, ", "))
			if len(conflicts) > len(preview) {
				candidate.Details += fmt.Sprintf(" (+%d)", len(conflicts)-len(preview))
			}
			if added > 0 {
				candidate.Details += fmt.Sprintf("; безопасно дозаполнится полей: %d", added)
			} else {
				candidate.Details += "; заполненные значения не будут перезаписаны"
			}
			continue
		}
		if added > 0 || !failureEventsEquivalent(old, merged) {
			candidate.Status = failureStatusUpdate
			candidate.Details = fmt.Sprintf("будут заполнены ранее пустые поля: %d", added)
		} else {
			candidate.Status = failureStatusDuplicate
			candidate.Details = "полностью совпадает с накопительным реестром"
		}
	}
	return rows
}

func failureImportUser() string {
	if currentSession == nil {
		return ""
	}
	if strings.TrimSpace(currentSession.DisplayName) != "" {
		return strings.TrimSpace(currentSession.DisplayName)
	}
	return strings.TrimSpace(currentSession.Login)
}

type decodedFailureSheet struct {
	Name      string
	HeaderRow int
	Header    map[int]string
	Rows      map[int]map[int]string
}

func readDecodedFailureSheet(z *zip.ReadCloser, shared []string, name string) (decodedFailureSheet, error) {
	path, err := xlsxWorksheetPath(z, name)
	if err != nil {
		return decodedFailureSheet{}, err
	}
	b, err := readZipEntry(z, path)
	if err != nil {
		return decodedFailureSheet{}, err
	}
	var worksheet xlsxSheet
	if err := xml.Unmarshal(b, &worksheet); err != nil {
		return decodedFailureSheet{}, err
	}
	out := decodedFailureSheet{Name: name, Rows: map[int]map[int]string{}}
	for index, sheetRow := range worksheet.Rows {
		rowNumber := sheetRow.R
		if rowNumber == 0 {
			rowNumber = index + 1
		}
		values := map[int]string{}
		for _, cell := range sheetRow.Cells {
			value := cell.V
			if cell.T == "s" {
				if n, parseErr := strconv.Atoi(value); parseErr == nil && n >= 0 && n < len(shared) {
					value = shared[n]
				}
			} else if cell.T == "inlineStr" {
				value = cell.Inline.T
			}
			values[colNumber(cell.R)] = strings.TrimSpace(value)
		}
		out.Rows[rowNumber] = values
		if out.HeaderRow == 0 && rowNumber <= 30 {
			hasField, hasWell := false, false
			for _, title := range values {
				n := normalizeTechHeader(title)
				hasField = hasField || strings.Contains(n, "месторождение")
				hasWell = hasWell || strings.Contains(n, "скваж")
			}
			if hasField && hasWell {
				out.HeaderRow = rowNumber
				out.Header = values
			}
		}
	}
	if out.HeaderRow == 0 {
		return decodedFailureSheet{}, fmt.Errorf("лист «%s»: не найдена строка заголовков", name)
	}
	return out, nil
}

func cloneFailureRow(row map[int]string) map[int]string {
	copyRow := make(map[int]string, len(row))
	for k, v := range row {
		copyRow[k] = v
	}
	return copyRow
}

func failureRowHasKeyData(row map[int]string, columns failureWorkbookColumns) bool {
	for _, column := range []int{columns.field, columns.well, columns.failureDate, columns.cycleStart} {
		if column > 0 && strings.TrimSpace(row[column]) != "" {
			return true
		}
	}
	return false
}

func finalizeFailureCandidate(candidate *FailureImportCandidate, columns failureWorkbookColumns, raw map[int]string) {
	if candidate == nil {
		return
	}
	event := &candidate.Event
	refreshFailureDerived(event)
	candidate.OperationalOnly = event.OperationalOnly
	if !failureHasValue(event.Field) || !failureHasValue(event.Well) || !failureHasValue(event.Date) {
		candidate.Status, candidate.Details = failureStatusKeyError, "не заполнены месторождение, скважина или дата отказа"
		return
	}
	if _, err := failureDate(event.Date); err != nil {
		candidate.Status, candidate.Details = failureStatusKeyError, err.Error()
		return
	}
	if failureHasValue(event.CycleStartDate) {
		if _, err := failureDate(event.CycleStartDate); err != nil {
			candidate.Status, candidate.Details = failureStatusKeyError, "дата пуска: "+err.Error()
			return
		}
	}
	if candidate.Status == "" {
		switch matches := failureObjectMatches(*event); len(matches) {
		case 0:
			event.ObjectKey = failureImportObjectKey(*event)
		case 1:
			event.ObjectKey = matches[0]
		default:
			candidate.Status = failureStatusKeyError
			candidate.Details = "несколько объектов Техрежима соответствуют скважине; требуется уточнение"
		}
	}
	_ = columns
	_ = raw
}

func readFailureWorkbook(path string) ([]FailureImportCandidate, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 80<<20 {
		return nil, errors.New("форма отказов превышает 80 МБ")
	}
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fileHash := sha256.Sum256(fileBytes)
	importID := "xlsx-" + hex.EncodeToString(fileHash[:12])
	importedAt := time.Now().Format(time.RFC3339)
	importedBy := failureImportUser()

	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	var shared []string
	if b, sharedErr := readZipEntry(z, "xl/sharedStrings.xml"); sharedErr == nil {
		var ss xlsxSharedStrings
		if xml.Unmarshal(b, &ss) == nil {
			for _, item := range ss.Items {
				shared = append(shared, sharedText(item))
			}
		}
	}
	workbookData, err := readZipEntry(z, "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var workbook xlsxWorkbook
	if err = xml.Unmarshal(workbookData, &workbook); err != nil {
		return nil, err
	}
	available := map[string]string{}
	for _, item := range workbook.Sheets {
		available[strings.ToLower(strings.TrimSpace(item.Name))] = item.Name
	}

	var operational, history *decodedFailureSheet
	if actual := available[strings.ToLower("Оперативный ввод")]; actual != "" {
		sheet, sheetErr := readDecodedFailureSheet(z, shared, actual)
		if sheetErr != nil {
			return nil, sheetErr
		}
		operational = &sheet
	}
	for _, name := range []string{"История отказов", "Расширенный реестр", "Накопительная_сводка"} {
		if actual := available[strings.ToLower(name)]; actual != "" {
			sheet, sheetErr := readDecodedFailureSheet(z, shared, actual)
			if sheetErr != nil {
				return nil, sheetErr
			}
			history = &sheet
			break
		}
	}
	if operational == nil && history == nil {
		return nil, errors.New("в файле не найдены листы «Оперативный ввод» или «История отказов»")
	}

	byRow := map[int]FailureImportCandidate{}
	if operational != nil {
		columns := failureWorkbookSchema(operational.Header)
		if columns.field == 0 || columns.well == 0 || columns.failureDate == 0 {
			return nil, errors.New("лист «Оперативный ввод»: не распознаны месторождение, № скважины или дата отказа")
		}
		for rowNumber, row := range operational.Rows {
			if rowNumber <= operational.HeaderRow || !failureRowHasKeyData(row, columns) {
				continue
			}
			event := failureEventFromWorkbookRowWithSource(row, columns, path, operational.Name, rowNumber, importID, importedAt, importedBy)
			candidate := FailureImportCandidate{Row: rowNumber, Event: event}
			finalizeFailureCandidate(&candidate, columns, row)
			byRow[rowNumber] = candidate
		}
	}

	if history != nil {
		columns := failureWorkbookSchema(history.Header)
		if columns.field == 0 || columns.well == 0 || columns.failureDate == 0 {
			return nil, errors.New("лист «История отказов»: не распознаны месторождение, № скважины или дата отказа")
		}
		for rowNumber, originalRow := range history.Rows {
			if rowNumber <= history.HeaderRow {
				continue
			}
			row := cloneFailureRow(originalRow)
			// In the current approved form columns A:R on «История отказов» are
			// formula mirrors of «Оперативный ввод». Some producers omit cached
			// values in XLSX; recover only these aligned common columns by row.
			if operational != nil {
				if opRow := operational.Rows[rowNumber]; opRow != nil {
					for column := 1; column <= 18; column++ {
						if strings.TrimSpace(row[column]) == "" && strings.TrimSpace(opRow[column]) != "" {
							row[column] = opRow[column]
						}
					}
				}
			}
			if !failureRowHasKeyData(row, columns) {
				continue
			}
			detail := failureEventFromWorkbookRowWithSource(row, columns, path, history.Name, rowNumber, importID, importedAt, importedBy)
			if base, ok := byRow[rowNumber]; ok {
				merged, _, _ := mergeImportedFailure(base.Event, detail)
				merged.ID = failureNaturalID(merged)
				merged.SourceSheet = operational.Name + " + " + history.Name
				merged.SourceRow = rowNumber
				refreshFailureDerived(&merged)
				base.Event = merged
				base.OperationalOnly = merged.OperationalOnly
				base.Status, base.Details = "", ""
				finalizeFailureCandidate(&base, columns, row)
				byRow[rowNumber] = base
			} else {
				candidate := FailureImportCandidate{Row: rowNumber, Event: detail}
				finalizeFailureCandidate(&candidate, columns, row)
				byRow[rowNumber] = candidate
			}
		}
	}

	rows := make([]FailureImportCandidate, 0, len(byRow))
	for _, candidate := range byRow {
		rows = append(rows, candidate)
	}
	if len(rows) == 0 {
		return nil, errors.New("в форме нет заполненных строк отказов")
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Row < rows[j].Row })
	return classifyFailureCandidates(rows), nil
}

func importFailureHistoryPath() string { return sharedPath("failure_import_history.jsonl") }

func recordFailureImport(row failureImportRecord) error {
	if err := os.MkdirAll(filepath.Dir(importFailureHistoryPath()), 0700); err != nil {
		return err
	}
	line, err := json.Marshal(row)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(importFailureHistoryPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(line, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func importFailureWorkbook(rows []FailureImportCandidate) (int, map[string]int, error) {
	counts := map[string]int{}
	applied := 0
	for _, row := range rows {
		counts[row.Status]++
		if row.Status != failureStatusNew && row.Status != failureStatusUpdate && row.Status != failureStatusConflict {
			continue
		}
		if row.Status == failureStatusConflict {
			if old, ok := failureExistingByCandidate(row.Event); ok && failureEventsEquivalent(old, row.Event) {
				continue
			}
		}
		row.Event.UpdatedAt = time.Now().Format(time.RFC3339)
		refreshFailureDerived(&row.Event)
		if err := recordFailureChange("save", row.Event); err != nil {
			return applied, counts, err
		}
		found := false
		for i := range failureEvents {
			if failureEvents[i].ID == row.Event.ID {
				failureEvents[i] = row.Event
				found = true
				break
			}
		}
		if !found {
			failureEvents = append(failureEvents, row.Event)
		}
		applied++
	}
	if applied > 0 {
		invalidateMethodsBackground()
	}
	return applied, counts, nil
}

func chooseFailureFile() string {
	buf := make([]uint16, 32768)
	filter := utf16.Encode([]rune("Excel (*.xlsx)\x00*.xlsx\x00\x00"))
	title := p("Выберите Excel-форму базы отказов РОСА")
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: title, Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST, LpstrDefExt: p("xlsx")}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func failurePreviewCounts(rows []FailureImportCandidate) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Status]++
	}
	return counts
}

func refreshFailureImportUI() {
	if failureImportList == 0 {
		return
	}
	if selectedFailureFile == "" {
		setText(failureImportFileLabel, "Файл: не выбран")
	} else {
		setText(failureImportFileLabel, "Источник формы: «Оперативный ввод» + «История отказов» · файл: "+filepath.Base(selectedFailureFile))
	}
	listBeginUpdate(failureImportList)
	defer listEndUpdate(failureImportList)
	listClear(failureImportList)
	visible := 0
	filterKey := selectedFailureObjectKey()
	for _, row := range pendingFailureImport {
		if filterKey != "" && !failureCandidateMatchesObject(row, filterKey) {
			continue
		}
		object := strings.Trim(strings.Join([]string{row.Event.Field, row.Event.Well, row.Event.ObjectPlast}, " / "), " / ")
		code := strings.TrimSpace(row.Event.SourceCauseCode)
		if code == "" {
			code = "—"
		}
		cause := strings.TrimSpace(row.Event.SourceCause)
		if cause == "" {
			cause = "—"
		}
		causeNorm := strings.TrimSpace(row.Event.NormalizedCause)
		if causeNorm == "" {
			causeNorm = "-"
		}
		details := row.Details
		if row.OperationalOnly {
			details = "Только оперативные данные: расширенная часть не заполнена" + func() string {
				if details == "" {
					return ""
				}
				return "; " + details
			}()
		}
		listAdd(failureImportList, visible, []string{row.Status, strconv.Itoa(row.Row), object, row.Event.Date, "код: " + code + "; причина: " + cause + " -> " + causeNorm, details})
		visible++
	}
	if len(pendingFailureImport) == 0 {
		switch {
		case failureImportError != "":
			setText(failureImportSummary, "Проверка не выполнена: "+failureImportError)
		case selectedFailureFile != "" && strings.TrimSpace(selectedObjectKey) != "":
			setText(failureImportSummary, "Файл выбран. Нажмите «Проверить». На вкладке будут показаны события закреплённого на Главной объекта.")
		case selectedFailureFile != "":
			setText(failureImportSummary, "Файл выбран. Для просмотра истории закрепите рабочий объект на Главной; импорт обработает всю книгу.")
		default:
			setText(failureImportSummary, "Выберите утверждённую Excel-форму отказов .xlsx")
		}
		return
	}
	c := failurePreviewCounts(pendingFailureImport)
	setText(failureImportSummary, fmt.Sprintf("Показано %d из %d | новых %d | обновлений %d | дублей %d | ошибок ключа %d | требуют проверки %d", visible, len(pendingFailureImport), c[failureStatusNew], c[failureStatusUpdate], c[failureStatusDuplicate], c[failureStatusKeyError], c[failureStatusConflict]))
}

func failureCandidateMatchesObject(candidate FailureImportCandidate, key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return true
	}
	if strings.TrimSpace(candidate.Event.ObjectKey) == key {
		return true
	}
	for _, st := range techCurrent {
		if st.ObjectKey == key && strings.EqualFold(strings.TrimSpace(st.Field), strings.TrimSpace(candidate.Event.Field)) && strings.EqualFold(strings.TrimSpace(st.Well), strings.TrimSpace(candidate.Event.Well)) {
			return true
		}
	}
	return false
}

func pendingFailureCountForObject(key string) int {
	count := 0
	for _, candidate := range pendingFailureImport {
		if failureCandidateMatchesObject(candidate, key) {
			count++
		}
	}
	return count
}

func refreshFailureObjectChooser() {
	if failureObjectCombo == 0 {
		return
	}
	pSendMessageW.Call(uintptr(failureObjectCombo), CB_RESETCONTENT, 0, 0)
	failureObjectKeys = nil
	seen := map[string]bool{}
	selected := -1
	// The chooser is a failure-history chooser, not a copy of the whole
	// TechRegime object list. Once a workbook is loaded, show only keys that
	// actually occur in that workbook. After acceptance, use persisted events.
	events := failurePreviewEvents()
	if len(events) == 0 {
		events = append(events, failureEvents...)
	}
	for _, event := range events {
		key := strings.TrimSpace(event.ObjectKey)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		failureObjectKeys = append(failureObjectKeys, key)
		caption := strings.Trim(strings.Join([]string{event.Field, "скв. " + event.Well, event.ObjectPlast}, " / "), " / ")
		for _, view := range allObjectViews() {
			if strings.EqualFold(strings.TrimSpace(objectViewKey(view)), key) {
				caption = objectViewLabel(view)
				break
			}
		}
		pSendMessageW.Call(uintptr(failureObjectCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
		if strings.EqualFold(key, strings.TrimSpace(failurePreviewObjectKey)) || failurePreviewObjectKey == "" && strings.EqualFold(key, strings.TrimSpace(selectedObjectKey)) {
			selected = len(failureObjectKeys) - 1
		}
	}
	// With no imported or persisted failure rows, fall back to the current
	// object list so the empty state remains usable for manual entry.
	if len(failureObjectKeys) == 0 {
		for _, v := range allObjectViews() {
			key := strings.TrimSpace(objectViewKey(v))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			failureObjectKeys = append(failureObjectKeys, key)
			caption := objectViewLabel(v)
			pSendMessageW.Call(uintptr(failureObjectCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
			if strings.EqualFold(key, strings.TrimSpace(failurePreviewObjectKey)) || failurePreviewObjectKey == "" && strings.EqualFold(key, strings.TrimSpace(selectedObjectKey)) {
				selected = len(failureObjectKeys) - 1
			}
		}
	}
	// A previously selected tech object may legitimately be absent from the
	// uploaded workbook. Do not leave the user with an empty filtered table:
	// select the first object that actually has imported rows and let the user
	// change it from the dropdown.
	if len(pendingFailureImport) > 0 && (selected < 0 || pendingFailureCountForObject(failureObjectKeys[selected]) == 0) {
		for i, key := range failureObjectKeys {
			if pendingFailureCountForObject(key) > 0 {
				selected = i
				break
			}
		}
	}
	if selected >= 0 {
		pSendMessageW.Call(uintptr(failureObjectCombo), CB_SETCURSEL, uintptr(selected), 0)
		if len(pendingFailureImport) > 0 {
			failurePreviewObjectKey = strings.TrimSpace(failureObjectKeys[selected])
			if strings.EqualFold(failurePreviewObjectKey, strings.TrimSpace(selectedObjectKey)) {
				failurePreviewObjectKey = ""
			}
		}
	}
	if failureObjectOpenButton != 0 {
		if failurePreviewObjectKey != "" && selectedObjectKey != "" {
			setText(failureObjectOpenButton, "К РАБОЧЕМУ ОБЪЕКТУ")
		} else {
			setText(failureObjectOpenButton, "ПОКАЗАТЬ ВСЕ ОТКАЗЫ")
		}
	}
}

func failurePreviewEvents() []FailureEvent {
	rows := make([]FailureEvent, 0, len(pendingFailureImport))
	for _, candidate := range pendingFailureImport {
		if candidate.Event.ObjectKey != "" {
			rows = append(rows, candidate.Event)
		}
	}
	return rows
}

func openFailureObjectHistoryUI() {
	if failurePreviewObjectKey != "" && !strings.EqualFold(failurePreviewObjectKey, selectedObjectKey) {
		returnToPinnedFailureObject()
		return
	}
	if failureObjectCombo == 0 {
		return
	}
	index, _, _ := pSendMessageW.Call(uintptr(failureObjectCombo), CB_GETCURSEL, 0, 0)
	if int(index) < 0 || int(index) >= len(failureObjectKeys) {
		msg("История отказов", "Сначала выберите объект из списка.", MB_OK|MB_ICONWARNING)
		return
	}
	key := strings.TrimSpace(failureObjectKeys[int(index)])
	if key == "" {
		return
	}
	if wellFormActive && wellDraftKey != key && !confirmFailureFormDiscard() {
		return
	}
	selectFailureObject(key)
}

// selectFailureObject applies the combo selection immediately.  The old flow
// changed only the dashboard selection and left the import preview unchanged,
// which made it appear that the object could not be selected.
func selectFailureObject(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	wellFormActive = false
	wellEditingID = ""
	failurePreviewObjectKey = key
	if strings.EqualFold(key, selectedObjectKey) {
		failurePreviewObjectKey = ""
	}
	if failureObjectOpenButton != 0 {
		if failurePreviewObjectKey != "" && selectedObjectKey != "" {
			setText(failureObjectOpenButton, "К РАБОЧЕМУ ОБЪЕКТУ")
		} else {
			setText(failureObjectOpenButton, "ПОКАЗАТЬ ВСЕ ОТКАЗЫ")
		}
	}
	failureObjectFilterActive = true
	// The dropdown is a temporary result filter. Only confirmation on the Main
	// page may change the persisted pinned object.
	switchPage(ID_OBJECTS)
	refreshWellCard()
	refreshFailureImportUI()
	refreshWellCardFailures()
}

var failurePreviewObjectKey string

func returnToPinnedFailureObject() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Закреплённый объект", "Сначала выберите и закрепите объект на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	failurePreviewObjectKey = ""
	setText(failureObjectOpenButton, "ПОКАЗАТЬ ВСЕ ОТКАЗЫ")
	refreshFailureObjectChooser()
	refreshWellCardFailures()
	refreshWellCard()
}

func selectedFailureObjectKey() string {
	// The register chooser is a local read-only filter. It never changes the
	// object pinned on Main or the context of any other workflow tab.
	if key := strings.TrimSpace(failurePreviewObjectKey); key != "" {
		return key
	}
	return strings.TrimSpace(selectedObjectKey)
}

func openFailureAnalysisUI() {
	key := selectedFailureObjectKey()
	if wellManualMode {
		key = strings.TrimSpace(selectedObjectKey)
	}
	if key == "" {
		msg("Анализ отказов объекта", "Сначала выберите объект в списке.", MB_OK|MB_ICONWARNING)
		return
	}
	rows := historyForWell(key, time.Now())
	if len(rows) == 0 {
		msg("Анализ отказов объекта", "Для выбранного объекта за последние 36 месяцев отказов не найдено.", MB_OK|MB_ICONINFORMATION)
		return
	}
	openFailureReportDialog("Анализ отказов объекта", failureAnalysisReport(rows, key), "анализ_отказов")
}

func failureNextActionRecommendations(rows []FailureEvent) []string {
	seen := map[string]bool{}
	var out []string
	add := func(text string) {
		if !seen[text] {
			seen[text] = true
			out = append(out, text)
		}
	}
	for _, event := range rows {
		class := strings.ToLower(strings.TrimSpace(event.SourceClass + " " + event.NormalizedClass + " " + event.ComplicationFactor))
		switch {
		case strings.Contains(class, "асп") || strings.Contains(class, "парафин"):
			add("Проверить фактическое дозирование и эффективность ингибитора АСПО, выполнить анализ состава отложений и уточнить периодичность обработки.")
		case strings.Contains(class, "корроз"):
			add("Проверить коррозионное состояние НКТ и штанг, защитное покрытие и работу ингибиторной защиты; зафиксировать результаты дефектовки.")
		case strings.Contains(class, "механ") || strings.Contains(class, "пес"):
			add("Проверить вынос механических примесей, состояние рабочих органов и фильтрацию; скорректировать режим эксплуатации при необходимости.")
		case strings.Contains(class, "сол"):
			add("Проверить склонность продукции к солеотложениям, работу противосолевой защиты и соответствие дозировки фактическому режиму.")
		case strings.Contains(class, "газ"):
			add("Проверить газовый фактор, герметичность и соответствие выбранного оборудования фактическому режиму работы скважины.")
		}
		if strings.TrimSpace(event.MethodAfterFailure) == "" {
			add("Заполнить применённый метод борьбы после отказа и оценить его результативность по следующему циклу работы.")
		}
		if strings.TrimSpace(event.CommissionConclusion) == "" {
			add("Дополнить заключение комиссии и прикрепить источник или акт расследования отказа.")
		}
	}
	if len(out) == 0 {
		out = append(out, "Проверить полноту расследования отказа, подтвердить причину комиссией и выбрать контролируемое мероприятие на следующий цикл.")
	}
	return out
}

func chooseFailureWorkbookUI() {
	file := chooseFailureFile()
	if file == "" {
		return
	}
	selectedFailureFile = file
	pendingFailureImport = nil
	failureImportError = ""
	failureObjectFilterActive = false
	refreshFailureImportUI()
	previewFailureWorkbookUI()
}

func safeReadFailureWorkbook(path string) (rows []FailureImportCandidate, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("внутренняя ошибка разбора XLSX: %v", r)
			diagLog("FAILURE preview panic=%v\n%s", r, debug.Stack())
		}
	}()
	return readFailureWorkbook(path)
}

func previewFailureWorkbookUI() {
	if selectedFailureFile == "" {
		msg("Проверка формы отказов", "Сначала выберите Excel-форму базы отказов.", MB_OK|MB_ICONWARNING)
		return
	}
	rows, err := safeReadFailureWorkbook(selectedFailureFile)
	if err != nil {
		pendingFailureImport = nil
		failureImportError = err.Error()
		refreshFailureImportUI()
		msg("Проверка формы отказов", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	failureImportError = ""
	pendingFailureImport = rows
	refreshFailureImportUI()
	refreshFailureObjectChooser()
	updateWellCardVisibility()
}

func applyFailureWorkbookUI() {
	if len(pendingFailureImport) == 0 {
		msg("Принятие формы отказов", "Нет проверенных строк. Сначала выберите файл и выполните проверку.", MB_OK|MB_ICONWARNING)
		return
	}
	c := failurePreviewCounts(pendingFailureImport)
	if c[failureStatusNew]+c[failureStatusUpdate]+c[failureStatusConflict] == 0 {
		msg("Принятие формы отказов", fmt.Sprintf("Нет строк для принятия. Ошибок ключа: %d, дублей: %d.", c[failureStatusKeyError], c[failureStatusDuplicate]), MB_OK|MB_ICONWARNING)
		return
	}
	noteText := fmt.Sprintf("Новые события: %d, безопасные обновления: %d, требуют проверки: %d. В строках с расхождениями будут заполнены только ранее пустые поля; уже заполненные значения не перезаписываются. Продолжить?", c[failureStatusNew], c[failureStatusUpdate], c[failureStatusConflict])
	note := p(noteText)
	title := p("РОСА — форма базы отказов")
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(note)), uintptr(unsafe.Pointer(title)), 0x4|MB_ICONWARNING)
	if r != 6 {
		return
	}
	applied, counts, err := importFailureWorkbook(pendingFailureImport)
	if err != nil {
		msg("Принятие формы отказов", fmt.Sprintf("Записано строк: %d. Ошибка: %v", applied, err), MB_OK|MB_ICONERROR)
		return
	}
	fileData, _ := os.ReadFile(selectedFailureFile)
	hash := sha256.Sum256(fileData)
	rowAudit := make([]failureImportRowRecord, 0, len(pendingFailureImport))
	for _, candidate := range pendingFailureImport {
		rowAudit = append(rowAudit, failureImportRowRecord{
			Sheet: candidate.Event.SourceSheet, Row: candidate.Row, ObjectKey: candidate.Event.ObjectKey,
			Status: candidate.Status, Details: candidate.Details,
		})
	}
	historyErr := recordFailureImport(failureImportRecord{When: time.Now().Format(time.RFC3339), User: failureImportUser(), File: filepath.Base(selectedFailureFile), FileSHA256: hex.EncodeToString(hash[:]), Total: len(pendingFailureImport), Counts: counts, Applied: applied, Rows: rowAudit})
	if historyErr != nil {
		msg("Принятие формы отказов", fmt.Sprintf("Данные записаны (%d), но журнал импорта сохранить не удалось: %v", applied, historyErr), MB_OK|MB_ICONWARNING)
	} else {
		msg("Принятие формы отказов", fmt.Sprintf("Записано строк: %d. Пропущено дублей: %d, ошибок ключа: %d, требуют проверки: %d. Исходные значения сохранены отдельно от нормализованных.", applied, counts[failureStatusDuplicate], counts[failureStatusKeyError], counts[failureStatusConflict]), MB_OK|MB_ICONINFORMATION)
	}
	pendingFailureImport = classifyFailureCandidates(pendingFailureImport)
	// The accepted rows are now visible in the first module's history table.
	// Clear the preview so the user cannot accidentally accept the same batch twice.
	pendingFailureImport = nil
	refreshData()
	refreshDashboard()
	refreshFailureObjectChooser()
	refreshWellCardFailures()
}

// Kept for older command routes and automated compatibility checks.
func importFailureWorkbookUI() { chooseFailureWorkbookUI() }
