//go:build windows

package main

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	ID_EXPORT_SELECTED = 222
	ID_IMPORT_ROSA     = 223
	ID_EXPORT_ARCHIVE  = 224
	ID_DELETE_HISTORY  = 225
	ID_CLEAR_CACHE     = 226
	ID_CLEAR_HISTORY   = 227
)

var pGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")

type VersionTombstone struct {
	VersionID string `json:"version_id"`
	ObjectKey string `json:"object_key,omitempty"`
	DeletedAt string `json:"deleted_at"`
}

type StorageStats struct {
	TotalBytes      int64
	DataFiles       int
	AttachmentBytes int64
	CacheBytes      int64
	BackupBytes     int64
}

type RosaEntryMeta struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type RosaManifest struct {
	Format        string                   `json:"format"`
	SchemaVersion int                      `json:"schema_version"`
	PackageID     string                   `json:"package_id"`
	PackageType   string                   `json:"package_type"`
	CreatedAt     string                   `json:"created_at"`
	Product       string                   `json:"product"`
	RecordCount   int                      `json:"record_count"`
	ObjectCount   int                      `json:"object_count"`
	Entries       map[string]RosaEntryMeta `json:"entries"`
	Note          string                   `json:"note,omitempty"`
}

type PackedDynamogram struct {
	Analysis     DynamogramAnalysis `json:"analysis"`
	SourceAsset  string             `json:"source_asset,omitempty"`
	PreviewAsset string             `json:"preview_asset,omitempty"`
	CurveAsset   string             `json:"curve_asset,omitempty"`
}

type RosaPackageData struct {
	Versions    []VersionRecord    `json:"versions"`
	Outcomes    []Outcome          `json:"outcomes,omitempty"`
	Plans       []PlanItem         `json:"plans,omitempty"`
	Labs        []LabRecord        `json:"laboratory,omitempty"`
	CurrentTech []TechObjectState  `json:"current_tech,omitempty"`
	Dynamograms []PackedDynamogram `json:"dynamograms,omitempty"`
}

type RosaImportReceipt struct {
	PackageID        string `json:"package_id"`
	PackageType      string `json:"package_type"`
	SourceFile       string `json:"source_file"`
	ImportedAt       string `json:"imported_at"`
	AddedVersions    int    `json:"added_versions"`
	SkippedVersions  int    `json:"skipped_versions"`
	AddedLabs        int    `json:"added_laboratory_records"`
	AddedDynamograms int    `json:"added_dynamograms"`
}

type ReceivedTechState struct {
	PackageID  string          `json:"package_id"`
	ImportedAt string          `json:"imported_at"`
	State      TechObjectState `json:"state"`
}

var deletedVersionIDs = map[string]bool{}
var versionTombstones []VersionTombstone

func loadDeletedState() {
	versionTombstones = nil
	loadJSON(filepath.Join(dataDir, "deleted_versions.json"), &versionTombstones)
	deletedVersionIDs = map[string]bool{}
	for _, t := range versionTombstones {
		if t.VersionID != "" {
			deletedVersionIDs[t.VersionID] = true
		}
	}
}

func filterDeletedVersions(vs []VersionRecord) []VersionRecord {
	if len(deletedVersionIDs) == 0 {
		return vs
	}
	out := make([]VersionRecord, 0, len(vs))
	for _, v := range vs {
		if !deletedVersionIDs[v.VersionID] {
			out = append(out, v)
		}
	}
	return out
}

func deleteSelectedVersion() {
	x := filteredVersions()
	i := selectedRow(historyList)
	if i < 0 || i >= len(x) {
		msg("Удаление расчёта", "Выберите расчёт в истории.", MB_OK|MB_ICONWARNING)
		return
	}
	v := x[i]
	text := fmt.Sprintf("Убрать выбранный расчёт из рабочего архива?\n\n%s\n%s\n\nИсходная запись останется в журнале целостности, но больше не будет отображаться и не восстановится при синхронизации.", objectLabel(v), v.Created)
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Удаление расчёта"))), 0x00000004|MB_ICONWARNING) // MB_YESNO
	if r != 6 {                                                                                                                                               // IDYES
		return
	}
	if deletedVersionIDs[v.VersionID] {
		return
	}
	versionTombstones = append(versionTombstones, VersionTombstone{VersionID: v.VersionID, ObjectKey: v.ObjectKey, DeletedAt: time.Now().Format(time.RFC3339)})
	if err := saveJSON(filepath.Join(dataDir, "deleted_versions.json"), versionTombstones); err != nil {
		msg("Удаление расчёта", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	deletedVersionIDs[v.VersionID] = true
	versions = filterDeletedVersions(versions)
	_ = saveJSON(filepath.Join(dataDir, "calculation_versions.json"), versions)
	addSessionTrashItem("Расчётная версия · "+v.VersionID, objectLabel(v)+"\r\nДата: "+v.Created, func() error {
		return restoreCalculationVersions([]VersionRecord{v})
	}, nil)
	refreshAll()
}

func restoreCalculationVersions(items []VersionRecord) error {
	restoreIDs := map[string]bool{}
	for _, item := range items {
		restoreIDs[item.VersionID] = true
		deletedVersionIDs[item.VersionID] = false
	}
	kept := versionTombstones[:0]
	for _, tombstone := range versionTombstones {
		if !restoreIDs[tombstone.VersionID] {
			kept = append(kept, tombstone)
		}
	}
	versionTombstones = kept
	if err := saveJSON(filepath.Join(dataDir, "deleted_versions.json"), versionTombstones); err != nil {
		return err
	}
	have := map[string]bool{}
	for _, item := range versions {
		have[item.VersionID] = true
	}
	for _, item := range items {
		if !have[item.VersionID] {
			versions = append(versions, item)
		}
	}
	sort.SliceStable(versions, func(i, j int) bool { return parseDate(versions[i].Created).After(parseDate(versions[j].Created)) })
	if err := saveJSON(filepath.Join(dataDir, "calculation_versions.json"), versions); err != nil {
		return err
	}
	refreshAll()
	return nil
}

func clearVisibleHistory() {
	if len(versions) == 0 {
		msg("Очистка истории", "В личной истории нет расчётов для удаления.", MB_OK|MB_ICONINFORMATION)
		return
	}
	text := fmt.Sprintf("Очистить всю личную историю расчётов пользователя %s?\n\nБудет скрыто записей: %d. Техрежим, лаборатория, телеметрия и динамограммы не удаляются.\n\nДействие нельзя отменить через интерфейс.", currentUserLabel(), len(versions))
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Очистка истории"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	removedVersions := append([]VersionRecord(nil), versions...)
	now := time.Now().Format(time.RFC3339)
	for _, v := range versions {
		if v.VersionID == "" || deletedVersionIDs[v.VersionID] {
			continue
		}
		versionTombstones = append(versionTombstones, VersionTombstone{VersionID: v.VersionID, ObjectKey: v.ObjectKey, DeletedAt: now})
		deletedVersionIDs[v.VersionID] = true
	}
	if err := saveJSON(filepath.Join(dataDir, "deleted_versions.json"), versionTombstones); err != nil {
		msg("Очистка истории", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	versions = nil
	_ = saveJSON(filepath.Join(dataDir, "calculation_versions.json"), versions)
	addSessionTrashItem(fmt.Sprintf("История расчётов · %d записей", len(removedVersions)), "Личная история текущего пользователя", func() error {
		return restoreCalculationVersions(removedVersions)
	}, nil)
	refreshAll()
	msg("Очистка истории", "Личная история расчётов очищена. Технический журнал целостности сохранён, чтобы удалённые версии не появились снова после импорта или объединения архивов.", MB_OK|MB_ICONINFORMATION)
}

func storageStats() StorageStats {
	var s StorageStats
	_ = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		s.TotalBytes += info.Size()
		s.DataFiles++
		rel, _ := filepath.Rel(dataDir, path)
		low := strings.ToLower(rel)
		if strings.HasPrefix(low, "assets"+string(filepath.Separator)) || strings.HasPrefix(low, "vault"+string(filepath.Separator)) || strings.HasPrefix(low, "dynamograms"+string(filepath.Separator)+"source"+string(filepath.Separator)) {
			s.AttachmentBytes += info.Size()
		}
		if strings.Contains(low, "_display_") && strings.HasSuffix(low, ".bmp") {
			s.CacheBytes += info.Size()
		}
		if strings.HasPrefix(low, "reserve_copies"+string(filepath.Separator)) || strings.HasPrefix(low, "state_backups"+string(filepath.Separator)) {
			s.BackupBytes += info.Size()
		}
		return nil
	})
	return s
}

func humanBytes(n int64) string {
	const k = int64(1024)
	if n < k {
		return fmt.Sprintf("%d Б", n)
	}
	if n < k*k {
		return fmt.Sprintf("%.1f КБ", float64(n)/float64(k))
	}
	if n < k*k*k {
		return fmt.Sprintf("%.1f МБ", float64(n)/float64(k*k))
	}
	return fmt.Sprintf("%.2f ГБ", float64(n)/float64(k*k*k))
}

func clearDisplayCache() (int, int64, error) {
	root := filepath.Join(dataDir, "dynamograms")
	count := 0
	var bytes int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		low := strings.ToLower(filepath.Base(path))
		if strings.Contains(low, "_display_") && strings.HasSuffix(low, ".bmp") {
			bytes += info.Size()
			if e := os.Remove(path); e != nil {
				return e
			}
			count++
		}
		return nil
	})
	return count, bytes, err
}

func assetStorePath(hash, ext string) string {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if len(hash) < 2 {
		hash = "00" + hash
	}
	ext = strings.ToLower(ext)
	if ext == "" || len(ext) > 12 {
		ext = ".bin"
	}
	dir := filepath.Join(dataDir, "assets", hash[:2])
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, hash+ext)
}

// storeAssetFile keeps heavy source files by content hash. Reusing the same input in
// several calculations therefore does not create several physical copies.
func storeAssetFile(src string) (string, string, error) {
	return secureStoreFile(src)
}

func createStateBackup() error {
	dir := filepath.Join(dataDir, "state_backups")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	target := filepath.Join(dir, "ROSA_state_"+time.Now().Format("20060102_150405")+".zip")
	zf, err := os.Create(target)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(zf)
	candidates := []string{
		"calculation_versions.jsonl", "calculation_versions.json", "deleted_versions.json",
		"outcomes.json", "plan.json", "technologies.json", "tech_current.json", "tech_imports.json",
		"laboratory_records.jsonl", filepath.Join("dynamograms", "analyses.json"), "rosa_imports.json", "exchange_tech_states.json",
	}
	for _, rel := range candidates {
		path := filepath.Join(dataDir, rel)
		fi, e := os.Stat(path)
		if e != nil || fi.IsDir() {
			continue
		}
		w, e := zw.Create(filepath.ToSlash(rel))
		if e != nil {
			zw.Close()
			zf.Close()
			return e
		}
		f, e := os.Open(path)
		if e != nil {
			zw.Close()
			zf.Close()
			return e
		}
		_, e = io.Copy(w, f)
		f.Close()
		if e != nil {
			zw.Close()
			zf.Close()
			return e
		}
	}
	if err = zw.Close(); err != nil {
		zf.Close()
		return err
	}
	return zf.Close()
}

func chooseRosaSavePath(defaultName string) string {
	buf := make([]uint16, 32768)
	if !strings.HasSuffix(strings.ToLower(defaultName), ".rosa") {
		defaultName += ".rosa"
	}
	q, _ := syscall.UTF16FromString(defaultName)
	copy(buf, q)
	filter := utf16.Encode([]rune("Пакет РОСА (*.rosa)\x00*.rosa\x00Все файлы (*.*)\x00*.*\x00\x00"))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrDefExt: p("rosa"), LpstrTitle: p("Сохранить пакет РОСА"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	if !strings.HasSuffix(strings.ToLower(path), ".rosa") {
		path += ".rosa"
	}
	return path
}

func chooseRosaOpenPath() string {
	buf := make([]uint16, 32768)
	filter := utf16.Encode([]rune("Пакет РОСА (*.rosa)\x00*.rosa\x00Все файлы (*.*)\x00*.*\x00\x00"))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: p("Открыть пакет РОСА"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func packageID(kind string, vs []VersionRecord) string {
	h := sha256.New()
	io.WriteString(h, kind+"|"+time.Now().Format(time.RFC3339Nano)+"|")
	for _, v := range vs {
		io.WriteString(h, v.VersionID+"|")
	}
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))[:16])
}

func objectKeys(vs []VersionRecord) map[string]bool {
	out := map[string]bool{}
	for _, v := range vs {
		if v.ObjectKey != "" {
			out[v.ObjectKey] = true
		}
	}
	return out
}

func packData(vs []VersionRecord, full bool) (RosaPackageData, map[string]string) {
	keys := objectKeys(vs)
	packedVersions := make([]VersionRecord, 0, len(vs))
	for _, v := range vs {
		packedVersions = append(packedVersions, hydrateVersionPayload(v))
	}
	d := RosaPackageData{Versions: packedVersions}
	versionIDs := map[string]bool{}
	for _, v := range vs {
		versionIDs[v.VersionID] = true
	}
	for _, o := range outcomes {
		if full || versionIDs[o.VersionID] {
			d.Outcomes = append(d.Outcomes, o)
		}
	}
	for _, p := range plans {
		if full || versionIDs[p.VersionID] || keys[p.ObjectKey] {
			d.Plans = append(d.Plans, p)
		}
	}
	if len(labRecords) == 0 {
		labRecords = loadLabRecords()
	}
	for _, r := range labRecords {
		if full || keys[r.ObjectKey] {
			d.Labs = append(d.Labs, r)
		}
	}
	for _, st := range techCurrent {
		if full || keys[st.ObjectKey] {
			d.CurrentTech = append(d.CurrentTech, st)
		}
	}

	assets := map[string]string{}
	for _, a0 := range loadDynamogramHistory() {
		if !full && !keys[a0.ObjectKey] {
			continue
		}
		a := a0
		// Package metadata never contains another employee's local paths. Only the
		// basename of the originally imported source is retained for provenance.
		sourceExt := filepath.Ext(a0.SourceFile)
		if a.SourceFile != "" {
			a.SourceFile = filepath.Base(a.SourceFile)
		}
		a.SourceCopy, a.PreviewPath, a.CSVPath = "", "", ""
		pd := PackedDynamogram{Analysis: a}
		add := func(path, kind, preferredExt string) string {
			if strings.TrimSpace(path) == "" {
				return ""
			}
			plain := path
			var err error
			if isSecureAssetPath(path) {
				plain, err = secureMaterialize(path, preferredExt)
				if err != nil {
					return ""
				}
			}
			fi, err := os.Stat(plain)
			if err != nil || fi.IsDir() {
				return ""
			}
			h := hashFile(plain)
			if h == "" {
				return ""
			}
			ext := strings.ToLower(preferredExt)
			if ext == "" {
				ext = strings.ToLower(filepath.Ext(plain))
			}
			if ext == "" {
				ext = ".bin"
			}
			entry := filepath.ToSlash(filepath.Join("assets", h[:2], h+ext))
			assets[entry] = plain
			return entry
		}
		pd.SourceAsset = add(a0.SourceCopy, "source", sourceExt)
		if pd.SourceAsset == "" {
			pd.SourceAsset = add(a0.SourceFile, "source", sourceExt)
		}
		pd.PreviewAsset = add(a0.PreviewPath, "preview", ".png")
		pd.CurveAsset = add(a0.CSVPath, "curve", ".csv")
		d.Dynamograms = append(d.Dynamograms, pd)
	}
	return d, assets
}

func writeRosaPackage(path, kind string, vs []VersionRecord, full bool) error {
	data, assets := packData(vs, full)
	dataBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	entries := map[string]RosaEntryMeta{}
	sum := sha256.Sum256(dataBytes)
	entries["data/package.json"] = RosaEntryMeta{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(dataBytes))}
	for entry, src := range assets {
		fi, err := os.Stat(src)
		if err != nil {
			return err
		}
		entries[entry] = RosaEntryMeta{SHA256: hashFile(src), Size: fi.Size()}
	}
	manifest := RosaManifest{
		Format: "ROSA_OFFLINE_EXCHANGE", SchemaVersion: 1, PackageID: packageID(kind, vs), PackageType: kind,
		CreatedAt: time.Now().Format(time.RFC3339), Product: "РОСА — Расчёт осложнений скважин: автоматизированная оценка",
		RecordCount: len(vs), ObjectCount: len(objectKeys(vs)), Entries: entries,
		Note: "Пакет создан локально. Импорт не заменяет текущий Техрежим получателя; расчёты добавляются как отдельные версии.",
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	zf, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(zf)
	writeBytes := func(name string, b []byte) error {
		w, e := zw.Create(name)
		if e != nil {
			return e
		}
		_, e = w.Write(b)
		return e
	}
	if err = writeBytes("manifest.json", manifestBytes); err == nil {
		err = writeBytes("data/package.json", dataBytes)
	}
	if err == nil {
		names := make([]string, 0, len(assets))
		for n := range assets {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, entry := range names {
			w, e := zw.Create(entry)
			if e != nil {
				err = e
				break
			}
			f, e := os.Open(assets[entry])
			if e != nil {
				err = e
				break
			}
			_, e = io.Copy(w, f)
			f.Close()
			if e != nil {
				err = e
				break
			}
		}
	}
	ce := zw.Close()
	fe := zf.Close()
	if err == nil {
		err = ce
	}
	if err == nil {
		err = fe
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return replaceFileAtomic(tmp, path)
}

func exportSelectedRosaPackage() {
	x := filteredVersions()
	i := selectedRow(historyList)
	if i < 0 || i >= len(x) {
		msg("Передать расчёт", "Выберите расчёт в истории.", MB_OK|MB_ICONWARNING)
		return
	}
	v := x[i]
	name := "РОСА_" + safeFilePart(v.Field) + "_" + safeFilePart(v.Well) + "_" + time.Now().Format("20060102") + ".rosa"
	path := chooseRosaSavePath(name)
	if path == "" {
		return
	}
	if err := writeRosaPackage(path, "calculation", []VersionRecord{v}, false); err != nil {
		msg("Передать расчёт", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Передать расчёт", "Пакет РОСА создан:\n"+path+"\n\nЕго можно открыть на другой офлайн-установке РОСА.", MB_OK|MB_ICONINFORMATION)
}

func exportArchiveRosaPackage() {
	path := chooseRosaSavePath("РОСА_архив_" + time.Now().Format("20060102_1504") + ".rosa")
	if path == "" {
		return
	}
	if err := writeRosaPackage(path, "archive", versions, true); err != nil {
		msg("Экспорт архива", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Экспорт архива", fmt.Sprintf("Архив РОСА создан.\nРасчётов: %d\nФайл: %s", len(versions), path), MB_OK|MB_ICONINFORMATION)
}

func safeFilePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "объект"
	}
	r := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	s = r.Replace(s)
	if len([]rune(s)) > 40 {
		s = string([]rune(s)[:40])
	}
	return s
}

func readZipJSON(z *zip.ReadCloser, name string, dst any, manifest RosaManifest) error {
	meta, ok := manifest.Entries[name]
	if !ok {
		return fmt.Errorf("в манифесте отсутствует %s", name)
	}
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		if f.UncompressedSize64 > 256<<20 {
			return fmt.Errorf("раздел %s слишком большой", name)
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		h := sha256.New()
		b, err := io.ReadAll(io.TeeReader(r, h))
		r.Close()
		if err != nil {
			return err
		}
		if int64(len(b)) != meta.Size || hex.EncodeToString(h.Sum(nil)) != meta.SHA256 {
			return fmt.Errorf("контрольная сумма %s не совпала", name)
		}
		return json.Unmarshal(b, dst)
	}
	return fmt.Errorf("раздел %s отсутствует", name)
}

func extractAsset(z *zip.ReadCloser, entry string, manifest RosaManifest) (string, error) {
	if entry == "" {
		return "", nil
	}
	meta, ok := manifest.Entries[entry]
	if !ok {
		return "", fmt.Errorf("нет описания вложения %s", entry)
	}
	var zf *zip.File
	for _, f := range z.File {
		if f.Name == entry {
			zf = f
			break
		}
	}
	if zf == nil {
		return "", fmt.Errorf("вложение %s отсутствует", entry)
	}
	if zf.UncompressedSize64 > 256<<20 {
		return "", fmt.Errorf("вложение %s превышает 256 МБ", entry)
	}
	td, err := ensureSessionTempDir()
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(entry)
	tmp := filepath.Join(td, "import-"+meta.SHA256+ext)
	r, err := zf.Open()
	if err != nil {
		return "", err
	}
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		r.Close()
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), r)
	if err == nil {
		err = out.Sync()
	}
	ce := out.Close()
	r.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if n != meta.Size || hex.EncodeToString(h.Sum(nil)) != meta.SHA256 {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("контрольная сумма вложения %s не совпала", entry)
	}
	stored, _, err := secureStoreFile(tmp)
	_ = os.Remove(tmp)
	if err != nil {
		return "", err
	}
	return stored, nil
}

func importRosaPackage(path string) (RosaImportReceipt, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return RosaImportReceipt{}, fmt.Errorf("не удалось открыть пакет: %w", err)
	}
	defer z.Close()
	var manifest RosaManifest
	found := false
	for _, f := range z.File {
		if f.Name == "manifest.json" {
			r, e := f.Open()
			if e != nil {
				return RosaImportReceipt{}, e
			}
			b, e := io.ReadAll(io.LimitReader(r, 2<<20))
			r.Close()
			if e != nil {
				return RosaImportReceipt{}, e
			}
			if e = json.Unmarshal(b, &manifest); e != nil {
				return RosaImportReceipt{}, fmt.Errorf("повреждён manifest.json: %w", e)
			}
			found = true
			break
		}
	}
	if !found || manifest.Format != "ROSA_OFFLINE_EXCHANGE" || manifest.SchemaVersion != 1 {
		return RosaImportReceipt{}, fmt.Errorf("файл не является совместимым пакетом РОСА")
	}
	var data RosaPackageData
	if err = readZipJSON(z, "data/package.json", &data, manifest); err != nil {
		return RosaImportReceipt{}, err
	}

	receipt := RosaImportReceipt{PackageID: manifest.PackageID, PackageType: manifest.PackageType, SourceFile: filepath.Base(path), ImportedAt: time.Now().Format(time.RFC3339)}
	have := map[string]bool{}
	for _, v := range versions {
		have[v.VersionID] = true
	}
	for id := range deletedVersionIDs {
		have[id] = true
	}
	for _, v := range data.Versions {
		if v.VersionID == "" || have[v.VersionID] {
			receipt.SkippedVersions++
			continue
		}
		v.ImportedAt = receipt.ImportedAt
		v = compactVersionPayload(v)
		if err = appendLedger(v); err != nil {
			return receipt, err
		}
		versions = append(versions, v)
		have[v.VersionID] = true
		receipt.AddedVersions++
	}
	sort.SliceStable(versions, func(i, j int) bool { return parseDate(versions[i].Created).After(parseDate(versions[j].Created)) })
	_ = saveJSON(filepath.Join(dataDir, "calculation_versions.json"), versions)

	knownOutcome := map[string]bool{}
	for _, o := range outcomes {
		knownOutcome[o.ID] = true
	}
	for _, o := range data.Outcomes {
		if o.ID != "" && !knownOutcome[o.ID] {
			outcomes = append(outcomes, o)
			knownOutcome[o.ID] = true
		}
	}
	_ = saveJSON(filepath.Join(dataDir, "outcomes.json"), outcomes)
	knownPlan := map[string]bool{}
	for _, p0 := range plans {
		knownPlan[p0.ID] = true
	}
	for _, p0 := range data.Plans {
		if p0.ID != "" && !knownPlan[p0.ID] {
			plans = append(plans, p0)
			knownPlan[p0.ID] = true
		}
	}
	_ = saveJSON(filepath.Join(dataDir, "plan.json"), plans)
	receipt.AddedLabs = mergeImportedLabRecords(data.Labs, receipt.ImportedAt)

	var recv []ReceivedTechState
	loadJSON(filepath.Join(dataDir, "exchange_tech_states.json"), &recv)
	recvKeys := map[string]bool{}
	for _, r := range recv {
		recvKeys[r.PackageID+"|"+r.State.ObjectKey] = true
	}
	for _, st := range data.CurrentTech {
		k := manifest.PackageID + "|" + st.ObjectKey
		if st.ObjectKey != "" && !recvKeys[k] {
			recv = append(recv, ReceivedTechState{PackageID: manifest.PackageID, ImportedAt: receipt.ImportedAt, State: st})
			recvKeys[k] = true
		}
	}
	_ = saveJSON(filepath.Join(dataDir, "exchange_tech_states.json"), recv)

	hist := loadDynamogramHistory()
	knownDyn := map[string]bool{}
	for _, a := range hist {
		knownDyn[a.ID] = true
	}
	for _, pd := range data.Dynamograms {
		if pd.Analysis.ID == "" || knownDyn[pd.Analysis.ID] {
			continue
		}
		a := pd.Analysis
		if a.SourceCopy, err = extractAsset(z, pd.SourceAsset, manifest); err != nil {
			return receipt, err
		}
		if a.PreviewPath, err = extractAsset(z, pd.PreviewAsset, manifest); err != nil {
			return receipt, err
		}
		if a.CSVPath, err = extractAsset(z, pd.CurveAsset, manifest); err != nil {
			return receipt, err
		}
		a.SourceFile = filepath.Base(a.SourceFile)
		hist = append(hist, a)
		knownDyn[a.ID] = true
		receipt.AddedDynamograms++
	}
	sort.SliceStable(hist, func(i, j int) bool { return parseDate(hist[i].Created).After(parseDate(hist[j].Created)) })
	_ = saveDynamogramHistory(hist)

	var receipts []RosaImportReceipt
	loadJSON(filepath.Join(dataDir, "rosa_imports.json"), &receipts)
	receipts = append(receipts, receipt)
	_ = saveJSON(filepath.Join(dataDir, "rosa_imports.json"), receipts)
	return receipt, nil
}

func importRosaPackageUI() {
	path := chooseRosaOpenPath()
	if path == "" {
		return
	}
	r, err := importRosaPackage(path)
	if err != nil {
		msg("Импорт пакета РОСА", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	refreshAll()
	msg("Импорт пакета РОСА", fmt.Sprintf("Пакет принят.\nДобавлено расчётов: %d\nУже имелось / удалено ранее: %d\nДобавлено лабораторных записей: %d\nДобавлено динамограмм: %d\n\nТекущий Техрежим не заменён.", r.AddedVersions, r.SkippedVersions, r.AddedLabs, r.AddedDynamograms), MB_OK|MB_ICONINFORMATION)
}

type LedgerTail struct {
	Sequence int    `json:"sequence"`
	Hash     string `json:"hash"`
	FileSize int64  `json:"file_size"`
}

func saveLedgerTail(sequence int, hash string) {
	fi, err := os.Stat(ledgerPath())
	if err != nil {
		return
	}
	_ = saveJSON(filepath.Join(dataDir, "ledger_tail.json"), LedgerTail{Sequence: sequence, Hash: hash, FileSize: fi.Size()})
}

func loadLedgerTail() (LedgerTail, bool) {
	var t LedgerTail
	loadJSON(filepath.Join(dataDir, "ledger_tail.json"), &t)
	if t.Sequence < 0 || t.FileSize < 0 {
		return LedgerTail{}, false
	}
	fi, err := os.Stat(ledgerPath())
	if err != nil {
		if os.IsNotExist(err) && t.Sequence == 0 {
			return t, true
		}
		return LedgerTail{}, false
	}
	if fi.Size() != t.FileSize {
		return LedgerTail{}, false
	}
	return t, true
}

func rebuildLedgerTail() (LedgerTail, error) {
	f, err := os.Open(ledgerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return LedgerTail{}, nil
		}
		return LedgerTail{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	prev := ""
	seq := 1
	count := 0
	for sc.Scan() {
		line, decErr := decryptPersonalLine(sc.Bytes())
		var le LedgerEntry
		if decErr != nil || json.Unmarshal(line, &le) != nil || le.Sequence != seq || le.PreviousHash != prev || le.Hash != computeLedgerHash(le.Sequence, le.PreviousHash, le.Record) {
			return LedgerTail{}, fmt.Errorf("нарушена цепочка журнала на записи %d", seq)
		}
		prev = le.Hash
		count = le.Sequence
		seq++
	}
	if err := sc.Err(); err != nil {
		return LedgerTail{}, err
	}
	fi, _ := os.Stat(ledgerPath())
	t := LedgerTail{Sequence: count, Hash: prev}
	if fi != nil {
		t.FileSize = fi.Size()
	}
	_ = saveJSON(filepath.Join(dataDir, "ledger_tail.json"), t)
	return t, nil
}

func compactVersionPayload(v VersionRecord) VersionRecord {
	if len(v.Raw) == 0 || v.VersionID == "" {
		return v
	}
	dir := filepath.Join(dataDir, "payloads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return v
	}
	sum := sha256.Sum256(v.Raw)
	h := hex.EncodeToString(sum[:])
	name := safeFilePart(v.VersionID) + ".json"
	path := filepath.Join(dir, name)
	validExisting := false
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		if plain, de := decryptPersonalBytes(b); de == nil {
			sum2 := sha256.Sum256(plain)
			validExisting = hex.EncodeToString(sum2[:]) == h
		}
	}
	if !validExisting {
		blob := v.Raw
		if personalPath(path) {
			if enc, err := encryptPersonalBytes(v.Raw); err == nil {
				blob = enc
			} else {
				return v
			}
		}
		if err := atomicWriteFile(path, blob, 0600); err != nil {
			return v
		}
	}
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		plain, de := decryptPersonalBytes(b)
		if de == nil {
			sum2 := sha256.Sum256(plain)
			if hex.EncodeToString(sum2[:]) == h {
				v.PayloadRef = filepath.ToSlash(filepath.Join("payloads", name))
				v.PayloadSHA256 = h
				v.Raw = nil
			}
		}
	}
	return v
}

func hydrateVersionPayload(v VersionRecord) VersionRecord {
	if len(v.Raw) > 0 || v.PayloadRef == "" {
		return v
	}
	path := filepath.Join(dataDir, filepath.FromSlash(v.PayloadRef))
	b, err := os.ReadFile(path)
	if err != nil {
		return v
	}
	b, err = decryptPersonalBytes(b)
	if err != nil {
		return v
	}
	if v.PayloadSHA256 != "" {
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != v.PayloadSHA256 {
			return v
		}
	}
	v.Raw = b
	return v
}
