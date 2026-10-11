//go:build windows

package main

import "testing"

func TestAuthActionLayoutProtectsLongCaption(t *testing.T) {
	loginX, loginW, createX, createW := authActionLayout()
	if loginX != 32 || createX-(loginX+loginW) < 10 {
		t.Fatalf("неверный промежуток между кнопками: %d %d %d", loginX, loginW, createX)
	}
	if createW < 210 {
		t.Fatalf("кнопка создания профиля снова стала узкой: %d", createW)
	}
	if createX+createW > 528 {
		t.Fatalf("кнопка вышла за правую границу формы: %d", createX+createW)
	}
}
