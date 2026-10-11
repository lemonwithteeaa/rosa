//go:build windows

package main

import (
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
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

// RC184 introduces a repository boundary and a durable SQLite store without an
// external Go dependency. Windows 10/11 ship winsqlite3.dll; older/unsupported
// systems transparently keep the legacy JSONL path operational.

var (
	winSQLiteDLL        = syscall.NewLazyDLL("winsqlite3.dll")
	pSQLiteOpen16       = winSQLiteDLL.NewProc("sqlite3_open16")
	pSQLiteClose        = winSQLiteDLL.NewProc("sqlite3_close")
	pSQLiteExec         = winSQLiteDLL.NewProc("sqlite3_exec")
	pSQLitePrepare      = winSQLiteDLL.NewProc("sqlite3_prepare_v2")
	pSQLiteBindText     = winSQLiteDLL.NewProc("sqlite3_bind_text")
	pSQLiteBindBlob     = winSQLiteDLL.NewProc("sqlite3_bind_blob")
	pSQLiteBindInt64    = winSQLiteDLL.NewProc("sqlite3_bind_int64")
	pSQLiteStep         = winSQLiteDLL.NewProc("sqlite3_step")
	pSQLiteFinalize     = winSQLiteDLL.NewProc("sqlite3_finalize")
	pSQLiteColumnBlob   = winSQLiteDLL.NewProc("sqlite3_column_blob")
	pSQLiteColumnBytes  = winSQLiteDLL.NewProc("sqlite3_column_bytes")
	pSQLiteColumnText   = winSQLiteDLL.NewProc("sqlite3_column_text")
	pSQLiteColumnInt64  = winSQLiteDLL.NewProc("sqlite3_column_int64")
	pSQLiteErrmsg       = winSQLiteDLL.NewProc("sqlite3_errmsg")
	pSQLiteBackupInit   = winSQLiteDLL.NewProc("sqlite3_backup_init")
	pSQLiteBackupStep   = winSQLiteDLL.NewProc("sqlite3_backup_step")
	pSQLiteBackupFinish = winSQLiteDLL.NewProc("sqlite3_backup_finish")
)

const (
	sqliteOK   = 0
	sqliteRow  = 100
	sqliteDone = 101
)

type rosaSQLiteStore struct {
	db      uintptr
	path    string
	writeMu sync.Mutex // RC189: single-writer gate for every mutating transaction.
}

var persistentStore *rosaSQLiteStore
var persistentStorageSafeModeReason string
var persistentStorageLastIntegrity string

func persistentDatabaseDir() string {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("APPDATA"))
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "ROSA", "Data")
}
func persistentDatabasePath() string { return filepath.Join(persistentDatabaseDir(), "rosa.db") }
func persistentBackupDir() string    { return filepath.Join(persistentDatabaseDir(), "Backup") }

func sqliteUTF8(s string) ([]byte, uintptr) {
	b := append([]byte(s), 0)
	return b, uintptr(unsafe.Pointer(&b[0]))
}
func sqliteQuote(s string) string       { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func sqliteBlobLiteral(b []byte) string { return "X'" + strings.ToUpper(hex.EncodeToString(b)) + "'" }

func (s *rosaSQLiteStore) errorText() string {
	if s == nil || s.db == 0 {
		return "SQLite недоступен"
	}
	p, _, _ := pSQLiteErrmsg.Call(s.db)
	if p == 0 {
		return "неизвестная ошибка SQLite"
	}
	raw := (*[1 << 20]byte)(unsafe.Pointer(p))
	n := 0
	for n < len(raw) && raw[n] != 0 {
		n++
	}
	return string(raw[:n])
}
func (s *rosaSQLiteStore) exec(sql string) error {
	if s == nil || s.db == 0 {
		return fmt.Errorf("SQLite не открыт")
	}
	b, p := sqliteUTF8(sql)
	rc, _, _ := pSQLiteExec.Call(s.db, p, 0, 0, 0)
	_ = b
	if int(rc) != sqliteOK {
		return fmt.Errorf("SQLite: %s", s.errorText())
	}
	return nil
}

type sqliteBindValue struct {
	kind int
	text string
	blob []byte
	i64  int64
}

const (
	sqliteBindTextKind = iota + 1
	sqliteBindBlobKind
	sqliteBindInt64Kind
)

func sqliteText(v string) sqliteBindValue { return sqliteBindValue{kind: sqliteBindTextKind, text: v} }
func sqliteBlob(v []byte) sqliteBindValue { return sqliteBindValue{kind: sqliteBindBlobKind, blob: v} }
func sqliteInt64(v int64) sqliteBindValue { return sqliteBindValue{kind: sqliteBindInt64Kind, i64: v} }

func (s *rosaSQLiteStore) prepareBound(sql string, values ...sqliteBindValue) (uintptr, error) {
	if s == nil || s.db == 0 {
		return 0, fmt.Errorf("SQLite не открыт")
	}
	b, psql := sqliteUTF8(sql)
	var stmt uintptr
	rc, _, _ := pSQLitePrepare.Call(s.db, psql, ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	_ = b
	if int(rc) != sqliteOK {
		return 0, fmt.Errorf("SQLite prepare: %s", s.errorText())
	}
	ok := false
	defer func() {
		if !ok && stmt != 0 {
			pSQLiteFinalize.Call(stmt)
		}
	}()
	for i, v := range values {
		idx := uintptr(i + 1)
		switch v.kind {
		case sqliteBindTextKind:
			buf, ptr := sqliteUTF8(v.text)
			rc, _, _ = pSQLiteBindText.Call(stmt, idx, ptr, uintptr(len(buf)-1), ^uintptr(0)) // SQLITE_TRANSIENT
			_ = buf
		case sqliteBindBlobKind:
			if len(v.blob) == 0 {
				rc, _, _ = pSQLiteBindBlob.Call(stmt, idx, 0, 0, ^uintptr(0))
			} else {
				rc, _, _ = pSQLiteBindBlob.Call(stmt, idx, uintptr(unsafe.Pointer(&v.blob[0])), uintptr(len(v.blob)), ^uintptr(0))
			}
		case sqliteBindInt64Kind:
			rc, _, _ = pSQLiteBindInt64.Call(stmt, idx, uintptr(v.i64))
		default:
			pSQLiteFinalize.Call(stmt)
			stmt = 0
			return 0, fmt.Errorf("неподдерживаемый тип SQLite bind")
		}
		if int(rc) != sqliteOK {
			return 0, fmt.Errorf("SQLite bind #%d: %s", i+1, s.errorText())
		}
	}
	ok = true
	return stmt, nil
}

func (s *rosaSQLiteStore) execBound(sql string, values ...sqliteBindValue) error {
	stmt, err := s.prepareBound(sql, values...)
	if err != nil {
		return err
	}
	defer pSQLiteFinalize.Call(stmt)
	rc, _, _ := pSQLiteStep.Call(stmt)
	if int(rc) != sqliteDone {
		return fmt.Errorf("SQLite step: %s", s.errorText())
	}
	return nil
}

func (s *rosaSQLiteStore) withWriteTransaction(fn func() error) error {
	if s == nil || s.db == 0 {
		return fmt.Errorf("SQLite не открыт")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.exec("BEGIN IMMEDIATE;"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.exec("ROLLBACK;")
		}
	}()
	if err := fn(); err != nil {
		return err
	}
	if err := s.exec("COMMIT;"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *rosaSQLiteStore) queryFirstText(sql string) (string, bool, error) {
	if s == nil || s.db == 0 {
		return "", false, fmt.Errorf("SQLite не открыт")
	}
	b, p := sqliteUTF8(sql)
	var stmt uintptr
	rc, _, _ := pSQLitePrepare.Call(s.db, p, ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	_ = b
	if int(rc) != sqliteOK {
		return "", false, fmt.Errorf("SQLite prepare: %s", s.errorText())
	}
	defer pSQLiteFinalize.Call(stmt)
	rc, _, _ = pSQLiteStep.Call(stmt)
	if int(rc) == sqliteDone {
		return "", false, nil
	}
	if int(rc) != sqliteRow {
		return "", false, fmt.Errorf("SQLite step: %s", s.errorText())
	}
	ptr, _, _ := pSQLiteColumnText.Call(stmt, 0)
	if ptr == 0 {
		return "", true, nil
	}
	n, _, _ := pSQLiteColumnBytes.Call(stmt, 0)
	raw := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(n))
	return string(raw), true, nil
}

func (s *rosaSQLiteStore) queryFirstInt64(sql string) (int64, bool, error) {
	if s == nil || s.db == 0 {
		return 0, false, fmt.Errorf("SQLite не открыт")
	}
	b, p := sqliteUTF8(sql)
	var stmt uintptr
	rc, _, _ := pSQLitePrepare.Call(s.db, p, ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	_ = b
	if int(rc) != sqliteOK {
		return 0, false, fmt.Errorf("SQLite prepare: %s", s.errorText())
	}
	defer pSQLiteFinalize.Call(stmt)
	rc, _, _ = pSQLiteStep.Call(stmt)
	if int(rc) == sqliteDone {
		return 0, false, nil
	}
	if int(rc) != sqliteRow {
		return 0, false, fmt.Errorf("SQLite step: %s", s.errorText())
	}
	v, _, _ := pSQLiteColumnInt64.Call(stmt, 0)
	return int64(v), true, nil
}

func (s *rosaSQLiteStore) integrityCheck() (string, error) {
	value, ok, err := s.queryFirstText("PRAGMA integrity_check;")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("SQLite integrity_check не вернул результат")
	}
	persistentStorageLastIntegrity = strings.TrimSpace(value)
	if !strings.EqualFold(strings.TrimSpace(value), "ok") {
		return value, fmt.Errorf("SQLite integrity_check: %s", value)
	}
	return value, nil
}

func (s *rosaSQLiteStore) quickCheck() (string, error) {
	value, ok, err := s.queryFirstText("PRAGMA quick_check;")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("SQLite quick_check не вернул результат")
	}
	if !strings.EqualFold(strings.TrimSpace(value), "ok") {
		return value, fmt.Errorf("SQLite quick_check: %s", value)
	}
	return value, nil
}

func verifySQLiteFile(path string) error {
	wp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var db uintptr
	rc, _, _ := pSQLiteOpen16.Call(uintptr(unsafe.Pointer(wp)), uintptr(unsafe.Pointer(&db)))
	if int(rc) != sqliteOK || db == 0 {
		return fmt.Errorf("не удалось открыть копию SQLite, код %d", rc)
	}
	st := &rosaSQLiteStore{db: db, path: path}
	defer st.close()
	_, err = st.integrityCheck()
	return err
}

func openPersistentStore() (*rosaSQLiteStore, error) {
	if err := os.MkdirAll(persistentDatabaseDir(), 0700); err != nil {
		return nil, err
	}
	if err := winSQLiteDLL.Load(); err != nil {
		return nil, fmt.Errorf("winsqlite3.dll недоступна: %w", err)
	}
	path := persistentDatabasePath()
	_, statErr := os.Stat(path)
	existed := statErr == nil
	wp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var db uintptr
	rc, _, _ := pSQLiteOpen16.Call(uintptr(unsafe.Pointer(wp)), uintptr(unsafe.Pointer(&db)))
	if int(rc) != sqliteOK || db == 0 {
		return nil, fmt.Errorf("не удалось открыть SQLite, код %d", rc)
	}
	st := &rosaSQLiteStore{db: db, path: path}
	preMigration := ""
	if existed {
		if bkp, backupErr := preparePreMigrationBackup(st); backupErr != nil {
			st.close()
			return nil, fmt.Errorf("не удалось создать проверенную копию перед миграцией схемы: %w", backupErr)
		} else {
			preMigration = bkp
		}
	}
	if err := st.initSchema(); err != nil {
		st.close()
		if existed && preMigration != "" {
			_ = os.Remove(path)
			if rollbackErr := copySQLiteFileSync(preMigration, path); rollbackErr != nil {
				return nil, fmt.Errorf("миграция схемы не выполнена (%v), и аварийный rollback также не удался (%v)", err, rollbackErr)
			}
			return nil, fmt.Errorf("миграция схемы не выполнена; предыдущая БД восстановлена из %s: %w", filepath.Base(preMigration), err)
		}
		if existed {
			return nil, fmt.Errorf("существующая база не прошла инициализацию схемы; пустая база не создавалась: %w", err)
		}
		return nil, err
	}
	if _, err := st.quickCheck(); err != nil {
		st.close()
		if existed {
			return nil, fmt.Errorf("существующая база повреждена; автоматическая замена отключена: %w", err)
		}
		return nil, err
	}
	if appID, ok, err := st.queryFirstInt64("PRAGMA application_id;"); err != nil || !ok || appID != rosaSQLiteApplicationID {
		st.close()
		return nil, fmt.Errorf("файл SQLite не идентифицирован как база РОСА: application_id=%d, err=%v", appID, err)
	}
	return st, nil
}
func (s *rosaSQLiteStore) close() {
	if s != nil && s.db != 0 {
		pSQLiteClose.Call(s.db)
		s.db = 0
	}
}

func (s *rosaSQLiteStore) initSchema() error {
	bootstrap := `PRAGMA journal_mode=WAL;
PRAGMA synchronous=FULL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;
PRAGMA trusted_schema=OFF;
PRAGMA cell_size_check=ON;
PRAGMA mmap_size=0;
CREATE TABLE IF NOT EXISTS rosa_meta(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS schema_migrations(
 version INTEGER PRIMARY KEY,
 applied_at TEXT NOT NULL,
 description TEXT NOT NULL
);`
	if err := s.exec(bootstrap); err != nil {
		return err
	}
	version := int64(0)
	if value, ok, err := s.queryFirstText("SELECT value FROM rosa_meta WHERE key='schema_version' LIMIT 1;"); err != nil {
		return err
	} else if ok && strings.TrimSpace(value) != "" {
		fmt.Sscanf(strings.TrimSpace(value), "%d", &version)
	}
	if version < 1 {
		migration := `BEGIN IMMEDIATE;
CREATE TABLE IF NOT EXISTS lab_pass_revisions(
 revision_id INTEGER PRIMARY KEY AUTOINCREMENT,
 pass_id TEXT NOT NULL,
 object_key TEXT NOT NULL,
 normalized_object_key TEXT NOT NULL,
 analysis_date TEXT,
 source TEXT,
 status TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 owner_user_id TEXT,
 payload_hash TEXT NOT NULL,
 payload BLOB NOT NULL,
 stored_at TEXT NOT NULL DEFAULT (datetime('now')),
 UNIQUE(pass_id,payload_hash)
);
CREATE INDEX IF NOT EXISTS ix_lab_pass_object ON lab_pass_revisions(normalized_object_key,analysis_date);
CREATE INDEX IF NOT EXISTS ix_lab_pass_passid ON lab_pass_revisions(pass_id,revision_id);
CREATE TABLE IF NOT EXISTS migration_log(
 source_path TEXT PRIMARY KEY,
 source_size INTEGER NOT NULL,
 source_mtime INTEGER NOT NULL,
 migrated_at TEXT NOT NULL,
 record_count INTEGER NOT NULL,
 source_hash TEXT NOT NULL
);
INSERT OR REPLACE INTO rosa_meta(key,value) VALUES('schema_version','1');
INSERT OR IGNORE INTO schema_migrations(version,applied_at,description) VALUES(1,datetime('now'),'Базовое долговременное хранилище лабораторных диагностик');
COMMIT;`
		if err := s.exec(migration); err != nil {
			_ = s.exec("ROLLBACK;")
			return err
		}
		version = 1
	}
	if version < 2 {
		migration := `BEGIN IMMEDIATE;
CREATE INDEX IF NOT EXISTS ix_lab_pass_saved_latest ON lab_pass_revisions(status,normalized_object_key,analysis_date,updated_at,revision_id);
CREATE INDEX IF NOT EXISTS ix_lab_pass_updated ON lab_pass_revisions(updated_at,revision_id);
INSERT OR REPLACE INTO rosa_meta(key,value) VALUES('schema_version','2');
INSERT OR IGNORE INTO schema_migrations(version,applied_at,description) VALUES(2,datetime('now'),'Индексы Repository для истории и массовой аналитики');
COMMIT;`
		if err := s.exec(migration); err != nil {
			_ = s.exec("ROLLBACK;")
			return err
		}
		version = 2
	}
	if version < 3 {
		migration := `BEGIN IMMEDIATE;
CREATE TABLE IF NOT EXISTS storage_events(
 event_id INTEGER PRIMARY KEY AUTOINCREMENT,
 event_time TEXT NOT NULL,
 event_type TEXT NOT NULL,
 result TEXT
 NOT NULL,
 details TEXT
);
CREATE INDEX IF NOT EXISTS ix_storage_events_time ON storage_events(event_time DESC,event_id DESC);
INSERT OR REPLACE INTO rosa_meta(key,value) VALUES('schema_version','3');
INSERT OR IGNORE INTO schema_migrations(version,applied_at,description) VALUES(3,datetime('now'),'Журнал backup/restore и контроль перехода от JSONL');
COMMIT;`
		if err := s.exec(migration); err != nil {
			_ = s.exec("ROLLBACK;")
			return err
		}
		version = 3
	}
	if version < 4 {
		migration := `BEGIN IMMEDIATE;
PRAGMA application_id=1380930369;
PRAGMA user_version=4;
INSERT OR REPLACE INTO rosa_meta(key,value) VALUES('schema_version','4');
INSERT OR IGNORE INTO schema_migrations(version,applied_at,description) VALUES(4,datetime('now'),'Усиленный профиль SQLite: идентификатор приложения и согласование версии схемы');
COMMIT;`
		if err := s.exec(migration); err != nil {
			_ = s.exec("ROLLBACK;")
			return err
		}
		version = 4
	}
	if version < 5 {
		migration := `BEGIN IMMEDIATE;
PRAGMA user_version=5;
INSERT OR REPLACE INTO rosa_meta(key,value) VALUES('schema_version','5');
INSERT OR IGNORE INTO schema_migrations(version,applied_at,description) VALUES(5,datetime('now'),'RC189: single-writer и parameter binding для пользовательских данных');
COMMIT;`
		if err := s.exec(migration); err != nil {
			_ = s.exec("ROLLBACK;")
			return err
		}
	}
	return nil
}

func canonicalLabPassBytes(pass LabPassRecord) ([]byte, string, error) {
	b, err := json.Marshal(pass)
	if err != nil {
		return nil, "", err
	}
	h := sha256.Sum256(b)
	return b, hex.EncodeToString(h[:]), nil
}
func currentStorageUserID() string {
	if currentSession == nil {
		return ""
	}
	return strings.TrimSpace(currentSession.UserID)
}
func (s *rosaSQLiteStore) insertLabPass(pass LabPassRecord) error {
	payload, hash, err := canonicalLabPassBytes(pass)
	if err != nil {
		return err
	}
	const q = `INSERT OR IGNORE INTO lab_pass_revisions(pass_id,object_key,normalized_object_key,analysis_date,source,status,updated_at,owner_user_id,payload_hash,payload) VALUES(?,?,?,?,?,?,?,?,?,?);`
	return s.withWriteTransaction(func() error {
		return s.execBound(q,
			sqliteText(pass.PassID), sqliteText(pass.ObjectKey), sqliteText(normalizeLaboratoryObjectKey(pass.ObjectKey)),
			sqliteText(pass.Date), sqliteText(pass.Source), sqliteText(pass.Status), sqliteText(pass.UpdatedAt),
			sqliteText(currentStorageUserID()), sqliteText(hash), sqliteBlob(payload))
	})
}

func (s *rosaSQLiteStore) insertLabPassBatch(passes []LabPassRecord) error {
	if len(passes) == 0 {
		return nil
	}
	const q = `INSERT OR IGNORE INTO lab_pass_revisions(pass_id,object_key,normalized_object_key,analysis_date,source,status,updated_at,owner_user_id,payload_hash,payload) VALUES(?,?,?,?,?,?,?,?,?,?);`
	return s.withWriteTransaction(func() error {
		for _, pass := range passes {
			payload, hash, err := canonicalLabPassBytes(pass)
			if err != nil {
				return err
			}
			if err = s.execBound(q,
				sqliteText(pass.PassID), sqliteText(pass.ObjectKey), sqliteText(normalizeLaboratoryObjectKey(pass.ObjectKey)),
				sqliteText(pass.Date), sqliteText(pass.Source), sqliteText(pass.Status), sqliteText(pass.UpdatedAt),
				sqliteText(currentStorageUserID()), sqliteText(hash), sqliteBlob(payload)); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *rosaSQLiteStore) queryLabPassPayloads() ([][]byte, error) {
	sql := "SELECT payload FROM lab_pass_revisions ORDER BY revision_id ASC;"
	b, p := sqliteUTF8(sql)
	var stmt uintptr
	rc, _, _ := pSQLitePrepare.Call(s.db, p, ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	_ = b
	if int(rc) != sqliteOK {
		return nil, fmt.Errorf("SQLite prepare: %s", s.errorText())
	}
	defer pSQLiteFinalize.Call(stmt)
	var rows [][]byte
	for {
		rc, _, _ = pSQLiteStep.Call(stmt)
		if int(rc) == sqliteDone {
			break
		}
		if int(rc) != sqliteRow {
			return nil, fmt.Errorf("SQLite step: %s", s.errorText())
		}
		ptr, _, _ := pSQLiteColumnBlob.Call(stmt, 0)
		n, _, _ := pSQLiteColumnBytes.Call(stmt, 0)
		if ptr == 0 || n == 0 {
			continue
		}
		src := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(n))
		cp := append([]byte(nil), src...)
		rows = append(rows, cp)
	}
	return rows, nil
}
func (s *rosaSQLiteStore) loadLabPasses() ([]LabPassRecord, error) {
	payloads, err := s.queryLabPassPayloads()
	if err != nil {
		return nil, err
	}
	latest := map[string]LabPassRecord{}
	order := map[string]int{}
	for i, b := range payloads {
		var pass LabPassRecord
		if json.Unmarshal(b, &pass) != nil || pass.PassID == "" || pass.ObjectKey == "" {
			continue
		}
		latest[pass.PassID] = pass
		order[pass.PassID] = i
	}
	out := make([]LabPassRecord, 0, len(latest))
	for _, p := range latest {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return order[out[i].PassID] > order[out[j].PassID]
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func decodeLabPassPayloads(payloads [][]byte, latestPerPass bool) []LabPassRecord {
	if !latestPerPass {
		out := make([]LabPassRecord, 0, len(payloads))
		for _, b := range payloads {
			var pass LabPassRecord
			if json.Unmarshal(b, &pass) == nil && pass.PassID != "" && pass.ObjectKey != "" {
				out = append(out, pass)
			}
		}
		return out
	}
	latest := map[string]LabPassRecord{}
	order := map[string]int{}
	for i, b := range payloads {
		var pass LabPassRecord
		if json.Unmarshal(b, &pass) != nil || pass.PassID == "" || pass.ObjectKey == "" {
			continue
		}
		latest[pass.PassID] = pass
		order[pass.PassID] = i
	}
	out := make([]LabPassRecord, 0, len(latest))
	for _, p := range latest {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return order[out[i].PassID] > order[out[j].PassID]
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

func (s *rosaSQLiteStore) queryLabPassPayloadsSQL(sql string, values ...sqliteBindValue) ([][]byte, error) {
	stmt, err := s.prepareBound(sql, values...)
	if err != nil {
		return nil, err
	}
	defer pSQLiteFinalize.Call(stmt)
	var rows [][]byte
	for {
		rc, _, _ := pSQLiteStep.Call(stmt)
		if int(rc) == sqliteDone {
			break
		}
		if int(rc) != sqliteRow {
			return nil, fmt.Errorf("SQLite step: %s", s.errorText())
		}
		ptr, _, _ := pSQLiteColumnBlob.Call(stmt, 0)
		n, _, _ := pSQLiteColumnBytes.Call(stmt, 0)
		if ptr == 0 || n == 0 {
			continue
		}
		src := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(n))
		rows = append(rows, append([]byte(nil), src...))
	}
	return rows, nil
}
func (s *rosaSQLiteStore) loadLabPassesForObject(objectKey string) ([]LabPassRecord, error) {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return nil, nil
	}
	const q = `SELECT payload FROM lab_pass_revisions WHERE normalized_object_key=? ORDER BY analysis_date ASC, updated_at ASC, revision_id ASC;`
	rows, err := s.queryLabPassPayloadsSQL(q, sqliteText(key))
	if err != nil {
		return nil, err
	}
	return decodeLabPassPayloads(rows, true), nil
}

func (s *rosaSQLiteStore) loadLatestSavedLabPasses() ([]LabPassRecord, error) {
	q := `SELECT r.payload
FROM lab_pass_revisions r
JOIN (
 SELECT normalized_object_key, MAX(revision_id) AS max_revision
 FROM lab_pass_revisions
 WHERE status='saved'
 GROUP BY normalized_object_key
) x ON x.max_revision=r.revision_id
ORDER BY r.normalized_object_key ASC;`
	rows, err := s.queryLabPassPayloadsSQL(q)
	if err != nil {
		return nil, err
	}
	return decodeLabPassPayloads(rows, false), nil
}

func storageFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (s *rosaSQLiteStore) migrationAlreadyDone(path string, size, mtime int64, hash string) bool {
	stmt, err := s.prepareBound(`SELECT COUNT(*) FROM migration_log WHERE source_path=? AND source_size=? AND source_mtime=? AND source_hash=?;`, sqliteText(path), sqliteInt64(size), sqliteInt64(mtime), sqliteText(hash))
	if err != nil {
		return false
	}
	defer pSQLiteFinalize.Call(stmt)
	rc, _, _ := pSQLiteStep.Call(stmt)
	if int(rc) != sqliteRow {
		return false
	}
	count, _, _ := pSQLiteColumnInt64.Call(stmt, 0)
	return int64(count) > 0
}
func (s *rosaSQLiteStore) migrateLegacyLabPassJSONL(path string) (int, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	sourceHash, err := storageFileSHA256(path)
	if err != nil {
		return 0, err
	}
	if s.migrationAlreadyDone(path, fi.Size(), fi.ModTime().UnixNano(), sourceHash) {
		return 0, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	count := 0
	err = s.withWriteTransaction(func() error {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
		const insertQ = `INSERT OR IGNORE INTO lab_pass_revisions(pass_id,object_key,normalized_object_key,analysis_date,source,status,updated_at,owner_user_id,payload_hash,payload) VALUES(?,?,?,?,?,?,?,?,?,?);`
		for sc.Scan() {
			line, decErr := decryptPersonalLine(sc.Bytes())
			var pass LabPassRecord
			if decErr != nil || json.Unmarshal(line, &pass) != nil || pass.PassID == "" || pass.ObjectKey == "" {
				continue
			}
			payload, hash, e := canonicalLabPassBytes(pass)
			if e != nil {
				continue
			}
			if e = s.execBound(insertQ,
				sqliteText(pass.PassID), sqliteText(pass.ObjectKey), sqliteText(normalizeLaboratoryObjectKey(pass.ObjectKey)),
				sqliteText(pass.Date), sqliteText(pass.Source), sqliteText(pass.Status), sqliteText(pass.UpdatedAt),
				sqliteText(currentStorageUserID()), sqliteText(hash), sqliteBlob(payload)); e != nil {
				return e
			}
			count++
		}
		if e := sc.Err(); e != nil {
			return e
		}
		return s.execBound(`INSERT OR REPLACE INTO migration_log(source_path,source_size,source_mtime,migrated_at,record_count,source_hash) VALUES(?,?,?,?,?,?);`,
			sqliteText(path), sqliteInt64(fi.Size()), sqliteInt64(fi.ModTime().UnixNano()), sqliteText(time.Now().UTC().Format(time.RFC3339)), sqliteInt64(int64(count)), sqliteText(sourceHash))
	})
	return count, err
}
func initPersistentStorage() {
	persistentStorageSafeModeReason = ""
	st, err := openPersistentStore()
	if err != nil {
		persistentStore = nil
		persistentStorageSafeModeReason = err.Error()
		diagLog("STORAGE safe mode: sqlite unavailable, legacy JSONL active: %v", err)
		return
	}
	persistentStore = st
	if result, err := st.integrityCheck(); err != nil {
		persistentStorageSafeModeReason = err.Error()
		diagLog("STORAGE integrity error: %v", err)
		st.close()
		persistentStore = nil
		return
	} else {
		persistentStorageLastIntegrity = result
	}
	n, err := st.migrateLegacyLabPassJSONL(labPassPath())
	if err != nil {
		diagLog("STORAGE migration error: %v", err)
	} else if n > 0 {
		diagLog("STORAGE migration scanned=%d source=%s", n, labPassPath())
	}
	if err := createPersistentDatabaseBackup(false); err != nil {
		diagLog("STORAGE backup warning: %v", err)
	}
}

func persistentStorageStatusText() string {
	if persistentStore == nil {
		if strings.TrimSpace(persistentStorageSafeModeReason) != "" {
			return "БЕЗОПАСНЫЙ РЕЖИМ: SQLite отключён; JSONL активен. Причина: " + persistentStorageSafeModeReason
		}
		return "SQLite не активен; используется JSONL"
	}
	result := strings.TrimSpace(persistentStorageLastIntegrity)
	if result == "" {
		result = "не проверено"
	}
	return fmt.Sprintf("SQLite: %s; integrity_check=%s; backups: %s; %s", persistentStore.path, result, persistentBackupSummary(), legacyJSONLMirrorStatus())
}

type persistentBackupInfo struct {
	Name     string
	Path     string
	Size     int64
	Modified time.Time
	Kind     string
}

func persistentBackupKind(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(lower, "rosa_manual_"):
		return "manual"
	case strings.HasPrefix(lower, "rosa_monthly_"):
		return "monthly"
	case strings.HasPrefix(lower, "rosa_") && strings.HasSuffix(lower, ".db"):
		return "daily"
	default:
		return "other"
	}
}

func listPersistentBackups() []persistentBackupInfo {
	entries, err := os.ReadDir(persistentBackupDir())
	if err != nil {
		return nil
	}
	out := make([]persistentBackupInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".db") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, persistentBackupInfo{Name: e.Name(), Path: filepath.Join(persistentBackupDir(), e.Name()), Size: fi.Size(), Modified: fi.ModTime(), Kind: persistentBackupKind(e.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out
}

func persistentBackupSummary() string {
	items := listPersistentBackups()
	if len(items) == 0 {
		return "копий нет"
	}
	var total int64
	daily, monthly, manual := 0, 0, 0
	for _, it := range items {
		total += it.Size
		switch it.Kind {
		case "daily":
			daily++
		case "monthly":
			monthly++
		case "manual":
			manual++
		}
	}
	return fmt.Sprintf("%d копий (%d ежедневных, %d месячных, %d ручных), %s; последняя %s", len(items), daily, monthly, manual, humanBytes(total), items[0].Modified.Format("02.01.2006 15:04"))
}

type storageEventRecord struct {
	Time    string
	Type    string
	Result  string
	Details string
}

func (s *rosaSQLiteStore) recentStorageEvents(limit int) ([]storageEventRecord, error) {
	if s == nil || s.db == 0 {
		return nil, fmt.Errorf("SQLite-хранилище не активно")
	}
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	q := fmt.Sprintf(`SELECT event_time,event_type,result,COALESCE(details,'') FROM storage_events ORDER BY event_id DESC LIMIT %d;`, limit)
	b, psql := sqliteUTF8(q)
	_ = b
	var stmt uintptr
	rc, _, _ := pSQLitePrepare.Call(s.db, psql, ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	if int(rc) != sqliteOK {
		return nil, fmt.Errorf("SQLite prepare журнала: %s", s.errorText())
	}
	defer pSQLiteFinalize.Call(stmt)
	out := make([]storageEventRecord, 0, limit)
	for {
		rc, _, _ = pSQLiteStep.Call(stmt)
		if int(rc) == sqliteDone {
			break
		}
		if int(rc) != sqliteRow {
			return nil, fmt.Errorf("SQLite чтение журнала: %s", s.errorText())
		}
		col := func(i uintptr) string {
			p, _, _ := pSQLiteColumnText.Call(stmt, i)
			if p == 0 {
				return ""
			}
			n, _, _ := pSQLiteColumnBytes.Call(stmt, i)
			if n == 0 {
				return ""
			}
			return string(unsafe.Slice((*byte)(unsafe.Pointer(p)), int(n)))
		}
		out = append(out, storageEventRecord{Time: col(0), Type: col(1), Result: col(2), Details: col(3)})
	}
	return out, nil
}

func storageRecentEventsText(limit int) string {
	if persistentStore == nil {
		return "SQLite-хранилище не активно.\r\n" + persistentStorageStatusText()
	}
	rows, err := persistentStore.recentStorageEvents(limit)
	if err != nil {
		return "Не удалось прочитать журнал операций: " + err.Error()
	}
	if len(rows) == 0 {
		return "Журнал операций с хранилищем пока пуст."
	}
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "%s | %s | %s", row.Time, row.Type, row.Result)
		if strings.TrimSpace(row.Details) != "" {
			fmt.Fprintf(&b, " | %s", row.Details)
		}
		b.WriteString("\r\n")
	}
	return strings.TrimSpace(b.String())
}

func verifyAllPersistentBackups() (int, int, []string) {
	items := listPersistentBackups()
	okN, badN := 0, 0
	problems := make([]string, 0)
	for _, it := range items {
		if err := verifySQLiteFile(it.Path); err != nil {
			badN++
			problems = append(problems, filepath.Base(it.Path)+": "+err.Error())
		} else {
			okN++
		}
	}
	if persistentStore != nil {
		result := "ok"
		details := fmt.Sprintf("проверено=%d; исправных=%d; поврежденных=%d", len(items), okN, badN)
		if badN > 0 {
			result = "warning"
		}
		persistentStore.recordStorageEvent("backup_verify", result, details)
	}
	return okN, badN, problems
}

func (s *rosaSQLiteStore) onlineBackupTo(dst string) error {
	if s == nil || s.db == 0 {
		return fmt.Errorf("SQLite не открыт")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	_ = os.Remove(tmp)
	wp, err := syscall.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	var dest uintptr
	rc, _, _ := pSQLiteOpen16.Call(uintptr(unsafe.Pointer(wp)), uintptr(unsafe.Pointer(&dest)))
	if int(rc) != sqliteOK || dest == 0 {
		return fmt.Errorf("не удалось открыть файл назначения SQLite backup, код %d", rc)
	}
	dstStore := &rosaSQLiteStore{db: dest, path: tmp}
	defer dstStore.close()
	mainName, mainPtr := sqliteUTF8("main")
	bkp, _, _ := pSQLiteBackupInit.Call(dest, mainPtr, s.db, mainPtr)
	_ = mainName
	if bkp == 0 {
		return fmt.Errorf("SQLite backup_init: %s", dstStore.errorText())
	}
	stepRC, _, _ := pSQLiteBackupStep.Call(bkp, uintptr(^uint32(0)))
	finishRC, _, _ := pSQLiteBackupFinish.Call(bkp)
	if int(stepRC) != sqliteDone || int(finishRC) != sqliteOK {
		_ = os.Remove(tmp)
		return fmt.Errorf("SQLite online backup не завершён: step=%d finish=%d", stepRC, finishRC)
	}
	dstStore.close()
	if err := verifySQLiteFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("копия SQLite не прошла integrity_check: %w", err)
	}
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func copySQLiteFileSync(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if cpErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if cpErr != nil {
			return cpErr
		}
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	return nil
}

func (s *rosaSQLiteStore) schemaVersion() int64 {
	if s == nil || s.db == 0 {
		return 0
	}
	value, ok, err := s.queryFirstText("SELECT value FROM rosa_meta WHERE key='schema_version' LIMIT 1;")
	if err != nil || !ok {
		return 0
	}
	var v int64
	fmt.Sscanf(strings.TrimSpace(value), "%d", &v)
	return v
}

const (
	persistentSchemaTarget  int64 = 5
	rosaSQLiteApplicationID int64 = 1380930369 // ASCII "ROSA"
)

func preparePreMigrationBackup(st *rosaSQLiteStore) (string, error) {
	if st == nil || st.db == 0 || st.schemaVersion() >= persistentSchemaTarget {
		return "", nil
	}
	if err := os.MkdirAll(persistentBackupDir(), 0700); err != nil {
		return "", err
	}
	dst := filepath.Join(persistentBackupDir(), "rosa_pre_migration_"+time.Now().Format("20060102_150405")+".db")
	if err := st.onlineBackupTo(dst); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *rosaSQLiteStore) recordStorageEvent(eventType, result, details string) {
	if s == nil || s.db == 0 {
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.execBound(`INSERT INTO storage_events(event_time,event_type,result,details) VALUES(?,?,?,?);`,
		sqliteText(time.Now().UTC().Format(time.RFC3339)), sqliteText(eventType), sqliteText(result), sqliteText(details))
}
func legacyJSONLMirrorReadyToDisable() bool {
	if persistentStore == nil || persistentStore.db == 0 {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(persistentStorageLastIntegrity), "ok") {
		return false
	}
	verifiedBackups := 0
	for _, it := range listPersistentBackups() {
		if it.Kind == "daily" || it.Kind == "monthly" || it.Kind == "manual" {
			verifiedBackups++
		}
	}
	return verifiedBackups >= 2
}

func legacyJSONLMirrorStatus() string {
	if legacyJSONLMirrorReadyToDisable() {
		return "переходная зеркальная запись JSONL активна; технические условия для будущего отключения выполнены"
	}
	return "переходная зеркальная запись JSONL активна; отключение пока запрещено"
}

func ensureMonthlyPersistentBackup(srcDaily string) error {
	if strings.TrimSpace(srcDaily) == "" {
		return nil
	}
	dst := filepath.Join(persistentBackupDir(), "rosa_monthly_"+time.Now().Format("2006-01")+".db")
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	in, err := os.Open(srcDaily)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if cpErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if cpErr != nil {
			return cpErr
		}
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	if err = os.Rename(tmp, dst); err != nil {
		return err
	}
	if err = verifySQLiteFile(dst); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

func rotatePersistentBackupsByPolicy() error {
	items := listPersistentBackups()
	keepDaily, keepMonthly := 35, 12
	dailyN, monthlyN := 0, 0
	for _, it := range items {
		switch it.Kind {
		case "daily":
			dailyN++
			if dailyN > keepDaily {
				_ = os.Remove(it.Path)
			}
		case "monthly":
			monthlyN++
			if monthlyN > keepMonthly {
				_ = os.Remove(it.Path)
			}
		}
	}
	return nil
}

func createPersistentDatabaseBackup(force bool) error {
	if persistentStore == nil || persistentStore.db == 0 {
		return nil
	}
	if err := os.MkdirAll(persistentBackupDir(), 0700); err != nil {
		return err
	}
	stampDay := time.Now().Format("2006-01-02")
	dst := filepath.Join(persistentBackupDir(), "rosa_"+stampDay+".db")
	if !force {
		if _, err := os.Stat(dst); err == nil {
			return nil
		}
	}
	if err := persistentStore.onlineBackupTo(dst); err != nil {
		persistentStore.recordStorageEvent("backup_daily", "error", err.Error())
		return err
	}
	if err := ensureMonthlyPersistentBackup(dst); err != nil {
		persistentStore.recordStorageEvent("backup_monthly", "error", err.Error())
		return err
	}
	_ = rotatePersistentBackupsByPolicy()
	persistentStore.recordStorageEvent("backup_daily", "ok", dst+"; method=sqlite_online_backup")
	return nil
}
func choosePersistentDatabaseBackup() string {
	buf := make([]uint16, 32768)
	filter := utf16.Encode([]rune("SQLite backup (*.db)\x00*.db\x00Все файлы (*.*)\x00*.*\x00\x00"))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: p("Выберите резервную копию SQLite"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func createManualPersistentDatabaseBackup() (string, error) {
	if persistentStore == nil || persistentStore.db == 0 {
		return "", fmt.Errorf("SQLite-хранилище не активно")
	}
	if err := os.MkdirAll(persistentBackupDir(), 0700); err != nil {
		return "", err
	}
	if _, err := persistentStore.integrityCheck(); err != nil {
		return "", fmt.Errorf("резервная копия отменена: исходная база не прошла integrity_check: %w", err)
	}
	dst := filepath.Join(persistentBackupDir(), "rosa_manual_"+time.Now().Format("20060102_150405")+".db")
	if err := persistentStore.onlineBackupTo(dst); err != nil {
		persistentStore.recordStorageEvent("backup_manual", "error", err.Error())
		return "", err
	}
	persistentStore.recordStorageEvent("backup_manual", "ok", dst+"; method=sqlite_online_backup")
	return dst, nil
}

func restorePersistentDatabaseFromBackup(backupPath string) error {
	backupPath = strings.TrimSpace(backupPath)
	if backupPath == "" {
		return fmt.Errorf("не указан файл резервной копии")
	}
	if _, err := os.Stat(backupPath); err != nil {
		return err
	}
	if persistentStore != nil {
		_ = persistentStore.exec("PRAGMA wal_checkpoint(FULL);")
		persistentStore.close()
		persistentStore = nil
	}
	path := persistentDatabasePath()
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	pre := path + ".pre_restore_" + time.Now().Format("20060102_150405")
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, pre); err != nil {
			return fmt.Errorf("не удалось сохранить текущую БД перед восстановлением: %w", err)
		}
	}
	in, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(path+".restore.tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if cpErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(path + ".restore.tmp")
		if cpErr != nil {
			return cpErr
		}
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	if err := os.Rename(path+".restore.tmp", path); err != nil {
		return err
	}
	st, err := openPersistentStore()
	if err != nil {
		_ = os.Remove(path)
		_ = os.Rename(pre, path)
		return fmt.Errorf("восстановленная БД не прошла проверку; возвращена предыдущая: %w", err)
	}
	if _, err := st.integrityCheck(); err != nil {
		st.close()
		_ = os.Remove(path)
		_ = os.Rename(pre, path)
		return fmt.Errorf("восстановленная БД повреждена; возвращена предыдущая: %w", err)
	}
	persistentStore = st
	persistentStorageSafeModeReason = ""
	persistentStore.recordStorageEvent("restore", "ok", backupPath)
	return nil
}

func rotatePersistentBackups(keep int) error {
	entries, err := os.ReadDir(persistentBackupDir())
	if err != nil {
		return err
	}
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "rosa_") && strings.HasSuffix(e.Name(), ".db") {
			files = append(files, e)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
	for _, e := range files[keep:] {
		_ = os.Remove(filepath.Join(persistentBackupDir(), e.Name()))
	}
	return nil
}

func appendLabPassLegacyJSONL(pass LabPassRecord) error {
	b, err := json.Marshal(pass)
	if err != nil {
		return err
	}
	if personalPath(labPassPath()) {
		b, err = encryptPersonalLine(b)
		if err != nil {
			return err
		}
	}
	f, err := os.OpenFile(labPassPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
func loadLabPassesLegacyJSONL() []LabPassRecord {
	f, err := os.Open(labPassPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	latest := map[string]LabPassRecord{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line, de := decryptPersonalLine(sc.Bytes())
		var p LabPassRecord
		if de != nil || json.Unmarshal(line, &p) != nil || p.PassID == "" || p.ObjectKey == "" {
			continue
		}
		old, ok := latest[p.PassID]
		if !ok || p.UpdatedAt >= old.UpdatedAt {
			latest[p.PassID] = p
		}
	}
	out := make([]LabPassRecord, 0, len(latest))
	for _, p := range latest {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}
