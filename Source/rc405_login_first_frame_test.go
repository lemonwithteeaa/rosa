//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC405LoginFirstFrameUsesOneFinalHiddenLayout(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`authFitHiddenWindowToWorkArea(hwnd)`,
		`pSystemParametersInfoWAuth.Call(SPI_GETWORKAREA`,
		`pSendMessageW.Call(hwnd, WM_SIZE`,
		`authRevealDeferredChildren()`,
		`pInvalidateRect.Call(hwnd, 0, 1)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing first-frame rule %q", want)
		}
	}
	if strings.Contains(s, `WS_OVERLAPPEDWINDOW|WS_MAXIMIZE_AUTH|WS_CLIPCHILDREN_AUTH`) {
		t.Fatal("login must not maximize during WM_CREATE")
	}
}
