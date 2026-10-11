//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC403LoginAssetsNeverUseWorkingDirectory(t *testing.T) {
	b, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "filepath.Join(dataDir,") {
		t.Fatal("login/brand raster cache still depends on dataDir and can fall back to the working directory")
	}
	for _, want := range []string{
		`func uiAssetCacheDir() string`,
		`filepath.Join(appRootDir(), "cache", "ui")`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing private UI cache rule: %s", want)
		}
	}
}

func TestRC403PreLoginStorageIsInitialized(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`dataDir = appDataDir()`,
		`os.MkdirAll(uiAssetCacheDir(), 0700)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing pre-login storage guard: %s", want)
		}
	}
}

func TestRC403StartupLogNeverWritesBesideExecutable(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "func startupLog(")
	if start < 0 {
		t.Fatal("startupLog missing")
	}
	end := strings.Index(s[start:], "\n}\n")
	if end < 0 {
		t.Fatal("startupLog end missing")
	}
	block := s[start : start+end+3]
	if strings.Contains(block, "os.Executable") || strings.Contains(block, "filepath.Dir(ex)") {
		t.Fatal("startup log can still be written beside ROSA.exe")
	}
	if !strings.Contains(block, `filepath.Join(appRootDir(), "logs", "ROSA_startup.log")`) {
		t.Fatal("startup log is not routed to private APPDATA storage")
	}
}
