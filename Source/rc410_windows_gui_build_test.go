//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC410BuildScriptUsesWindowsGUISubsystem(t *testing.T) {
	b, err := os.ReadFile("build_windows_gui.ps1")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "-H=windowsgui") {
		t.Fatal("Windows release build must use GUI subsystem; console subsystem opens a black terminal window")
	}
}
