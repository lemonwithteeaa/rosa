//go:build windows

package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

const (
	ID_TELEMETRY              = 115
	ID_TEL_IMPORT_TELESCOPE   = 240
	ID_TEL_IMPORT_MEGAWEB     = 241
	ID_TEL_IMPORT_WEBSCADA    = 242
	ID_TEL_IMPORT_GENERIC     = 243
	ID_TEL_USE_SELECTED       = 244
	ID_TEL_DELETE_IMPORT      = 245
	ID_TEL_CLEAR_OBJECT       = 246
	ID_TEL_PERIOD_7           = 247
	ID_TEL_PERIOD_30          = 248
	ID_TEL_PERIOD_90          = 249
	ID_TEL_PERIOD_ALL         = 250
	ID_TEL_ANALYZE_SCREENSHOT = 251
)

type TelemetryChannelSummary struct {
	Canonical  string  `json:"canonical"`
	Display    string  `json:"display"`
	SourceName string  `json:"source_name"`
	Unit       string  `json:"unit,omitempty"`
	Count      int     `json:"count"`
	Missing    int     `json:"missing"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Mean       float64 `json:"mean"`
	StdDev     float64 `json:"stddev"`
	TrendPct   float64 `json:"trend_pct,omitempty"`
	Outliers   int     `json:"outliers,omitempty"`
}

type TelemetryEvent struct {
	Kind        string `json:"kind"`
	Channel     string `json:"channel,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	EndedAt     string `json:"ended_at,omitempty"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type TelemetryImport struct {
	ID                string                    `json:"id"`
	ObjectKey         string                    `json:"object_key"`
	SourceSystem      string                    `json:"source_system"`
	SourceFile        string                    `json:"source_file"`
	SourceAsset       string                    `json:"source_asset,omitempty"`
	SourceSHA256      string                    `json:"source_sha256"`
	ImportedAt        string                    `json:"imported_at"`
	StartedAt         string                    `json:"started_at"`
	EndedAt           string                    `json:"ended_at"`
	Rows              int                       `json:"rows"`
	ValidTimeRows     int                       `json:"valid_time_rows"`
	InvalidTimeRows   int                       `json:"invalid_time_rows"`
	DuplicateTimes    int                       `json:"duplicate_times"`
	MedianIntervalSec float64                   `json:"median_interval_sec,omitempty"`
	MaxGapSec         float64                   `json:"max_gap_sec,omitempty"`
	Quality           string                    `json:"quality"`
	NormalizedPath    string                    `json:"normalized_path"`
	Channels          []TelemetryChannelSummary `json:"channels"`
	Events            []TelemetryEvent          `json:"events,omitempty"`
	Installation      InstallationProfile       `json:"installation,omitempty"`
}

type telemetryMap struct {
	index      int
	canonical  string
	display    string
	sourceName string
	unit       string
}

type runningStat struct {
	n       int
	missing int
	mean    float64
	m2      float64
	min     float64
	max     float64
	first   []float64
	last    []float64
	outlier int
}

func (s *runningStat) add(v float64) {
	if s.n == 0 {
		s.min, s.max = v, v
	} else {
		if v < s.min {
			s.min = v
		}
		if v > s.max {
			s.max = v
		}
	}
	// Count strong deviations only after the running baseline is established.
	if s.n >= 30 && s.m2 > 0 {
		sd := math.Sqrt(s.m2 / float64(s.n-1))
		if sd > 0 && math.Abs(v-s.mean) > 6*sd {
			s.outlier++
		}
	}
	s.n++
	d := v - s.mean
	s.mean += d / float64(s.n)
	s.m2 += d * (v - s.mean)
	if len(s.first) < 100 {
		s.first = append(s.first, v)
	}
	if len(s.last) < 100 {
		s.last = append(s.last, v)
	} else {
		copy(s.last, s.last[1:])
		s.last[len(s.last)-1] = v
	}
}

func (s *runningStat) summary(m telemetryMap) TelemetryChannelSummary {
	sd := 0.0
	if s.n > 1 {
		sd = math.Sqrt(s.m2 / float64(s.n-1))
	}
	avg := func(x []float64) float64 {
		if len(x) == 0 {
			return 0
		}
		z := 0.0
		for _, v := range x {
			z += v
		}
		return z / float64(len(x))
	}
	tr := 0.0
	a, b := avg(s.first), avg(s.last)
	if len(s.first) >= 5 && len(s.last) >= 5 && math.Abs(a) > 1e-12 {
		tr = (b - a) / math.Abs(a) * 100
	}
	return TelemetryChannelSummary{Canonical: m.canonical, Display: m.display, SourceName: m.sourceName, Unit: m.unit, Count: s.n, Missing: s.missing, Min: s.min, Max: s.max, Mean: s.mean, StdDev: sd, TrendPct: tr, Outliers: s.outlier}
}

var (
	telemetryImports                                                                             []TelemetryImport
	telObjectEdit, telInfo, telHistoryList, telChannelList, telEventList                         syscall.Handle
	telPageTitle, telDescription, telObjectLabel, telPeriodLabel, telChannelTitle, telEventTitle syscall.Handle
	telImportButtons                                                                             [4]syscall.Handle
	telAnalyzeScreenshotButton, telUseSelectedButton, telDeleteButton, telClearButton            syscall.Handle
	telPeriodButtons                                                                             [4]syscall.Handle
	telPeriodDays                                                                                int
)

func telemetryIndexPath() string { return filepath.Join(dataDir, "telemetry", "index.json") }
func loadTelemetry() {
	telemetryImports = nil
	loadJSON(telemetryIndexPath(), &telemetryImports)
	if ensureTelemetryExamples() {
		_ = saveTelemetry()
	}
}
func saveTelemetry() error {
	_ = os.MkdirAll(filepath.Dir(telemetryIndexPath()), 0755)
	return saveJSON(telemetryIndexPath(), telemetryImports)
}

func buildTelemetryUI() {
	buildGNOSubmoduleTabs(ID_TELEMETRY)
	telPageTitle = pageTitle(ID_TELEMETRY, "Состояние оборудования — мониторинг параметров ГНО", 242, 148, 980, 32)
	telDescription = labelEllipsis(ID_TELEMETRY, "Офлайн-импорт штатных выгрузок: Телескоп, MegaWeb, WebSCADA и универсальный CSV/XLSX. РОСА сохраняет исходные теги и приводит каналы к единому справочнику.", 242, 182, 1010, 24)
	telImportButtons[0] = button(ID_TELEMETRY, "Телескоп", 242, 232, 150, 40, ID_TEL_IMPORT_TELESCOPE)
	telImportButtons[1] = button(ID_TELEMETRY, "MegaWeb", 402, 232, 150, 40, ID_TEL_IMPORT_MEGAWEB)
	telImportButtons[2] = button(ID_TELEMETRY, "WebSCADA", 562, 232, 150, 40, ID_TEL_IMPORT_WEBSCADA)
	telImportButtons[3] = button(ID_TELEMETRY, "Другой CSV/XLSX", 722, 232, 190, 40, ID_TEL_IMPORT_GENERIC)
	telAnalyzeScreenshotButton = button(ID_TELEMETRY, "Анализировать скриншот", 922, 232, 220, 40, ID_TEL_ANALYZE_SCREENSHOT)
	telObjectLabel = label(ID_TELEMETRY, "Ключ объекта", 242, 288, 100, 24)
	telObjectEdit = edit(ID_TELEMETRY, "", 1030, 232, 222, 32, false, false, 0)
	telUseSelectedButton = button(ID_TELEMETRY, "Подставить выбранный объект", 242, 280, 250, 40, ID_TEL_USE_SELECTED)
	telPeriodLabel = label(ID_TELEMETRY, "Период анализа", 510, 288, 110, 24)
	telPeriodButtons[0] = button(ID_TELEMETRY, "7 дней", 625, 280, 90, 40, ID_TEL_PERIOD_7)
	telPeriodButtons[1] = button(ID_TELEMETRY, "30 дней", 722, 280, 90, 40, ID_TEL_PERIOD_30)
	telPeriodButtons[2] = button(ID_TELEMETRY, "90 дней", 819, 280, 90, 40, ID_TEL_PERIOD_90)
	telPeriodButtons[3] = button(ID_TELEMETRY, "Весь период", 916, 280, 100, 40, ID_TEL_PERIOD_ALL)
	telDeleteButton = button(ID_TELEMETRY, "Удалить импорт", 1025, 280, 125, 40, ID_TEL_DELETE_IMPORT)
	telClearButton = button(ID_TELEMETRY, "Очистить объект", 1159, 280, 135, 40, ID_TEL_CLEAR_OBJECT)
	telInfo = label(ID_TELEMETRY, "Импорт ещё не выполнен.", 242, 326, 1010, 66)
	telHistoryList = newList(ID_TELEMETRY, 242, 400, 1010, 118, []struct {
		n string
		w int32
	}{
		{"Дата импорта", 125}, {"Источник", 105}, {"Объект", 210}, {"Период", 220}, {"Строк", 80}, {"Каналов", 80}, {"Качество", 150},
	})
	setListHeaderFont(telHistoryList, hFontHeaderSmall)
	centerListHeaders(telHistoryList, 7)
	telChannelTitle = label(ID_TELEMETRY, "Каналы", 242, 532, 150, 24)
	telEventTitle = label(ID_TELEMETRY, "События данных (не диагноз)", 787, 532, 300, 24)
	telChannelList = newList(ID_TELEMETRY, 242, 560, 530, 210, []struct {
		n string
		w int32
	}{
		{"Параметр", 145}, {"Исходный тег", 115}, {"ед.", 50}, {"min", 60}, {"max", 60}, {"тренд", 68}, {"Пропуски", 78}, {"Выбросы", 72},
	})
	setListHeaderFont(telChannelList, hFontHeaderSmall)
	centerListHeaders(telChannelList, 8)
	telEventList = newList(ID_TELEMETRY, 787, 560, 465, 210, []struct {
		n string
		w int32
	}{
		{"Уровень", 80}, {"Канал", 120}, {"Событие", 250},
	})
	setListHeaderFont(telEventList, hFontHeaderSmall)
	centerListHeaders(telEventList, 3)
}

func layoutTelemetry(w, h int32) {
	contentW := max32(720, w-270)
	left := int32(242)
	setPos(telPageTitle, left, py(148), contentW, 42)
	setPos(telDescription, left, py(182), contentW, 24)

	// Keep all data sources aligned in one row. The screenshot analysis action
	// is the final accent control, consistent with other primary actions.
	gap := int32(8)
	importW := (contentW - 4*gap) / 5
	for i, control := range telImportButtons {
		setPos(control, left+int32(i)*(importW+gap), py(214), importW, 44)
	}
	setPos(telAnalyzeScreenshotButton, left+4*(importW+gap), py(214), importW, 44)

	infoY, historyY, sectionY, listBaseY := int32(332), int32(400), int32(532), int32(560)
	if contentW < 850 {
		// On compact windows, use a second filter row so captions keep their
		// standard size and buttons never collide.
		objectW := (contentW - gap) * 42 / 100
		selectedW := contentW - objectW - gap
		labelW := min32(82, max32(66, objectW/3))
		setPos(telObjectLabel, left, py(286), labelW, 24)
		setPos(telObjectEdit, left+labelW, py(280), objectW-labelW, 36)
		setPos(telUseSelectedButton, left+objectW+gap, py(280), selectedW, 44)

		periodW := contentW * 59 / 100
		actionsW := contentW - periodW - gap
		periodLabelW := int32(94)
		periodGap := int32(4)
		periodButtonsX := left + periodLabelW + 4
		periodButtonW := (periodW - periodLabelW - 4 - 3*periodGap) / 4
		setPos(telPeriodLabel, left, py(334), periodLabelW, 24)
		for i, control := range telPeriodButtons {
			setPos(control, periodButtonsX+int32(i)*(periodButtonW+periodGap), py(328), periodButtonW, 44)
		}
		actionX := left + periodW + gap
		actionGap := int32(6)
		actionButtonW := (actionsW - actionGap) / 2
		setPos(telDeleteButton, actionX, py(328), actionButtonW, 44)
		setPos(telClearButton, actionX+actionButtonW+actionGap, py(328), actionsW-actionButtonW-actionGap, 44)
		infoY, historyY, sectionY, listBaseY = 380, 448, 580, 608
	} else {
		// Object selection, period filters, and history actions form three groups.
		usable := contentW - 3*gap
		objectW := max32(210, usable*24/100)
		selectedW := max32(160, usable*18/100)
		periodW := max32(300, usable*34/100)
		actionsW := usable - objectW - selectedW - periodW
		x := left
		labelW := min32(82, max32(66, objectW/3))
		setPos(telObjectLabel, x, py(286), labelW, 24)
		setPos(telObjectEdit, x+labelW, py(280), objectW-labelW, 36)
		x += objectW + gap
		setPos(telUseSelectedButton, x, py(280), selectedW, 44)
		x += selectedW + gap
		periodLabelW := min32(88, max32(72, periodW/4))
		setPos(telPeriodLabel, x, py(286), periodLabelW, 24)
		periodButtonsX := x + periodLabelW + 4
		periodButtonsW := periodW - periodLabelW - 4
		periodGap := int32(4)
		periodButtonW := (periodButtonsW - 3*periodGap) / 4
		for i, control := range telPeriodButtons {
			setPos(control, periodButtonsX+int32(i)*(periodButtonW+periodGap), py(280), periodButtonW, 44)
		}
		x += periodW + gap
		actionGap := int32(6)
		actionButtonW := (actionsW - actionGap) / 2
		setPos(telDeleteButton, x, py(280), actionButtonW, 44)
		setPos(telClearButton, x+actionButtonW+actionGap, py(280), actionsW-actionButtonW-actionGap, 44)
	}

	setPos(telInfo, left, py(infoY), contentW, 60)
	setPos(telHistoryList, left, py(historyY), contentW, 122)
	leftW := max32(400, (contentW-15)/2)
	rightX := left + leftW + 15
	rightW := max32(300, contentW-leftW-15)
	listY := py(listBaseY)
	listH := max32(160, h-listY-24)
	setPos(telChannelTitle, left, py(sectionY), leftW, 24)
	setPos(telEventTitle, rightX, py(sectionY), rightW, 24)
	setPos(telChannelList, left, listY, leftW, listH)
	setPos(telEventList, rightX, listY, rightW, listH)
	fitTelemetryListColumns(telHistoryList, []int32{125, 105, 210, 220, 80, 80, 150})
	fitTelemetryListColumns(telChannelList, []int32{145, 115, 50, 60, 60, 68, 78, 72})
	fitTelemetryListColumns(telEventList, []int32{80, 120, 250})
}

func fitTelemetryListColumns(list syscall.Handle, base []int32) {
	if list == 0 || len(base) == 0 {
		return
	}
	var client RECT
	if result, _, _ := pGetClientRect.Call(uintptr(list), uintptr(unsafe.Pointer(&client))); result == 0 {
		return
	}
	available := client.Right - client.Left - 2
	if available <= 0 {
		return
	}
	total := int32(0)
	for _, width := range base {
		total += width
	}
	widths := make([]int32, len(base))
	used := int32(0)
	if total <= available {
		copy(widths, base)
		used = total
	} else {
		for i, width := range base {
			widths[i] = available * width / total
			used += widths[i]
		}
	}
	// The final named column absorbs any rounding or spare width.
	widths[len(widths)-1] += available - used
	for i, width := range widths {
		setListColumnWidth(list, i, max32(36, width))
	}
}

func refreshTelemetry() {
	if telHistoryList == 0 {
		return
	}
	listBeginUpdate(telHistoryList)
	defer listEndUpdate(telHistoryList)
	listClear(telHistoryList)
	q := strings.TrimSpace(getText(telObjectEdit))
	row := 0
	for _, x := range telemetryImports {
		if q != "" && x.ObjectKey != q {
			continue
		}
		p := x.StartedAt
		if x.EndedAt != "" && x.EndedAt != x.StartedAt {
			p += " — " + x.EndedAt
		}
		listAdd(telHistoryList, row, []string{x.ImportedAt, x.SourceSystem, x.ObjectKey, p, strconv.Itoa(x.Rows), strconv.Itoa(len(x.Channels)), x.Quality})
		row++
	}
	if q == "" {
		setText(telInfo, fmt.Sprintf("Импортов телеметрии: %d. Выберите объект, чтобы ограничить историю и анализ.", len(telemetryImports)))
	} else {
		setText(telInfo, fmt.Sprintf("Объект: %s. Период анализа: %s.", q, telemetryPeriodLabel()))
	}
	listClear(telChannelList)
	listClear(telEventList)
}

func telemetryPeriodLabel() string {
	if telPeriodDays <= 0 {
		return "весь доступный ряд"
	}
	return fmt.Sprintf("последние %d дней", telPeriodDays)
}

func filteredTelemetryImports() []TelemetryImport {
	q := strings.TrimSpace(getText(telObjectEdit))
	out := make([]TelemetryImport, 0, len(telemetryImports))
	for _, x := range telemetryImports {
		if q == "" || x.ObjectKey == q {
			out = append(out, x)
		}
	}
	return out
}

func selectTelemetryHistory() {
	a := filteredTelemetryImports()
	i := selectedRow(telHistoryList)
	if i < 0 || i >= len(a) {
		return
	}
	x := a[i]
	channels, events, periodStart, periodEnd, rows, err := telemetryPeriodSummary(x, telPeriodDays)
	if err != nil {
		setText(telInfo, "Не удалось рассчитать выбранный период: "+err.Error())
		return
	}
	listBeginUpdate(telChannelList)
	defer listEndUpdate(telChannelList)
	listBeginUpdate(telEventList)
	defer listEndUpdate(telEventList)
	listClear(telChannelList)
	listClear(telEventList)
	for j, c := range channels {
		tr := "—"
		if math.Abs(c.TrendPct) >= 0.01 {
			tr = fmt.Sprintf("%+.1f%%", c.TrendPct)
		}
		listAdd(telChannelList, j, []string{c.Display, c.SourceName, c.Unit, fmtTelemetryNumber(c.Min), fmtTelemetryNumber(c.Max), tr, telemetryMissingLabel(c), strconv.Itoa(c.Outliers)})
	}
	for j, e := range events {
		listAdd(telEventList, j, []string{e.Severity, e.Channel, e.Description})
	}
	if telPeriodDays <= 0 {
		setText(telInfo, fmt.Sprintf("%s • %s • %d строк • %d валидных отметок времени • медианный шаг %s • максимальный разрыв %s • качество: %s\r\n%s", x.SourceSystem, filepath.Base(x.SourceFile), x.Rows, x.ValidTimeRows, fmtSeconds(x.MedianIntervalSec), fmtSeconds(x.MaxGapSec), x.Quality, telemetryInstallationContext(x, channels)))
		return
	}
	setText(telInfo, fmt.Sprintf("%s • %s • анализ: %s — %s • точек: %d • %s\r\n%s", x.SourceSystem, filepath.Base(x.SourceFile), periodStart, periodEnd, rows, telemetryPeriodLabel(), telemetryInstallationContext(x, channels)))
}

func telemetryInstallationContext(x TelemetryImport, channels []TelemetryChannelSummary) string {
	p := x.Installation
	used, missing := installationProfileValueLines(p)
	if len(used) == 0 {
		return "Параметры установки на момент импорта отсутствовали; текущий профиль задним числом не подмешивается. Вывод ограничен каналами телеметрии."
	}
	var comparisons []string
	for _, c := range channels {
		switch c.Canonical {
		case "current":
			if p.NominalCurrent > 0 && (strings.TrimSpace(c.Unit) == "" || strings.Contains(strings.ToUpper(c.Unit), "А")) {
				comparisons = append(comparisons, fmt.Sprintf("средний ток %.3g А = %.1f %% номинального", c.Mean, c.Mean/p.NominalCurrent*100))
			}
		case "liquid_rate":
			if p.NominalRate > 0 {
				comparisons = append(comparisons, fmt.Sprintf("средний дебит жидкости %.3g м³/сут = %.1f %% номинальной подачи", c.Mean, c.Mean/p.NominalRate*100))
			}
		}
	}
	comparison := "совместимые каналы для числового сравнения не найдены"
	if len(comparisons) > 0 {
		comparison = strings.Join(comparisons, "; ")
	}
	missingText := "критически недостающие параметры не выявлены"
	if len(missing) > 0 {
		missingText = "отсутствует: " + strings.Join(missing, "; ")
	}
	return fmt.Sprintf("Профиль установки: снимок от %s. Сопоставлено: %s. %s. Это контекст, а не окончательный диагноз.", dash(p.UpdatedAt), comparison, missingText)
}

func telemetryPeriodSummary(x TelemetryImport, days int) ([]TelemetryChannelSummary, []TelemetryEvent, string, string, int, error) {
	if days <= 0 || strings.TrimSpace(x.NormalizedPath) == "" {
		return x.Channels, x.Events, x.StartedAt, x.EndedAt, x.ValidTimeRows, nil
	}
	end, err := time.ParseInLocation("02.01.2006 15:04", x.EndedAt, time.Local)
	if err != nil {
		return nil, nil, "", "", 0, fmt.Errorf("не распознана дата окончания ряда")
	}
	cutoff := end.Add(-time.Duration(days) * 24 * time.Hour)
	path, err := secureMaterialize(x.NormalizedPath, ".csv.gz")
	if err != nil {
		return nil, nil, "", "", 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, "", "", 0, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, "", "", 0, err
	}
	defer gz.Close()
	cr := csv.NewReader(gz)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err != nil || len(header) < 2 {
		return nil, nil, "", "", 0, fmt.Errorf("повреждён нормализованный ряд")
	}
	meta := map[string]TelemetryChannelSummary{}
	for _, c := range x.Channels {
		meta[c.Canonical] = c
	}
	maps := make([]telemetryMap, 0, len(header)-1)
	for i := 1; i < len(header); i++ {
		c := meta[header[i]]
		display := c.Display
		if display == "" {
			display = header[i]
		}
		maps = append(maps, telemetryMap{index: i, canonical: header[i], display: display, sourceName: c.SourceName, unit: c.Unit})
	}
	stats := make([]runningStat, len(maps))
	intervals := make([]float64, 0, 2048)
	var firstT, lastT, prevT time.Time
	rows := 0
	maxGap := 0.0
	for {
		rec, er := cr.Read()
		if er == io.EOF {
			break
		}
		if er != nil {
			return nil, nil, "", "", 0, er
		}
		if len(rec) == 0 {
			continue
		}
		t, er := time.Parse(time.RFC3339, strings.TrimSpace(rec[0]))
		if er != nil || t.Before(cutoff) || t.After(end.Add(time.Minute)) {
			continue
		}
		rows++
		if firstT.IsZero() || t.Before(firstT) {
			firstT = t
		}
		if lastT.IsZero() || t.After(lastT) {
			lastT = t
		}
		if !prevT.IsZero() {
			d := t.Sub(prevT).Seconds()
			if d > 0 {
				if len(intervals) < 10000 {
					intervals = append(intervals, d)
				}
				if d > maxGap {
					maxGap = d
				}
			}
		}
		prevT = t
		for j, m := range maps {
			raw := ""
			if m.index < len(rec) {
				raw = rec[m.index]
			}
			v, ok := parseTelemetryFloat(raw)
			if !ok {
				stats[j].missing++
				continue
			}
			stats[j].add(v)
		}
	}
	if rows == 0 {
		return nil, nil, cutoff.Format("02.01.2006 15:04"), end.Format("02.01.2006 15:04"), 0, fmt.Errorf("в выбранном диапазоне нет точек")
	}
	channels := make([]TelemetryChannelSummary, 0, len(maps))
	for i, m := range maps {
		if stats[i].n > 0 {
			channels = append(channels, stats[i].summary(m))
		}
	}
	median := medianFloat(intervals)
	events := buildTelemetryEvents(channels, median, maxGap, firstT, lastT)
	return channels, events, firstT.Format("02.01.2006 15:04"), lastT.Format("02.01.2006 15:04"), rows, nil
}

func fmtTelemetryNumber(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "—"
	}
	a := math.Abs(v)
	if a >= 1000 {
		return fmt.Sprintf("%.0f", v)
	}
	if a >= 100 {
		return fmt.Sprintf("%.1f", v)
	}
	if a >= 1 {
		return fmt.Sprintf("%.2f", v)
	}
	return fmt.Sprintf("%.4g", v)
}
func fmtSeconds(v float64) string {
	if v <= 0 {
		return "—"
	}
	d := time.Duration(v * float64(time.Second))
	if d >= 24*time.Hour {
		return fmt.Sprintf("%.1f сут.", d.Hours()/24)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%.1f ч", d.Hours())
	}
	if d >= time.Minute {
		return fmt.Sprintf("%.1f мин", d.Minutes())
	}
	return fmt.Sprintf("%.0f с", d.Seconds())
}

func useSelectedObjectForTelemetry() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Телеметрия", "Сначала выберите объект на странице «Объект и исходные данные».", MB_OK|MB_ICONWARNING)
		return
	}
	setText(telObjectEdit, selectedObjectKey)
	refreshTelemetry()
}

func handleTelemetryCommand(id int) bool {
	switch id {
	case ID_TEL_IMPORT_TELESCOPE:
		importTelemetryUI("Телескоп")
		return true
	case ID_TEL_IMPORT_MEGAWEB:
		importTelemetryUI("MegaWeb")
		return true
	case ID_TEL_IMPORT_WEBSCADA:
		importTelemetryUI("WebSCADA")
		return true
	case ID_TEL_IMPORT_GENERIC:
		importTelemetryUI("Универсальный")
		return true
	case ID_TEL_ANALYZE_SCREENSHOT:
		if strings.TrimSpace(getText(dynObjectEdit)) == "" && strings.TrimSpace(selectedObjectKey) != "" {
			setText(dynObjectEdit, selectedObjectKey)
		}
		switchPage(ID_DYN)
		// Reuse the existing image-analysis flow, which creates the in-program
		// reconstructed graph and technical summary for the selected screenshot.
		pPostMessageW.Call(uintptr(hMain), WM_COMMAND, uintptr(ID_DYN_CHOOSE), 0)
		return true
	case ID_TEL_USE_SELECTED:
		useSelectedObjectForTelemetry()
		return true
	case ID_TEL_DELETE_IMPORT:
		deleteSelectedTelemetryImport()
		return true
	case ID_TEL_CLEAR_OBJECT:
		clearTelemetryForObject()
		return true
	case ID_TEL_PERIOD_7:
		telPeriodDays = 7
		selectTelemetryHistory()
		return true
	case ID_TEL_PERIOD_30:
		telPeriodDays = 30
		selectTelemetryHistory()
		return true
	case ID_TEL_PERIOD_90:
		telPeriodDays = 90
		selectTelemetryHistory()
		return true
	case ID_TEL_PERIOD_ALL:
		telPeriodDays = 0
		selectTelemetryHistory()
		return true
	}
	return false
}

func chooseTelemetryFile() string {
	filter := "Телеметрия (*.csv;*.txt;*.xlsx)\x00*.csv;*.txt;*.xlsx\x00CSV (*.csv)\x00*.csv\x00Excel (*.xlsx)\x00*.xlsx\x00Все файлы\x00*.*\x00\x00"
	ft := utf16.Encode([]rune(filter))
	pathBuf := make([]uint16, 32768)
	ofn := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &ft[0], LpstrFile: &pathBuf[0], NMaxFile: uint32(len(pathBuf)), Flags: OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST | OFN_EXPLORER}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(pathBuf)
}

func importTelemetryUI(source string) {
	beginBusyLoading("Загрузка файла телеметрии…")
	defer endBusyLoading()
	objectKey := strings.TrimSpace(getText(telObjectEdit))
	if objectKey == "" {
		objectKey = strings.TrimSpace(selectedObjectKey)
	}
	if objectKey == "" {
		msg("Импорт телеметрии", "Сначала выберите объект или укажите его ключ.", MB_OK|MB_ICONWARNING)
		return
	}
	path := chooseTelemetryFile()
	if path == "" {
		return
	}
	setText(telInfo, "Импорт и анализ телеметрии…")
	imp, err := importTelemetry(path, source, objectKey)
	if err != nil {
		msg("Импорт телеметрии", err.Error(), MB_OK|MB_ICONERROR)
		refreshTelemetry()
		return
	}
	// Deduplicate by source hash + object. Same export should not silently multiply.
	for _, old := range telemetryImports {
		if old.ObjectKey == imp.ObjectKey && old.SourceSHA256 == imp.SourceSHA256 {
			msg("Импорт телеметрии", "Эта выгрузка уже была импортирована для выбранного объекта.", MB_OK|MB_ICONINFORMATION)
			refreshTelemetry()
			return
		}
	}
	telemetryImports = append(telemetryImports, imp)
	sort.SliceStable(telemetryImports, func(i, j int) bool { return telemetryImports[i].ImportedAt > telemetryImports[j].ImportedAt })
	if err := saveTelemetry(); err != nil {
		msg("Импорт телеметрии", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	refreshTelemetry()
	msg("Импорт телеметрии", fmt.Sprintf("Источник: %s\nОбъект: %s\nСтрок: %d\nКаналов: %d\nПериод: %s — %s\nКачество данных: %s", imp.SourceSystem, imp.ObjectKey, imp.Rows, len(imp.Channels), imp.StartedAt, imp.EndedAt, imp.Quality), MB_OK|MB_ICONINFORMATION)
}

func deleteSelectedTelemetryImport() {
	a := filteredTelemetryImports()
	i := selectedRow(telHistoryList)
	if i < 0 || i >= len(a) {
		msg("Телеметрия", "Выберите импорт в истории.", MB_OK|MB_ICONWARNING)
		return
	}
	target := a[i]
	text := fmt.Sprintf("Удалить выбранный импорт телеметрии?\n\n%s\n%s\n%s\n\nРасчёты, лаборатория и динамограммы не затрагиваются.", target.SourceSystem, target.ObjectKey, target.ImportedAt)
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Удаление телеметрии"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	out := telemetryImports[:0]
	for _, x := range telemetryImports {
		if x.ID != target.ID {
			out = append(out, x)
		}
	}
	telemetryImports = out
	if err := saveTelemetry(); err != nil {
		msg("Телеметрия", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	addSessionTrashItem("Импорт телеметрии", target.ObjectKey+"\r\n"+target.SourceSystem+"\r\n"+target.ImportedAt, func() error {
		telemetryImports = append(telemetryImports, target)
		sort.SliceStable(telemetryImports, func(i, j int) bool { return telemetryImports[i].ImportedAt > telemetryImports[j].ImportedAt })
		if err := saveTelemetry(); err != nil {
			return err
		}
		refreshTelemetry()
		return nil
	}, func() { removeTelemetryDataIfUnused(target.NormalizedPath) })
	refreshTelemetry()
}

func clearTelemetryForObject() {
	key := strings.TrimSpace(getText(telObjectEdit))
	if key == "" {
		key = strings.TrimSpace(selectedObjectKey)
	}
	if key == "" {
		msg("Телеметрия", "Укажите объект. Полная очистка телеметрии всего устройства из этого экрана не выполняется.", MB_OK|MB_ICONWARNING)
		return
	}
	count := 0
	for _, x := range telemetryImports {
		if x.ObjectKey == key {
			count++
		}
	}
	if count == 0 {
		msg("Телеметрия", "Для выбранного объекта нет импортов.", MB_OK|MB_ICONINFORMATION)
		return
	}
	text := fmt.Sprintf("Очистить всю телеметрию объекта?\n\n%s\nИмпортов: %d\n\nДействие не удаляет Техрежим, лабораторию, динамограммы и расчёты.", key, count)
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Очистка телеметрии"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	var removed []string
	removedImports := make([]TelemetryImport, 0, count)
	out := telemetryImports[:0]
	for _, x := range telemetryImports {
		if x.ObjectKey == key {
			removed = append(removed, x.NormalizedPath)
			removedImports = append(removedImports, x)
		} else {
			out = append(out, x)
		}
	}
	telemetryImports = out
	if err := saveTelemetry(); err != nil {
		msg("Телеметрия", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	paths := append([]string(nil), removed...)
	addSessionTrashItem(fmt.Sprintf("Телеметрия · %d импортов", count), key, func() error {
		telemetryImports = append(telemetryImports, removedImports...)
		sort.SliceStable(telemetryImports, func(i, j int) bool { return telemetryImports[i].ImportedAt > telemetryImports[j].ImportedAt })
		if err := saveTelemetry(); err != nil {
			return err
		}
		refreshTelemetry()
		return nil
	}, func() {
		for _, path := range paths {
			removeTelemetryDataIfUnused(path)
		}
	})
	refreshTelemetry()
}
func removeTelemetryDataIfUnused(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	for _, x := range telemetryImports {
		if x.NormalizedPath == path {
			return
		}
	}
	_ = os.Remove(path)
}

func importTelemetry(path, source, objectKey string) (TelemetryImport, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return TelemetryImport{}, err
	}
	if fi.Size() > 4*1024*1024*1024 {
		return TelemetryImport{}, fmt.Errorf("выгрузка больше 4 ГБ; разделите период на несколько файлов")
	}
	hash, err := fileSHA256(path)
	if err != nil {
		return TelemetryImport{}, err
	}
	sourceAsset, _, err := secureStoreFile(path)
	if err != nil {
		return TelemetryImport{}, fmt.Errorf("не удалось сохранить исходную телеметрию в защищённом хранилище: %w", err)
	}

	// Read only a small prefix to identify the header. The complete export is then
	// processed row-by-row. This keeps RAM bounded even for multi-gigabyte CSV/XLSX
	// telemetry exports.
	sample, err := telemetrySampleRows(path, 20)
	if err != nil {
		return TelemetryImport{}, err
	}
	if len(sample) < 2 {
		return TelemetryImport{}, fmt.Errorf("в выгрузке недостаточно строк")
	}
	headerRow := detectTelemetryHeader(sample)
	if headerRow < 0 || headerRow >= len(sample) {
		return TelemetryImport{}, fmt.Errorf("не удалось определить строку заголовков")
	}
	headers := sample[headerRow]
	maps, timeCols := mapTelemetryHeaders(headers, source)
	if timeCols.timestamp < 0 && timeCols.date < 0 {
		return TelemetryImport{}, fmt.Errorf("не найден столбец даты/времени. Нужен столбец «Дата и время» (DateTime) / «Метка времени» (Timestamp) либо отдельные «Дата» и «Время»")
	}
	if len(maps) == 0 {
		return TelemetryImport{}, fmt.Errorf("не найдено числовых/технологических каналов для анализа")
	}

	dir := filepath.Join(dataDir, "telemetry", "rows")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return TelemetryImport{}, err
	}
	normalized := filepath.Join(dir, hash+".csv.gz")
	out, err := os.Create(normalized)
	if err != nil {
		return TelemetryImport{}, err
	}
	gz := gzip.NewWriter(out)
	cw := csv.NewWriter(gz)
	cw.Comma = ';'
	normHeader := []string{"timestamp"}
	for _, m := range maps {
		normHeader = append(normHeader, m.canonical)
	}
	if err = cw.Write(normHeader); err != nil {
		_ = gz.Close()
		_ = out.Close()
		_ = os.Remove(normalized)
		return TelemetryImport{}, err
	}

	stats := make([]runningStat, len(maps))
	intervals := make([]float64, 0, 2048)
	var firstT, lastT, prevT time.Time
	rowsCount, validT, invalidT, dups := 0, 0, 0, 0
	maxGap := 0.0
	seen := map[int64]struct{}{}
	writeErr := error(nil)
	walkErr := walkTelemetryRows(path, func(rowIndex int, row []string) bool {
		if rowIndex <= headerRow || rowEmpty(row) {
			return true
		}
		rowsCount++
		t, ok := parseTelemetryTime(row, timeCols)
		if !ok {
			invalidT++
			return true
		}
		validT++
		if firstT.IsZero() || t.Before(firstT) {
			firstT = t
		}
		if lastT.IsZero() || t.After(lastT) {
			lastT = t
		}
		ts := t.UnixNano()
		if _, ok := seen[ts]; ok {
			dups++
		} else if len(seen) < 2_000_000 {
			seen[ts] = struct{}{}
		}
		if !prevT.IsZero() {
			d := t.Sub(prevT).Seconds()
			if d > 0 {
				if len(intervals) < 10000 {
					intervals = append(intervals, d)
				}
				if d > maxGap {
					maxGap = d
				}
			}
		}
		prevT = t
		rec := make([]string, 1+len(maps))
		rec[0] = t.Format(time.RFC3339)
		for j, m := range maps {
			raw := ""
			if m.index < len(row) {
				raw = strings.TrimSpace(row[m.index])
			}
			v, ok := parseTelemetryFloat(raw)
			if !ok {
				stats[j].missing++
				continue
			}
			stats[j].add(v)
			rec[j+1] = strconv.FormatFloat(v, 'g', -1, 64)
		}
		if err := cw.Write(rec); err != nil {
			writeErr = err
			return false
		}
		return true
	})
	cw.Flush()
	cerr := cw.Error()
	gerr := gz.Close()
	oerr := out.Close()
	if writeErr != nil {
		err = writeErr
	} else if walkErr != nil {
		err = walkErr
	} else if cerr != nil {
		err = cerr
	} else if gerr != nil {
		err = gerr
	} else if oerr != nil {
		err = oerr
	}
	if err != nil {
		_ = os.Remove(normalized)
		return TelemetryImport{}, err
	}
	if validT == 0 {
		_ = os.Remove(normalized)
		return TelemetryImport{}, fmt.Errorf("не удалось распознать ни одной отметки времени")
	}
	channels := make([]TelemetryChannelSummary, 0, len(maps))
	for i, m := range maps {
		if stats[i].n > 0 {
			channels = append(channels, stats[i].summary(m))
		}
	}
	if len(channels) == 0 {
		_ = os.Remove(normalized)
		return TelemetryImport{}, fmt.Errorf("в технологических каналах не найдено числовых значений")
	}
	median := medianFloat(intervals)
	quality := telemetryQuality(rowsCount, invalidT, dups, channels)
	events := buildTelemetryEvents(channels, median, maxGap, firstT, lastT)
	id := hash[:16] + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	secureNorm, _, secErr := secureStoreFile(normalized)
	_ = os.Remove(normalized)
	if secErr != nil {
		return TelemetryImport{}, fmt.Errorf("не удалось защитить нормализованный ряд: %w", secErr)
	}
	return TelemetryImport{ID: id, ObjectKey: objectKey, SourceSystem: source, SourceFile: filepath.Base(path), SourceAsset: sourceAsset, SourceSHA256: hash, ImportedAt: time.Now().Format("02.01.2006 15:04"), StartedAt: firstT.Format("02.01.2006 15:04"), EndedAt: lastT.Format("02.01.2006 15:04"), Rows: rowsCount, ValidTimeRows: validT, InvalidTimeRows: invalidT, DuplicateTimes: dups, MedianIntervalSec: median, MaxGapSec: maxGap, Quality: quality, NormalizedPath: secureNorm, Channels: channels, Events: events, Installation: installationProfileForObject(objectKey)}, nil
}

// walkTelemetryRows invokes fn for every physical row without retaining the full
// export in RAM. rowIndex is zero-based and includes pre-header rows.
func walkTelemetryRows(path string, fn func(rowIndex int, row []string) bool) error {
	if strings.EqualFold(filepath.Ext(path), ".xlsx") {
		return walkTelemetryXLSX(path, fn)
	}
	return walkTelemetryDelimited(path, fn)
}

func telemetrySampleRows(path string, limit int) ([][]string, error) {
	if limit < 1 {
		limit = 20
	}
	rows := make([][]string, 0, limit)
	err := walkTelemetryRows(path, func(_ int, row []string) bool {
		cpy := append([]string(nil), row...)
		rows = append(rows, cpy)
		return len(rows) < limit
	})
	return rows, err
}

func walkTelemetryDelimited(path string, fn func(rowIndex int, row []string) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	rowIndex := 0
	delim := rune(0)
	for sc.Scan() {
		b := append([]byte(nil), sc.Bytes()...)
		if rowIndex == 0 {
			b = []byte(strings.TrimPrefix(string(b), "\ufeff"))
		}
		if !utf8.Valid(b) {
			b = []byte(decodeCP1251(b))
		}
		line := strings.TrimRight(string(b), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if delim == 0 {
			delim = detectDelimiter(line)
		}
		r := csv.NewReader(strings.NewReader(line))
		r.Comma = delim
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		rec, er := r.Read()
		if er != nil {
			return fmt.Errorf("строка %d: %v", rowIndex+1, er)
		}
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
		}
		if !fn(rowIndex, rec) {
			return nil
		}
		rowIndex++
	}
	return sc.Err()
}

func loadTelemetrySharedStrings(z *zip.ReadCloser) []string {
	var shared []string
	if b, er := readZipEntry(z, "xl/sharedStrings.xml"); er == nil {
		var ss xlsxSharedStrings
		if xml.Unmarshal(b, &ss) == nil {
			for _, it := range ss.Items {
				shared = append(shared, sharedText(it))
			}
		}
	}
	return shared
}

func walkTelemetryXLSX(path string, fn func(rowIndex int, row []string) bool) error {
	if fi, err := os.Stat(path); err != nil {
		return err
	} else if fi.Size() > 512*1024*1024 {
		return fmt.Errorf("XLSX больше 512 МБ; для больших периодов используйте CSV-выгрузку")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer z.Close()
	shared := loadTelemetrySharedStrings(z)
	sheetPath, sheetErr := xlsxWorksheetPath(z, "")
	if sheetErr != nil {
		return fmt.Errorf("в XLSX не найден первый лист: %v", sheetErr)
	}
	var target *zip.File
	for _, f := range z.File {
		if f.Name == sheetPath {
			target = f
			break
		}
	}
	if target == nil {
		return fmt.Errorf("в XLSX не найден XML первого листа: %s", sheetPath)
	}
	rc, err := target.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	rowIndex := 0
	for {
		tok, er := dec.Token()
		if er == io.EOF {
			break
		}
		if er != nil {
			return er
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var rr struct {
			Cells []struct {
				R      string `xml:"r,attr"`
				T      string `xml:"t,attr"`
				V      string `xml:"v"`
				Inline struct {
					T string `xml:"t"`
				} `xml:"is"`
			} `xml:"c"`
		}
		if er = dec.DecodeElement(&rr, &se); er != nil {
			return er
		}
		maxCol := 0
		vals := map[int]string{}
		for _, c := range rr.Cells {
			col := colNumber(c.R)
			if col > maxCol {
				maxCol = col
			}
			v := c.V
			if c.T == "s" {
				i, _ := strconv.Atoi(c.V)
				if i >= 0 && i < len(shared) {
					v = shared[i]
				}
			} else if c.T == "inlineStr" {
				v = c.Inline.T
			}
			vals[col] = strings.TrimSpace(v)
		}
		if maxCol == 0 {
			continue
		}
		row := make([]string, maxCol)
		for i := 1; i <= maxCol; i++ {
			row[i-1] = vals[i]
		}
		if !fn(rowIndex, row) {
			return nil
		}
		rowIndex++
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func rowEmpty(r []string) bool {
	for _, x := range r {
		if strings.TrimSpace(x) != "" {
			return false
		}
	}
	return true
}

type telemetryTimeCols struct{ timestamp, date, time int }

func mapTelemetryHeaders(headers []string, source string) ([]telemetryMap, telemetryTimeCols) {
	tc := telemetryTimeCols{-1, -1, -1}
	var out []telemetryMap
	used := map[string]int{}
	profile := telemetryProfileFor(source)
	for i, h := range headers {
		raw := strings.TrimSpace(h)
		n := normalizeTelemetryName(raw)
		if profileAliasMatch(n, profile.TimestampAliases) {
			tc.timestamp = i
			continue
		}
		if profileAliasMatch(n, profile.DateAliases) {
			tc.date = i
			continue
		}
		if profileAliasMatch(n, profile.TimeAliases) {
			tc.time = i
			continue
		}
		if profileAliasMatch(n, profile.WellAliases) {
			continue
		}
		canonical, display, unit, ok := profileChannelMatch(profile, n)
		if !ok {
			canonical, display = canonicalTelemetryChannel(n, raw)
			unit = extractTelemetryUnit(raw)
		}
		if canonical == "" {
			continue
		}
		if unit == "" {
			unit = extractTelemetryUnit(raw)
		}
		if k := used[canonical]; k > 0 {
			canonical = fmt.Sprintf("%s_%d", canonical, k+1)
			display = display + fmt.Sprintf(" (%d)", k+1)
		}
		used[canonical]++
		out = append(out, telemetryMap{index: i, canonical: canonical, display: display, sourceName: raw, unit: unit})
	}
	return out, tc
}
func isAlias(n string, a []string) bool {
	for _, x := range a {
		if n == normalizeTelemetryName(x) {
			return true
		}
	}
	return false
}
func normalizeTelemetryName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func canonicalTelemetryChannel(n, raw string) (string, string) {
	tests := []struct {
		key, display string
		aliases      []string
	}{
		{"buffer_pressure", "Буферное давление (Рбуф)", []string{"pbuf", "рбуф", "pбуф", "буферноедавление", "давлениебуферное", "устьевоедавление", "wellheadpressure"}},
		{"annulus_pressure", "Затрубное давление (Рзат)", []string{"pzat", "рзат", "pзат", "затрубноедавление", "давлениезатрубное", "annuluspressure", "casingpressure"}},
		{"line_pressure", "Линейное давление (Рлин)", []string{"pline", "рлин", "pлин", "линейноедавление", "давлениелинейное", "linepressure"}},
		{"intake_pressure", "Давление (Р) на приёме", []string{"pintake", "рприем", "pприем", "давлениенаприеме", "приемноедавление", "intakepressure"}},
		{"bottomhole_pressure", "Забойное давление (Рзаб)", []string{"pbh", "рзаб", "pзаб", "забойноедавление", "bottomholepressure"}},
		{"reservoir_pressure", "Пластовое давление (Рпл)", []string{"pres", "рпл", "pпл", "пластовоедавление", "reservoirpressure"}},
		{"temperature", "Температура", []string{"t", "temp", "temperature", "температура", "темп"}},
		{"liquid_rate", "Дебит жидкости", []string{"qliq", "qж", "дебитжидкости", "liquidrate", "qfluid"}},
		{"oil_rate", "Дебит нефти", []string{"qoil", "qн", "дебитнефти", "oilrate"}},
		{"water_cut", "Обводнённость", []string{"watercut", "обводненность", "обводнённость", "wc", "водасодержание"}},
		{"frequency", "Частота", []string{"frequency", "freq", "частота", "частотавращения", "hz"}},
		{"current", "Ток", []string{"current", "ток", "i", "motorcurrent"}},
		{"voltage", "Напряжение", []string{"voltage", "напряжение", "u", "motorvoltage"}},
		{"vibration", "Вибрация", []string{"vibration", "вибрация", "vib"}},
	}
	for _, t := range tests {
		for _, a := range t.aliases {
			an := normalizeTelemetryName(a)
			if n == an || strings.Contains(n, an) && len(an) >= 4 {
				return t.key, t.display
			}
		}
	}
	// Preserve unknown numeric-looking technology columns instead of silently discarding them.
	// Metadata/text columns are excluded by common-name heuristics.
	if n == "" || strings.Contains(n, "comment") || strings.Contains(n, "коммент") || strings.Contains(n, "status") || strings.Contains(n, "статус") || strings.Contains(n, "name") || strings.Contains(n, "наимен") {
		return "", ""
	}
	slug := n
	if len([]rune(slug)) > 40 {
		slug = string([]rune(slug)[:40])
	}
	return "tag_" + slug, strings.TrimSpace(raw)
}
func extractTelemetryUnit(s string) string {
	for _, p := range [][2]string{{"[", "]"}, {"(", ")"}} {
		a := strings.LastIndex(s, p[0])
		b := strings.LastIndex(s, p[1])
		if a >= 0 && b > a {
			return strings.TrimSpace(s[a+1 : b])
		}
	}
	return ""
}

func parseTelemetryFloat(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\u00a0", ""))
	if s == "" || s == "-" || strings.EqualFold(s, "nan") {
		return 0, false
	}
	s = strings.ReplaceAll(s, " ", "")
	if strings.Count(s, ",") == 1 && strings.Count(s, ".") == 0 {
		s = strings.ReplaceAll(s, ",", ".")
	}
	v, e := strconv.ParseFloat(s, 64)
	return v, e == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func parseTelemetryTime(row []string, c telemetryTimeCols) (time.Time, bool) {
	s := ""
	if c.timestamp >= 0 && c.timestamp < len(row) {
		s = strings.TrimSpace(row[c.timestamp])
	} else {
		d := ""
		tm := ""
		if c.date >= 0 && c.date < len(row) {
			d = strings.TrimSpace(row[c.date])
		}
		if c.time >= 0 && c.time < len(row) {
			tm = strings.TrimSpace(row[c.time])
		}
		s = strings.TrimSpace(d + " " + tm)
	}
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "02.01.2006 15:04:05", "02.01.2006 15:04", "02.01.06 15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04", "02/01/2006 15:04:05", "01/02/2006 15:04:05", "02.01.2006"}
	for _, l := range layouts {
		if t, e := time.ParseInLocation(l, s, time.Local); e == nil {
			return t, true
		}
	}
	// Excel serial dates occasionally appear in exported worksheets.
	if v, e := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64); e == nil && v > 20000 && v < 100000 {
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.Local).Add(time.Duration(v * 24 * float64(time.Hour))), true
	}
	return time.Time{}, false
}

func medianFloat(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	y := append([]float64(nil), x...)
	sort.Float64s(y)
	m := len(y) / 2
	if len(y)%2 == 1 {
		return y[m]
	}
	return (y[m-1] + y[m]) / 2
}
func telemetryQuality(rows, invalid, dups int, ch []TelemetryChannelSummary) string {
	if rows <= 0 {
		return "Нет данных"
	}
	bad := float64(invalid+dups) / float64(rows)
	miss := 0.0
	for _, c := range ch {
		den := c.Count + c.Missing
		if den > 0 {
			r := float64(c.Missing) / float64(den)
			if r > miss {
				miss = r
			}
		}
	}
	if bad > 0.10 || miss > 0.50 {
		return "Низкое"
	}
	if bad > 0.02 || miss > 0.20 {
		return "Требует проверки"
	}
	return "Хорошее"
}
func telemetryMissingLabel(c TelemetryChannelSummary) string {
	den := c.Count + c.Missing
	if den <= 0 || c.Missing <= 0 {
		return "0"
	}
	return fmt.Sprintf("%d (%.0f%%)", c.Missing, float64(c.Missing)/float64(den)*100)
}

func buildTelemetryEvents(ch []TelemetryChannelSummary, median, maxGap float64, first, last time.Time) []TelemetryEvent {
	var out []TelemetryEvent
	for _, c := range ch {
		den := c.Count + c.Missing
		if den > 0 && c.Missing > 0 {
			ratio := float64(c.Missing) / float64(den)
			if ratio > 0.20 {
				sev := "Проверить"
				if ratio > 0.50 {
					sev = "Низкое качество"
				}
				out = append(out, TelemetryEvent{Kind: "missing", Channel: c.Display, Severity: sev, Description: fmt.Sprintf("Пропуски канала: %d из %d (%.0f%%). Не интерполировать незаметно; проверить выгрузку, связь и датчик.", c.Missing, den, ratio*100)})
			}
		}
		if c.Count >= 20 && math.Abs(c.TrendPct) >= 10 {
			dir := "рост"
			if c.TrendPct < 0 {
				dir = "снижение"
			}
			sev := "Наблюдение"
			if math.Abs(c.TrendPct) >= 25 {
				sev = "Выраженное"
			}
			out = append(out, TelemetryEvent{Kind: "trend", Channel: c.Display, StartedAt: first.Format("02.01.2006 15:04"), EndedAt: last.Format("02.01.2006 15:04"), Severity: sev, Description: fmt.Sprintf("Устойчивое изменение среднего уровня: %s %+.1f%% между началом и концом выгрузки", dir, c.TrendPct)})
		}
		if c.Outliers >= 3 {
			out = append(out, TelemetryEvent{Kind: "outliers", Channel: c.Display, Severity: "Проверить", Description: fmt.Sprintf("Обнаружены резкие выбросы: %d. Сначала исключите сбой датчика/связи.", c.Outliers)})
		}
	}
	if median > 0 && maxGap > median*5 {
		out = append(out, TelemetryEvent{Kind: "gap", Severity: "Проверить", Description: fmt.Sprintf("Разрыв временного ряда: до %s при типичном шаге %s.", fmtSeconds(maxGap), fmtSeconds(median))})
	}
	return out
}

func detectTelemetryHeader(rows [][]string) int {
	limit := len(rows)
	if limit > 20 {
		limit = 20
	}
	best, bestScore := -1, -1
	for i := 0; i < limit; i++ {
		score := 0
		for _, h := range rows[i] {
			n := normalizeTelemetryName(h)
			if isAlias(n, []string{"datetime", "timestamp", "дата", "время", "date", "time", "скважина", "well", "pbuf", "pzat", "температура", "temperature", "current", "ток"}) {
				score += 3
			} else if strings.TrimSpace(h) != "" {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	return best
}

func readTelemetryDelimited(path string) ([][]string, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var rows [][]string
	delim := rune(0)
	lineNo := 0
	for sc.Scan() {
		b := append([]byte(nil), sc.Bytes()...)
		if lineNo == 0 {
			b = []byte(strings.TrimPrefix(string(b), "\ufeff"))
		}
		if !utf8.Valid(b) {
			b = []byte(decodeCP1251(b))
		}
		line := strings.TrimRight(string(b), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if delim == 0 {
			delim = detectDelimiter(line)
		}
		r := csv.NewReader(strings.NewReader(line))
		r.Comma = delim
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		rec, er := r.Read()
		if er != nil {
			return nil, fmt.Errorf("строка %d: %v", lineNo+1, er)
		}
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
		}
		rows = append(rows, rec)
		lineNo++
		if len(rows) > 2_000_000 {
			return nil, fmt.Errorf("более 2 млн строк в одной выгрузке; разделите период, чтобы не перегружать локальный анализ")
		}
	}
	if e = sc.Err(); e != nil {
		return nil, e
	}
	return rows, nil
}
func detectDelimiter(s string) rune {
	cands := []rune{';', '\t', ','}
	best := ';'
	n := -1
	for _, c := range cands {
		k := strings.Count(s, string(c))
		if k > n {
			n = k
			best = c
		}
	}
	return best
}

func decodeCP1251(b []byte) string {
	var r []rune
	for _, c := range b {
		switch {
		case c < 0x80:
			r = append(r, rune(c))
		case c >= 0xC0:
			r = append(r, rune(0x0410+int(c)-0xC0))
		default:
			r = append(r, cp1251Rune(c))
		}
	}
	return string(r)
}
func cp1251Rune(c byte) rune {
	table := map[byte]rune{0xA8: 'Ё', 0xB8: 'ё', 0xAA: 'Є', 0xBA: 'є', 0xAF: 'Ї', 0xBF: 'ї', 0xB2: 'І', 0xB3: 'і', 0xA5: 'Ґ', 0xB4: 'ґ', 0x96: '–', 0x97: '—', 0xAB: '«', 0xBB: '»', 0xB0: '°', 0xB9: '№'}
	if r, ok := table[c]; ok {
		return r
	}
	return rune(c)
}

func readTelemetryXLSX(path string) ([][]string, error) {
	if fi, e := os.Stat(path); e != nil {
		return nil, e
	} else if fi.Size() > 512*1024*1024 {
		return nil, fmt.Errorf("XLSX больше 512 МБ; для больших периодов используйте CSV-выгрузку")
	}
	z, e := zip.OpenReader(path)
	if e != nil {
		return nil, e
	}
	defer z.Close()
	var shared []string
	if b, er := readZipEntry(z, "xl/sharedStrings.xml"); er == nil {
		var ss xlsxSharedStrings
		if xml.Unmarshal(b, &ss) == nil {
			for _, it := range ss.Items {
				shared = append(shared, sharedText(it))
			}
		}
	}
	sheet, sheetErr := xlsxWorksheetPath(z, "")
	if sheetErr != nil {
		return nil, fmt.Errorf("в XLSX не найден первый лист: %v", sheetErr)
	}
	var target *zip.File
	for _, f := range z.File {
		if f.Name == sheet {
			target = f
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("в XLSX не найден XML первого листа: %s", sheet)
	}
	rc, e := target.Open()
	if e != nil {
		return nil, e
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	var rows [][]string
	for {
		tok, er := dec.Token()
		if er == io.EOF {
			break
		}
		if er != nil {
			return nil, er
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var rr struct {
			Cells []struct {
				R      string `xml:"r,attr"`
				T      string `xml:"t,attr"`
				V      string `xml:"v"`
				Inline struct {
					T string `xml:"t"`
				} `xml:"is"`
			} `xml:"c"`
		}
		if er = dec.DecodeElement(&rr, &se); er != nil {
			return nil, er
		}
		max := 0
		vals := map[int]string{}
		for _, c := range rr.Cells {
			col := colNumber(c.R)
			if col > max {
				max = col
			}
			v := c.V
			if c.T == "s" {
				i, _ := strconv.Atoi(c.V)
				if i >= 0 && i < len(shared) {
					v = shared[i]
				}
			} else if c.T == "inlineStr" {
				v = c.Inline.T
			}
			vals[col] = strings.TrimSpace(v)
		}
		if max == 0 {
			continue
		}
		row := make([]string, max)
		for i := 1; i <= max; i++ {
			row[i-1] = vals[i]
		}
		rows = append(rows, row)
		if len(rows) > 2_000_000 {
			return nil, fmt.Errorf("более 2 млн строк в одной XLSX-выгрузке; разделите период")
		}
	}
	return rows, nil
}
