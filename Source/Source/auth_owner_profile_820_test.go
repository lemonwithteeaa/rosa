//go:build windows

package main

import "testing"

func TestOwnerNameMigrationPreservesCredentialsAndIdentity(t *testing.T) {
	original := LocalAccount{ID: "user-katya", Login: "Катя", DisplayName: "Катя", Salt: "unchanged-salt", PasswordHash: "unchanged-hash", Iterations: 180000, IsAdmin: true, PublicKey: "same-key", CITS: "АУП", CDNG: "ЦДНГ-1", NGDO: "РИТЭК", Position: "ведущий инженер"}
	st := AuthStore{SchemaVersion: 1, Accounts: []LocalAccount{original}}
	if !upgradeLegacyOwnerProfile(&st) {
		t.Fatal("legacy owner profile was not updated")
	}
	a := st.Accounts[0]
	if a.DisplayName != "Ромадина Екатерина Викторовна" || shortPersonName(a.LastName, a.FirstName, a.MiddleName) != "Ромадина Е.В." {
		t.Fatalf("incorrect displayed name: %#v", a)
	}
	if a.ID != original.ID || a.Login != original.Login || a.PasswordHash != original.PasswordHash || a.Salt != original.Salt || a.Iterations != original.Iterations || a.PublicKey != original.PublicKey || a.IsAdmin != original.IsAdmin || a.CITS != original.CITS || a.CDNG != original.CDNG || a.NGDO != original.NGDO || a.Position != original.Position {
		t.Fatal("migration altered credentials or identity")
	}
	if upgradeLegacyOwnerProfile(&st) {
		t.Fatal("profile upgrade should be idempotent")
	}
}

func TestOwnerNameMigrationNeverChangesAmbiguousProfiles(t *testing.T) {
	tests := []AuthStore{
		{Accounts: []LocalAccount{{DisplayName: "Катя"}, {DisplayName: "Катя"}}},
		{Accounts: []LocalAccount{{DisplayName: "Катя"}, {DisplayName: "Ромадина Екатерина Викторовна"}}},
		{Accounts: []LocalAccount{{DisplayName: "Екатерина"}}},
		{Accounts: []LocalAccount{{DisplayName: "Катя", LastName: "Другая", FirstName: "Катя"}}},
	}
	for i, st := range tests {
		if upgradeLegacyOwnerProfile(&st) {
			t.Errorf("unrelated/ambiguous profile was renamed in case %d", i)
		}
	}
}
