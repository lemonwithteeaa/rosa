//go:build windows

package main

import "testing"

func TestRC624MethodsUsesFourLevelTypeScale(t *testing.T) {
	got := []int{methodsTypeDisplay, methodsTypeSection, methodsTypeBody, methodsTypeCompact}
	want := []int{24, 15, 13, 12}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("type token %d: got %d, want %d", i, got[i], want[i])
		}
		if i > 0 && got[i-1] <= got[i] {
			t.Fatalf("type hierarchy is not strictly descending: %v", got)
		}
	}
}
