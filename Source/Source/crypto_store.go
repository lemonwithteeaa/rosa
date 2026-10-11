//go:build windows

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

var encryptedFileMagic = []byte("ROSAENC1\x00")

const encryptedLinePrefix = "ROSAENC1:"

func personalPath(path string) bool {
	if currentSession == nil || dataDir == "" {
		return false
	}
	ap, err1 := filepath.Abs(path)
	ad, err2 := filepath.Abs(dataDir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(ad, ap)
	if err != nil {
		return false
	}
	return rel != "." && rel != "" && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func sessionAEAD() (cipher.AEAD, error) {
	if currentSession == nil || len(currentSession.masterKey) != 32 {
		return nil, fmt.Errorf("пользовательское хранилище не разблокировано")
	}
	block, err := aes.NewCipher(currentSession.masterKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptPersonalBytes(plain []byte) ([]byte, error) {
	aead, err := sessionAEAD()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, plain, encryptedFileMagic)
	out := make([]byte, 0, len(encryptedFileMagic)+len(nonce)+len(sealed))
	out = append(out, encryptedFileMagic...)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

func decryptPersonalBytes(blob []byte) ([]byte, error) {
	if !bytes.HasPrefix(blob, encryptedFileMagic) {
		return blob, nil
	}
	aead, err := sessionAEAD()
	if err != nil {
		return nil, err
	}
	off := len(encryptedFileMagic)
	if len(blob) < off+aead.NonceSize() {
		return nil, fmt.Errorf("зашифрованный файл повреждён")
	}
	nonce := blob[off : off+aead.NonceSize()]
	ct := blob[off+aead.NonceSize():]
	return aead.Open(nil, nonce, ct, encryptedFileMagic)
}

func encryptPersonalLine(plain []byte) ([]byte, error) {
	enc, err := encryptPersonalBytes(plain)
	if err != nil {
		return nil, err
	}
	return []byte(encryptedLinePrefix + base64.StdEncoding.EncodeToString(enc)), nil
}

func decryptPersonalLine(line []byte) ([]byte, error) {
	s := strings.TrimSpace(string(line))
	if !strings.HasPrefix(s, encryptedLinePrefix) {
		return line, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, encryptedLinePrefix))
	if err != nil {
		return nil, err
	}
	return decryptPersonalBytes(raw)
}
