//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC404LoginFirstFrameDoesNotUseCompositedParent(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `pCreateWindowExW.Call(WS_EX_COMPOSITED_AUTH`) {
		t.Fatal("login parent must not use WS_EX_COMPOSITED; it can leave native children blank")
	}
	for _, want := range []string{
		`authConstructing = true`,
		`style &^= WS_VISIBLE`,
		`authDeferredVisible = append(authDeferredVisible, z)`,
		`authRevealDeferredChildren()`,
		`NOREDRAW|NOACTIVATE|SHOWWINDOW`,
		`RDW_ALLCHILDREN`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing RC404 first-frame guard: %s", want)
		}
	}
}
