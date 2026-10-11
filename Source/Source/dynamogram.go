//go:build windows

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DynamogramPoint struct {
	XNorm     float64 `json:"x_norm"`
	UpperNorm float64 `json:"upper_norm"`
	LowerNorm float64 `json:"lower_norm"`
	X         float64 `json:"x,omitempty"`
	Upper     float64 `json:"upper,omitempty"`
	Lower     float64 `json:"lower,omitempty"`
}

type DynamogramAnalysis struct {
	ID                  string            `json:"id"`
	Created             string            `json:"created"`
	ObjectKey           string            `json:"object_key,omitempty"`
	SourceFile          string            `json:"source_file"`
	SourceCopy          string            `json:"source_copy,omitempty"`
	SourceSHA256        string            `json:"source_sha256"`
	PreviewPath         string            `json:"preview_path"`                  // контрольное изображение: исходник + распознанный контур
	ReconstructionPath  string            `json:"reconstruction_path,omitempty"` // чисто перерисованный контур
	CSVPath             string            `json:"csv_path"`
	ImageWidth          int               `json:"image_width"`
	ImageHeight         int               `json:"image_height"`
	WorkingWidth        int               `json:"working_width"`
	WorkingHeight       int               `json:"working_height"`
	DetectionMode       string            `json:"detection_mode"`
	Confidence          float64           `json:"confidence"`
	Quality             string            `json:"quality"`
	PointCount          int               `json:"point_count"`
	BBox                [4]int            `json:"bbox"`
	AreaRatio           float64           `json:"area_ratio"`
	MeanSeparation      float64           `json:"mean_separation"`
	Symmetry            float64           `json:"symmetry"`
	ClosureQuality      float64           `json:"closure_quality"`
	Roughness           float64           `json:"roughness"`
	MeanSourceDeviation float64           `json:"mean_source_deviation,omitempty"`
	MaxSourceDeviation  float64           `json:"max_source_deviation,omitempty"`
	MissingSourceRatio  float64           `json:"missing_source_ratio,omitempty"`
	MaxBranchJump       float64           `json:"max_branch_jump,omitempty"`
	FlatRunRatio        float64           `json:"flat_run_ratio,omitempty"`
	FrameCaptureRatio   float64           `json:"frame_capture_ratio,omitempty"`
	QualityGate         string            `json:"quality_gate,omitempty"` // pass / warning / blocked
	QualityIssues       []string          `json:"quality_issues,omitempty"`
	LeftSeparation      float64           `json:"left_separation,omitempty"`
	MiddleSeparation    float64           `json:"middle_separation,omitempty"`
	RightSeparation     float64           `json:"right_separation,omitempty"`
	FillUniformity      float64           `json:"fill_uniformity,omitempty"`
	UpperTrend          float64           `json:"upper_trend,omitempty"`
	LowerTrend          float64           `json:"lower_trend,omitempty"`
	Calibrated          bool              `json:"calibrated"`
	XMin                float64           `json:"x_min,omitempty"`
	XMax                float64           `json:"x_max,omitempty"`
	YMin                float64           `json:"y_min,omitempty"`
	YMax                float64           `json:"y_max,omitempty"`
	XUnit               string            `json:"x_unit,omitempty"`
	YUnit               string            `json:"y_unit,omitempty"`
	Findings            []string          `json:"findings"`
	MethodNote          string            `json:"method_note"`
	ReviewStatus        string            `json:"review_status,omitempty"` // not_reviewed / confirmed / rejected / digital
	ReviewNote          string            `json:"review_note,omitempty"`
	PDFPage             int               `json:"pdf_page,omitempty"`
	ReportData          map[string]string `json:"report_data,omitempty"`
}

type component struct {
	pixels                 []int
	minX, minY, maxX, maxY int
}

const (
	dynSourceInkThreshold = 185
	dynQualityGatePass    = "pass"
	dynQualityGateWarning = "warning"
	dynQualityGateBlocked = "blocked"
)

func nearestSourceInkDeviation(img *image.RGBA, x, y, maxDistance int) (int, bool) {
	best := maxDistance + 1
	for dx := -2; dx <= 2; dx++ {
		xx := x + dx
		if xx < 0 || xx >= img.Bounds().Dx() {
			continue
		}
		for dy := -maxDistance; dy <= maxDistance; dy++ {
			yy := y + dy
			if yy < 0 || yy >= img.Bounds().Dy() || pixelBrightness(img, xx, yy) >= dynSourceInkThreshold {
				continue
			}
			d := absInt(dy)
			if d < best {
				best = d
			}
		}
	}
	return best, best <= maxDistance
}

func longestFlatBranchRun(pts []DynamogramPoint) float64 {
	if len(pts) < 5 {
		return 0
	}
	start := maxInt(1, len(pts)/12)
	end := minInt(len(pts), len(pts)-len(pts)/12)
	longest, upperRun, lowerRun := 0, 0, 0
	for i := start; i < end; i++ {
		if math.Abs(pts[i].UpperNorm-pts[i-1].UpperNorm) < 0.0015 {
			upperRun++
		} else {
			upperRun = 0
		}
		if math.Abs(pts[i].LowerNorm-pts[i-1].LowerNorm) < 0.0015 {
			lowerRun++
		} else {
			lowerRun = 0
		}
		longest = maxInt(longest, maxInt(upperRun, lowerRun))
	}
	return float64(longest) / math.Max(1, float64(end-start))
}

func contourSourceQuality(img *image.RGBA, c component, pts []DynamogramPoint) (meanDeviation, maxDeviation, missingRatio, maxJump, flatRunRatio, frameCaptureRatio float64) {
	if img == nil || len(pts) < 3 || c.maxX <= c.minX || c.maxY <= c.minY {
		return
	}
	bw, bh := c.maxX-c.minX+1, c.maxY-c.minY+1
	maxDistance := maxInt(5, bh/9)
	start := maxInt(1, len(pts)/12)
	end := minInt(len(pts)-1, len(pts)-len(pts)/12)
	measured, missing, frameHits := 0, 0, 0
	totalDeviation := 0.0
	for i := start; i < end; i++ {
		p := pts[i]
		x := c.minX + int(math.Round(p.XNorm*float64(maxInt(1, bw-1))))
		for _, yn := range []float64{p.UpperNorm, p.LowerNorm} {
			y := c.minY + int(math.Round((1-yn)*float64(maxInt(1, bh-1))))
			if d, ok := nearestSourceInkDeviation(img, x, y, maxDistance); ok {
				normalized := float64(d) / math.Max(1, float64(bh-1))
				totalDeviation += normalized
				maxDeviation = math.Max(maxDeviation, normalized)
				measured++
			} else {
				missing++
			}
		}
		if p.UpperNorm > 0.985 || p.LowerNorm < 0.015 {
			frameHits++
		}
		maxJump = math.Max(maxJump, math.Abs(pts[i].UpperNorm-pts[i-1].UpperNorm))
		maxJump = math.Max(maxJump, math.Abs(pts[i].LowerNorm-pts[i-1].LowerNorm))
	}
	if measured > 0 {
		meanDeviation = totalDeviation / float64(measured)
	}
	missingRatio = float64(missing) / math.Max(1, float64(measured+missing))
	frameCaptureRatio = float64(frameHits) / math.Max(1, float64(end-start))
	flatRunRatio = longestFlatBranchRun(pts)
	return
}

func dynamogramQualityGate(a *DynamogramAnalysis) (string, []string) {
	if a == nil || strings.Contains(strings.ToLower(a.DetectionMode), "цифров") {
		return dynQualityGatePass, nil
	}
	var blocked, warnings []string
	if a.Confidence < 0.45 {
		blocked = append(blocked, fmt.Sprintf("уверенность %.0f %% ниже минимально допустимой 45 %%", a.Confidence*100))
	}
	if a.MissingSourceRatio > 0.28 {
		blocked = append(blocked, fmt.Sprintf("не найден видимый штрих рядом с %.0f %% точек", a.MissingSourceRatio*100))
	} else if a.MissingSourceRatio > 0.15 {
		warnings = append(warnings, fmt.Sprintf("доля точек без близкого видимого штриха %.0f %%", a.MissingSourceRatio*100))
	}
	if a.MeanSourceDeviation > 0.060 || a.MaxSourceDeviation > 0.180 {
		blocked = append(blocked, fmt.Sprintf("отклонение от видимого штриха слишком велико: среднее %.1f %%, максимум %.1f %% высоты контура", a.MeanSourceDeviation*100, a.MaxSourceDeviation*100))
	} else if a.MeanSourceDeviation > 0.035 || a.MaxSourceDeviation > 0.120 {
		warnings = append(warnings, fmt.Sprintf("заметное отклонение от видимого штриха: среднее %.1f %%, максимум %.1f %%", a.MeanSourceDeviation*100, a.MaxSourceDeviation*100))
	}
	if a.MaxBranchJump > 0.180 {
		blocked = append(blocked, fmt.Sprintf("резкий скачок ветви %.1f %% высоты контура", a.MaxBranchJump*100))
	} else if a.MaxBranchJump > 0.100 {
		warnings = append(warnings, fmt.Sprintf("скачок ветви %.1f %%", a.MaxBranchJump*100))
	}
	if a.FlatRunRatio > 0.42 {
		blocked = append(blocked, fmt.Sprintf("длинная искусственная горизонтальная полка занимает %.0f %% хода", a.FlatRunRatio*100))
	} else if a.FlatRunRatio > 0.28 {
		warnings = append(warnings, fmt.Sprintf("протяжённый почти горизонтальный участок %.0f %% хода", a.FlatRunRatio*100))
	}
	if a.FrameCaptureRatio > 0.30 {
		blocked = append(blocked, fmt.Sprintf("вероятный захват рамки/границы у %.0f %% точек", a.FrameCaptureRatio*100))
	} else if a.FrameCaptureRatio > 0.18 {
		warnings = append(warnings, fmt.Sprintf("возможное приближение к рамке у %.0f %% точек", a.FrameCaptureRatio*100))
	}
	if len(blocked) > 0 {
		return dynQualityGateBlocked, append(blocked, warnings...)
	}
	if len(warnings) > 0 {
		return dynQualityGateWarning, warnings
	}
	return dynQualityGatePass, nil
}

func dynRoot() string {
	p := filepath.Join(dataDir, "dynamograms")
	_ = os.MkdirAll(p, 0755)
	_ = os.MkdirAll(filepath.Join(p, "source"), 0755)
	_ = os.MkdirAll(filepath.Join(p, "preview"), 0755)
	_ = os.MkdirAll(filepath.Join(p, "curve"), 0755)
	return p
}

func loadDynamogramHistory() []DynamogramAnalysis {
	var x []DynamogramAnalysis
	loadJSON(filepath.Join(dynRoot(), "analyses.json"), &x)
	sort.SliceStable(x, func(i, j int) bool { return parseDate(x[i].Created).After(parseDate(x[j].Created)) })
	return x
}

func saveDynamogramHistory(x []DynamogramAnalysis) error {
	return saveJSON(filepath.Join(dynRoot(), "analyses.json"), x)
}

func decodeDynamogramImage(path string) (image.Image, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	const maxSourceBytes int64 = 50 * 1024 * 1024
	if info.Size() > maxSourceBytes {
		return nil, "", fmt.Errorf("файл слишком большой: %.1f МБ; допустимо не более 50 МБ", float64(info.Size())/(1024*1024))
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".bmp" {
		img, err := decodeBMP24(path)
		return img, "BMP", err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, "", err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 12000 || cfg.Height > 12000 || int64(cfg.Width)*int64(cfg.Height) > 60_000_000 {
		return nil, "", fmt.Errorf("слишком большое разрешение изображения: %d×%d; уменьшите изображение перед анализом", cfg.Width, cfg.Height)
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, "", err
	}
	img, decodedFormat, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}
	if decodedFormat != "" {
		format = decodedFormat
	}
	return img, strings.ToUpper(format), nil
}

// decodeBMP24 handles ordinary uncompressed 24/32-bit Windows BMP files.
func decodeBMP24(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	hdr := make([]byte, 54)
	if _, err = r.Read(hdr); err != nil {
		return nil, err
	}
	if string(hdr[0:2]) != "BM" {
		return nil, errors.New("файл не является BMP")
	}
	off := binary.LittleEndian.Uint32(hdr[10:14])
	dib := binary.LittleEndian.Uint32(hdr[14:18])
	if dib < 40 {
		return nil, errors.New("неподдерживаемый старый формат BMP")
	}
	w := int(int32(binary.LittleEndian.Uint32(hdr[18:22])))
	hRaw := int32(binary.LittleEndian.Uint32(hdr[22:26]))
	topDown := hRaw < 0
	h := int(hRaw)
	if h < 0 {
		h = -h
	}
	bpp := int(binary.LittleEndian.Uint16(hdr[28:30]))
	compression := binary.LittleEndian.Uint32(hdr[30:34])
	if w <= 0 || h <= 0 || (bpp != 24 && bpp != 32) || compression != 0 {
		return nil, errors.New("поддерживаются только несжатые BMP 24/32 бита")
	}
	if w > 12000 || h > 12000 || int64(w)*int64(h) > 60_000_000 {
		return nil, fmt.Errorf("слишком большое разрешение BMP: %d×%d; уменьшите изображение перед анализом", w, h)
	}
	if _, err = f.Seek(int64(off), 0); err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rowBytes := ((w*bpp + 31) / 32) * 4
	row := make([]byte, rowBytes)
	for y0 := 0; y0 < h; y0++ {
		if _, err = f.Read(row); err != nil {
			return nil, err
		}
		y := h - 1 - y0
		if topDown {
			y = y0
		}
		for x := 0; x < w; x++ {
			i := x * bpp / 8
			b, g, rr := row[i], row[i+1], row[i+2]
			a := byte(255)
			if bpp == 32 {
				a = row[i+3]
				if a == 0 {
					a = 255
				}
			}
			img.SetRGBA(x, y, color.RGBA{R: rr, G: g, B: b, A: a})
		}
	}
	return img, nil
}

func resizeNearest(src image.Image, maxW, maxH int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	scale := math.Min(1, math.Min(float64(maxW)/float64(sw), float64(maxH)/float64(sh)))
	dw, dh := int(math.Max(1, math.Round(float64(sw)*scale))), int(math.Max(1, math.Round(float64(sh)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy := b.Min.Y + int(float64(y)*float64(sh)/float64(dh))
		if sy >= b.Max.Y {
			sy = b.Max.Y - 1
		}
		for x := 0; x < dw; x++ {
			sx := b.Min.X + int(float64(x)*float64(sw)/float64(dw))
			if sx >= b.Max.X {
				sx = b.Max.X - 1
			}
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func pixRGB(c color.Color) (r, g, b uint8) {
	rr, gg, bb, _ := c.RGBA()
	return uint8(rr >> 8), uint8(gg >> 8), uint8(bb >> 8)
}
func max3(a, b, c uint8) uint8 {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}
func min3(a, b, c uint8) uint8 {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}

func buildMask(img *image.RGBA) ([]bool, string, int) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	colorMask := make([]bool, w*h)
	darkMask := make([]bool, w*h)
	colorCount, darkCount := 0, 0
	mx, my := maxInt(4, w/40), maxInt(4, h/40)
	for y := my; y < h-my; y++ {
		for x := mx; x < w-mx; x++ {
			r, g, b := pixRGB(img.At(x, y))
			ma, mi := max3(r, g, b), min3(r, g, b)
			sat := int(ma) - int(mi)
			bright := (int(r) + int(g) + int(b)) / 3
			if sat >= 48 && ma < 250 && mi < 235 {
				colorMask[y*w+x] = true
				colorCount++
			}
			if bright < 108 {
				darkMask[y*w+x] = true
				darkCount++
			}
		}
	}
	// Prefer colored curves if they occupy a non-trivial but not absurd part of the graph.
	minColor := maxInt(60, (w*h)/5000)
	if colorCount >= minColor && colorCount < (w*h)/5 {
		suppressLongLines(colorMask, w, h)
		return dilateMask(colorMask, w, h, 1), "цветная кривая", colorCount
	}
	suppressLongLines(darkMask, w, h)
	return dilateMask(darkMask, w, h, 1), "тёмная кривая", darkCount
}

func suppressLongLines(mask []bool, w, h int) {
	row := make([]int, h)
	col := make([]int, w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask[y*w+x] {
				row[y]++
				col[x]++
			}
		}
	}
	for y, c := range row {
		if c > int(float64(w)*0.48) {
			for yy := maxInt(0, y-1); yy <= minInt(h-1, y+1); yy++ {
				for x := 0; x < w; x++ {
					mask[yy*w+x] = false
				}
			}
		}
	}
	for x, c := range col {
		if c > int(float64(h)*0.48) {
			for xx := maxInt(0, x-1); xx <= minInt(w-1, x+1); xx++ {
				for y := 0; y < h; y++ {
					mask[y*w+xx] = false
				}
			}
		}
	}
}
func dilateMask(src []bool, w, h, r int) []bool {
	dst := make([]bool, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !src[y*w+x] {
				continue
			}
			for dy := -r; dy <= r; dy++ {
				yy := y + dy
				if yy < 0 || yy >= h {
					continue
				}
				for dx := -r; dx <= r; dx++ {
					xx := x + dx
					if xx < 0 || xx >= w {
						continue
					}
					dst[yy*w+xx] = true
				}
			}
		}
	}
	return dst
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

type plotLineCandidate struct {
	x, start, end, length int
}

func pixelBrightness(img *image.RGBA, x, y int) int {
	r, g, b := pixRGB(img.At(x, y))
	return (int(r) + int(g) + int(b)) / 3
}

func longestVerticalInkRun(img *image.RGBA, x, fromY, toY int) plotLineCandidate {
	best := plotLineCandidate{x: x}
	start, lastDark, gaps := -1, -1, 0
	finish := func() {
		if start >= 0 && lastDark-start+1 > best.length {
			best.start, best.end, best.length = start, lastDark, lastDark-start+1
		}
	}
	for y := fromY; y < toY; y++ {
		if pixelBrightness(img, x, y) < 215 {
			if start < 0 {
				start = y
			}
			lastDark, gaps = y, 0
			continue
		}
		if start >= 0 {
			gaps++
			if gaps > 2 {
				finish()
				start, lastDark, gaps = -1, -1, 0
			}
		}
	}
	finish()
	return best
}

func longestHorizontalInkRun(img *image.RGBA, y, fromX, toX int) int {
	best, current, gaps := 0, 0, 0
	for x := fromX; x < toX; x++ {
		if pixelBrightness(img, x, y) < 215 {
			current++
			gaps = 0
			continue
		}
		if current > 0 && gaps < 2 {
			current++
			gaps++
			continue
		}
		best = maxInt(best, current-gaps)
		current, gaps = 0, 0
	}
	return maxInt(best, current-gaps)
}

// detectDynamogramPlot isolates the actual plot from scanned report headers and
// parameter tables. It looks for the two tall sides of a wide graph frame in
// the lower part of the page; a random table row cannot satisfy these gates.
func detectDynamogramPlot(img *image.RGBA) (*image.RGBA, bool) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w < 120 || h < 120 {
		return img, false
	}
	fromY, toY := h*38/100, h*94/100
	minRun := maxInt(28, h*16/100)
	var lines []plotLineCandidate
	for x := 2; x < w-2; x++ {
		line := longestVerticalInkRun(img, x, fromY, toY)
		if line.length >= minRun {
			lines = append(lines, line)
		}
	}
	bestScore := -1
	var left, right plotLineCandidate
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			width := lines[j].x - lines[i].x
			if width < w*52/100 {
				continue
			}
			overlap := minInt(lines[i].end, lines[j].end) - maxInt(lines[i].start, lines[j].start) + 1
			if overlap < h*10/100 {
				continue
			}
			score := width*4 + minInt(lines[i].length, lines[j].length)*3 + overlap
			if score > bestScore {
				bestScore, left, right = score, lines[i], lines[j]
			}
		}
	}
	if bestScore < 0 {
		return img, false
	}
	x0, x1 := left.x+4, right.x-3
	y0 := minInt(left.start, right.start) + 5
	y1 := maxInt(left.end, right.end) - 4
	// Locate the actual bottom frame only in the bottom 18% of the candidate.
	// The former 62% search started among horizontal grid lines; on QUANTOR cards
	// it selected the 700-unit grid line and cut away the real lower branch.
	searchBottomFrom := y0 + (y1-y0)*82/100
	lastClusterStart := -1
	inCluster := false
	for y := searchBottomFrom; y <= y1; y++ {
		// A plot frame is nearly continuous. Dotted/grey grid lines and the curve
		// itself cannot satisfy this stronger coverage gate.
		strong := longestHorizontalInkRun(img, y, left.x, right.x+1) >= (right.x-left.x)*72/100
		if strong && !inCluster {
			lastClusterStart = y
			inCluster = true
		} else if !strong {
			inCluster = false
		}
	}
	if lastClusterStart > 0 {
		y1 = lastClusterStart - 3
	}
	if x1-x0 < w/2 || y1-y0 < h/7 || y1 <= h/2 {
		return img, false
	}
	out := image.NewRGBA(image.Rect(0, 0, x1-x0+1, y1-y0+1))
	draw.Draw(out, out.Bounds(), img, image.Pt(x0, y0), draw.Src)
	return out, true
}

func interpolateSeries(values []int) (int, bool) {
	first, last, valid := -1, -1, 0
	for i, v := range values {
		if v < 0 {
			continue
		}
		valid++
		if first < 0 {
			first = i
		}
		if last >= 0 && i-last > 1 {
			for k := last + 1; k < i; k++ {
				t := float64(k-last) / float64(i-last)
				values[k] = int(math.Round(float64(values[last])*(1-t) + float64(v)*t))
			}
		}
		last = i
	}
	if first < 0 {
		return 0, false
	}
	for i := 0; i < first; i++ {
		values[i] = values[first]
	}
	for i := last + 1; i < len(values); i++ {
		values[i] = values[last]
	}
	return valid, true
}

func medianSmooth(values []int, radius int) []int {
	out := append([]int(nil), values...)
	for i := range values {
		var window []int
		for j := maxInt(0, i-radius); j <= minInt(len(values)-1, i+radius); j++ {
			window = append(window, values[j])
		}
		sort.Ints(window)
		out[i] = window[len(window)/2]
	}
	return out
}

// suppressInteriorBranchExcursions removes narrow excursions caused by table
// borders, scale ticks and broken grid lines. A real dynamogram branch changes
// over a substantial part of the stroke, while these artefacts jump away from
// the local robust trend and return to it after a short interval.
func suppressInteriorBranchExcursions(values []int, plotH int) []int {
	if len(values) < 24 {
		return append([]int(nil), values...)
	}
	trend := medianSmooth(values, maxInt(7, len(values)/18))
	out := append([]int(nil), values...)
	threshold := maxInt(7, plotH/13)
	edgeGuard := maxInt(3, len(values)/12)
	maxArtifactRun := maxInt(5, len(values)/9)
	// Replace only a short excursion that leaves the robust trend and returns
	// to it. The previous point-by-point gate also erased broad real bends of a
	// dynamogram, especially along the lower working branch.
	for i := edgeGuard; i < len(values)-edgeGuard; {
		delta := absInt(values[i] - trend[i])
		if delta <= threshold {
			i++
			continue
		}
		start := i
		for i < len(values)-edgeGuard && absInt(values[i]-trend[i]) > threshold {
			i++
		}
		end := i
		if end-start <= maxArtifactRun {
			left := maxInt(0, start-1)
			right := minInt(len(values)-1, end)
			for x := start; x < end; x++ {
				t := float64(x-left) / math.Max(1, float64(right-left))
				out[x] = int(math.Round(float64(out[left])*(1-t) + float64(out[right])*t))
			}
		}
	}
	return medianSmooth(out, maxInt(1, len(values)/220))
}

func inkClusterCenters(mask []bool, w, h, x int) []int {
	var centers []int
	for y := 1; y < h-h/20; {
		if !mask[y*w+x] {
			y++
			continue
		}
		start := y
		for y < h-h/20 && mask[y*w+x] {
			y++
		}
		centers = append(centers, (start+y-1)/2)
	}
	return centers
}

func traceContinuousBranch(mask []bool, w, h, anchorX, anchorY, direction int) []int {
	out := make([]int, w)
	for i := range out {
		out[i] = -1
	}
	out[anchorX] = anchorY
	previous := anchorY
	maxJump := maxInt(10, h/9)
	for x := anchorX + direction; x >= 0 && x < w; x += direction {
		candidates := inkClusterCenters(mask, w, h, x)
		best, bestDistance := -1, h+1
		for _, y := range candidates {
			distance := absInt(y - previous)
			if distance < bestDistance {
				best, bestDistance = y, distance
			}
		}
		if best >= 0 && bestDistance <= maxJump {
			out[x], previous = best, best
		}
	}
	return out
}

// digitizePlotBranches follows upper and lower branches independently. This is
// deliberately different from generic connected-component selection: grid
// intersections often split a scanned dynamogram into several components.
func digitizePlotBranches(img *image.RGBA) (component, []DynamogramPoint, float64, float64, float64, float64, int, error) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	raw := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			raw[y*w+x] = pixelBrightness(img, x, y) < 170
		}
	}
	clean := append([]bool(nil), raw...)
	rows, cols := make([]int, h), make([]int, w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if clean[y*w+x] {
				rows[y]++
				cols[x]++
			}
		}
	}
	// Remove grid/axis strokes before reading either branch. Keeping the raw
	// lower mask allowed a horizontal grid line to become a false rectangle.
	for y, count := range rows {
		if count > w*14/100 {
			for yy := maxInt(0, y-1); yy <= minInt(h-1, y+1); yy++ {
				for x := 0; x < w; x++ {
					clean[yy*w+x] = false
				}
			}
		}
	}
	for x, count := range cols {
		if count > h*18/100 {
			for xx := maxInt(0, x-1); xx <= minInt(w-1, x+1); xx++ {
				for y := 0; y < h; y++ {
					clean[y*w+xx] = false
					raw[y*w+xx] = false
				}
			}
		}
	}
	// Start in the middle, where both branches are normally separated, then
	// follow the nearest continuous ink cluster in both directions. This avoids
	// treating a plot frame, a vertical connector or a scale tick as an entire
	// branch simply because it is the topmost/bottommost pixel in a column.
	anchorX, upperAnchor, lowerAnchor := -1, -1, -1
	for radius := 0; radius < w/3 && anchorX < 0; radius++ {
		for _, x := range []int{w/2 - radius, w/2 + radius} {
			if x < 0 || x >= w {
				continue
			}
			candidates := inkClusterCenters(clean, w, h, x)
			if len(candidates) >= 2 && candidates[len(candidates)-1]-candidates[0] >= h/10 {
				anchorX, upperAnchor, lowerAnchor = x, candidates[0], candidates[len(candidates)-1]
				break
			}
		}
	}
	if anchorX < 0 {
		return component{}, nil, 0, 0, 0, 0, 0, errors.New("не найдена область устойчивого разделения ветвей")
	}
	upperLeft := traceContinuousBranch(clean, w, h, anchorX, upperAnchor, -1)
	upperRight := traceContinuousBranch(clean, w, h, anchorX, upperAnchor, 1)
	lowerLeft := traceContinuousBranch(clean, w, h, anchorX, lowerAnchor, -1)
	lowerRight := traceContinuousBranch(clean, w, h, anchorX, lowerAnchor, 1)
	upper, lower := make([]int, w), make([]int, w)
	for x := 0; x < w; x++ {
		upper[x], lower[x] = upperLeft[x], lowerLeft[x]
		if x >= anchorX {
			upper[x], lower[x] = upperRight[x], lowerRight[x]
		}
	}
	upperValid, okUpper := interpolateSeries(upper)
	lowerValid, okLower := interpolateSeries(lower)
	if !okUpper || !okLower || upperValid < w*35/100 || lowerValid < w*45/100 {
		return component{}, nil, 0, 0, 0, 0, 0, errors.New("ветви графика не удалось устойчиво отделить от сетки")
	}
	// Preserve measured local geometry. The earlier 1/28 and 1/18 windows
	// flattened genuine bends and reduced a scanned curve to a coarse polygon.
	smoothRadius := maxInt(3, w/60)
	upper = medianSmooth(upper, smoothRadius)
	// The lower half of scanned cards contains axis ticks and numeric labels much
	// more often than the upper half. A wider robust window removes those short
	// rectangular fragments while preserving the broad working branch.
	lower = medianSmooth(lower, maxInt(smoothRadius, w/48))
	// A second, broader trend gate removes the characteristic inner rectangle
	// visible on scanned QUANTOR cards without flattening the outer working loop.
	upper = suppressInteriorBranchExcursions(upper, h)
	lower = suppressInteriorBranchExcursions(lower, h)
	// The two branches must converge at both ends of a closed working cycle.
	// Reconstruct only short edge zones; the measured middle remains untouched.
	leftZone := minInt(w/9, w-1)
	if leftZone > 1 {
		closureY := lower[leftZone]
		for x := 0; x < leftZone; x++ {
			t := float64(x) / float64(leftZone)
			upper[x] = int(math.Round(float64(closureY)*(1-t) + float64(upper[leftZone])*t))
			lower[x] = int(math.Round(float64(closureY)*(1-t) + float64(lower[leftZone])*t))
		}
		// On ordinary dynamograms the upper branch leaves the left closure
		// towards its first load maximum. A short horizontal grid fragment must
		// not replace this sloping branch, so reconstruct it between two measured
		// anchors.
		peak := leftZone
		for x := leftZone; x < minInt(w/3, w); x++ {
			if upper[x] < upper[peak] {
				peak = x
			}
		}
		if peak > leftZone {
			for x := 0; x < peak; x++ {
				t := float64(x) / float64(peak)
				upper[x] = int(math.Round(float64(closureY)*(1-t) + float64(upper[peak])*t))
			}
		}
	}
	rightZone := minInt(w/12, w-1)
	if rightZone > 1 {
		start := w - 1 - rightZone
		// At the right edge the descending upper trace remains observable longer
		// than the rising lower trace. Close on that measured endpoint instead of
		// a residual baseline pixel from the lower half of the plot.
		closureY := upper[w-1]
		for x := start + 1; x < w; x++ {
			t := float64(x-start) / float64(rightZone)
			upper[x] = int(math.Round(float64(upper[start])*(1-t) + float64(closureY)*t))
			lower[x] = int(math.Round(float64(lower[start])*(1-t) + float64(closureY)*t))
		}
	}
	c := component{minX: 0, minY: h, maxX: w - 1, maxY: 0}
	for x := 0; x < w; x++ {
		if lower[x] < upper[x] {
			upper[x], lower[x] = lower[x], upper[x]
		}
		if lower[x]-upper[x] < 2 {
			continue
		}
		c.pixels = append(c.pixels, upper[x]*w+x, lower[x]*w+x)
		c.minY = minInt(c.minY, upper[x])
		c.maxY = maxInt(c.maxY, lower[x])
	}
	if len(c.pixels) < w {
		return component{}, nil, 0, 0, 0, 0, 0, errors.New("распознанная петля не имеет устойчивого раскрытия")
	}
	pts, area, sep, sym, closure, _ := digitizeComponent(c, w, h)
	return c, pts, area, sep, sym, closure, minInt(upperValid, lowerValid), nil
}

func components(mask []bool, w, h int) []component {
	seen := make([]bool, len(mask))
	var out []component
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	for idx, on := range mask {
		if !on || seen[idx] {
			continue
		}
		q := []int{idx}
		seen[idx] = true
		c := component{minX: w, minY: h, maxX: 0, maxY: 0}
		for len(q) > 0 {
			n := q[len(q)-1]
			q = q[:len(q)-1]
			x, y := n%w, n/w
			c.pixels = append(c.pixels, n)
			if x < c.minX {
				c.minX = x
			}
			if x > c.maxX {
				c.maxX = x
			}
			if y < c.minY {
				c.minY = y
			}
			if y > c.maxY {
				c.maxY = y
			}
			for _, d := range dirs {
				xx, yy := x+d[0], y+d[1]
				if xx < 0 || xx >= w || yy < 0 || yy >= h {
					continue
				}
				ni := yy*w + xx
				if mask[ni] && !seen[ni] {
					seen[ni] = true
					q = append(q, ni)
				}
			}
		}
		if len(c.pixels) >= 20 {
			out = append(out, c)
		}
	}
	return out
}

func bestCurveComponent(cs []component, w, h int) (component, error) {
	if len(cs) == 0 {
		return component{}, errors.New("не удалось выделить связанную кривую")
	}
	bestScore := -1.0
	var best component
	for _, c := range cs {
		bw, bh := c.maxX-c.minX+1, c.maxY-c.minY+1
		if bw < maxInt(18, w/18) || bh < maxInt(12, h/25) {
			continue
		}
		area := float64(bw * bh)
		density := float64(len(c.pixels)) / math.Max(area, 1)
		span := math.Sqrt(float64(bw * bh))
		score := float64(len(c.pixels))*0.35 + span*12 + math.Min(density, 0.35)*1000
		// Penalize components hugging the full frame; these are usually borders/axes.
		if bw > int(float64(w)*0.94) || bh > int(float64(h)*0.94) {
			score *= 0.35
		}
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	if bestScore < 0 {
		return component{}, errors.New("кривая слишком мала или неотделима от подписей/сетки")
	}
	return best, nil
}

func digitizeComponent(c component, w, h int) ([]DynamogramPoint, float64, float64, float64, float64, int) {
	bw := c.maxX - c.minX + 1
	bh := c.maxY - c.minY + 1
	upper := make([]int, bw)
	lower := make([]int, bw)
	count := make([]int, bw)
	for i := range upper {
		upper[i] = h + 1
		lower[i] = -1
	}
	for _, idx := range c.pixels {
		x, y := idx%w, idx/w
		j := x - c.minX
		if y < upper[j] {
			upper[j] = y
		}
		if y > lower[j] {
			lower[j] = y
		}
		count[j]++
	}
	valid := 0
	for i := 0; i < bw; i++ {
		if count[i] > 0 {
			valid++
		}
	}
	// interpolate missing columns from nearest valid neighbours
	last := -1
	for i := 0; i < bw; i++ {
		if count[i] > 0 {
			if last >= 0 && i-last > 1 {
				for k := last + 1; k < i; k++ {
					t := float64(k-last) / float64(i-last)
					upper[k] = int(math.Round(float64(upper[last])*(1-t) + float64(upper[i])*t))
					lower[k] = int(math.Round(float64(lower[last])*(1-t) + float64(lower[i])*t))
					count[k] = 1
				}
			}
			last = i
		}
	}
	first := -1
	last = -1
	for i := 0; i < bw; i++ {
		if count[i] > 0 {
			first = i
			break
		}
	}
	for i := bw - 1; i >= 0; i-- {
		if count[i] > 0 {
			last = i
			break
		}
	}
	if first < 0 {
		return nil, 0, 0, 0, 0, 0
	}
	for i := 0; i < first; i++ {
		upper[i] = upper[first]
		lower[i] = lower[first]
	}
	for i := last + 1; i < bw; i++ {
		upper[i] = upper[last]
		lower[i] = lower[last]
	}
	// Keep up to one sample per source column for ordinary scans. A 240-point
	// ceiling visibly quantised wide QUANTOR plots and hid smaller inflections.
	target := minInt(480, bw)
	if target < 40 {
		target = bw
	}
	pts := make([]DynamogramPoint, 0, target)
	for k := 0; k < target; k++ {
		i := 0
		if target > 1 {
			i = int(math.Round(float64(k) * float64(bw-1) / float64(target-1)))
		}
		xn := 0.0
		if bw > 1 {
			xn = float64(i) / float64(bw-1)
		}
		un := 1 - float64(upper[i]-c.minY)/math.Max(1, float64(bh-1))
		ln := 1 - float64(lower[i]-c.minY)/math.Max(1, float64(bh-1))
		if un < ln {
			un, ln = ln, un
		}
		pts = append(pts, DynamogramPoint{XNorm: clamp01(xn), UpperNorm: clamp01(un), LowerNorm: clamp01(ln)})
	}
	sep := make([]float64, len(pts))
	area := 0.0
	for i, p := range pts {
		sep[i] = math.Max(0, p.UpperNorm-p.LowerNorm)
		area += sep[i]
	}
	area /= float64(maxInt(1, len(pts)))
	meanSep := area
	symErr := 0.0
	for i := range sep {
		symErr += math.Abs(sep[i] - sep[len(sep)-1-i])
	}
	symErr /= float64(maxInt(1, len(sep)))
	symmetry := 1.0
	if meanSep > 1e-6 {
		symmetry = clamp01(1 - symErr/(2*meanSep))
	}
	edgeSep := (sep[0] + sep[len(sep)-1]) / 2
	closure := 1.0
	if meanSep > 1e-6 {
		closure = clamp01(1 - edgeSep/(meanSep*1.5))
	}
	rough := 0.0
	for i := 1; i < len(sep)-1; i++ {
		rough += math.Abs(sep[i+1] - 2*sep[i] + sep[i-1])
	}
	if len(sep) > 2 {
		rough /= float64(len(sep) - 2)
	}
	return pts, area, meanSep, symmetry, closure, valid
}

func analysisFindings(a *DynamogramAnalysis) []string {
	var f []string
	if a.QualityGate == dynQualityGateBlocked {
		f = append(f, "Автоматический контроль качества обнаружил грубый дефект восстановления. Подтверждение контура заблокировано до повторного анализа или ручной коррекции.")
	} else if a.QualityGate == dynQualityGateWarning {
		f = append(f, "Автоматический контроль качества выявил отклонения, требующие особенно внимательной визуальной сверки.")
	}
	for _, issue := range a.QualityIssues {
		f = append(f, "Контроль качества: "+issue+".")
	}
	if a.Confidence < 0.60 {
		f = append(f, "Распознавание требует визуальной проверки: исходное изображение или контур недостаточно уверенно отделены от фона.")
	}
	if a.ClosureQuality < 0.55 {
		f = append(f, "Контур выглядит неполностью замкнутым либо крайние участки кривой распознаны неустойчиво.")
	} else {
		f = append(f, "Контур по крайним участкам распознан как преимущественно замкнутый.")
	}
	if a.Symmetry < 0.62 {
		f = append(f, "Выражена асимметрия формы между левой и правой частями контура.")
	} else {
		f = append(f, "Грубая геометрическая асимметрия формы не выражена.")
	}
	if a.AreaRatio < 0.22 {
		f = append(f, "Контур относительно узкий: площадь петли мала относительно ограничивающего прямоугольника.")
	} else if a.AreaRatio > 0.62 {
		f = append(f, "Контур занимает большую долю ограничивающего прямоугольника.")
	} else {
		f = append(f, "Площадь контура находится в среднем геометрическом диапазоне.")
	}
	if a.Roughness > 0.055 {
		f = append(f, "На ветвях присутствуют заметные локальные неровности/изломы; желательно проверить качество исходного изображения.")
	}
	f = append(f, "Вывод описывает форму динамограммы и не является самостоятельным основанием для окончательного диагноза или класса осложнения.")
	return f
}

func populateDynamogramProfile(a *DynamogramAnalysis, pts []DynamogramPoint) {
	if a == nil || len(pts) < 6 {
		return
	}
	third := len(pts) / 3
	avgSep := func(from, to int) float64 {
		if from < 0 {
			from = 0
		}
		if to > len(pts) {
			to = len(pts)
		}
		if to <= from {
			return 0
		}
		s := 0.0
		for _, p := range pts[from:to] {
			s += math.Max(0, p.UpperNorm-p.LowerNorm)
		}
		return s / float64(to-from)
	}
	a.LeftSeparation = avgSep(0, third)
	a.MiddleSeparation = avgSep(third, len(pts)-third)
	a.RightSeparation = avgSep(len(pts)-third, len(pts))
	maxSep := 0.0
	for _, p := range pts {
		maxSep = math.Max(maxSep, math.Max(0, p.UpperNorm-p.LowerNorm))
	}
	if maxSep > 1e-9 {
		a.FillUniformity = clamp01(a.MeanSeparation / maxSep)
	}
	q := maxInt(1, len(pts)/4)
	avgBranch := func(from, to int, upper bool) float64 {
		s := 0.0
		for _, p := range pts[from:to] {
			if upper {
				s += p.UpperNorm
			} else {
				s += p.LowerNorm
			}
		}
		return s / float64(to-from)
	}
	a.UpperTrend = avgBranch(len(pts)-q, len(pts), true) - avgBranch(0, q, true)
	a.LowerTrend = avgBranch(len(pts)-q, len(pts), false) - avgBranch(0, q, false)
}

const (
	dynMinAssessmentConfidence = 0.60
	dynNarrowAreaThreshold     = 0.20
	dynNarrowMeanThreshold     = 0.16
	dynEndSeparationRatio      = 0.68
	dynMinMiddleSeparation     = 0.12
	dynFillUniformityThreshold = 0.55
	dynClosureThreshold        = 0.58
	dynRoughnessThreshold      = 0.050
	dynSymmetryThreshold       = 0.64
)

type DynamogramHypothesis struct {
	Title     string
	Evidence  string
	Tech      string
	Telemetry string
	Repair    string
	Lab       string
}

func formatDynamogramHypothesis(h DynamogramHypothesis) string {
	return fmt.Sprintf("Предварительная гипотеза: %s.\r\nОснование: %s\r\nПроверить по источникам:\r\n- Техрежим: %s\r\n- Телеметрия: %s\r\n- Ремонт: %s\r\n- Лаборатория: %s", h.Title, h.Evidence, h.Tech, h.Telemetry, h.Repair, h.Lab)
}

func dynamogramOperatingAssessment(a *DynamogramAnalysis) (string, []string) {
	if a == nil {
		return "не сформирован", nil
	}
	if strings.EqualFold(a.ReviewStatus, "rejected") {
		return "техническая оценка отключена", []string{"Распознанный контур отклонён пользователем; технические гипотезы по нему не формируются."}
	}
	if a.Confidence < dynMinAssessmentConfidence {
		h := DynamogramHypothesis{
			Title:     "качество исходника недостаточно для инженерной интерпретации",
			Evidence:  fmt.Sprintf("уверенность распознавания %.0f %% ниже рабочей границы %.0f %%", a.Confidence*100, dynMinAssessmentConfidence*100),
			Tech:      "не назначать режимную причину до подтверждения контура; после повторного анализа сверить оборудование, ход, частоту и дебит",
			Telemetry: "сопоставить с цифровой динамограммой, током привода и повторяемостью рабочих циклов",
			Repair:    "не связывать форму с отказом до проверки качества; после подтверждения учесть последние работы с насосом, штангами и клапанами",
			Lab:       "не назначать осложнение по некачественному контуру; после подтверждения проверить доступные данные по газу, вязкости, АСПО и мехпримесям",
		}
		return "недостаточное качество исходника", []string{formatDynamogramHypothesis(h)}
	}

	var out []string
	mid := math.Max(a.MiddleSeparation, 1e-6)
	leftRatio := a.LeftSeparation / mid
	rightRatio := a.RightSeparation / mid
	status := "цикл без выраженного доминирующего дефекта по форме"

	if a.AreaRatio < dynNarrowAreaThreshold || a.MeanSeparation < dynNarrowMeanThreshold {
		status = "узкая рабочая петля / малый диапазон изменения нагрузки"
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "узкая рабочая петля или малый диапазон изменения нагрузки",
			Evidence:  fmt.Sprintf("Площадь петли %.3f и среднее раскрытие %.3f; рабочие границы внимания — ниже %.3f или %.3f", a.AreaRatio, a.MeanSeparation, dynNarrowAreaThreshold, dynNarrowMeanThreshold),
			Tech:      "сверить тип и подачу насоса, фактическую длину хода, частоту качаний, дебит и динамический уровень",
			Telemetry: "проверить диапазон нагрузки, ток привода, повторяемость циклов и фактический режим откачки",
			Repair:    "уточнить компоновку штанг, глубину подвески, замену насоса и изменения оборудования после последнего ремонта",
			Lab:       "проверить вязкость, газосодержание, АСПО и мехпримеси как возможные факторы изменения нагрузки",
		}))
	}
	if rightRatio < dynEndSeparationRatio && a.MiddleSeparation > dynMinMiddleSeparation {
		status = "возможное неполное заполнение насоса или газовое влияние"
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "возможное неполное заполнение насоса или газовое влияние",
			Evidence:  fmt.Sprintf("Раскрытие справа составляет %.0f %% от среднего участка при рабочей границе %.0f %%", rightRatio*100, dynEndSeparationRatio*100),
			Tech:      "сверить дебит, динамический уровень, газовый фактор, подачу насоса, частоту и режим откачки",
			Telemetry: "проверить давление (Р) на приёме, коэффициент наполнения, ток привода и устойчивость последовательных циклов",
			Repair:    "уточнить состояние газосепаратора, приёмной части насоса и клапанных узлов после последнего ремонта",
			Lab:       "проверить свободный и растворённый газ, данные о фазовом поведении флюида, вязкость и наличие устойчивой эмульсии",
		}))
	}
	if leftRatio < dynEndSeparationRatio && a.MiddleSeparation > dynMinMiddleSeparation {
		if status == "цикл без выраженного доминирующего дефекта по форме" {
			status = "возможное нарушение формирования нагрузки в начале хода"
		}
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "возможное нарушение формирования нагрузки в начале хода или влияние клапанов",
			Evidence:  fmt.Sprintf("Раскрытие слева составляет %.0f %% от среднего участка при рабочей границе %.0f %%", leftRatio*100, dynEndSeparationRatio*100),
			Tech:      "сверить длину хода, частоту качаний, подачу насоса, дебит и динамический уровень",
			Telemetry: "проверить начало восходящей и нисходящей ветвей, повторяемость циклов, ток и нагрузку привода",
			Repair:    "уточнить состояние всасывающего и нагнетательного клапанов, штанг и посадки насоса",
			Lab:       "проверить мехпримеси, минеральные и органические отложения, вязкость и эмульсию, способные влиять на клапаны",
		}))
	}
	if a.FillUniformity < dynFillUniformityThreshold {
		if status == "цикл без выраженного доминирующего дефекта по форме" {
			status = "неравномерное раскрытие рабочей петли"
		}
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "неравномерное заполнение или нестабильность рабочего цикла",
			Evidence:  fmt.Sprintf("Равномерность раскрытия %.0f %% ниже рабочей границы %.0f %%", a.FillUniformity*100, dynFillUniformityThreshold*100),
			Tech:      "сверить коэффициент наполнения, газовый фактор, динамический уровень, дебит и установленный режим",
			Telemetry: "проверить разброс последовательных циклов, ток, давление (Р) на приёме и изменения частоты",
			Repair:    "уточнить состояние насоса, клапанов, газосепаратора и изменения компоновки после ремонта",
			Lab:       "проверить газосодержание, вязкость, эмульсию, АСПО и мехпримеси",
		}))
	}
	if a.ClosureQuality < dynClosureThreshold {
		status = "возможная негерметичность клапанов либо неполный цикл записи"
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "возможная негерметичность клапанов либо неполный цикл записи",
			Evidence:  fmt.Sprintf("Замкнутость %.0f %% ниже рабочей границы %.0f %%", a.ClosureQuality*100, dynClosureThreshold*100),
			Tech:      "сверить длину хода, частоту, подачу насоса и устойчивость текущего режима",
			Telemetry: "исключить обрезанный цикл; проверить повторные карты, ток, нагрузку и давление (Р) на приёме",
			Repair:    "проверить историю отказов и работ по всасывающему и нагнетательному клапанам и насосу",
			Lab:       "проверить мехпримеси, соли, АСПО и продукты коррозии, способные нарушать герметичность клапанов",
		}))
	}
	if a.Roughness > dynRoughnessThreshold {
		if status == "цикл без выраженного доминирующего дефекта по форме" {
			status = "неустойчивая нагрузка / возможное повышенное трение"
		}
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "неустойчивая нагрузка или возможное повышенное механическое трение",
			Evidence:  fmt.Sprintf("Неровность %.4f выше рабочей границы %.4f", a.Roughness, dynRoughnessThreshold),
			Tech:      "сверить компоновку ГНО, глубину подвески, длину хода, частоту, дебит и динамический уровень",
			Telemetry: "проверить ток, вибрацию, разброс нагрузки и повторяемость локальных изломов на последовательных циклах",
			Repair:    "уточнить износ штанг и НКТ, искривление ствола, центраторы и замечания последнего ремонта",
			Lab:       "проверить вязкость, мехпримеси, АСПО, соли и продукты коррозии как факторы дополнительного сопротивления",
		}))
	}
	if a.Symmetry < dynSymmetryThreshold {
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "асимметрия ветвей с возможным влиянием клапанов, газа или трения",
			Evidence:  fmt.Sprintf("Симметрия %.0f %% ниже рабочей границы %.0f %%", a.Symmetry*100, dynSymmetryThreshold*100),
			Tech:      "сверить насос, подачу, частоту, длину хода, газовый фактор, дебит и динамический уровень",
			Telemetry: "сопоставить восходящую и нисходящую ветви на повторных циклах, ток, давление (Р) на приёме и коэффициент наполнения",
			Repair:    "уточнить состояние клапанов, штанг, НКТ, газосепаратора и последние изменения компоновки",
			Lab:       "проверить газ, вязкость, эмульсию, мехпримеси, АСПО и соли",
		}))
	}
	if len(out) == 0 {
		out = append(out, formatDynamogramHypothesis(DynamogramHypothesis{
			Title:     "выраженный доминирующий дефект по геометрии формы не выделен",
			Evidence:  "контур плавный и достаточно равномерный, показатели находятся вне рабочих границ внимания",
			Tech:      "подтвердить соответствие карты текущим оборудованию, длине хода, частоте, дебиту и динамическому уровню",
			Telemetry: "проверить устойчивость нескольких последовательных циклов, ток и давление (Р) на приёме",
			Repair:    "учесть дату и содержание последнего ремонта и отсутствие новых отказов после записи карты",
			Lab:       "проверить доступные данные по газу, вязкости, мехпримесям, АСПО, солям и коррозии; нормальная форма не исключает осложнения",
		}))
	}
	return status, out
}

type DynamogramObjectMatch struct {
	ObjectKey           string
	TechFound           bool
	Tech                TechObjectState
	TelemetryCount      int
	LatestTelemetry     TelemetryImport
	Installation        InstallationProfile
	InstallationSource  string
	PreviousDynamograms int
	LatestPrevious      DynamogramAnalysis
	Missing             []string
}

func dynamogramRegimeSummary(st TechObjectState) string {
	var parts []string
	if strings.TrimSpace(st.CurrentMRP) != "" {
		parts = append(parts, "МРП "+strings.TrimSpace(st.CurrentMRP)+" сут.")
	}
	if strings.TrimSpace(st.BufferPressure) != "" {
		parts = append(parts, "буферное давление (Рбуф) "+strings.TrimSpace(st.BufferPressure))
	}
	if strings.TrimSpace(st.LinePressure) != "" {
		parts = append(parts, "линейное давление (Рлин) "+strings.TrimSpace(st.LinePressure))
	}
	if strings.TrimSpace(st.TMSTemperature) != "" {
		parts = append(parts, "T ТМС "+strings.TrimSpace(st.TMSTemperature))
	}
	if strings.TrimSpace(st.PlannedActivities) != "" {
		parts = append(parts, "план "+strings.TrimSpace(st.PlannedActivities))
	}
	return strings.Join(parts, "; ")
}

func matchDynamogramObjectData(a *DynamogramAnalysis) DynamogramObjectMatch {
	match := DynamogramObjectMatch{}
	if a == nil {
		match.Missing = append(match.Missing, "результат динамометрии")
		return match
	}
	match.ObjectKey = strings.TrimSpace(a.ObjectKey)
	if match.ObjectKey == "" {
		match.Missing = append(match.Missing, "ключ объекта", "строка Техрежима", "телеметрия объекта", "предыдущие динамограммы объекта")
		return match
	}
	for _, st := range techCurrent {
		if strings.EqualFold(strings.TrimSpace(st.ObjectKey), match.ObjectKey) {
			match.TechFound = true
			match.Tech = st
			break
		}
	}
	if !match.TechFound {
		match.Missing = append(match.Missing, "строка Техрежима")
	} else {
		fields := []struct {
			name  string
			value string
		}{
			{"оборудование", match.Tech.Equipment},
			{"насос", match.Tech.PumpModel},
			{"дебит жидкости", match.Tech.LiquidRate},
			{"дебит нефти", match.Tech.OilRate},
			{"обводнённость", match.Tech.WaterCut},
			{"динамический уровень", match.Tech.DynamicLevel},
			{"режимные параметры", dynamogramRegimeSummary(match.Tech)},
		}
		for _, field := range fields {
			if strings.TrimSpace(field.value) == "" {
				match.Missing = append(match.Missing, field.name+" в Техрежиме")
			}
		}
	}
	for _, item := range telemetryImports {
		if !strings.EqualFold(strings.TrimSpace(item.ObjectKey), match.ObjectKey) {
			continue
		}
		match.TelemetryCount++
		if match.TelemetryCount == 1 || parseDate(item.ImportedAt).After(parseDate(match.LatestTelemetry.ImportedAt)) {
			match.LatestTelemetry = item
		}
	}
	if match.TelemetryCount == 0 {
		match.Missing = append(match.Missing, "телеметрия объекта")
	}
	match.Installation = installationProfileForObject(match.ObjectKey)
	if used, _ := installationProfileValueLines(match.Installation); len(used) > 0 {
		match.InstallationSource = "сохранённый профиль установки"
	} else if match.TelemetryCount > 0 {
		if used, _ := installationProfileValueLines(match.LatestTelemetry.Installation); len(used) > 0 {
			match.Installation = match.LatestTelemetry.Installation
			match.InstallationSource = "снимок профиля из последнего импорта телеметрии"
		}
	}
	if match.InstallationSource == "" && match.TechFound && (strings.TrimSpace(match.Tech.Equipment) != "" || strings.TrimSpace(match.Tech.PumpModel) != "") {
		match.Installation = InstallationProfile{ObjectKey: match.ObjectKey, EquipmentType: match.Tech.Equipment, PumpModel: match.Tech.PumpModel, UpdatedAt: match.Tech.ImportedAt}
		match.InstallationSource = "текущий Техрежим (только тип оборудования и модель насоса)"
	}
	if dynHistory == nil {
		dynHistory = loadDynamogramHistory()
	}
	for _, previous := range dynHistory {
		if !strings.EqualFold(strings.TrimSpace(previous.ObjectKey), match.ObjectKey) {
			continue
		}
		if a.ID != "" && previous.ID == a.ID {
			continue
		}
		match.PreviousDynamograms++
		if match.PreviousDynamograms == 1 || parseDate(previous.Created).After(parseDate(match.LatestPrevious.Created)) {
			match.LatestPrevious = previous
		}
	}
	if match.PreviousDynamograms == 0 {
		match.Missing = append(match.Missing, "предыдущие динамограммы объекта")
	}
	return match
}

func dynamogramObjectContext(a *DynamogramAnalysis) string {
	match := matchDynamogramObjectData(a)
	var b strings.Builder
	b.WriteString("АВТОМАТИЧЕСКОЕ СОПОСТАВЛЕНИЕ ПО КЛЮЧУ ОБЪЕКТА\r\n")
	if match.ObjectKey == "" {
		b.WriteString("Ключ объекта: отсутствует. Привяжите динамограмму к объекту для автоматической сверки.\r\n")
	} else {
		fmt.Fprintf(&b, "Ключ объекта: найден — %s.\r\n", match.ObjectKey)
	}
	if match.TechFound {
		b.WriteString("Техрежим: найден.\r\n")
		values := []struct {
			name  string
			value string
		}{
			{"Оборудование", match.Tech.Equipment},
			{"Насос", match.Tech.PumpModel},
			{"Дебит жидкости", match.Tech.LiquidRate},
			{"Дебит нефти", match.Tech.OilRate},
			{"Обводнённость", match.Tech.WaterCut},
			{"Динамический уровень", match.Tech.DynamicLevel},
			{"Режим", dynamogramRegimeSummary(match.Tech)},
		}
		for _, value := range values {
			if strings.TrimSpace(value.value) == "" {
				fmt.Fprintf(&b, "- %s: отсутствует.\r\n", value.name)
			} else {
				fmt.Fprintf(&b, "- %s: найдено — %s.\r\n", value.name, strings.TrimSpace(value.value))
			}
		}
	} else {
		b.WriteString("Техрежим: строка объекта не найдена.\r\n")
	}
	if match.TelemetryCount > 0 {
		t := match.LatestTelemetry
		fmt.Fprintf(&b, "Телеметрия: найдено импортов %d; последний — %s, период %s — %s, каналов %d, качество %s.\r\n", match.TelemetryCount, dash(t.SourceSystem), dash(t.StartedAt), dash(t.EndedAt), len(t.Channels), dash(t.Quality))
	} else {
		b.WriteString("Телеметрия: для объекта не найдена.\r\n")
	}
	b.WriteString("ПАРАМЕТРЫ УСТАНОВКИ\r\n")
	if match.InstallationSource == "" {
		b.WriteString("Профиль отсутствует. Геометрическая оценка контура доступна, но привязка к фактической компоновке ограничена.\r\n")
	} else {
		b.WriteString(installationProfileContext(match.Installation, match.InstallationSource) + "\r\n")
		b.WriteString("Параметры используются как контекст интерпретации и сами по себе не превращают геометрию контура в окончательный диагноз.\r\n")
	}
	if match.PreviousDynamograms > 0 {
		previous := match.LatestPrevious
		fmt.Fprintf(&b, "Предыдущие динамограммы: найдено %d; последняя — %s, качество %s, проверка %s, режим распознавания %s.\r\n", match.PreviousDynamograms, dash(previous.Created), dash(previous.Quality), dynReviewLabel(previous.ReviewStatus), dash(previous.DetectionMode))
	} else {
		b.WriteString("Предыдущие динамограммы: для объекта не найдены.\r\n")
	}
	if len(match.Missing) == 0 {
		b.WriteString("Отсутствующие обязательные сопоставления: не выявлены.")
	} else {
		b.WriteString("Отсутствующие данные: " + strings.Join(match.Missing, "; ") + ".")
	}
	return b.String()
}

func dynamogramRecommendations(a *DynamogramAnalysis) []string {
	if a == nil {
		return nil
	}
	var r []string
	if strings.EqualFold(strings.TrimSpace(a.ReviewStatus), "rejected") {
		r = append(r, "Контур отмечен пользователем как распознанный некорректно. Не использовать автоматическую интерпретацию; загрузить более качественный исходник либо цифровой массив и выполнить анализ заново.")
		return r
	}
	r = append(r, "Проверить контрольное изображение: восстановленный контур должен совпадать с исходной динамограммой без захвата осей, сетки и подписей.")
	if !a.Calibrated && !strings.Contains(strings.ToLower(a.DetectionMode), "цифров") {
		r = append(r, "Задать физические шкалы хода и нагрузки, если значения осей известны. Без шкал численная интерпретация нагрузки ограничена.")
	}
	if strings.Contains(strings.ToLower(a.DetectionMode), "цифров") {
		r = append(r, "Подтвердить единицы измерения двух исходных колонок и соответствие первой колонки ходу, второй - нагрузке.")
	}
	if a.Confidence < 0.75 {
		r = append(r, "Повторить анализ на более чётком исходнике или цифровом массиве: уверенность автоматического выделения контура недостаточна для уверенной инженерной интерпретации.")
	}
	if a.ClosureQuality < 0.65 {
		r = append(r, "Проверить полноту одного рабочего цикла и крайние участки контура. Незамкнутость может быть как признаком режима, так и следствием неполного/некачественного исходника.")
	}
	if a.Symmetry < 0.65 {
		r = append(r, "Сопоставить различия восходящей и нисходящей ветвей с режимом работы ШГН/УШГН; отдельно проверить возможное влияние трения, клапанов, газового фактора и неполного заполнения насоса.")
	}
	if a.AreaRatio < 0.24 || a.MeanSeparation < 0.18 {
		r = append(r, "Для узкой петли проверить корректность шкал и фактический диапазон нагрузок; затем сопоставить с дебитом, динамическим уровнем и коэффициентом наполнения насоса.")
	}
	if a.Roughness > 0.05 {
		r = append(r, "При выраженных локальных изломах проверить качество оцифровки, а при подтверждении формы - исключить нестабильность нагрузки и повышенное механическое трение.")
	}
	r = append(r, "Сравнить результат с эталонной/предыдущей динамограммой той же скважины при сопоставимом режиме, а также с током привода, дебитом, динамическим уровнем и данными ремонта.")
	r = append(r, "Окончательный вывод и решение по осложнению принимать после подтверждения промысловыми и лабораторными данными, ОЛИ/ОПИ/ОПР по применимости.")
	return r
}

func dynamogramInterpretation(a *DynamogramAnalysis) string {
	if a == nil {
		return "Результат ещё не сформирован."
	}
	var b strings.Builder
	status, operating := dynamogramOperatingAssessment(a)
	b.WriteString("ТЕХНИЧЕСКАЯ ИНТЕРПРЕТАЦИЯ ШГН / УШГН\r\n")
	if report := formatDynamogramPDFReport(a.ReportData); report != "" {
		b.WriteString(report)
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "Предварительный режим: %s.\r\n", status)
	for i, f := range operating {
		fmt.Fprintf(&b, "%d. %s\r\n", i+1, f)
	}
	b.WriteString("\r\nСВЕРКА С ДАННЫМИ ОБЪЕКТА\r\n")
	b.WriteString(dynamogramObjectContext(a) + "\r\n")
	b.WriteString("\r\nКОНТРОЛЬ РАСПОЗНАВАНИЯ И ГЕОМЕТРИЯ\r\n")
	fmt.Fprintf(&b, "Качество распознавания: %s. Уверенность: %.0f %%. Точек контура: %d.\r\n", a.Quality, a.Confidence*100, a.PointCount)
	gateLabel := "пройден"
	if a.QualityGate == dynQualityGateWarning {
		gateLabel = "есть предупреждения"
	} else if a.QualityGate == dynQualityGateBlocked {
		gateLabel = "подтверждение заблокировано"
	} else if a.QualityGate == "" {
		gateLabel = "не рассчитан для старой записи"
	}
	fmt.Fprintf(&b, "Автоконтроль контура: %s. Среднее/максимальное отклонение от видимого штриха: %.1f/%.1f %% высоты; пропуски %.0f %%; максимальный скачок %.1f %%; полка %.0f %%; признаки захвата рамки %.0f %%.\r\n", gateLabel, a.MeanSourceDeviation*100, a.MaxSourceDeviation*100, a.MissingSourceRatio*100, a.MaxBranchJump*100, a.FlatRunRatio*100, a.FrameCaptureRatio*100)
	switch strings.ToLower(strings.TrimSpace(a.ReviewStatus)) {
	case "confirmed":
		b.WriteString("Визуальная проверка: перерисованный контур подтверждён пользователем.\r\n")
	case "rejected":
		b.WriteString("Визуальная проверка: контур отклонён пользователем. Этот результат нельзя использовать как диагностическое основание до повторного анализа.\r\n")
	case "digital":
		b.WriteString("Источник: цифровой массив; подтверждение графического распознавания не требуется.\r\n")
	default:
		b.WriteString("Визуальная проверка: ещё не выполнена. Перед использованием результата сопоставьте исходник и перерисованный контур.\r\n")
	}
	fmt.Fprintf(&b, "Площадь петли: %.3f. Среднее раскрытие: %.3f. Симметрия: %.0f %%. Замкнутость: %.0f %%. Неровность: %.4f.\r\n", a.AreaRatio, a.MeanSeparation, a.Symmetry*100, a.ClosureQuality*100, a.Roughness)
	fmt.Fprintf(&b, "Раскрытие по ходу: начало %.3f; середина %.3f; конец %.3f. Равномерность заполнения: %.0f %%.\r\n", a.LeftSeparation, a.MiddleSeparation, a.RightSeparation, a.FillUniformity*100)
	fmt.Fprintf(&b, "Рабочие границы внимания: площадь < %.3f; среднее раскрытие < %.3f; край/середина < %.0f %%; равномерность < %.0f %%; замкнутость < %.0f %%; неровность > %.4f; симметрия < %.0f %%. Это скрининговые ориентиры, а не диагноз.\r\n", dynNarrowAreaThreshold, dynNarrowMeanThreshold, dynEndSeparationRatio*100, dynFillUniformityThreshold*100, dynClosureThreshold*100, dynRoughnessThreshold, dynSymmetryThreshold*100)
	if a.Calibrated {
		fmt.Fprintf(&b, "Шкалы: X %.3g - %.3g %s; Y %.3g - %.3g %s.\r\n", a.XMin, a.XMax, a.XUnit, a.YMin, a.YMax, a.YUnit)
	} else {
		b.WriteString("Физические шкалы не подтверждены; оценка выполняется по геометрии формы.\r\n")
	}
	b.WriteString("\r\nНАБЛЮДЕНИЯ\r\n")
	for i, f := range a.Findings {
		fmt.Fprintf(&b, "%d. %s\r\n", i+1, f)
	}
	b.WriteString("\r\nРЕКОМЕНДАЦИИ\r\n")
	for i, r := range dynamogramRecommendations(a) {
		fmt.Fprintf(&b, "%d. %s\r\n", i+1, r)
	}
	b.WriteString("\r\nОГРАНИЧЕНИЕ МЕТОДА\r\n")
	b.WriteString(a.MethodNote)
	return b.String()
}

func previewBMPPath(previewPNG string, targetW, targetH int) string {
	ext := filepath.Ext(previewPNG)
	// v2 guarantees that previews made before the contain-with-margins fix are
	// not reused from the on-disk cache after an application update.
	suffix := fmt.Sprintf("_display_contain_v2_%dx%d.bmp", maxInt(1, targetW), maxInt(1, targetH))
	if ext == "" {
		return previewPNG + suffix
	}
	return strings.TrimSuffix(previewPNG, ext) + suffix
}

func resizeContain(src image.Image, targetW, targetH int) *image.RGBA {
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.Draw(out, out.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return out
	}
	// Keep a visible white field on every side. Four percent scales naturally
	// on wide monitors, while eight pixels remains readable in compact panes.
	pad := maxInt(8, int(math.Round(float64(minInt(targetW, targetH))*0.04)))
	if pad*2 >= targetW {
		pad = maxInt(1, targetW/10)
	}
	if pad*2 >= targetH {
		pad = maxInt(1, targetH/10)
	}
	usableW := maxInt(1, targetW-2*pad)
	usableH := maxInt(1, targetH-2*pad)
	scale := math.Min(float64(usableW)/float64(sw), float64(usableH)/float64(sh))
	dw := int(math.Max(1, math.Round(float64(sw)*scale)))
	dh := int(math.Max(1, math.Round(float64(sh)*scale)))
	ox := (targetW - dw) / 2
	oy := (targetH - dh) / 2
	for y := 0; y < dh; y++ {
		sy := b.Min.Y + int(float64(y)*float64(sh)/float64(dh))
		if sy >= b.Max.Y {
			sy = b.Max.Y - 1
		}
		for x := 0; x < dw; x++ {
			sx := b.Min.X + int(float64(x)*float64(sw)/float64(dw))
			if sx >= b.Max.X {
				sx = b.Max.X - 1
			}
			out.Set(ox+x, oy+y, src.At(sx, sy))
		}
	}
	return out
}

func writeBMP24(img image.Image, path string) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return errors.New("пустое изображение")
	}
	rowSize := ((w*3 + 3) / 4) * 4
	pixelSize := rowSize * h
	fileSize := 54 + pixelSize
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := make([]byte, 54)
	hdr[0], hdr[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(hdr[2:6], uint32(fileSize))
	binary.LittleEndian.PutUint32(hdr[10:14], 54)
	binary.LittleEndian.PutUint32(hdr[14:18], 40)
	binary.LittleEndian.PutUint32(hdr[18:22], uint32(w))
	binary.LittleEndian.PutUint32(hdr[22:26], uint32(h))
	binary.LittleEndian.PutUint16(hdr[26:28], 1)
	binary.LittleEndian.PutUint16(hdr[28:30], 24)
	binary.LittleEndian.PutUint32(hdr[34:38], uint32(pixelSize))
	if _, err = f.Write(hdr); err != nil {
		return err
	}
	row := make([]byte, rowSize)
	for yy := h - 1; yy >= 0; yy-- {
		for i := range row {
			row[i] = 0
		}
		for x := 0; x < w; x++ {
			r, g, bv, _ := img.At(b.Min.X+x, b.Min.Y+yy).RGBA()
			i := x * 3
			row[i] = byte(bv >> 8)
			row[i+1] = byte(g >> 8)
			row[i+2] = byte(r >> 8)
		}
		if _, err = f.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func ensurePreviewBMP(previewPNG string, targetW, targetH int) (string, error) {
	if strings.TrimSpace(previewPNG) == "" {
		return "", errors.New("путь к изображению не задан")
	}
	bmp := previewBMPPath(previewPNG, targetW, targetH)
	if fi, err := os.Stat(bmp); err == nil && fi.Size() > 54 {
		return bmp, nil
	}
	f, err := os.Open(previewPNG)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return "", err
	}
	return bmp, writeBMP24(resizeContain(img, targetW, targetH), bmp)
}

func analyzeDynamogram(path, objectKey string, pdfPage ...int) (DynamogramAnalysis, []DynamogramPoint, error) {
	imagePath := path
	var reportData map[string]string
	page := 1
	if len(pdfPage) > 0 && pdfPage[0] > 0 {
		page = pdfPage[0]
	}
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		var renderErr error
		imagePath, renderErr = renderPDFPageToPNG(path, page)
		if renderErr != nil {
			return DynamogramAnalysis{}, nil, renderErr
		}
		if page == 1 {
			if text, textErr := extractPDFText(path); textErr == nil {
				reportData = parseDynamogramPDFReport(text)
			}
		}
	}
	src, format, err := decodeDynamogramImage(imagePath)
	if err != nil {
		return DynamogramAnalysis{}, nil, fmt.Errorf("не удалось открыть изображение: %w", err)
	}
	origW, origH := src.Bounds().Dx(), src.Bounds().Dy()
	if origW < 80 || origH < 80 {
		return DynamogramAnalysis{}, nil, errors.New("изображение слишком маленькое для устойчивой оцифровки")
	}
	work := resizeNearest(src, 1200, 900)
	plot, plotDetected := detectDynamogramPlot(work)
	if plotDetected {
		work = plot
	}
	w, h := work.Bounds().Dx(), work.Bounds().Dy()
	var c component
	var pts []DynamogramPoint
	var area, sep, sym, closure float64
	var validCols, maskCount int
	mode := "тёмная кривая"
	if plotDetected {
		mode = "ветви в найденной области графика"
		c, pts, area, sep, sym, closure, validCols, err = digitizePlotBranches(work)
		maskCount = maxInt(1, len(c.pixels))
	} else {
		var mask []bool
		mask, mode, maskCount = buildMask(work)
		cs := components(mask, w, h)
		c, err = bestCurveComponent(cs, w, h)
		if err == nil {
			pts, area, sep, sym, closure, validCols = digitizeComponent(c, w, h)
		}
	}
	if err != nil {
		return DynamogramAnalysis{}, nil, fmt.Errorf("автоматическое выделение кривой: %w", err)
	}
	if len(pts) < 12 {
		return DynamogramAnalysis{}, nil, errors.New("получено слишком мало точек кривой")
	}
	dominance := float64(len(c.pixels)) / math.Max(1, float64(maskCount))
	continuity := float64(validCols) / math.Max(1, float64(c.maxX-c.minX+1))
	spanW := float64(c.maxX-c.minX+1) / float64(w)
	spanH := float64(c.maxY-c.minY+1) / float64(h)
	spanScore := clamp01(math.Min(spanW/0.30, spanH/0.18))
	modeBase := 0.64
	if mode == "цветная кривая" {
		modeBase = 0.86
	} else if plotDetected {
		modeBase = 0.82
	}
	confidence := clamp01(0.22*modeBase + 0.28*clamp01(dominance*4) + 0.30*continuity + 0.20*spanScore)
	// A plausible dynamogram must form a wide, tall, visibly open loop. This
	// prevents table fragments from receiving a misleading high confidence.
	if spanW < 0.50 || spanH < 0.28 || sep < 0.10 {
		confidence = math.Min(confidence, 0.45)
	}
	maxStep := 0.0
	for i := 1; i < len(pts); i++ {
		maxStep = math.Max(maxStep, math.Abs(pts[i].UpperNorm-pts[i-1].UpperNorm))
		maxStep = math.Max(maxStep, math.Abs(pts[i].LowerNorm-pts[i-1].LowerNorm))
	}
	// A near-vertical jump between adjacent samples is characteristic of a
	// captured frame/grid intersection, not of a continuously digitized branch.
	if maxStep > 0.12 {
		confidence = math.Min(confidence, 0.45)
	}
	// Image recognition remains provisional until the operator has compared the
	// redraw with the source. Never report an unreviewed scan as "high quality".
	if plotDetected {
		confidence = math.Min(confidence, 0.79)
	}
	meanDeviation, maxDeviation, missingRatio, measuredMaxJump, flatRunRatio, frameCaptureRatio := contourSourceQuality(work, c, pts)
	maxStep = math.Max(maxStep, measuredMaxJump)
	quality := "низкое"
	if confidence >= 0.82 {
		quality = "высокое"
	} else if confidence >= 0.62 {
		quality = "приемлемое (до проверки)"
	}
	id := fmt.Sprintf("DYN-%s-%06d", time.Now().Format("20060102-150405"), time.Now().UnixNano()%1000000)
	root := dynRoot()
	srcCopy, srcHash, storeErr := storeAssetFile(path)
	if storeErr != nil {
		return DynamogramAnalysis{}, nil, fmt.Errorf("не удалось сохранить исходник динамограммы: %w", storeErr)
	}
	preview := filepath.Join(root, "preview", id+"_preview.png")
	reconstructed := filepath.Join(root, "preview", id+"_reconstructed.png")
	csvPath := filepath.Join(root, "curve", id+"_curve.csv")
	detectionMode := mode + " / " + format
	methodNote := "Автоматическая оцифровка формы без оптического распознавания символов (OCR) шкал и подписей. Абсолютные физические значения используются только после подтверждённой калибровки или поступления из структурированного источника."
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		detectionMode = fmt.Sprintf("PDF, страница %d / %s", page, detectionMode)
		methodNote = fmt.Sprintf("Страница %d PDF преобразована в изображение локальными средствами Windows. Текстовый слой и шкалы автоматически считываются только для первой страницы; все извлечённые реквизиты и контур требуется сверить с исходным отчётом.", page)
	}
	a := DynamogramAnalysis{ID: id, Created: time.Now().Format(time.RFC3339), ObjectKey: strings.TrimSpace(objectKey), SourceFile: path, SourceCopy: srcCopy, SourceSHA256: srcHash, PreviewPath: preview, ReconstructionPath: reconstructed, CSVPath: csvPath, ImageWidth: origW, ImageHeight: origH, WorkingWidth: w, WorkingHeight: h, DetectionMode: detectionMode, Confidence: confidence, Quality: quality, PointCount: len(pts), BBox: [4]int{c.minX, c.minY, c.maxX, c.maxY}, AreaRatio: area, MeanSeparation: sep, Symmetry: sym, ClosureQuality: closure, Roughness: roughnessOf(pts), MeanSourceDeviation: meanDeviation, MaxSourceDeviation: maxDeviation, MissingSourceRatio: missingRatio, MaxBranchJump: maxStep, FlatRunRatio: flatRunRatio, FrameCaptureRatio: frameCaptureRatio, MethodNote: methodNote, ReviewStatus: "not_reviewed"}
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		a.PDFPage = page
	}
	a.QualityGate, a.QualityIssues = dynamogramQualityGate(&a)
	if a.QualityGate == dynQualityGateBlocked {
		a.Quality = "блокировано: грубый дефект контура"
	}
	populateDynamogramProfile(&a, pts)
	applyPDFReportToAnalysis(&a, pts, reportData)
	a.Findings = analysisFindings(&a)
	if err = writeDynPreview(work, c, pts, preview); err != nil {
		return DynamogramAnalysis{}, nil, err
	}
	if err = writeDigitalDynPreview(&a, pts, reconstructed); err != nil {
		return DynamogramAnalysis{}, nil, err
	}
	if err = writeDynCSV(a, pts); err != nil {
		return DynamogramAnalysis{}, nil, err
	}
	if err = secureDynamogramOutputs(&a); err != nil {
		return DynamogramAnalysis{}, nil, fmt.Errorf("не удалось защитить результаты динамограммы: %w", err)
	}
	hist := loadDynamogramHistory()
	hist = append([]DynamogramAnalysis{a}, hist...)
	if len(hist) > 5000 {
		hist = hist[:5000]
	}
	_ = saveDynamogramHistory(hist)
	return a, pts, nil
}

func parseDynNumber(v string) (float64, error) {
	v = strings.TrimSpace(strings.TrimPrefix(v, "\ufeff"))
	v = strings.TrimPrefix(v, "ï»¿")
	v = strings.ReplaceAll(v, "\u200b", "")
	v = strings.ReplaceAll(v, " ", "")
	v = strings.ReplaceAll(v, ",", ".")
	return strconv.ParseFloat(v, 64)
}

func readDynamogramXY(path string) ([][2]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out [][2]float64
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		line = strings.TrimPrefix(line, "ï»¿")
		line = strings.ReplaceAll(line, "\u200b", "")
		if line == "" {
			continue
		}
		var fields []string
		switch {
		case strings.Contains(line, ";"):
			fields = strings.Split(line, ";")
		case strings.Contains(line, "\t"):
			fields = strings.Split(line, "\t")
		default:
			// Comma-separated decimal-dot files are accepted here. Decimal-comma files
			// should use semicolon, which is the common Russian Excel export.
			fields = strings.Split(line, ",")
		}
		if len(fields) < 2 {
			continue
		}
		x, ex := parseDynNumber(fields[0])
		y, ey := parseDynNumber(fields[1])
		if ex != nil || ey != nil || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			// Headers and comments are skipped; malformed numeric rows are ignored.
			continue
		}
		out = append(out, [2]float64{x, y})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) < 20 {
		return nil, fmt.Errorf("в CSV/TXT найдено только %d числовых строк; нужно не менее 20", len(out))
	}
	return out, nil
}

func envelopeFromXY(raw [][2]float64) ([]DynamogramPoint, float64, float64, float64, float64, error) {
	if len(raw) < 20 {
		return nil, 0, 0, 0, 0, errors.New("недостаточно точек")
	}
	xmin, xmax := raw[0][0], raw[0][0]
	ymin, ymax := raw[0][1], raw[0][1]
	for _, q := range raw[1:] {
		if q[0] < xmin {
			xmin = q[0]
		}
		if q[0] > xmax {
			xmax = q[0]
		}
		if q[1] < ymin {
			ymin = q[1]
		}
		if q[1] > ymax {
			ymax = q[1]
		}
	}
	if !(xmax > xmin) || !(ymax > ymin) {
		return nil, xmin, xmax, ymin, ymax, errors.New("диапазон хода или нагрузки равен нулю")
	}
	n := 200
	if len(raw) < n {
		n = len(raw)
	}
	if n < 40 {
		n = 40
	}
	upper := make([]float64, n)
	lower := make([]float64, n)
	seen := make([]bool, n)
	for i := range upper {
		upper[i] = -math.MaxFloat64
		lower[i] = math.MaxFloat64
	}
	for _, q := range raw {
		xn := clamp01((q[0] - xmin) / (xmax - xmin))
		yn := clamp01((q[1] - ymin) / (ymax - ymin))
		idx := int(math.Round(xn * float64(n-1)))
		if yn > upper[idx] {
			upper[idx] = yn
		}
		if yn < lower[idx] {
			lower[idx] = yn
		}
		seen[idx] = true
	}
	// Fill gaps by linear interpolation between nearest populated bins.
	prev := -1
	for i := 0; i < n; i++ {
		if !seen[i] {
			continue
		}
		if prev >= 0 && i-prev > 1 {
			for k := prev + 1; k < i; k++ {
				t := float64(k-prev) / float64(i-prev)
				upper[k] = upper[prev]*(1-t) + upper[i]*t
				lower[k] = lower[prev]*(1-t) + lower[i]*t
				seen[k] = true
			}
		}
		prev = i
	}
	first, last := -1, -1
	for i := 0; i < n; i++ {
		if seen[i] {
			first = i
			break
		}
	}
	for i := n - 1; i >= 0; i-- {
		if seen[i] {
			last = i
			break
		}
	}
	if first < 0 {
		return nil, xmin, xmax, ymin, ymax, errors.New("не удалось построить контур")
	}
	for i := 0; i < first; i++ {
		upper[i], lower[i], seen[i] = upper[first], lower[first], true
	}
	for i := last + 1; i < n; i++ {
		upper[i], lower[i], seen[i] = upper[last], lower[last], true
	}
	pts := make([]DynamogramPoint, n)
	for i := 0; i < n; i++ {
		xn := float64(i) / float64(n-1)
		if upper[i] < lower[i] {
			upper[i], lower[i] = lower[i], upper[i]
		}
		pts[i] = DynamogramPoint{
			XNorm: xn, UpperNorm: clamp01(upper[i]), LowerNorm: clamp01(lower[i]),
			X:     xmin + xn*(xmax-xmin),
			Upper: ymin + clamp01(upper[i])*(ymax-ymin),
			Lower: ymin + clamp01(lower[i])*(ymax-ymin),
		}
	}
	return pts, xmin, xmax, ymin, ymax, nil
}

func dynMetrics(pts []DynamogramPoint) (area, sep, symmetry, closure float64) {
	if len(pts) == 0 {
		return
	}
	d := make([]float64, len(pts))
	for i, p := range pts {
		d[i] = math.Max(0, p.UpperNorm-p.LowerNorm)
		area += d[i]
	}
	area /= float64(len(pts))
	sep = area
	err := 0.0
	for i := range d {
		err += math.Abs(d[i] - d[len(d)-1-i])
	}
	err /= float64(len(d))
	symmetry = 1
	if sep > 1e-9 {
		symmetry = clamp01(1 - err/(2*sep))
	}
	edge := (d[0] + d[len(d)-1]) / 2
	closure = 1
	if sep > 1e-9 {
		closure = clamp01(1 - edge/(sep*1.5))
	}
	return
}

func writeDigitalDynPreview(a *DynamogramAnalysis, pts []DynamogramPoint, path string) error {
	const w, h = 1000, 600
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)
	graphite := color.RGBA{R: 43, G: 46, B: 50, A: 255}
	red := color.RGBA{R: 201, G: 20, B: 45, A: 255}
	left, top, right, bottom := 78, 34, w-34, h-76
	drawRect(img, left, top, right, bottom, graphite)
	grid := color.RGBA{R: 220, G: 224, B: 228, A: 255}
	xmin, xmax, ymin, ymax := 0.0, 1.0, 0.0, 1.0
	xunit, yunit := "norm", "norm"
	if a != nil && a.Calibrated {
		xmin, xmax, ymin, ymax = a.XMin, a.XMax, a.YMin, a.YMax
		xunit, yunit = axisUnitASCII(a.XUnit), axisUnitASCII(a.YUnit)
	}
	for tick := 0; tick <= 5; tick++ {
		x := left + tick*(right-left)/5
		y := bottom - tick*(bottom-top)/5
		if tick > 0 && tick < 5 {
			drawLine(img, x, top, x, bottom, grid)
			drawLine(img, left, y, right, y, grid)
		}
		drawLine(img, x, bottom, x, bottom+6, graphite)
		drawLine(img, left-6, y, left, y, graphite)
		drawBitmapText(img, x-14, bottom+12, compactAxisNumber(xmin+(xmax-xmin)*float64(tick)/5), graphite, 2)
		drawBitmapText(img, 4, y-6, compactAxisNumber(ymin+(ymax-ymin)*float64(tick)/5), graphite, 2)
	}
	drawBitmapText(img, (left+right)/2-72, h-30, "X stroke ["+xunit+"]", graphite, 2)
	drawBitmapText(img, 8, 8, "Y load ["+yunit+"]", graphite, 2)
	drawBitmapText(img, right-250, 8, "step X="+compactAxisNumber((xmax-xmin)/5)+" Y="+compactAxisNumber((ymax-ymin)/5), graphite, 2)
	for i := 1; i < len(pts); i++ {
		x0 := left + int(pts[i-1].XNorm*float64(right-left))
		x1 := left + int(pts[i].XNorm*float64(right-left))
		yu0 := top + int((1-pts[i-1].UpperNorm)*float64(bottom-top))
		yu1 := top + int((1-pts[i].UpperNorm)*float64(bottom-top))
		yl0 := top + int((1-pts[i-1].LowerNorm)*float64(bottom-top))
		yl1 := top + int((1-pts[i].LowerNorm)*float64(bottom-top))
		drawLine(img, x0, yu0, x1, yu1, red)
		drawLine(img, x0, yl0, x1, yl1, red)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func compactAxisNumber(v float64) string { return strconv.FormatFloat(v, 'g', 3, 64) }

func axisUnitASCII(unit string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch {
	case strings.Contains(u, "кн") || strings.Contains(u, "kn"):
		return "kN"
	case u == "м" || u == "m":
		return "m"
	case strings.Contains(u, "норм"):
		return "norm"
	case strings.Contains(u, "исх"):
		return "src"
	case u == "":
		return "unit"
	default:
		return "unit"
	}
}

var bitmapGlyphs = map[rune][7]byte{
	'0': {14, 17, 19, 21, 25, 17, 14}, '1': {4, 12, 4, 4, 4, 4, 14}, '2': {14, 17, 1, 2, 4, 8, 31},
	'3': {30, 1, 1, 14, 1, 1, 30}, '4': {2, 6, 10, 18, 31, 2, 2}, '5': {31, 16, 16, 30, 1, 1, 30},
	'6': {14, 16, 16, 30, 17, 17, 14}, '7': {31, 1, 2, 4, 8, 8, 8}, '8': {14, 17, 17, 14, 17, 17, 14},
	'9': {14, 17, 17, 15, 1, 1, 14}, 'X': {17, 17, 10, 4, 10, 17, 17}, 'Y': {17, 17, 10, 4, 4, 4, 4},
	'N': {17, 25, 21, 19, 17, 17, 17}, 'k': {16, 18, 20, 24, 20, 18, 17}, 'm': {0, 0, 26, 21, 21, 21, 21},
	'n': {0, 0, 30, 17, 17, 17, 17}, 'o': {0, 0, 14, 17, 17, 17, 14}, 'r': {0, 0, 22, 25, 16, 16, 16},
	's': {0, 0, 15, 16, 14, 1, 30}, 't': {8, 8, 30, 8, 8, 9, 6}, 'e': {0, 0, 14, 17, 31, 16, 14},
	'p': {0, 0, 30, 17, 30, 16, 16}, 'u': {0, 0, 17, 17, 17, 19, 13}, 'i': {4, 0, 12, 4, 4, 4, 14},
	'c': {0, 0, 14, 17, 16, 17, 14}, 'l': {12, 4, 4, 4, 4, 4, 14}, 'a': {0, 0, 14, 1, 15, 17, 15},
	'd': {1, 1, 15, 17, 17, 17, 15}, '=': {0, 0, 31, 0, 31, 0, 0}, '.': {0, 0, 0, 0, 0, 6, 6},
	'-': {0, 0, 0, 31, 0, 0, 0}, '+': {0, 4, 4, 31, 4, 4, 0}, '[': {14, 8, 8, 8, 8, 8, 14}, ']': {14, 2, 2, 2, 2, 2, 14}, ' ': {},
}

func drawBitmapText(img *image.RGBA, x, y int, text string, c color.RGBA, scale int) {
	for _, r := range text {
		g, ok := bitmapGlyphs[r]
		if !ok {
			g = bitmapGlyphs[' ']
		}
		for row, bits := range g {
			for col := 0; col < 5; col++ {
				if bits&(1<<uint(4-col)) == 0 {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						dot(img, x+col*scale+dx, y+row*scale+dy, c, 0)
					}
				}
			}
		}
		x += 6 * scale
	}
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := absInt(x1 - x0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -absInt(y1 - y0)
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		dot(img, x0, y0, c, 1)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func importDynamogramCSV(path, objectKey string) (DynamogramAnalysis, []DynamogramPoint, error) {
	raw, err := readDynamogramXY(path)
	if err != nil {
		return DynamogramAnalysis{}, nil, err
	}
	pts, xmin, xmax, ymin, ymax, err := envelopeFromXY(raw)
	if err != nil {
		return DynamogramAnalysis{}, nil, err
	}
	area, sep, sym, closure := dynMetrics(pts)
	id := fmt.Sprintf("DYN-%s-%06d", time.Now().Format("20060102-150405"), time.Now().UnixNano()%1000000)
	root := dynRoot()
	srcCopy, srcHash, storeErr := storeAssetFile(path)
	if storeErr != nil {
		return DynamogramAnalysis{}, nil, fmt.Errorf("не удалось сохранить исходник цифровой динамограммы: %w", storeErr)
	}
	preview := filepath.Join(root, "preview", id+"_preview.png")
	csvPath := filepath.Join(root, "curve", id+"_curve.csv")
	a := DynamogramAnalysis{
		ID: id, Created: time.Now().Format(time.RFC3339), ObjectKey: strings.TrimSpace(objectKey), SourceFile: path, SourceCopy: srcCopy,
		SourceSHA256: srcHash, PreviewPath: preview, CSVPath: csvPath, DetectionMode: "цифровой массив CSV/TXT",
		Confidence: 0.995, Quality: "высокое", PointCount: len(pts), AreaRatio: area, MeanSeparation: sep, Symmetry: sym,
		ClosureQuality: closure, Roughness: roughnessOf(pts), Calibrated: true, XMin: xmin, XMax: xmax, YMin: ymin, YMax: ymax,
		XUnit: "исх. ед.", YUnit: "исх. ед.", MethodNote: "Цифровой контур импортирован из двух числовых колонок. Физические единицы следует подтвердить по источнику данных.", ReviewStatus: "digital",
	}
	populateDynamogramProfile(&a, pts)
	a.Findings = analysisFindings(&a)
	if err = writeDigitalDynPreview(&a, pts, preview); err != nil {
		return DynamogramAnalysis{}, nil, err
	}
	if err = writeDynCSV(a, pts); err != nil {
		return DynamogramAnalysis{}, nil, err
	}
	if err = secureDynamogramOutputs(&a); err != nil {
		return DynamogramAnalysis{}, nil, fmt.Errorf("не удалось защитить результаты динамограммы: %w", err)
	}
	hist := loadDynamogramHistory()
	hist = append([]DynamogramAnalysis{a}, hist...)
	if len(hist) > 5000 {
		hist = hist[:5000]
	}
	_ = saveDynamogramHistory(hist)
	return a, pts, nil
}

func roughnessOf(pts []DynamogramPoint) float64 {
	if len(pts) < 3 {
		return 0
	}
	s := make([]float64, len(pts))
	for i, p := range pts {
		s[i] = p.UpperNorm - p.LowerNorm
	}
	r := 0.0
	for i := 1; i < len(s)-1; i++ {
		r += math.Abs(s[i+1] - 2*s[i] + s[i-1])
	}
	return r / float64(len(s)-2)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = bufio.NewReader(in).WriteTo(out)
	return err
}

func writeDynCSV(a DynamogramAnalysis, pts []DynamogramPoint) error {
	f, err := os.Create(a.CSVPath)
	if err != nil {
		return err
	}
	defer f.Close()
	// UTF-8 BOM prevents Excel on Russian Windows from displaying mojibake in
	// the first/header row when units contain Cyrillic characters.
	if _, err = f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	w := csv.NewWriter(f)
	defer w.Flush()
	if a.Calibrated {
		_ = w.Write([]string{"x_norm", "upper_norm", "lower_norm", "x_" + a.XUnit, "upper_" + a.YUnit, "lower_" + a.YUnit})
	} else {
		_ = w.Write([]string{"x_norm", "upper_norm", "lower_norm"})
	}
	for _, p := range pts {
		row := []string{ff(p.XNorm), ff(p.UpperNorm), ff(p.LowerNorm)}
		if a.Calibrated {
			row = append(row, ff(p.X), ff(p.Upper), ff(p.Lower))
		}
		_ = w.Write(row)
	}
	return w.Error()
}
func ff(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }

func applyDynCalibration(a *DynamogramAnalysis, pts []DynamogramPoint, xmin, xmax, ymin, ymax float64, xunit, yunit string) error {
	if a == nil {
		return errors.New("сначала выполните анализ изображения")
	}
	if !(xmax > xmin) || !(ymax > ymin) {
		return errors.New("максимум шкалы должен быть больше минимума")
	}
	for i := range pts {
		pts[i].X = xmin + pts[i].XNorm*(xmax-xmin)
		pts[i].Upper = ymin + pts[i].UpperNorm*(ymax-ymin)
		pts[i].Lower = ymin + pts[i].LowerNorm*(ymax-ymin)
	}
	a.Calibrated = true
	a.XMin = xmin
	a.XMax = xmax
	a.YMin = ymin
	a.YMax = ymax
	a.XUnit = strings.TrimSpace(xunit)
	a.YUnit = strings.TrimSpace(yunit)
	if a.XUnit == "" {
		a.XUnit = "ед."
	}
	if a.YUnit == "" {
		a.YUnit = "ед."
	}
	if err := writeSecureDynCSV(a, pts); err != nil {
		return err
	}
	if err := writeSecureDynReconstruction(a, pts); err != nil {
		return err
	}
	hist := loadDynamogramHistory()
	for i := range hist {
		if hist[i].ID == a.ID {
			hist[i] = *a
			break
		}
	}
	return saveDynamogramHistory(hist)
}

func writeDynPreview(img *image.RGBA, c component, pts []DynamogramPoint, path string) error {
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, image.Point{}, draw.Src)
	red := color.RGBA{R: 201, G: 20, B: 45, A: 255}
	graphite := color.RGBA{R: 43, G: 46, B: 50, A: 255}
	drawRect(out, c.minX, c.minY, c.maxX, c.maxY, graphite)
	bw := c.maxX - c.minX + 1
	bh := c.maxY - c.minY + 1
	for _, p := range pts {
		x := c.minX + int(math.Round(p.XNorm*float64(maxInt(1, bw-1))))
		yu := c.minY + int(math.Round((1-p.UpperNorm)*float64(maxInt(1, bh-1))))
		yl := c.minY + int(math.Round((1-p.LowerNorm)*float64(maxInt(1, bh-1))))
		dot(out, x, yu, red, 2)
		dot(out, x, yl, red, 2)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, out)
}
func drawRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for x := x0; x <= x1; x++ {
		dot(img, x, y0, c, 1)
		dot(img, x, y1, c, 1)
	}
	for y := y0; y <= y1; y++ {
		dot(img, x0, y, c, 1)
		dot(img, x1, y, c, 1)
	}
}
func dot(img *image.RGBA, x, y int, c color.RGBA, r int) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			xx, yy := x+dx, y+dy
			if image.Pt(xx, yy).In(img.Bounds()) {
				img.SetRGBA(xx, yy, c)
			}
		}
	}
}

// Ensure image formats are linked into image.Decode.
var _ = jpeg.DefaultQuality
var _ = png.BestCompression
var _ = json.Valid
var _ = hex.EncodeToString
var _ = sha256.Size
