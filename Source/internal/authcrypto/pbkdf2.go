package authcrypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
)

const (
	CurrentIterations = 180_000
	MinimumIterations = 10_000
	MaximumIterations = 2_000_000
	maximumKeyLength  = 1 << 20
)

// StoredIterations keeps compatibility with profiles created before the
// iteration count was persisted, while rejecting corrupt values that could
// otherwise keep the login screen busy for an unbounded amount of time.
func StoredIterations(stored int) (int, error) {
	if stored < MinimumIterations {
		return CurrentIterations, nil
	}
	if stored > MaximumIterations {
		return 0, fmt.Errorf("число итераций профиля превышает допустимый предел")
	}
	return stored, nil
}

// DeriveKey implements PBKDF2-HMAC-SHA256 without an external dependency so
// the offline Windows build remains self-contained.
func DeriveKey(password string, salt []byte, iterations, keyLen int) ([]byte, error) {
	if len(salt) == 0 {
		return nil, fmt.Errorf("пустая соль пароля")
	}
	if iterations < 1 || iterations > MaximumIterations {
		return nil, fmt.Errorf("недопустимое число итераций: %d", iterations)
	}
	if keyLen < 1 || keyLen > maximumKeyLength {
		return nil, fmt.Errorf("недопустимая длина ключа: %d", keyLen)
	}

	const hashLen = 32
	blocks := (keyLen + hashLen - 1) / hashLen
	out := make([]byte, 0, blocks*hashLen)
	secret := []byte(password)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, secret)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, secret)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen], nil
}
