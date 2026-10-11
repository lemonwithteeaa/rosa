//go:build windows

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var crypt32 = syscall.NewLazyDLL("crypt32.dll")
var pCryptProtectData = crypt32.NewProc("CryptProtectData")
var pCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
var pLocalFreeRemember = kernel32.NewProc("LocalFree")

var advapi32Remember = syscall.NewLazyDLL("advapi32.dll")
var pCredWriteW = advapi32Remember.NewProc("CredWriteW")
var pCredReadW = advapi32Remember.NewProc("CredReadW")
var pCredFree = advapi32Remember.NewProc("CredFree")
var pCredDeleteW = advapi32Remember.NewProc("CredDeleteW")

const cryptprotectUIForbidden = 0x1

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

const rememberedCredentialTarget = "ROSA/RememberedLogin"

type dataBlob struct {
	cbData uint32
	pbData *byte
}

type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWrittenLow     uint32
	LastWrittenHigh    uint32
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func saveCredentialManager(login, password string) error {
	login = strings.TrimSpace(login)
	if login == "" || password == "" {
		return fmt.Errorf("пустые данные для Credential Manager")
	}
	target, err := syscall.UTF16PtrFromString(rememberedCredentialTarget)
	if err != nil {
		return err
	}
	user, err := syscall.UTF16PtrFromString(login)
	if err != nil {
		return err
	}
	blob := []byte(password)
	if len(blob) > 512 {
		return fmt.Errorf("пароль слишком длинный для Windows Credential Manager")
	}
	c := credentialW{Type: credTypeGeneric, TargetName: target, Persist: credPersistLocalMachine, UserName: user, CredentialBlobSize: uint32(len(blob))}
	if len(blob) > 0 {
		c.CredentialBlob = &blob[0]
	}
	r, _, e := pCredWriteW.Call(uintptr(unsafe.Pointer(&c)), 0)
	for i := range blob {
		blob[i] = 0
	}
	if r == 0 {
		return fmt.Errorf("CredWriteW: %v", e)
	}
	return nil
}

func loadCredentialManager() (login, password string, ok bool) {
	target, err := syscall.UTF16PtrFromString(rememberedCredentialTarget)
	if err != nil {
		return "", "", false
	}
	var pc uintptr
	r, _, _ := pCredReadW.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&pc)))
	if r == 0 || pc == 0 {
		return "", "", false
	}
	defer pCredFree.Call(pc)
	c := (*credentialW)(unsafe.Pointer(pc))
	if c.UserName != nil {
		login = syscall.UTF16ToString((*[32768]uint16)(unsafe.Pointer(c.UserName))[:])
	}
	if c.CredentialBlob != nil && c.CredentialBlobSize > 0 {
		b := append([]byte(nil), unsafe.Slice(c.CredentialBlob, int(c.CredentialBlobSize))...)
		password = string(b)
		for i := range b {
			b[i] = 0
		}
	}
	login = strings.TrimSpace(login)
	return login, password, login != "" && password != ""
}

func clearCredentialManager() {
	target, err := syscall.UTF16PtrFromString(rememberedCredentialTarget)
	if err != nil {
		return
	}
	_, _, _ = pCredDeleteW.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
}

type rememberedCredential struct {
	SchemaVersion int    `json:"schema_version"`
	Login         string `json:"login"`
	PasswordDPAPI string `json:"password_dpapi"`
}

func rememberedCredentialPath() string {
	return filepath.Join(sharedDataDir(), "remembered_login.json")
}

func bytesToBlob(b []byte) dataBlob {
	if len(b) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

func blobBytes(b dataBlob) []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(b.pbData, int(b.cbData))...)
}

func protectForCurrentWindowsUser(plain []byte) ([]byte, error) {
	in := bytesToBlob(plain)
	entropyBytes := []byte("ROSA:remember-user:v1")
	entropy := bytesToBlob(entropyBytes)
	var out dataBlob
	r, _, e := pCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		uintptr(unsafe.Pointer(&entropy)),
		0,
		0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("DPAPI CryptProtectData: %v", e)
	}
	defer pLocalFreeRemember.Call(uintptr(unsafe.Pointer(out.pbData)))
	return blobBytes(out), nil
}

func unprotectForCurrentWindowsUser(sealed []byte) ([]byte, error) {
	in := bytesToBlob(sealed)
	entropyBytes := []byte("ROSA:remember-user:v1")
	entropy := bytesToBlob(entropyBytes)
	var out dataBlob
	r, _, e := pCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		uintptr(unsafe.Pointer(&entropy)),
		0,
		0,
		cryptprotectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("DPAPI CryptUnprotectData: %v", e)
	}
	defer pLocalFreeRemember.Call(uintptr(unsafe.Pointer(out.pbData)))
	return blobBytes(out), nil
}

func saveRememberedCredential(login, password string) error {
	// RC188: Microsoft recommends Windows Credential Manager for new desktop
	// applications. DPAPI file storage is retained only as a compatibility fallback.
	if err := saveCredentialManager(login, password); err == nil {
		_ = os.Remove(rememberedCredentialPath())
		return nil
	}
	login = strings.TrimSpace(login)
	if login == "" || password == "" {
		return fmt.Errorf("пустые данные для запоминания")
	}
	sealed, err := protectForCurrentWindowsUser([]byte(password))
	if err != nil {
		return err
	}
	rc := rememberedCredential{SchemaVersion: 1, Login: login, PasswordDPAPI: base64Encode(sealed)}
	b, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(rememberedCredentialPath()), 0700); err != nil {
		return err
	}
	return atomicWriteFile(rememberedCredentialPath(), b, 0600)
}

func loadRememberedCredential() (login, password string, ok bool) {
	if login, password, ok = loadCredentialManager(); ok {
		return login, password, true
	}
	// Legacy DPAPI migration path.
	b, err := os.ReadFile(rememberedCredentialPath())
	if err != nil {
		return "", "", false
	}
	var rc rememberedCredential
	if json.Unmarshal(b, &rc) != nil || rc.SchemaVersion != 1 || strings.TrimSpace(rc.Login) == "" {
		return "", "", false
	}
	sealed, err := base64Decode(rc.PasswordDPAPI)
	if err != nil {
		return "", "", false
	}
	plain, err := unprotectForCurrentWindowsUser(sealed)
	if err != nil {
		return "", "", false
	}
	defer func() {
		for i := range plain {
			plain[i] = 0
		}
	}()
	login, password = strings.TrimSpace(rc.Login), string(plain)
	if login != "" && password != "" {
		if saveCredentialManager(login, password) == nil {
			_ = os.Remove(rememberedCredentialPath())
		}
		return login, password, true
	}
	return "", "", false
}

func clearRememberedCredential() {
	clearCredentialManager()
	_ = os.Remove(rememberedCredentialPath())
}

func base64Encode(b []byte) string          { return base64.StdEncoding.EncodeToString(b) }
func base64Decode(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }
