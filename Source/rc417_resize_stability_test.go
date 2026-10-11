package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC417AuthResizeUsesAtomicBatchAndFullRedraw(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	required := []string{
		"SIZE_MINIMIZED_AUTH",
		"pBeginDeferWindowPos.Call",
		"pDeferWindowPos.Call",
		"pEndDeferWindowPos.Call",
		"authApplyResizeLayout(clientW, clientH)",
		"authScheduleFullRedraw(hwnd)",
		"case WM_AUTH_FULL_REDRAW:",
		"ALLCHILDREN|UPDATENOW|FRAME",
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Fatalf("RC417 resize guard missing %q", want)
		}
	}
	start := strings.Index(s, "case WM_SIZE:")
	end := strings.Index(s[start:], "case WM_COMMAND:")
	if start < 0 || end < 0 {
		t.Fatal("WM_SIZE block not found")
	}
	block := s[start : start+end]
	if strings.Contains(block, "pInvalidateRect.Call(hwnd") {
		t.Fatal("WM_SIZE must not repaint mid-layout; redraw is queued after the atomic batch")
	}
}
