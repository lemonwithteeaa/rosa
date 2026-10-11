//go:build windows

package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
)

const pdfRenderPowerShell = `param([string]$InputPath,[string]$OutputPath,[int]$PageNumber)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1' })[0]
$asTaskAction = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and -not $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 })[0]
function Await-Operation($operation, [Type]$resultType) {
  $task = $asTaskGeneric.MakeGenericMethod($resultType).Invoke($null, @($operation))
  $task.Wait()
  return $task.Result
}
function Await-Action($operation) {
  $task = $asTaskAction.Invoke($null, @($operation))
  $task.Wait()
}
$storageFile = Await-Operation ([Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime]::GetFileFromPathAsync($InputPath)) ([Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime])
$document = Await-Operation ([Windows.Data.Pdf.PdfDocument,Windows.Data.Pdf,ContentType=WindowsRuntime]::LoadFromFileAsync($storageFile)) ([Windows.Data.Pdf.PdfDocument,Windows.Data.Pdf,ContentType=WindowsRuntime])
if ($document.PageCount -lt 1) { throw 'PDF не содержит страниц' }
if ($PageNumber -lt 1 -or $PageNumber -gt $document.PageCount) { throw "Страница $PageNumber отсутствует; всего страниц: $($document.PageCount)" }
$page = $document.GetPage([uint32]($PageNumber - 1))
$memory = [Windows.Storage.Streams.InMemoryRandomAccessStream,Windows.Storage.Streams,ContentType=WindowsRuntime]::new()
$options = [Windows.Data.Pdf.PdfPageRenderOptions,Windows.Data.Pdf,ContentType=WindowsRuntime]::new()
$options.DestinationWidth = 1800
Await-Action ($page.RenderToStreamAsync($memory, $options))
$memory.Seek(0)
$input = [System.IO.WindowsRuntimeStreamExtensions]::AsStreamForRead($memory)
$output = [System.IO.File]::Create($OutputPath)
try { $input.CopyTo($output) } finally { $output.Dispose(); $input.Dispose(); $memory.Dispose(); $page.Dispose() }
`

const pdfPageCountPowerShell = `param([string]$InputPath)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1' })[0]
function Await-Operation($operation, [Type]$resultType) {
  $task = $asTaskGeneric.MakeGenericMethod($resultType).Invoke($null, @($operation))
  $task.Wait()
  return $task.Result
}
$storageFile = Await-Operation ([Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime]::GetFileFromPathAsync($InputPath)) ([Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime])
$document = Await-Operation ([Windows.Data.Pdf.PdfDocument,Windows.Data.Pdf,ContentType=WindowsRuntime]::LoadFromFileAsync($storageFile)) ([Windows.Data.Pdf.PdfDocument,Windows.Data.Pdf,ContentType=WindowsRuntime])
[Console]::Out.Write($document.PageCount)
`

type pdfObject struct {
	body   []byte
	stream []byte
}

type pdfTextFragment struct {
	x, y float64
	text string
}

func renderPDFPageToPNG(path string, pageNumber ...int) (string, error) {
	page := 1
	if len(pageNumber) > 0 && pageNumber[0] > 0 {
		page = pageNumber[0]
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > 80*1024*1024 {
		return "", fmt.Errorf("PDF слишком большой: %.1f МБ; допустимо не более 80 МБ", float64(info.Size())/(1024*1024))
	}
	tmpDir, err := ensureSessionTempDir()
	if err != nil {
		return "", err
	}
	base := fmt.Sprintf("pdf_%x", sha256Bytes([]byte(path + info.ModTime().String()))[:8])
	scriptPath := filepath.Join(tmpDir, base+"_render.ps1")
	pngPath := filepath.Join(tmpDir, fmt.Sprintf("%s_page%d.png", base, page))
	if _, err = os.Stat(pngPath); err == nil {
		return pngPath, nil
	}
	if err = os.WriteFile(scriptPath, []byte("\xef\xbb\xbf"+pdfRenderPowerShell), 0600); err != nil {
		return "", err
	}
	defer os.Remove(scriptPath)
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, path, pngPath, strconv.Itoa(page))
	cmd.SysProcAttr = hiddenProcessAttributes()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("не удалось преобразовать страницу %d PDF в изображение средствами Windows: %v (%s)", page, err, strings.TrimSpace(string(output)))
	}
	if stat, statErr := os.Stat(pngPath); statErr != nil || stat.Size() < 1024 {
		return "", fmt.Errorf("Windows не создала изображение страницы PDF")
	}
	return pngPath, nil
}

func pdfPageCount(path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if info.Size() > 80*1024*1024 {
		return 0, fmt.Errorf("PDF слишком большой: %.1f МБ; допустимо не более 80 МБ", float64(info.Size())/(1024*1024))
	}
	tmpDir, err := ensureSessionTempDir()
	if err != nil {
		return 0, err
	}
	scriptPath := filepath.Join(tmpDir, fmt.Sprintf("pdf_%x_count.ps1", sha256Bytes([]byte(path + info.ModTime().String()))[:8]))
	if err = os.WriteFile(scriptPath, []byte("\xef\xbb\xbf"+pdfPageCountPowerShell), 0600); err != nil {
		return 0, err
	}
	defer os.Remove(scriptPath)
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, path)
	cmd.SysProcAttr = hiddenProcessAttributes()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("не удалось определить количество страниц PDF: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || count < 1 {
		return 0, fmt.Errorf("Windows вернула некорректное количество страниц PDF: %q", strings.TrimSpace(string(output)))
	}
	return count, nil
}

func sha256Bytes(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func hiddenProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

func parsePDFObjects(data []byte) map[int]pdfObject {
	objects := map[int]pdfObject{}
	re := regexp.MustCompile(`(?s)(\d+)\s+\d+\s+obj\b(.*?)\bendobj`)
	for _, match := range re.FindAllSubmatch(data, -1) {
		n, err := strconv.Atoi(string(match[1]))
		if err != nil {
			continue
		}
		body := append([]byte(nil), match[2]...)
		obj := pdfObject{body: body}
		streamAt := bytes.Index(body, []byte("stream"))
		endAt := bytes.LastIndex(body, []byte("endstream"))
		if streamAt >= 0 && endAt > streamAt {
			start := streamAt + len("stream")
			if start < len(body) && body[start] == '\r' {
				start++
			}
			if start < len(body) && body[start] == '\n' {
				start++
			}
			raw := body[start:endAt]
			raw = bytes.TrimSuffix(raw, []byte("\r\n"))
			raw = bytes.TrimSuffix(raw, []byte("\n"))
			if bytes.Contains(body[:streamAt], []byte("/FlateDecode")) {
				zr, zerr := zlib.NewReader(bytes.NewReader(raw))
				if zerr == nil {
					decoded, readErr := io.ReadAll(io.LimitReader(zr, 64*1024*1024))
					_ = zr.Close()
					if readErr == nil {
						obj.stream = decoded
					}
				}
			} else {
				obj.stream = append([]byte(nil), raw...)
			}
		}
		objects[n] = obj
	}
	return objects
}

func decodeUTF16Hex(value string) string {
	b, err := hex.DecodeString(value)
	if err != nil || len(b) < 2 {
		return ""
	}
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return string(utf16.Decode(units))
}

func parsePDFCMap(data []byte) map[uint16]string {
	out := map[uint16]string{}
	re := regexp.MustCompile(`(?i)<([0-9a-f]{4})>\s*<([0-9a-f]{4,})>`)
	for _, m := range re.FindAllSubmatch(data, -1) {
		src, err := strconv.ParseUint(string(m[1]), 16, 16)
		if err == nil {
			out[uint16(src)] = decodeUTF16Hex(string(m[2]))
		}
	}
	return out
}

func decodePDFHexText(value string, cmap map[uint16]string) string {
	b, err := hex.DecodeString(value)
	if err != nil {
		return ""
	}
	var out strings.Builder
	for i := 0; i+1 < len(b); i += 2 {
		code := uint16(b[i])<<8 | uint16(b[i+1])
		if s := cmap[code]; s != "" {
			out.WriteString(s)
		}
	}
	return out.String()
}

func extractPDFText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	objects := parsePDFObjects(data)
	fontRefs := map[string]int{}
	fontRefRE := regexp.MustCompile(`/((?:F|TT)\d+)\s+(\d+)\s+0\s+R`)
	for _, obj := range objects {
		if !bytes.Contains(obj.body, []byte("/Font")) {
			continue
		}
		for _, m := range fontRefRE.FindAllSubmatch(obj.body, -1) {
			n, _ := strconv.Atoi(string(m[2]))
			fontRefs[string(m[1])] = n
		}
	}
	fontMaps := map[string]map[uint16]string{}
	toUnicodeRE := regexp.MustCompile(`/ToUnicode\s+(\d+)\s+0\s+R`)
	for name, ref := range fontRefs {
		fontObj, ok := objects[ref]
		if !ok {
			continue
		}
		m := toUnicodeRE.FindSubmatch(fontObj.body)
		if len(m) == 0 {
			continue
		}
		n, _ := strconv.Atoi(string(m[1]))
		cmapObj, ok := objects[n]
		if !ok {
			continue
		}
		fontMaps[name] = parsePDFCMap(cmapObj.stream)
	}
	contents := [][]byte{}
	contentsRE := regexp.MustCompile(`(?s)/Contents\s*(?:\[([^]]+)\]|(\d+)\s+0\s+R)`)
	refRE := regexp.MustCompile(`(\d+)\s+0\s+R`)
	for _, obj := range objects {
		if !bytes.Contains(obj.body, []byte("/Type /Page")) || bytes.Contains(obj.body, []byte("/Type /Pages")) {
			continue
		}
		m := contentsRE.FindSubmatch(obj.body)
		if len(m) == 0 {
			continue
		}
		refs := m[2]
		if len(m[1]) > 0 {
			refs = m[1]
		}
		for _, rm := range refRE.FindAllSubmatch(refs, -1) {
			n, _ := strconv.Atoi(string(rm[1]))
			if stream := objects[n].stream; len(stream) > 0 {
				contents = append(contents, stream)
			}
		}
		break
	}
	if len(contents) == 0 {
		for _, obj := range objects {
			if bytes.Contains(obj.stream, []byte("BT")) {
				contents = append(contents, obj.stream)
			}
		}
	}
	blockRE := regexp.MustCompile(`(?s)q(.*?)Q`)
	cmRE := regexp.MustCompile(`([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+cm`)
	tmRE := regexp.MustCompile(`([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+([-+0-9.]+)\s+Tm`)
	fontRE := regexp.MustCompile(`/((?:F|TT)\d+)\s+[-+0-9.]+\s+Tf`)
	hexRE := regexp.MustCompile(`<([0-9A-Fa-f]+)>`)
	var fragments []pdfTextFragment
	for _, content := range contents {
		for _, blockMatch := range blockRE.FindAllSubmatch(content, -1) {
			block := blockMatch[1]
			if !bytes.Contains(block, []byte("BT")) {
				continue
			}
			fm, tm := fontRE.FindSubmatch(block), tmRE.FindSubmatch(block)
			if len(fm) == 0 || len(tm) == 0 {
				continue
			}
			cmap := fontMaps[string(fm[1])]
			var text strings.Builder
			for _, hm := range hexRE.FindAllSubmatch(block, -1) {
				text.WriteString(decodePDFHexText(string(hm[1]), cmap))
			}
			value := strings.TrimSpace(text.String())
			if value == "" {
				continue
			}
			tx, _ := strconv.ParseFloat(string(tm[5]), 64)
			ty, _ := strconv.ParseFloat(string(tm[6]), 64)
			x, y := tx, ty
			if cm := cmRE.FindSubmatch(block); len(cm) > 0 {
				a, _ := strconv.ParseFloat(string(cm[1]), 64)
				b, _ := strconv.ParseFloat(string(cm[2]), 64)
				c, _ := strconv.ParseFloat(string(cm[3]), 64)
				d, _ := strconv.ParseFloat(string(cm[4]), 64)
				e, _ := strconv.ParseFloat(string(cm[5]), 64)
				f, _ := strconv.ParseFloat(string(cm[6]), 64)
				x, y = a*tx+c*ty+e, b*tx+d*ty+f
			}
			fragments = append(fragments, pdfTextFragment{x: x, y: y, text: value})
		}
	}
	sort.SliceStable(fragments, func(i, j int) bool {
		if absFloat(fragments[i].y-fragments[j].y) > 4 {
			return fragments[i].y > fragments[j].y
		}
		return fragments[i].x < fragments[j].x
	})
	var lines []string
	for i := 0; i < len(fragments); {
		y := fragments[i].y
		var row []pdfTextFragment
		for i < len(fragments) && absFloat(fragments[i].y-y) <= 4 {
			row = append(row, fragments[i])
			i++
		}
		sort.SliceStable(row, func(i, j int) bool { return row[i].x < row[j].x })
		parts := make([]string, 0, len(row))
		for _, item := range row {
			parts = append(parts, item.text)
		}
		lines = append(lines, strings.Join(parts, "\t"))
	}
	return strings.Join(lines, "\n"), nil
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func parseDynamogramPDFReport(text string) map[string]string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	patterns := map[string]string{
		"well": `(?i)Скважина:\s*([^\s\t]+)`, "bush": `(?i)Куст:\s*([^\s\t]+)`,
		"shop": `(?i)Цех:\s*(.+?)(?:\t|Месторождение:)`, "field": `(?i)Месторождение:\s*(.+?)(?:\n|\tДата:)`,
		"date":         `(?i)Дата:\s*([0-9]{2}\.[0-9]{2}\.[0-9]{4}(?:\s+[0-9]{2}:[0-9]{2})?)`,
		"pumping_unit": `(?i)Станок:\s*([^\t\n]+)`, "pump_depth_m": `(?i)Глубина подвеса насоса\s*\(м\)\s*([0-9.,]+)`,
		"pump_type": `(?i)Тип насоса\s*([^\t\n]+)`, "plunger_diameter_mm": `(?i)Диаметр плунжера\s*\(мм\)\s*([0-9.,]+)`,
		"liquid_density_g_cm3": `(?i)Плотность жидкости\s*\(г/см3\)\s*([0-9.,]+)`, "watercut_pct": `(?i)Обводн[её]нность\s*\(%\)\s*([0-9.,]+)`,
		"water_density_g_cm3": `(?i)Плотность воды\s*\(г/см3\)\s*([0-9.,]+)`, "oil_density_g_cm3": `(?i)Плотность нефти\s*\(г/см3\)\s*([0-9.,]+)`,
		"dynamic_level_m": `(?i)Уровень динамический\s*\(м\)\s*([0-9.,]+)`, "annulus_pressure_at": `(?i)Затрубное давление\s*\(ат\)\s*([0-9.,]+)`,
		"min_dyn_load_kgf": `(?i)Мин\.\s*динам\.\s*нагрузка\s*\(кгс\)\s*([0-9.,]+)`, "max_dyn_load_kgf": `(?i)Макс\.\s*динам\.\s*нагрузка\s*\(кгс\)\s*([0-9.,]+)`,
		"strokes_per_min": `(?i)Число качаний в мин\.\s*([0-9.,]+)`, "rod_stroke_m": `(?i)Режимный ход штока\s*\(м\)\s*([0-9.,]+)`,
		"plunger_stroke_m": `(?i)Эфф\.\s*ход плунжера\s*\(м\)\s*([0-9.,]+)`, "calculated_rate_m3_day": `(?i)Расчетный дебит\s*\(м3/сут\)\s*([0-9.,]+)`,
		"fill_coefficient": `(?i)Коэф\.\s*наполнения насоса\s*([0-9.,]+)`, "delivery_coefficient": `(?i)Коэф\.\s*подачи насоса\s*([0-9.,]+)`,
		"sensor": `(?i)Датчик\s*([^\t\n]+)`, "recorder": `(?i)Блок регистрации\s*([^\t\n]+)`,
	}
	out := map[string]string{}
	for key, pattern := range patterns {
		if m := regexp.MustCompile(pattern).FindStringSubmatch(text); len(m) > 1 {
			out[key] = strings.TrimSpace(m[1])
		}
	}
	return out
}

func pdfReportNumber(data map[string]string, key string) (float64, bool) {
	value := strings.ReplaceAll(strings.TrimSpace(data[key]), ",", ".")
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil
}

func matchPDFReportObject(data map[string]string) string {
	well := strings.TrimSpace(data["well"])
	if well == "" {
		return ""
	}
	field := strings.ToLower(strings.TrimSpace(data["field"]))
	var matches []string
	for _, state := range techCurrent {
		if !strings.EqualFold(strings.TrimSpace(state.Well), well) {
			continue
		}
		if field != "" && state.Field != "" && !strings.Contains(field, strings.ToLower(strings.TrimSpace(state.Field))) {
			continue
		}
		matches = append(matches, state.ObjectKey)
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func applyPDFReportToAnalysis(a *DynamogramAnalysis, pts []DynamogramPoint, data map[string]string) {
	if a == nil || len(data) == 0 {
		return
	}
	if a.PDFPage < 1 {
		a.PDFPage = 1
	}
	a.ReportData = data
	if strings.TrimSpace(a.ObjectKey) == "" {
		a.ObjectKey = matchPDFReportObject(data)
	}
	xmax, xOK := pdfReportNumber(data, "rod_stroke_m")
	ymin, minOK := pdfReportNumber(data, "min_dyn_load_kgf")
	ymax, maxOK := pdfReportNumber(data, "max_dyn_load_kgf")
	if xOK && maxOK && xmax > 0 && ymax > ymin {
		if !minOK {
			ymin = 0
		}
		_ = applyDynCalibration(a, pts, 0, xmax, ymin, ymax, "м", "кгс")
	}
}

func formatDynamogramPDFReport(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	items := []struct{ k, label string }{{"well", "Скважина"}, {"date", "Дата отчёта"}, {"pumping_unit", "Станок"}, {"pump_type", "Тип насоса"}, {"pump_depth_m", "Глубина подвеса насоса, м"}, {"watercut_pct", "Обводнённость, %"}, {"dynamic_level_m", "Динамический уровень, м"}, {"annulus_pressure_at", "Затрубное давление (Рзат), ат"}, {"strokes_per_min", "Число качаний, мин⁻¹"}, {"rod_stroke_m", "Ход штока, м"}, {"calculated_rate_m3_day", "Расчётный дебит, м³/сут"}, {"fill_coefficient", "Коэффициент наполнения"}, {"sensor", "Датчик"}, {"recorder", "Блок регистрации"}}
	var parts []string
	for _, item := range items {
		if value := strings.TrimSpace(data[item.k]); value != "" {
			parts = append(parts, item.label+": "+value)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "ДАННЫЕ, СЧИТАННЫЕ ИЗ PDF (проверьте по исходному отчёту)\r\n" + strings.Join(parts, "; ") + ".\r\n"
}
