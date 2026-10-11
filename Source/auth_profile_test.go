//go:build windows

package main

import "testing"

func TestRC182ProfileLoginKeyContainsIdentity(t *testing.T) {
	got := accountProfileLogin("ЦИТС-1", "ЦДНГ-2", "Иванов И.И.")
	if got != "ЦИТС-1 | ЦДНГ-2 | Иванов И.И." {
		t.Fatalf("unexpected profile login key: %q", got)
	}
}

func TestRC182CurrentUserLabelIsOnlyFIO(t *testing.T) {
	old := currentSession
	defer func() { currentSession = old }()
	currentSession = &UserSession{Login: "internal", DisplayName: "Иванов Иван Иванович", CITS: "ЦИТС", CDNG: "ЦДНГ", Position: "Технолог"}
	if got := currentUserLabel(); got != "Иванов Иван Иванович" {
		t.Fatalf("header label=%q", got)
	}
}

func TestRC188CredentialManagerTargetIsStable(t *testing.T) {
	if rememberedCredentialTarget != "ROSA/RememberedLogin" {
		t.Fatalf("credential target changed: %q", rememberedCredentialTarget)
	}
}
