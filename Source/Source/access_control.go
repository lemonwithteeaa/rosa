//go:build windows

package main

import (
	"archive/zip"
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	ID_SHARE_SELECTED = 260
	ID_SHARED_WITH_ME = 261
	ID_MANAGE_SHARES  = 262

	ID_SHARE_USERS    = 9401
	ID_SHARE_VIEW     = 9402
	ID_SHARE_REVIEW   = 9403
	ID_SHARE_COPY     = 9404
	ID_SHARE_CANCEL   = 9405
	ID_ACCESS_LIST    = 9410
	ID_ACCESS_PREVIEW = 9411
	ID_ACCESS_COPY    = 9412
	ID_ACCESS_REVOKE  = 9413
	ID_ACCESS_CLOSE   = 9414
)

const (
	accessPermissionView   = "view"
	accessPermissionReview = "review"
	accessPermissionCopy   = "copy"
)

var sharedPackageMagic = []byte("ROSASHARE1\x00")

const sharedPackageChunk = 1 << 20

type AccessGrant struct {
	GrantID         string `json:"grant_id"`
	OwnerUserID     string `json:"owner_user_id"`
	RecipientUserID string `json:"recipient_user_id"`
	Permission      string `json:"permission"`
	ResourceType    string `json:"resource_type"`
	ResourceID      string `json:"resource_id"`
	CreatedAt       string `json:"created_at"`
	RevokedAt       string `json:"revoked_at,omitempty"`
	PackageFile     string `json:"package_file"`
	WrappedKey      string `json:"wrapped_key"`
}

var accessMu sync.Mutex

func accessGrantsPath() string  { return filepath.Join(sharedDataDir(), "access_grants.json") }
func accessPackagesDir() string { return filepath.Join(sharedDataDir(), "access_packages") }

func loadAccessGrants() []AccessGrant {
	accessMu.Lock()
	defer accessMu.Unlock()
	var out []AccessGrant
	b, err := os.ReadFile(accessGrantsPath())
	if err == nil {
		_ = json.Unmarshal(b, &out)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func saveAccessGrants(grants []AccessGrant) error {
	b, err := json.MarshalIndent(grants, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(accessGrantsPath(), b, 0600)
}

func permissionLabel(p string) string {
	switch p {
	case accessPermissionView:
		return "Просмотр"
	case accessPermissionReview:
		return "Проверка"
	case accessPermissionCopy:
		return "Рабочая версия"
	default:
		return p
	}
}

func accountLabelByID(id string) string {
	a, ok := localAccountByID(id)
	if !ok {
		return id
	}
	if strings.TrimSpace(a.DisplayName) != "" {
		return a.DisplayName
	}
	return a.Login
}

func randomAccessID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("grant-%x", b)
}

func encryptFileWithContentKey(src, dst string, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("некорректный ключ пакета доступа")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	bw := bufio.NewWriterSize(out, sharedPackageChunk)
	if _, err = bw.Write(sharedPackageMagic); err != nil {
		_ = out.Close()
		return err
	}
	if err = binary.Write(bw, binary.LittleEndian, uint32(sharedPackageChunk)); err != nil {
		_ = out.Close()
		return err
	}
	buf := make([]byte, sharedPackageChunk)
	idx := uint64(0)
	for {
		n, er := in.Read(buf)
		if n > 0 {
			nonce := make([]byte, aead.NonceSize())
			if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
				_ = out.Close()
				return err
			}
			aad := make([]byte, 8)
			binary.LittleEndian.PutUint64(aad, idx)
			sealed := aead.Seal(nil, nonce, buf[:n], aad)
			if err = binary.Write(bw, binary.LittleEndian, uint32(n)); err != nil {
				_ = out.Close()
				return err
			}
			if _, err = bw.Write(nonce); err != nil {
				_ = out.Close()
				return err
			}
			if _, err = bw.Write(sealed); err != nil {
				_ = out.Close()
				return err
			}
			idx++
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			_ = out.Close()
			return er
		}
	}
	if err = bw.Flush(); err != nil {
		_ = out.Close()
		return err
	}
	if err = out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if err = replaceFileAtomic(tmp, dst); err != nil {
		return err
	}
	ok = true
	return nil
}

func decryptFileWithContentKey(src, dst string, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("некорректный ключ пакета доступа")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	br := bufio.NewReaderSize(in, sharedPackageChunk)
	magic := make([]byte, len(sharedPackageMagic))
	if _, err = io.ReadFull(br, magic); err != nil || string(magic) != string(sharedPackageMagic) {
		return fmt.Errorf("повреждён защищённый пакет доступа")
	}
	var chunk uint32
	if err = binary.Read(br, binary.LittleEndian, &chunk); err != nil || chunk == 0 || chunk > 16<<20 {
		return fmt.Errorf("повреждён заголовок пакета доступа")
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
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
			_ = out.Close()
			return er
		}
		if plainLen == 0 || plainLen > chunk {
			_ = out.Close()
			return fmt.Errorf("повреждён блок пакета доступа")
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err = io.ReadFull(br, nonce); err != nil {
			_ = out.Close()
			return err
		}
		ct := make([]byte, int(plainLen)+aead.Overhead())
		if _, err = io.ReadFull(br, ct); err != nil {
			_ = out.Close()
			return err
		}
		aad := make([]byte, 8)
		binary.LittleEndian.PutUint64(aad, idx)
		plain, err := aead.Open(nil, nonce, ct, aad)
		if err != nil {
			_ = out.Close()
			return fmt.Errorf("не удалось открыть блок пакета доступа")
		}
		if _, err = out.Write(plain); err != nil {
			_ = out.Close()
			return err
		}
		idx++
	}
	if err = out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if err = replaceFileAtomic(tmp, dst); err != nil {
		return err
	}
	ok = true
	return nil
}

func materializeGrantPackage(g AccessGrant) (string, error) {
	if currentSession == nil || currentSession.UserID != g.RecipientUserID {
		return "", fmt.Errorf("этот доступ выдан другому пользователю")
	}
	priv, err := currentPrivateKey()
	if err != nil {
		return "", err
	}
	wrapped, err := base64.StdEncoding.DecodeString(g.WrappedKey)
	if err != nil {
		return "", err
	}
	key, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, wrapped, []byte(g.GrantID))
	if err != nil {
		return "", fmt.Errorf("не удалось открыть ключ доступа: %w", err)
	}
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	td, err := ensureSessionTempDir()
	if err != nil {
		return "", err
	}
	dst := filepath.Join(td, g.GrantID+".rosa")
	if _, err = os.Stat(dst); err == nil {
		return dst, nil
	}
	src := filepath.Join(accessPackagesDir(), filepath.Base(g.PackageFile))
	if err = decryptFileWithContentKey(src, dst, key); err != nil {
		return "", err
	}
	return dst, nil
}

func previewRosaPackage(path string) (RosaManifest, RosaPackageData, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return RosaManifest{}, RosaPackageData{}, err
	}
	defer z.Close()
	var manifest RosaManifest
	found := false
	for _, f := range z.File {
		if f.Name != "manifest.json" {
			continue
		}
		r, e := f.Open()
		if e != nil {
			return manifest, RosaPackageData{}, e
		}
		b, e := io.ReadAll(io.LimitReader(r, 2<<20))
		r.Close()
		if e != nil {
			return manifest, RosaPackageData{}, e
		}
		if e = json.Unmarshal(b, &manifest); e != nil {
			return manifest, RosaPackageData{}, e
		}
		found = true
		break
	}
	if !found || manifest.Format != "ROSA_OFFLINE_EXCHANGE" {
		return manifest, RosaPackageData{}, fmt.Errorf("пакет РОСА не распознан")
	}
	var data RosaPackageData
	if err = readZipJSON(z, "data/package.json", &data, manifest); err != nil {
		return manifest, data, err
	}
	return manifest, data, nil
}

func createGrantForVersion(v VersionRecord, recipientID, permission string) error {
	if currentSession == nil {
		return fmt.Errorf("пользователь не авторизован")
	}
	if recipientID == "" || recipientID == currentSession.UserID {
		return fmt.Errorf("выберите другого пользователя")
	}
	recipient, ok := localAccountByID(recipientID)
	if !ok || recipient.Disabled {
		return fmt.Errorf("получатель недоступен")
	}
	pub, err := parseAccountPublicKey(recipient)
	if err != nil {
		return err
	}
	grantID := randomAccessID()
	td, err := ensureSessionTempDir()
	if err != nil {
		return err
	}
	plain := filepath.Join(td, grantID+".rosa")
	if err = writeRosaPackage(plain, "shared_calculation", []VersionRecord{v}, false); err != nil {
		return err
	}
	defer os.Remove(plain)
	contentKey := make([]byte, 32)
	if _, err = rand.Read(contentKey); err != nil {
		return err
	}
	defer func() {
		for i := range contentKey {
			contentKey[i] = 0
		}
	}()
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, contentKey, []byte(grantID))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(accessPackagesDir(), 0700); err != nil {
		return err
	}
	packageFile := grantID + ".rsh"
	if err = encryptFileWithContentKey(plain, filepath.Join(accessPackagesDir(), packageFile), contentKey); err != nil {
		return err
	}
	grants := loadAccessGrants()
	grants = append(grants, AccessGrant{
		GrantID: grantID, OwnerUserID: currentSession.UserID, RecipientUserID: recipientID,
		Permission: permission, ResourceType: "calculation", ResourceID: v.VersionID,
		CreatedAt: time.Now().Format(time.RFC3339), PackageFile: packageFile,
		WrappedKey: base64.StdEncoding.EncodeToString(wrapped),
	})
	if err = saveAccessGrants(grants); err != nil {
		_ = os.Remove(filepath.Join(accessPackagesDir(), packageFile))
		return err
	}
	return nil
}

func receivedActiveGrants() []AccessGrant {
	if currentSession == nil {
		return nil
	}
	var out []AccessGrant
	for _, g := range loadAccessGrants() {
		if g.RecipientUserID == currentSession.UserID && g.RevokedAt == "" {
			out = append(out, g)
		}
	}
	return out
}

func ownedActiveGrants() []AccessGrant {
	if currentSession == nil {
		return nil
	}
	var out []AccessGrant
	for _, g := range loadAccessGrants() {
		if g.OwnerUserID == currentSession.UserID && g.RevokedAt == "" {
			out = append(out, g)
		}
	}
	return out
}

func revokeGrant(grantID string) error {
	if currentSession == nil {
		return fmt.Errorf("пользователь не авторизован")
	}
	grants := loadAccessGrants()
	found := false
	for i := range grants {
		if grants[i].GrantID != grantID {
			continue
		}
		if grants[i].OwnerUserID != currentSession.UserID {
			return fmt.Errorf("закрыть доступ может только владелец")
		}
		if grants[i].RevokedAt == "" {
			grants[i].RevokedAt = time.Now().Format(time.RFC3339)
			_ = os.Remove(filepath.Join(accessPackagesDir(), filepath.Base(grants[i].PackageFile)))
		}
		found = true
		break
	}
	if !found {
		return fmt.Errorf("доступ не найден")
	}
	return saveAccessGrants(grants)
}

// --- compact modal dialogs -------------------------------------------------

var (
	shareDialogDone       bool
	shareDialogRecipient  string
	shareDialogPermission string
	shareDialogUsers      []LocalAccount
	shareDialogList       syscall.Handle
	shareDialogFont       syscall.Handle

	accessDialogDone   bool
	accessDialogAction string
	accessDialogGrant  string
	accessDialogMode   string
	accessDialogItems  []AccessGrant
	accessDialogList   syscall.Handle
)

func modalCreate(class, text string, style uint32, x, y, w, h int32, parent syscall.Handle, id int) syscall.Handle {
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(p(class))), uintptr(unsafe.Pointer(p(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	z := syscall.Handle(hwnd)
	font := hFont
	if shareDialogFont != 0 {
		font = shareDialogFont
	}
	if z != 0 && font != 0 {
		pSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	registerButtonContextHelp(z, class)
	return z
}

func selectedListIndex(h syscall.Handle) int {
	r, _, _ := pSendMessageW.Call(uintptr(h), LB_GETCURSEL, 0, 0)
	return int(int32(r))
}

func shareDialogWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		modalCreate("STATIC", "Кому открыть доступ", WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 18, 430, 28, syscall.Handle(hwnd), 0)
		modalCreate("STATIC", "Получатель увидит только явно переданный расчёт, а не ваш личный архив.", WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 48, 500, 40, syscall.Handle(hwnd), 0)
		shareDialogList = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|LBS_NOTIFY, 22, 94, 500, 150, syscall.Handle(hwnd), ID_SHARE_USERS)
		for _, a := range shareDialogUsers {
			name := a.DisplayName
			if name == "" {
				name = a.Login
			}
			if a.PublicKey == "" {
				name += "  (нужно войти один раз)"
			}
			pSendMessageW.Call(uintptr(shareDialogList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(name))))
		}
		modalCreate("BUTTON", "Просмотр", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 22, 266, 115, 36, syscall.Handle(hwnd), ID_SHARE_VIEW)
		modalCreate("BUTTON", "Проверка", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 145, 266, 115, 36, syscall.Handle(hwnd), ID_SHARE_REVIEW)
		modalCreate("BUTTON", "Рабочая версия", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 268, 266, 140, 36, syscall.Handle(hwnd), ID_SHARE_COPY)
		modalCreate("BUTTON", "Отмена", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 416, 266, 106, 36, syscall.Handle(hwnd), ID_SHARE_CANCEL)
		return 0
	case WM_COMMAND:
		id := int(loWord(wParam))
		if id == ID_SHARE_CANCEL {
			shareDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
		if id == ID_SHARE_VIEW || id == ID_SHARE_REVIEW || id == ID_SHARE_COPY {
			i := selectedListIndex(shareDialogList)
			if i < 0 || i >= len(shareDialogUsers) {
				pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(p("Выберите пользователя."))), uintptr(unsafe.Pointer(p("Доступ"))), MB_OK|MB_ICONWARNING)
				return 0
			}
			shareDialogRecipient = shareDialogUsers[i].ID
			switch id {
			case ID_SHARE_VIEW:
				shareDialogPermission = accessPermissionView
			case ID_SHARE_REVIEW:
				shareDialogPermission = accessPermissionReview
			case ID_SHARE_COPY:
				shareDialogPermission = accessPermissionCopy
			}
			shareDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case 0x0010: // WM_CLOSE
		shareDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func chooseShareRecipient() (string, string, bool) {
	if currentSession == nil {
		return "", "", false
	}
	shareDialogUsers = nil
	for _, a := range loadAuthStore().Accounts {
		if !a.Disabled && a.ID != currentSession.UserID {
			shareDialogUsers = append(shareDialogUsers, a)
		}
	}
	if len(shareDialogUsers) == 0 {
		msg("Доступ", "На этом устройстве пока нет другого пользователя.", MB_OK|MB_ICONINFORMATION)
		return "", "", false
	}
	shareDialogDone, shareDialogRecipient, shareDialogPermission = false, "", ""
	cls := p("ROSAShareDialogV1")
	hInstRaw, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(shareDialogWndProc), HInstance: syscall.Handle(hInstRaw), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — открыть доступ"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 560, 360, uintptr(hMain), 0, hInstRaw, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return "", "", false
	}
	var m MSG
	for !shareDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return shareDialogRecipient, shareDialogPermission, shareDialogRecipient != "" && shareDialogPermission != ""
}

func accessDialogWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		title := "Доступно мне"
		if accessDialogMode == "owned" {
			title = "Кому открыт доступ"
		}
		modalCreate("STATIC", title, WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 18, 470, 28, syscall.Handle(hwnd), 0)
		accessDialogList = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|LBS_NOTIFY, 22, 56, 580, 210, syscall.Handle(hwnd), ID_ACCESS_LIST)
		for _, g := range accessDialogItems {
			party := accountLabelByID(g.OwnerUserID)
			if accessDialogMode == "owned" {
				party = accountLabelByID(g.RecipientUserID)
			}
			line := fmt.Sprintf("%s  |  %s  |  %s", party, permissionLabel(g.Permission), g.CreatedAt)
			pSendMessageW.Call(uintptr(accessDialogList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(line))))
		}
		if accessDialogMode == "owned" {
			modalCreate("BUTTON", "Закрыть доступ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 326, 286, 145, 36, syscall.Handle(hwnd), ID_ACCESS_REVOKE)
		} else {
			modalCreate("BUTTON", "Просмотреть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 190, 286, 128, 36, syscall.Handle(hwnd), ID_ACCESS_PREVIEW)
			modalCreate("BUTTON", "Создать мою копию", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 326, 286, 170, 36, syscall.Handle(hwnd), ID_ACCESS_COPY)
		}
		modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 504, 286, 98, 36, syscall.Handle(hwnd), ID_ACCESS_CLOSE)
		return 0
	case WM_COMMAND:
		id := int(loWord(wParam))
		if id == ID_ACCESS_CLOSE {
			accessDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
		if id == ID_ACCESS_PREVIEW || id == ID_ACCESS_COPY || id == ID_ACCESS_REVOKE {
			i := selectedListIndex(accessDialogList)
			if i < 0 || i >= len(accessDialogItems) {
				pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(p("Выберите запись доступа."))), uintptr(unsafe.Pointer(p("Доступ"))), MB_OK|MB_ICONWARNING)
				return 0
			}
			accessDialogGrant = accessDialogItems[i].GrantID
			switch id {
			case ID_ACCESS_PREVIEW:
				accessDialogAction = "preview"
			case ID_ACCESS_COPY:
				accessDialogAction = "copy"
			case ID_ACCESS_REVOKE:
				accessDialogAction = "revoke"
			}
			accessDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case 0x0010:
		accessDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func runAccessDialog(mode string, items []AccessGrant) (string, string, bool) {
	if len(items) == 0 {
		if mode == "owned" {
			msg("Доступ", "Вы ещё не открывали доступ к расчётам.", MB_OK|MB_ICONINFORMATION)
		} else {
			msg("Доступ", "Другие пользователи пока ничего вам не передавали.", MB_OK|MB_ICONINFORMATION)
		}
		return "", "", false
	}
	accessDialogMode, accessDialogItems = mode, items
	accessDialogDone, accessDialogAction, accessDialogGrant = false, "", ""
	cls := p("ROSAAccessDialogV1")
	hInstRaw, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(accessDialogWndProc), HInstance: syscall.Handle(hInstRaw), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — доступы"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 640, 380, uintptr(hMain), 0, hInstRaw, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return "", "", false
	}
	var m MSG
	for !accessDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return accessDialogGrant, accessDialogAction, accessDialogGrant != "" && accessDialogAction != ""
}

func grantByID(id string) (AccessGrant, bool) {
	for _, g := range loadAccessGrants() {
		if g.GrantID == id && g.RevokedAt == "" {
			return g, true
		}
	}
	return AccessGrant{}, false
}

func shareSelectedCalculationUI() {
	x := filteredVersions()
	i := selectedRow(historyList)
	if i < 0 || i >= len(x) {
		msg("Открыть доступ", "Выберите расчёт в истории.", MB_OK|MB_ICONWARNING)
		return
	}
	recipient, perm, ok := chooseShareRecipient()
	if !ok {
		return
	}
	if err := createGrantForVersion(x[i], recipient, perm); err != nil {
		msg("Открыть доступ", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Открыть доступ", fmt.Sprintf("Доступ открыт пользователю %s.\nУровень: %s.\n\nВаш личный архив при этом не раскрывается.", accountLabelByID(recipient), permissionLabel(perm)), MB_OK|MB_ICONINFORMATION)
}

func sharedWithMeUI() {
	grantID, action, ok := runAccessDialog("received", receivedActiveGrants())
	if !ok {
		return
	}
	g, ok := grantByID(grantID)
	if !ok {
		msg("Доступ", "Доступ уже закрыт владельцем.", MB_OK|MB_ICONWARNING)
		return
	}
	path, err := materializeGrantPackage(g)
	if err != nil {
		msg("Доступ", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	if action == "preview" {
		_, data, err := previewRosaPackage(path)
		if err != nil {
			msg("Доступ", err.Error(), MB_OK|MB_ICONERROR)
			return
		}
		if len(data.Versions) == 0 {
			msg("Доступ", "В пакете нет расчётной версии.", MB_OK|MB_ICONWARNING)
			return
		}
		v := data.Versions[0]
		text := fmt.Sprintf("Владелец: %s\nПраво: %s\n\n%s\nДата расчёта: %s\nОсложнение: %s\nМетод: %s\nМРП: %s → %s сут.\nРешение: %s\n\nПросмотр не добавляет расчёт в ваш личный архив.", accountLabelByID(g.OwnerUserID), permissionLabel(g.Permission), objectLabel(v), v.Created, v.Complication, v.Method, v.CurrentMRP, v.ForecastMRP, v.Decision)
		msg("Доступный расчёт", text, MB_OK|MB_ICONINFORMATION)
		return
	}
	if action == "copy" {
		if g.Permission != accessPermissionCopy {
			msg("Доступ", "Владелец разрешил только просмотр/проверку. Создание личной рабочей копии не разрешено.", MB_OK|MB_ICONWARNING)
			return
		}
		r, err := importRosaPackage(path)
		if err != nil {
			msg("Доступ", err.Error(), MB_OK|MB_ICONERROR)
			return
		}
		refreshAll()
		msg("Доступ", fmt.Sprintf("Расчёт добавлен в ваш личный архив как переданная версия.\nДобавлено расчётов: %d.\n\nПосле создания личной копии её нельзя отозвать удалением доступа владельцем.", r.AddedVersions), MB_OK|MB_ICONINFORMATION)
	}
}

func manageOwnedSharesUI() {
	grantID, action, ok := runAccessDialog("owned", ownedActiveGrants())
	if !ok || action != "revoke" {
		return
	}
	g, ok := grantByID(grantID)
	if !ok {
		return
	}
	text := fmt.Sprintf("Закрыть доступ пользователю %s?\n\nНовые открытия пакета будут невозможны. Уже созданная пользователем самостоятельная рабочая копия, если она была создана ранее, не удаляется.", accountLabelByID(g.RecipientUserID))
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Закрыть доступ"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	if err := revokeGrant(grantID); err != nil {
		msg("Доступ", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Доступ", "Доступ закрыт.", MB_OK|MB_ICONINFORMATION)
}
