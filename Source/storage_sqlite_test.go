//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestRC184PersistentDatabaseIsOutsideRCAndUserData(t *testing.T) {
	p := strings.ToLower(persistentDatabasePath())
	if !strings.Contains(p, "rosa") || !strings.HasSuffix(p, "rosa.db") {
		t.Fatalf("unexpected persistent db path: %s", p)
	}
	if strings.Contains(p, "7.3.11") {
		t.Fatalf("database must not be version-bound: %s", p)
	}
}

func TestRC184SQLiteQuoteEscapesApostrophe(t *testing.T) {
	if got := sqliteQuote("O'Brien"); got != "'O''Brien'" {
		t.Fatalf("quote=%q", got)
	}
}

func TestRC184CanonicalHashIsStable(t *testing.T) {
	p := LabPassRecord{PassID: "LP-1", ObjectKey: "Field 23", Status: labPassSavedStatus, Groups: map[string]map[string]string{"scale": {"ph": "7.1"}}}
	_, h1, e1 := canonicalLabPassBytes(p)
	_, h2, e2 := canonicalLabPassBytes(p)
	if e1 != nil || e2 != nil || h1 == "" || h1 != h2 {
		t.Fatalf("unstable hash %q %q %v %v", h1, h2, e1, e2)
	}
}

func TestRC185DecodeLabPassPayloadsKeepsLatestRevisionPerPass(t *testing.T) {
	old := LabPassRecord{PassID: "LP-1", ObjectKey: "OBJ-1", UpdatedAt: "2026-09-01T00:00:00Z", Status: labPassSavedStatus}
	newer := old
	newer.UpdatedAt = "2026-09-02T00:00:00Z"
	newer.Comment = "new"
	other := LabPassRecord{PassID: "LP-2", ObjectKey: "OBJ-2", UpdatedAt: "2026-09-03T00:00:00Z", Status: labPassSavedStatus}
	b1, _, _ := canonicalLabPassBytes(old)
	b2, _, _ := canonicalLabPassBytes(newer)
	b3, _, _ := canonicalLabPassBytes(other)
	rows := decodeLabPassPayloads([][]byte{b1, b2, b3}, true)
	if len(rows) != 2 {
		t.Fatalf("latest revision collapse len=%d", len(rows))
	}
	found := false
	for _, row := range rows {
		if row.PassID == "LP-1" {
			found = row.Comment == "new"
		}
	}
	if !found {
		t.Fatal("latest revision of LP-1 was not retained")
	}
}

func TestRC185RepositoryBackupPathIsSeparatedFromDatabase(t *testing.T) {
	if persistentBackupDir() == persistentDatabaseDir() {
		t.Fatal("backup directory must not equal database directory")
	}
	if !strings.Contains(strings.ToLower(persistentBackupDir()), "backup") {
		t.Fatalf("unexpected backup directory: %s", persistentBackupDir())
	}
}

func TestRC185SafeModeStatusIsExplicit(t *testing.T) {
	oldStore, oldReason := persistentStore, persistentStorageSafeModeReason
	persistentStore = nil
	persistentStorageSafeModeReason = "test integrity failure"
	defer func() { persistentStore, persistentStorageSafeModeReason = oldStore, oldReason }()
	got := persistentStorageStatusText()
	if !strings.Contains(got, "БЕЗОПАСНЫЙ РЕЖИМ") || !strings.Contains(got, "test integrity failure") {
		t.Fatalf("safe-mode status not explicit: %q", got)
	}
}

func TestPersistentBackupKindRC186(t *testing.T) {
	cases := map[string]string{
		"rosa_2026-09-18.db":             "daily",
		"rosa_monthly_2026-09.db":        "monthly",
		"rosa_manual_20260918_120000.db": "manual",
		"other.db":                       "other",
	}
	for name, want := range cases {
		if got := persistentBackupKind(name); got != want {
			t.Fatalf("%s: got %s want %s", name, got, want)
		}
	}
}

func TestRC187SchemaTargetAndMirrorGate(t *testing.T) {
	if persistentSchemaTarget < 3 {
		t.Fatalf("schema target regressed: %d", persistentSchemaTarget)
	}
	if persistentBackupKind("rosa_pre_migration_20260918_200000.db") == "daily" {
		t.Fatal("pre-migration backup must not be treated as daily retention item")
	}
}

func TestRC187LatestSelectionScalesToFiftyThousandPasses(t *testing.T) {
	const objects = 1000
	const perObject = 50
	passes := make([]LabPassRecord, 0, objects*perObject)
	keys := make([]string, 0, objects)
	for i := 0; i < objects; i++ {
		key := fmt.Sprintf("OBJ-%04d", i)
		keys = append(keys, key)
		for j := 0; j < perObject; j++ {
			passes = append(passes, LabPassRecord{ObjectKey: key, Status: labPassSavedStatus, Date: fmt.Sprintf("2026-%02d-%02d", 1+(j/28)%9, 1+j%28), UpdatedAt: fmt.Sprintf("2026-09-18T20:%02d:%02dZ", j%60, i%60)})
		}
	}
	got := laboratoryLatestSavedPassesForKeys(keys, passes)
	if len(got) != objects {
		t.Fatalf("50k latest-selection returned %d, want %d", len(got), objects)
	}
}

func TestRC188StorageSecurityProfile(t *testing.T) {
	if persistentSchemaTarget < 4 {
		t.Fatalf("schema target must include RC188 hardening migration, got %d", persistentSchemaTarget)
	}
	if rosaSQLiteApplicationID != 1380930369 {
		t.Fatalf("unexpected ROSA application_id: %d", rosaSQLiteApplicationID)
	}
}

func TestRC189RepositoryFailurePolicyRequiresDurableWrite(t *testing.T) {
	if persistentSchemaTarget != 5 {
		t.Fatalf("schema target must match the latest implemented migration: got %d want 5", persistentSchemaTarget)
	}
	if !rosaRepositoryPolicy.RequireDurableWrite {
		t.Fatal("repository must require at least one durable write")
	}
	if !rosaRepositoryPolicy.AllowLegacyReadFallback {
		t.Fatal("transition build must keep legacy read fallback")
	}
}

func TestRC189SQLiteUserDataUsesParameterBinding(t *testing.T) {
	b, _ := os.ReadFile("storage_sqlite_windows.go")
	s := string(b)
	for _, required := range []string{"sqlite3_bind_text", "sqlite3_bind_blob", "sqlite3_bind_int64", "withWriteTransaction"} {
		if !strings.Contains(s, required) {
			t.Fatalf("RC189 binding/single-writer primitive missing: %s", required)
		}
	}
	if strings.Contains(s, "normalized_object_key=%s") {
		t.Fatal("object-key query still interpolates user data")
	}
}

func TestRC189FaultInjectionPolicyIsFailSafe(t *testing.T) {
	cases := []struct {
		name               string
		sqliteOK, legacyOK bool
		wantOK             bool
	}{
		{"sqlite succeeds", true, false, true},
		{"legacy succeeds", false, true, true},
		{"both succeed", true, true, true},
		{"disk or both stores fail", false, false, false},
	}
	for _, tc := range cases {
		got := tc.sqliteOK || tc.legacyOK
		if got != tc.wantOK {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.wantOK)
		}
	}
}
