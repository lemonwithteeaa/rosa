package authcrypto

import (
	"encoding/hex"
	"testing"
)

func TestDeriveKeyMatchesPublishedSHA256Vectors(t *testing.T) {
	tests := []struct {
		iterations int
		want       string
	}{
		{1, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{2, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{4096, "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
	}
	for _, tt := range tests {
		got, err := DeriveKey("password", []byte("salt"), tt.iterations, 32)
		if err != nil {
			t.Fatalf("iterations %d: %v", tt.iterations, err)
		}
		if hex.EncodeToString(got) != tt.want {
			t.Fatalf("iterations %d: got %x, want %s", tt.iterations, got, tt.want)
		}
	}
}

func TestStoredIterationsBoundsWorkFactor(t *testing.T) {
	for _, legacy := range []int{0, 1, MinimumIterations - 1} {
		got, err := StoredIterations(legacy)
		if err != nil || got != CurrentIterations {
			t.Fatalf("legacy %d: got %d, err %v", legacy, got, err)
		}
	}
	if got, err := StoredIterations(MinimumIterations); err != nil || got != MinimumIterations {
		t.Fatalf("minimum: got %d, err %v", got, err)
	}
	if _, err := StoredIterations(MaximumIterations + 1); err == nil {
		t.Fatal("unbounded work factor must be rejected")
	}
}

func TestDeriveKeyRejectsCorruptParameters(t *testing.T) {
	for _, tc := range []struct {
		salt       []byte
		iterations int
		keyLen     int
	}{
		{nil, 1, 32},
		{[]byte("salt"), 0, 32},
		{[]byte("salt"), MaximumIterations + 1, 32},
		{[]byte("salt"), 1, 0},
	} {
		if _, err := DeriveKey("password", tc.salt, tc.iterations, tc.keyLen); err == nil {
			t.Fatalf("parameters unexpectedly accepted: %+v", tc)
		}
	}
}
