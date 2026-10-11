//go:build windows

package main

import "testing"

func TestOwnerOrganization821PreservesCredentials(t *testing.T) {
	a := LocalAccount{ID: "stable-identifier", Login: "Катя", DisplayName: "Ромадина Екатерина Викторовна", LastName: "Ромадина", FirstName: "Екатерина", MiddleName: "Викторовна", NGDO: "старое", CITS: "старое", Position: "инженер", CDNG: "сохранить", Salt: "salt", PasswordHash: "hash", PublicKey: "public-key", IsAdmin: true}
	st := AuthStore{SchemaVersion: 1, Accounts: []LocalAccount{a}}
	if !upgradeOwnerOrganizationProfile(&st) {
		t.Fatal("owner profile not updated")
	}
	got := st.Accounts[0]
	if got.NGDO != "РИТЭК" || got.CITS != "АУ" || got.Position != "ведущий инженер" {
		t.Fatalf("incorrect org profile: %+v", got)
	}
	if got.ID != a.ID || got.Login != a.Login || got.DisplayName != a.DisplayName || got.PasswordHash != a.PasswordHash || got.Salt != a.Salt || got.PublicKey != a.PublicKey || got.CDNG != "-" || got.IsAdmin != a.IsAdmin {
		t.Fatal("credentials or unrelated fields modified")
	}
	if upgradeOwnerOrganizationProfile(&st) {
		t.Fatal("second upgrade should be inert")
	}
}

func TestOwnerOrganization821UnrelatedAndAmbiguousAccounts(t *testing.T) {
	st := AuthStore{Accounts: []LocalAccount{{DisplayName: "Другая Екатерина", NGDO: "сохранить"}, {DisplayName: "Ромадина Екатерина Викторовна", NGDO: "старое"}, {DisplayName: "Ромадина Екатерина Викторовна", NGDO: "другое"}}}
	if upgradeOwnerOrganizationProfile(&st) {
		t.Fatal("ambiguous update")
	}
	for i, a := range st.Accounts {
		if i == 0 && a.NGDO != "сохранить" {
			t.Fatal("changed unrelated account")
		}
		if i == 1 && a.NGDO != "старое" {
			t.Fatal("changed ambiguous account")
		}
		if i == 2 && a.NGDO != "другое" {
			t.Fatal("changed ambiguous account")
		}
	}
}

func TestNNOAxisStep821Validation(t *testing.T) {
	for _, test := range []struct {
		value    string
		max      int
		expected int
		valid    bool
	}{{"Авто", 12, 0, true}, {"", 12, 0, true}, {"3", 12, 3, true}, {"7", 6, 0, false}, {"-2", 12, 0, false}, {"abc", 1000, 0, false}, {"100", 10000, 100, true}} {
		got, err := nnoExpandedParseAxisStep(test.value, test.max)
		if got != test.expected || (err == nil) != test.valid {
			t.Errorf("parse(%q,%d) = %d,%v", test.value, test.max, got, err)
		}
	}
}
