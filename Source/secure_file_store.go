//go:build windows

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var secureAssetMagic = []byte("ROSAAS2\x00")

const secureAssetChunk = 1 << 20 // 1 MiB; bounded RAM for multi-GB telemetry files.

func isSecureAssetPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".rse")
}

func ensureSessionTempDir() (string, error) {
	if currentSession == nil {
		return "", fmt.Errorf("пользователь не авторизован")
	}
	if currentSession.TempDir == "" {
		rnd := make([]byte, 8)
		if _, err := rand.Read(rnd); err != nil {
			return "", err
		}
		currentSession.TempDir = filepath.Join(os.TempDir(), "ROSA", currentSession.UserID, fmt.Sprintf("%x", rnd))
	}
	if err := os.MkdirAll(currentSession.TempDir, 0700); err != nil {
		return "", err
	}
	return currentSession.TempDir, nil
}

func cleanupSessionTemp() {
	if currentSession == nil || currentSession.TempDir == "" {
		return
	}
	_ = os.RemoveAll(currentSession.TempDir)
	currentSession.TempDir = ""
}

func secureVaultPath(hash string) string {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if len(hash) < 2 {
		hash = "00" + hash
	}
	dir := filepath.Join(dataDir, "vault", hash[:2])
	_ = os.MkdirAll(dir, 0700)
	return filepath.Join(dir, hash+".rse")
}

func secureStoreFile(src string) (string, string, error) {
	hash := hashFile(src)
	if hash == "" {
		return "", "", fmt.Errorf("не удалось вычислить SHA-256")
	}
	dst := secureVaultPath(hash)
	if fi, err := os.Stat(dst); err == nil && fi.Size() > int64(len(secureAssetMagic)) {
		return dst, hash, nil
	}
	in, err := os.Open(src)
	if err != nil {
		return "", "", err
	}
	defer in.Close()
	aead, err := sessionAEAD()
	if err != nil {
		return "", "", err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	bw := bufio.NewWriterSize(out, 1<<20)
	if _, err = bw.Write(secureAssetMagic); err != nil {
		out.Close()
		return "", "", err
	}
	if err = binary.Write(bw, binary.LittleEndian, uint32(secureAssetChunk)); err != nil {
		out.Close()
		return "", "", err
	}
	buf := make([]byte, secureAssetChunk)
	idx := uint64(0)
	for {
		n, er := in.Read(buf)
		if n > 0 {
			nonce := make([]byte, aead.NonceSize())
			if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
				out.Close()
				return "", "", err
			}
			aad := make([]byte, 8)
			binary.LittleEndian.PutUint64(aad, idx)
			sealed := aead.Seal(nil, nonce, buf[:n], aad)
			if err = binary.Write(bw, binary.LittleEndian, uint32(n)); err != nil {
				out.Close()
				return "", "", err
			}
			if _, err = bw.Write(nonce); err != nil {
				out.Close()
				return "", "", err
			}
			if _, err = bw.Write(sealed); err != nil {
				out.Close()
				return "", "", err
			}
			idx++
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			out.Close()
			return "", "", er
		}
	}
	if err = bw.Flush(); err != nil {
		out.Close()
		return "", "", err
	}
	if err = out.Sync(); err != nil {
		out.Close()
		return "", "", err
	}
	if err = out.Close(); err != nil {
		return "", "", err
	}
	if err = replaceFileAtomic(tmp, dst); err != nil {
		return "", "", err
	}
	ok = true
	return dst, hash, nil
}

func secureMaterialize(storedPath, extension string) (string, error) {
	if strings.TrimSpace(storedPath) == "" {
		return "", fmt.Errorf("пустой путь вложения")
	}
	if !isSecureAssetPath(storedPath) {
		return storedPath, nil
	}
	temp, err := ensureSessionTempDir()
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(storedPath), filepath.Ext(storedPath))
	if extension == "" {
		extension = ".bin"
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	dst := filepath.Join(temp, base+extension)
	if fi, err := os.Stat(dst); err == nil && fi.Size() > 0 {
		return dst, nil
	}
	in, err := os.Open(storedPath)
	if err != nil {
		return "", err
	}
	defer in.Close()
	br := bufio.NewReaderSize(in, 1<<20)
	magic := make([]byte, len(secureAssetMagic))
	if _, err = io.ReadFull(br, magic); err != nil || string(magic) != string(secureAssetMagic) {
		return "", fmt.Errorf("неподдерживаемый формат защищённого вложения")
	}
	var chunk uint32
	if err = binary.Read(br, binary.LittleEndian, &chunk); err != nil || chunk == 0 || chunk > 16<<20 {
		return "", fmt.Errorf("повреждён заголовок защищённого вложения")
	}
	aead, err := sessionAEAD()
	if err != nil {
		return "", err
	}
	out, err := os.OpenFile(dst+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	tmp := dst + ".tmp"
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	idx := uint64(0)
	for {
		var plainLen uint32
		er := binary.Read(br, binary.LittleEndian, &plainLen)
		if er == io.EOF {
			break
		}
		if er != nil {
			out.Close()
			return "", er
		}
		if plainLen == 0 || plainLen > chunk {
			out.Close()
			return "", fmt.Errorf("повреждён блок защищённого вложения")
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err = io.ReadFull(br, nonce); err != nil {
			out.Close()
			return "", err
		}
		ct := make([]byte, int(plainLen)+aead.Overhead())
		if _, err = io.ReadFull(br, ct); err != nil {
			out.Close()
			return "", err
		}
		aad := make([]byte, 8)
		binary.LittleEndian.PutUint64(aad, idx)
		plain, err := aead.Open(nil, nonce, ct, aad)
		if err != nil {
			out.Close()
			return "", fmt.Errorf("не удалось расшифровать блок %d", idx+1)
		}
		if _, err = out.Write(plain); err != nil {
			out.Close()
			return "", err
		}
		idx++
	}
	if err = out.Sync(); err != nil {
		out.Close()
		return "", err
	}
	if err = out.Close(); err != nil {
		return "", err
	}
	if err = replaceFileAtomic(tmp, dst); err != nil {
		return "", err
	}
	ok = true
	return dst, nil
}

func secureReadAll(storedPath, extension string, maxBytes int64) ([]byte, error) {
	pth, err := secureMaterialize(storedPath, extension)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(pth)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && fi.Size() > maxBytes {
		return nil, fmt.Errorf("вложение слишком большое для чтения целиком")
	}
	return os.ReadFile(pth)
}
