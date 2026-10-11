//go:build windows

package main

import "testing"

func TestOwner822DepartmentAndStableCredentials(t *testing.T) {
	original := LocalAccount{ID: "owner-immutable", Login: "saved-owner", DisplayName: "Ромадина Екатерина Викторовна", LastName: "Ромадина", FirstName: "Екатерина", MiddleName: "Викторовна", NGDO: "РИТЭК", CITS: "АУП", CDNG: "ЦДНГ-2", Position: "ведущий инженер", PasswordHash: "secret", Salt: "salt", PublicKey: "key", IsAdmin: true}
	st := AuthStore{Accounts: []LocalAccount{original}}
	if !upgradeOwnerOrganizationProfile(&st) {
		t.Fatal("missing upgrade")
	}
	got := st.Accounts[0]
	if got.CITS != "АУ" || got.CDNG != "-" || got.NGDO != "РИТЭК" || got.Position != "ведущий инженер" {
		t.Fatalf("unexpected owner fields: %+v", got)
	}
	if got.ID != original.ID || got.Login != original.Login || got.PasswordHash != original.PasswordHash || got.Salt != original.Salt || got.PublicKey != original.PublicKey || got.IsAdmin != original.IsAdmin {
		t.Fatal("credentials changed")
	}
	if upgradeOwnerOrganizationProfile(&st) {
		t.Fatal("upgrade not idempotent")
	}
}
