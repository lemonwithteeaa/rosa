//go:build windows

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	ID_AUTH_LOGIN_EDIT     = 9101
	ID_AUTH_PASSWORD       = 9102
	ID_AUTH_USERS          = 9103
	ID_AUTH_LOGIN          = 9104
	ID_AUTH_CREATE         = 9105
	ID_AUTH_CANCEL         = 9106
	ID_AUTH_STATUS         = 9107
	ID_AUTH_REMEMBER       = 9108
	ID_AUTH_CITS           = 9109
	ID_AUTH_CDNG           = 9110
	ID_AUTH_POSITION       = 9111
	ID_AUTH_LASTNAME       = 9112
	ID_AUTH_NGDO           = 9113
	ID_AUTH_FIRSTNAME      = 9114
	ID_AUTH_MIDDLENAME     = 9115
	ID_AUTH_FORGOT         = 9116
	ID_AUTH_GUEST          = 9117
	ID_AUTH_RECOVERY_CLOSE = 9118
	ID_AUTH_SHOW_PASSWORD  = 9119

	LB_ADDSTRING          = 0x0180
	LB_GETCURSEL          = 0x0188
	LB_GETTEXT            = 0x0189
	LB_RESETCONTENT       = 0x0184
	LB_SETCURSEL          = 0x0186
	LBN_SELCHANGE         = 1
	LBS_OWNERDRAWFIXED    = 0x0010
	LBS_HASSTRINGS        = 0x0040
	LB_SETITEMHEIGHT      = 0x01A0
	ODT_LISTBOX           = 2
	ODT_STATIC            = 5
	ES_PASSWORD           = 0x0020
	ES_CENTER             = 0x0001
	BS_AUTOCHECKBOX       = 0x00000003
	BM_GETCHECK           = 0x00F0
	BM_SETCHECK           = 0x00F1
	BST_CHECKED           = 1
	ES_AUTOHSCROLL        = 0x0080
	EN_SETFOCUS           = 0x0100
	EN_KILLFOCUS          = 0x0200
	EN_CHANGE             = 0x0300
	EM_SETPASSWORDCHAR    = 0x00CC
	CBS_DROPDOWN_AUTH     = 0x0002
	CBN_SETFOCUS_AUTH     = 3
	WM_ERASEBKGND_AUTH    = 0x0014
	SRCCOPY_AUTH          = 0x00CC0020
	HALFTONE_AUTH         = 4
	WS_EX_COMPOSITED_AUTH = 0x02000000
	WS_CLIPCHILDREN_AUTH  = 0x02000000
	WS_MAXIMIZE_AUTH      = 0x01000000
	SIZE_MINIMIZED_AUTH   = 1
	WM_AUTH_FULL_REDRAW   = 0x8000 + 0x51
)

type LocalAccount struct {
	ID                 string `json:"id"`
	Login              string `json:"login"`
	DisplayName        string `json:"display_name"`
	LastName           string `json:"last_name,omitempty"`
	FirstName          string `json:"first_name,omitempty"`
	MiddleName         string `json:"middle_name,omitempty"`
	CITS               string `json:"cits,omitempty"`
	CDNG               string `json:"cdng,omitempty"`
	NGDO               string `json:"ngdo,omitempty"`
	Position           string `json:"position,omitempty"`
	Salt               string `json:"salt"`
	PasswordHash       string `json:"password_hash"`
	Iterations         int    `json:"iterations"`
	SecurityQuestion   string `json:"security_question,omitempty"`
	SecurityAnswerSalt string `json:"security_answer_salt,omitempty"`
	SecurityAnswerHash string `json:"security_answer_hash,omitempty"`
	SecurityIterations int    `json:"security_iterations,omitempty"`
	IsAdmin            bool   `json:"is_admin"`
	Disabled           bool   `json:"disabled"`
	CreatedAt          string `json:"created_at"`
	PublicKey          string `json:"public_key,omitempty"`
	KeyAlgorithm       string `json:"key_algorithm,omitempty"`
}

type AuthStore struct {
	SchemaVersion int            `json:"schema_version"`
	Accounts      []LocalAccount `json:"accounts"`
}

type UserSession struct {
	UserID      string
	Login       string
	DisplayName string
	LastName    string
	FirstName   string
	MiddleName  string
	CITS        string
	CDNG        string
	NGDO        string
	Position    string
	IsAdmin     bool
	IsGuest     bool
	TempDir     string
	masterKey   []byte
}

var currentSession *UserSession

func appRootDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "OFModel")
}

func sharedDataDir() string        { return filepath.Join(appRootDir(), "shared") }
func accountsPath() string         { return filepath.Join(sharedDataDir(), "accounts.json") }
func userBaseDir(id string) string { return filepath.Join(appRootDir(), "users", id) }
func userDataDir(id string) string { return filepath.Join(userBaseDir(id), "data") }
func identityPrivatePath(id string) string {
	return filepath.Join(userBaseDir(id), "identity_private.bin")
}

func sanitizeAccountID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() < 3 {
		raw := make([]byte, 8)
		_, _ = rand.Read(raw)
		return fmt.Sprintf("user-%x", raw)
	}
	return b.String()
}

func normalizeLogin(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func loadAuthStore() AuthStore {
	st := AuthStore{SchemaVersion: 1}
	b, err := os.ReadFile(accountsPath())
	if err == nil {
		if json.Unmarshal(b, &st) != nil || st.SchemaVersion <= 0 {
			st = AuthStore{SchemaVersion: 1}
		}
	}
	// Upgrade only the uniquely identified legacy owner named «Катя». Keep
	// the account ID, login, password hashes, keys, permissions and data paths.
	// A real account with a different identity must never be renamed.
	if err == nil {
		updated := st
		updated.Accounts = append([]LocalAccount(nil), st.Accounts...)
		changedName := upgradeLegacyOwnerProfile(&updated)
		changedOrganization := upgradeOwnerOrganizationProfile(&updated)
		if changedName || changedOrganization {
			backup := filepath.Join(sharedDataDir(), "accounts.before-owner-profile-821.json")
			if _, backupErr := os.Stat(backup); os.IsNotExist(backupErr) {
				if backupErr = atomicWriteFile(backup, b, 0600); backupErr != nil {
					diagLog("AUTH profile backup failed: %v", backupErr)
				}
			}
			if _, backupErr := os.Stat(backup); backupErr == nil {
				if saveErr := saveAuthStore(updated); saveErr == nil {
					st = updated
				} else {
					diagLog("AUTH profile update failed: %v", saveErr)
				}
			}
		}
	}
	sort.SliceStable(st.Accounts, func(i, j int) bool {
		return strings.ToLower(st.Accounts[i].DisplayName) < strings.ToLower(st.Accounts[j].DisplayName)
	})
	return st
}

// Only upgrade an unambiguous old short-name account. No password reset,
// duplicate user, or privilege change is allowed.
func upgradeLegacyOwnerProfile(st *AuthStore) bool {
	const legacy = "Катя"
	const desired = "Ромадина Екатерина Викторовна"
	if st == nil {
		return false
	}
	idx, matches := -1, 0
	for i, a := range st.Accounts {
		last, first, middle := accountPersonNames(a)
		if strings.EqualFold(strings.TrimSpace(a.DisplayName), desired) ||
			(strings.EqualFold(last, "Ромадина") && strings.EqualFold(first, "Екатерина")) {
			return false // a corrected identity is already registered
		}
		legacyDisplay := strings.EqualFold(strings.TrimSpace(a.DisplayName), legacy) && strings.TrimSpace(a.LastName) == ""
		legacyFields := last == "" && middle == "" && strings.EqualFold(first, legacy)
		legacyLogin := strings.EqualFold(strings.TrimSpace(a.Login), legacy) && strings.TrimSpace(a.LastName) == ""
		if legacyDisplay || legacyFields || legacyLogin {
			idx, matches = i, matches+1
		}
	}
	if matches != 1 {
		return false
	}
	a := &st.Accounts[idx]
	a.DisplayName = desired
	a.LastName, a.FirstName, a.MiddleName = "Ромадина", "Екатерина", "Викторовна"
	return true
}

// Fill the owner affiliation for an unambiguous matching identity only.
// Leave all credentials, keys, account IDs and unrelated accounts untouched.
func upgradeOwnerOrganizationProfile(st *AuthStore) bool {
	if st == nil {
		return false
	}
	index := -1
	for i, a := range st.Accounts {
		last, first, middle := accountPersonNames(a)
		if strings.EqualFold(strings.TrimSpace(last), "Ромадина") &&
			strings.EqualFold(strings.TrimSpace(first), "Екатерина") &&
			strings.EqualFold(strings.TrimSpace(middle), "Викторовна") {
			if index >= 0 {
				return false
			} // Ambiguous identity; never change either.
			index = i
		}
	}
	if index < 0 {
		return false
	}
	a := &st.Accounts[index]
	if a.NGDO == "РИТЭК" && a.CITS == "АУ" && a.CDNG == "-" && a.Position == "ведущий инженер" {
		return false
	}
	// Align the owner's organization fields with the actual authorization
	// directory. Never change credentials, role flags, keys or account ID.
	a.NGDO, a.CITS, a.CDNG, a.Position = "РИТЭК", "АУ", "-", "ведущий инженер"
	return true
}

func saveAuthStore(st AuthStore) error {
	if st.SchemaVersion == 0 {
		st.SchemaVersion = 1
	}
	if err := os.MkdirAll(sharedDataDir(), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(accountsPath(), b, 0600)
}

// pbkdf2SHA256 is implemented locally to keep the offline Windows build self-contained.
func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	if iterations < 1 {
		iterations = 1
	}
	hLen := 32
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

func derivePasswordKey(password string, salt []byte, iterations int) []byte {
	return pbkdf2SHA256([]byte(password), salt, iterations, 32)
}

func secureEqualB64(a string, b []byte) bool {
	x, err := base64.StdEncoding.DecodeString(a)
	if err != nil {
		return false
	}
	return hmac.Equal(x, b)
}

const authSecurityQuestion = "Какое контрольное слово вы указали при создании профиля?"

func normalizeSecurityAnswer(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func setAccountSecurityAnswer(accountID, answer string) error {
	answer = normalizeSecurityAnswer(answer)
	if len([]rune(answer)) < 4 {
		return fmt.Errorf("контрольное слово должно содержать не менее 4 символов")
	}
	st := loadAuthStore()
	for i := range st.Accounts {
		if st.Accounts[i].ID != accountID {
			continue
		}
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return err
		}
		const iterations = 180000
		key := derivePasswordKey(answer, salt, iterations)
		st.Accounts[i].SecurityQuestion = authSecurityQuestion
		st.Accounts[i].SecurityAnswerSalt = base64.StdEncoding.EncodeToString(salt)
		st.Accounts[i].SecurityAnswerHash = base64.StdEncoding.EncodeToString(key)
		st.Accounts[i].SecurityIterations = iterations
		for j := range key {
			key[j] = 0
		}
		return saveAuthStore(st)
	}
	return fmt.Errorf("учётная запись не найдена")
}

func verifyAccountSecurityAnswer(a LocalAccount, answer string) bool {
	if strings.TrimSpace(a.SecurityAnswerHash) == "" || strings.TrimSpace(a.SecurityAnswerSalt) == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(a.SecurityAnswerSalt)
	if err != nil {
		return false
	}
	it := a.SecurityIterations
	if it < 10000 {
		it = 180000
	}
	key := derivePasswordKey(normalizeSecurityAnswer(answer), salt, it)
	ok := secureEqualB64(a.SecurityAnswerHash, key)
	for i := range key {
		key[i] = 0
	}
	return ok
}

func authAccountByLogin(login string) (LocalAccount, bool) {
	login = normalizeLogin(login)
	for _, a := range loadAuthStore().Accounts {
		if normalizeLogin(a.Login) == login {
			return a, true
		}
	}
	return LocalAccount{}, false
}

var identityMagic = []byte("ROSAID1\x00")

func sealIdentityWithKey(key, plain []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("некорректный ключ локальной учётной записи")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, plain, identityMagic)
	out := make([]byte, 0, len(identityMagic)+len(nonce)+len(sealed))
	out = append(out, identityMagic...)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

func openIdentityWithKey(key, blob []byte) ([]byte, error) {
	if len(key) != 32 || len(blob) < len(identityMagic) || string(blob[:len(identityMagic)]) != string(identityMagic) {
		return nil, fmt.Errorf("повреждён локальный ключ пользователя")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	off := len(identityMagic)
	if len(blob) < off+aead.NonceSize() {
		return nil, fmt.Errorf("повреждён локальный ключ пользователя")
	}
	return aead.Open(nil, blob[off:off+aead.NonceSize()], blob[off+aead.NonceSize():], identityMagic)
}

func updateAccountIdentity(updated LocalAccount) error {
	st := loadAuthStore()
	for i := range st.Accounts {
		if st.Accounts[i].ID == updated.ID {
			st.Accounts[i].PublicKey = updated.PublicKey
			st.Accounts[i].KeyAlgorithm = updated.KeyAlgorithm
			return saveAuthStore(st)
		}
	}
	return fmt.Errorf("учётная запись не найдена")
}

// ensureLocalIdentity equips every offline user with a local asymmetric identity.
// The public key is stored in the shared account directory; the private key is
// encrypted by the user's password-derived key and is never stored in plaintext.
// This is the basis for granting access to a calculation without exposing the
// owner's personal archive or password.
func ensureLocalIdentity(a LocalAccount, passwordKey []byte) (LocalAccount, error) {
	privPath := identityPrivatePath(a.ID)
	if strings.TrimSpace(a.PublicKey) != "" {
		if _, err := os.Stat(privPath); err == nil {
			return a, nil
		}
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return a, fmt.Errorf("не удалось создать локальный ключ пользователя: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return a, err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return a, err
	}
	sealed, err := sealIdentityWithKey(passwordKey, privDER)
	if err != nil {
		return a, err
	}
	if err = os.MkdirAll(userBaseDir(a.ID), 0700); err != nil {
		return a, err
	}
	if err = atomicWriteFile(privPath, sealed, 0600); err != nil {
		return a, err
	}
	a.PublicKey = base64.StdEncoding.EncodeToString(pubDER)
	a.KeyAlgorithm = "RSA-OAEP-SHA256"
	if err = updateAccountIdentity(a); err != nil {
		return a, err
	}
	return a, nil
}

func localAccountByID(id string) (LocalAccount, bool) {
	for _, a := range loadAuthStore().Accounts {
		if a.ID == id {
			return a, true
		}
	}
	return LocalAccount{}, false
}

func parseAccountPublicKey(a LocalAccount) (*rsa.PublicKey, error) {
	if strings.TrimSpace(a.PublicKey) == "" {
		return nil, fmt.Errorf("у пользователя %s ещё нет ключа обмена; ему нужно один раз войти в РОСА", a.DisplayName)
	}
	der, err := base64.StdEncoding.DecodeString(a.PublicKey)
	if err != nil {
		return nil, err
	}
	v, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	pub, ok := v.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("неподдерживаемый тип открытого ключа")
	}
	return pub, nil
}

func currentPrivateKey() (*rsa.PrivateKey, error) {
	if currentSession == nil || len(currentSession.masterKey) != 32 {
		return nil, fmt.Errorf("пользователь не авторизован")
	}
	blob, err := os.ReadFile(identityPrivatePath(currentSession.UserID))
	if err != nil {
		return nil, fmt.Errorf("не найден локальный ключ пользователя: %w", err)
	}
	der, err := openIdentityWithKey(currentSession.masterKey, blob)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range der {
			der[i] = 0
		}
	}()
	v, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	priv, ok := v.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("неподдерживаемый тип закрытого ключа")
	}
	return priv, nil
}

func createLocalAccount(login, password string, first bool) (LocalAccount, error) {
	login = strings.TrimSpace(login)
	if len([]rune(login)) < 2 {
		return LocalAccount{}, fmt.Errorf("имя пользователя должно содержать не менее 2 символов")
	}
	if len([]rune(password)) < 6 {
		return LocalAccount{}, fmt.Errorf("пароль должен содержать не менее 6 символов")
	}
	st := loadAuthStore()
	for _, a := range st.Accounts {
		if normalizeLogin(a.Login) == normalizeLogin(login) {
			return LocalAccount{}, fmt.Errorf("пользователь с таким именем уже существует")
		}
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return LocalAccount{}, err
	}
	const iterations = 180000
	key := derivePasswordKey(password, salt, iterations)
	var err error
	idBase := sanitizeAccountID(login)
	id := idBase
	for n := 2; ; n++ {
		exists := false
		for _, a := range st.Accounts {
			if a.ID == id {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		id = fmt.Sprintf("%s-%d", idBase, n)
	}
	a := LocalAccount{
		ID: id, Login: login, DisplayName: login,
		Salt:         base64.StdEncoding.EncodeToString(salt),
		PasswordHash: base64.StdEncoding.EncodeToString(key),
		Iterations:   iterations, IsAdmin: first || len(st.Accounts) == 0,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	st.Accounts = append(st.Accounts, a)
	if err := saveAuthStore(st); err != nil {
		return LocalAccount{}, err
	}
	if err := os.MkdirAll(userDataDir(a.ID), 0700); err != nil {
		return LocalAccount{}, err
	}
	a, err = ensureLocalIdentity(a, key)
	if err != nil {
		return LocalAccount{}, err
	}
	return a, nil
}

func personFullName(lastName, firstName, middleName string) string {
	parts := []string{}
	for _, v := range []string{lastName, firstName, middleName} {
		if v = strings.TrimSpace(v); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

func splitPersonName(full string) (lastName, firstName, middleName string) {
	parts := strings.Fields(strings.TrimSpace(full))
	if len(parts) > 0 {
		lastName = parts[0]
	}
	if len(parts) > 1 {
		firstName = parts[1]
	}
	if len(parts) > 2 {
		middleName = strings.Join(parts[2:], " ")
	}
	return
}

func accountPersonNames(a LocalAccount) (lastName, firstName, middleName string) {
	lastName, firstName, middleName = strings.TrimSpace(a.LastName), strings.TrimSpace(a.FirstName), strings.TrimSpace(a.MiddleName)
	if lastName == "" && firstName == "" && middleName == "" {
		return splitPersonName(a.DisplayName)
	}
	return
}

func firstInitial(s string) string {
	for _, r := range strings.TrimSpace(s) {
		return string(r) + "."
	}
	return ""
}

func shortPersonName(lastName, firstName, middleName string) string {
	lastName = strings.TrimSpace(lastName)
	initials := firstInitial(firstName) + firstInitial(middleName)
	if lastName == "" {
		return strings.TrimSpace(initials)
	}
	if initials == "" {
		return lastName
	}
	return lastName + " " + initials
}

func accountProfileLogin(cits, cdng, fio string) string {
	return strings.TrimSpace(cits) + " | " + strings.TrimSpace(cdng) + " | " + strings.TrimSpace(fio)
}

func createLocalAccountNamedProfile(cits, cdng, ngdo, position, lastName, firstName, middleName, password string, first bool) (LocalAccount, error) {
	cits, cdng, ngdo, position = strings.TrimSpace(cits), strings.TrimSpace(cdng), strings.TrimSpace(ngdo), strings.TrimSpace(position)
	lastName, firstName, middleName = strings.TrimSpace(lastName), strings.TrimSpace(firstName), strings.TrimSpace(middleName)
	if cits == "" || cdng == "" || ngdo == "" || position == "" || lastName == "" || firstName == "" {
		return LocalAccount{}, fmt.Errorf("заполните НГДО, ЦИТС/ТПП, ЦДНГ, должность, фамилию и имя")
	}
	fullName := personFullName(lastName, firstName, middleName)
	login := accountProfileLogin(cits, cdng, fullName)
	a, err := createLocalAccount(login, password, first)
	if err != nil {
		return LocalAccount{}, err
	}
	st := loadAuthStore()
	for i := range st.Accounts {
		if st.Accounts[i].ID == a.ID {
			st.Accounts[i].DisplayName = fullName
			st.Accounts[i].LastName = lastName
			st.Accounts[i].FirstName = firstName
			st.Accounts[i].MiddleName = middleName
			st.Accounts[i].CITS = cits
			st.Accounts[i].CDNG = cdng
			st.Accounts[i].NGDO = ngdo
			st.Accounts[i].Position = position
			a = st.Accounts[i]
			break
		}
	}
	if err := saveAuthStore(st); err != nil {
		return LocalAccount{}, err
	}
	return a, nil
}

// Legacy wrapper keeps older profiles and tests compatible.
func createLocalAccountProfile(cits, cdng, ngdo, position, fio, password string, first bool) (LocalAccount, error) {
	lastName, firstName, middleName := splitPersonName(fio)
	return createLocalAccountNamedProfile(cits, cdng, ngdo, position, lastName, firstName, middleName, password, first)
}

func authenticateLocalNamedProfile(cits, cdng, ngdo, position, lastName, firstName, middleName, password string) (*UserSession, error) {
	cits, cdng, ngdo, position = strings.TrimSpace(cits), strings.TrimSpace(cdng), strings.TrimSpace(ngdo), strings.TrimSpace(position)
	lastName, firstName, middleName = strings.TrimSpace(lastName), strings.TrimSpace(firstName), strings.TrimSpace(middleName)
	st := loadAuthStore()
	for _, a := range st.Accounts {
		if a.Disabled {
			continue
		}
		al, af, am := accountPersonNames(a)
		if strings.EqualFold(strings.TrimSpace(a.CITS), cits) &&
			strings.EqualFold(strings.TrimSpace(a.CDNG), cdng) &&
			strings.EqualFold(strings.TrimSpace(a.NGDO), ngdo) &&
			strings.EqualFold(strings.TrimSpace(a.Position), position) &&
			strings.EqualFold(al, lastName) && strings.EqualFold(af, firstName) && strings.EqualFold(am, middleName) {
			return authenticateLocal(a.Login, password)
		}
	}
	return nil, fmt.Errorf("профиль с указанными организационными данными и ФИО не найден")
}

func authenticateLocalProfile(cits, cdng, ngdo, position, fio, password string) (*UserSession, error) {
	lastName, firstName, middleName := splitPersonName(fio)
	return authenticateLocalNamedProfile(cits, cdng, ngdo, position, lastName, firstName, middleName, password)
}

func authenticateLocal(login, password string) (*UserSession, error) {
	st := loadAuthStore()
	login = normalizeLogin(login)
	for _, a := range st.Accounts {
		if normalizeLogin(a.Login) != login {
			continue
		}
		if a.Disabled {
			return nil, fmt.Errorf("учётная запись заблокирована")
		}
		salt, err := base64.StdEncoding.DecodeString(a.Salt)
		if err != nil {
			return nil, fmt.Errorf("повреждены данные учётной записи")
		}
		it := a.Iterations
		if it < 10000 {
			it = 180000
		}
		key := derivePasswordKey(password, salt, it)
		if !secureEqualB64(a.PasswordHash, key) {
			for i := range key {
				key[i] = 0
			}
			return nil, fmt.Errorf("неверный пароль")
		}
		a, err = ensureLocalIdentity(a, key)
		if err != nil {
			for i := range key {
				key[i] = 0
			}
			return nil, err
		}
		lastName, firstName, middleName := accountPersonNames(a)
		return &UserSession{UserID: a.ID, Login: a.Login, DisplayName: personFullName(lastName, firstName, middleName), LastName: lastName, FirstName: firstName, MiddleName: middleName, CITS: a.CITS, CDNG: a.CDNG, NGDO: a.NGDO, Position: a.Position, IsAdmin: a.IsAdmin, masterKey: key}, nil
	}
	return nil, fmt.Errorf("пользователь не найден")
}

func clearCurrentSession() {
	finishSessionTrash()
	cleanupSessionTemp()
	if currentSession != nil {
		for i := range currentSession.masterKey {
			currentSession.masterKey[i] = 0
		}
	}
	currentSession = nil
}

func migrateLegacyDataForFirstUser(userID string) {
	legacy := filepath.Join(appRootDir(), "data")
	dst := userDataDir(userID)
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	if entries, _ := os.ReadDir(dst); len(entries) > 0 {
		return
	}
	_ = os.MkdirAll(dst, 0700)
	entries, _ := os.ReadDir(legacy)
	for _, ent := range entries {
		oldPath := filepath.Join(legacy, ent.Name())
		newPath := filepath.Join(dst, ent.Name())
		// The technology snapshot is shared between local users. Copy it to the
		// shared area, while retaining the legacy source as a fallback.
		if ent.Name() == "tech_current.json" || ent.Name() == "tech_imports.json" || ent.Name() == "technologies.json" {
			_ = os.MkdirAll(sharedDataDir(), 0700)
			if b, err := os.ReadFile(oldPath); err == nil {
				_ = atomicWriteFile(filepath.Join(sharedDataDir(), ent.Name()), b, 0600)
			}
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			if b, e := os.ReadFile(oldPath); e == nil {
				_ = atomicWriteFile(newPath, b, 0600)
			}
		}
	}
}

var pDestroyWindowAuth = user32.NewProc("DestroyWindow")
var pEllipseAuth = gdi32.NewProc("Ellipse")
var pCreatePenAuth = gdi32.NewProc("CreatePen")
var pCreateRoundRectRgnAuth = gdi32.NewProc("CreateRoundRectRgn")
var pSetWindowRgnAuth = user32.NewProc("SetWindowRgn")
var pSystemParametersInfoWAuth = user32.NewProc("SystemParametersInfoW")

type authLayoutItem struct {
	h            syscall.Handle
	x, y, w, hgt int32
}

const (
	authBaseWidth              int32 = 1060
	authHeaderHeight           int32 = 132
	authBackgroundSourceWidth  int32 = 1672
	authBackgroundSourceHeight int32 = 941
)

var (
	authWnd, authLoginEdit, authPasswordEdit, authUsersList, authStatus, authRemember                                                syscall.Handle
	authCITSEdit, authCDNGEdit, authNGDOEdit, authPositionEdit, authLastNameEdit, authFirstNameEdit, authMiddleNameEdit              syscall.Handle
	authSelectedLogin                                                                                                                string
	authFillingProfile                                                                                                               bool
	authBrand, authRosaLogo, authLukoilLogo, authRitekLogo, authProductLabel, authEntryTitle                                         syscall.Handle
	authInfoLead                                                                                                                     syscall.Handle
	authRecoveryPanel, authRecoveryAccent, authRecoveryTitle, authRecoveryText, authRecoveryClose                                    syscall.Handle
	authRosaSubtitle, authLukoilTagline, authFooterText                                                                              syscall.Handle
	authHeaderRule                                                                                                                   syscall.Handle
	authTitlePanel, authTitleAccent, authLeftBottomAccent, authEntryUnderline, authInfoLeadUnderline                                 syscall.Handle
	authLeftCard, authRightCard, authFooterCard                                                                                      syscall.Handle
	authInfoAccent, authInfoBottomAccent, authInfoPanel, authCancelButton, authThemeControl                                          syscall.Handle
	authInfoNumbers                                                                                                                  [4]syscall.Handle
	authFeatureIconControls, authNavIconControls                                                                                     [4]syscall.Handle
	authFeatureIconBitmaps, authNavIconBitmaps                                                                                       [4]syscall.Handle
	authSideLeft, authSideRight                                                                                                      syscall.Handle
	authRosaBitmap, authLukoilBitmap, authRitekBitmap, authThemeBitmap                                                               syscall.Handle
	authBackgroundBitmap                                                                                                             syscall.Handle
	authLeftCardBitmap, authRightCardBitmap                                                                                          syscall.Handle
	authSideLeftBitmap, authSideRightBitmap                                                                                          syscall.Handle
	authAccepted, authCancelled                                                                                                      bool
	authFont, authInputFont, authTitleFont, authBrandFont, authBoldFont, authButtonFont                                              syscall.Handle
	authInfoLeadFont                                                                                                                 syscall.Handle
	authFieldLabelFont, authProfileCaptionFont, authFooterFont, authBrandCaptionFont, authBodyFont, authNavLabelFont, authNumberFont syscall.Handle
	authBrushRed, authBrushBlue, authBrushLight, authBrushWhite, authBrushMuted                                                      syscall.Handle
	authBrushCard, authBrushPanel, authBrushCircle                                                                                   syscall.Handle
	authPenCircle                                                                                                                    syscall.Handle
	authCardText                                                                                                                     []syscall.Handle
	authCenteredItems                                                                                                                []authLayoutItem
	authConstructing                                                                                                                 bool
	authResizeRedrawPending                                                                                                          bool
	authDeferredVisible                                                                                                              []syscall.Handle
	authPasswordLabel, authPasswordShowButton, authLoginButton, authCreateButton, authForgotButton, authGuestButton                  syscall.Handle
	authFlowMode                                                                                                                     int
	authPasswordVisible                                                                                                              bool
	authRecoveryLogin                                                                                                                string
	authPendingCreate                                                                                                                authPendingProfile
)

type authPendingProfile struct {
	cits, cdng, ngdo, position      string
	lastName, firstName, middleName string
	password                        string
}

const (
	authFlowNormal = iota
	authFlowCreateSecurity
	authFlowRecoverSecurity
	authFlowLegacySecuritySetup
)

func authCreate(class, text string, style uint32, x, y, w, h int32, parent syscall.Handle, id int) syscall.Handle {
	// Build every visible child hidden while WM_CREATE is assembling the login UI.
	// Editable combo boxes can otherwise expose their own native child window before
	// the surrounding cards and header have painted.
	deferVisible := authConstructing && style&WS_VISIBLE != 0
	if deferVisible {
		style &^= WS_VISIBLE
	}
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(p(class))), uintptr(unsafe.Pointer(p(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	z := syscall.Handle(hwnd)
	if z != 0 && authFont != 0 {
		pSendMessageW.Call(hwnd, WM_SETFONT, uintptr(authFont), 1)
	}
	if z != 0 && deferVisible {
		authDeferredVisible = append(authDeferredVisible, z)
	}
	registerButtonContextHelp(z, class)
	return z
}

func authRevealDeferredChildren() {
	// The parent is still hidden here. Mark every intended child visible without
	// allowing individual redraws; the complete hierarchy is painted after the
	// parent becomes visible.
	const flags = 0x0001 | 0x0002 | 0x0004 | 0x0008 | 0x0010 | 0x0040 // NOSIZE|NOMOVE|NOZORDER|NOREDRAW|NOACTIVATE|SHOWWINDOW
	for _, h := range authDeferredVisible {
		if h != 0 {
			pSetWindowPos.Call(uintptr(h), 0, 0, 0, 0, 0, flags)
		}
	}
	authDeferredVisible = authDeferredVisible[:0]
}

func authCreateCentered(class, text string, style uint32, x, y, w, h int32, parent syscall.Handle, id int) syscall.Handle {
	z := authCreate(class, text, style, x, y, w, h, parent, id)
	if z != 0 {
		authCenteredItems = append(authCenteredItems, authLayoutItem{h: z, x: x, y: y, w: w, hgt: h})
	}
	return z
}

func authBackgroundCoverCrop(dstW, dstH int32) (sx, sy, cropW, cropH int32) {
	sx, sy = 0, 0
	cropW, cropH = authBackgroundSourceWidth, authBackgroundSourceHeight
	if dstW <= 0 || dstH <= 0 {
		return
	}
	// Cover the body below the fixed header without stretching. Wide viewports
	// crop only the sky from the top so the pumpjacks and lower-right RITEK mark
	// stay visible. Narrow/tall viewports crop from the left for the same reason.
	if int64(dstW)*int64(authBackgroundSourceHeight) > int64(dstH)*int64(authBackgroundSourceWidth) {
		cropH = int32(int64(authBackgroundSourceWidth) * int64(dstH) / int64(dstW))
		if cropH < 1 {
			cropH = 1
		}
		sy = authBackgroundSourceHeight - cropH
	} else {
		cropW = int32(int64(authBackgroundSourceHeight) * int64(dstW) / int64(dstH))
		if cropW < 1 {
			cropW = 1
		}
		sx = authBackgroundSourceWidth - cropW
	}
	return
}

func authCenterOffset(clientW int32) int32 {
	if clientW <= authBaseWidth {
		return 0
	}
	// Keep the approved left optical bias of the card pair so the large pumpjack
	// remains visible on the right. The header itself is centered independently.
	offset := (clientW-authBaseWidth)/2 - 210
	if offset < 0 {
		return 0
	}
	return offset
}

func authApplyCenteredLayout(clientW, clientH int32) {
	offset := authCenterOffset(clientW)
	// Center the card group in the full area below the fixed header. The background
	// itself now extends to the true bottom edge, so no footer band is reserved.
	usableBottom := clientH
	desiredTop := int32(120)
	if usableBottom > authHeaderHeight+768 {
		desiredTop = authHeaderHeight + (usableBottom-authHeaderHeight-768)/2
	}
	yOffset := desiredTop - authHeaderHeight
	if yOffset < 0 {
		yOffset = 0
	}
	for _, item := range authCenteredItems {
		if item.h == 0 {
			continue
		}
		pSetWindowPos.Call(uintptr(item.h), 0, uintptr(item.x+offset), uintptr(item.y+yOffset), uintptr(item.w), uintptr(item.hgt), 0x0004)
	}
}

func authLayoutOffsets(clientW, clientH int32) (offset, yOffset int32) {
	offset = authCenterOffset(clientW)
	desiredTop := int32(120)
	if clientH > authHeaderHeight+768 {
		desiredTop = authHeaderHeight + (clientH-authHeaderHeight-768)/2
	}
	yOffset = desiredTop - authHeaderHeight
	if yOffset < 0 {
		yOffset = 0
	}
	return
}

func authDeferMove(hdwp uintptr, h syscall.Handle, x, y, w, hgt int32) uintptr {
	if h == 0 {
		return hdwp
	}
	const flags = uintptr(0x0004 | 0x0008 | 0x0010) // NOZORDER | NOREDRAW | NOACTIVATE
	if hdwp != 0 {
		next, _, _ := pDeferWindowPos.Call(hdwp, uintptr(h), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), flags)
		if next != 0 {
			return next
		}
	}
	// Rare fallback if Begin/DeferWindowPos is unavailable or fails. Keep redraw
	// disabled here as well; the parent is repainted once after the full layout.
	pSetWindowPos.Call(uintptr(h), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), flags)
	return 0
}

// WELCOME_444_LAYOUT_BEGIN
func authApplyResizeLayout(clientW, clientH int32) {
	if clientW <= 0 || clientH <= 0 {
		return
	}
	offset, yOffset := authLayoutOffsets(clientW, clientH)
	count := len(authCenteredItems) + 12
	hdwp, _, _ := pBeginDeferWindowPos.Call(uintptr(count))

	for _, item := range authCenteredItems {
		if item.h == 0 {
			continue
		}
		hdwp = authDeferMove(hdwp, item.h, item.x+offset, item.y+yOffset, item.w, item.hgt)
	}

	hdwp = authDeferMove(hdwp, authBrand, 0, 0, clientW, authHeaderHeight)
	hdwp = authDeferMove(hdwp, authHeaderRule, 0, 128, clientW, 4)
	hdwp = authDeferMove(hdwp, authRosaLogo, 28, 4, 256, 78)
	hdwp = authDeferMove(hdwp, authRosaSubtitle, 0, 82, 312, 44)

	rightBrandX := max32(320, clientW-270)
	hdwp = authDeferMove(hdwp, authLukoilLogo, rightBrandX, 0, 258, 74)
	hdwp = authDeferMove(hdwp, authLukoilTagline, rightBrandX, 74, 258, 22)

	rightEdge := rightBrandX - 24
	if rightEdge < 1152 {
		rightEdge = 1072
	}
	hdwp = authDeferMove(hdwp, authProductLabel, 292, 30, rightEdge-292, 72)

	hdwp = authDeferMove(hdwp, authSideLeft, 0, 132, 360, 650)
	hdwp = authDeferMove(hdwp, authSideRight, max32(0, clientW-360), 132, 360, 650)

	if hdwp != 0 {
		pEndDeferWindowPos.Call(hdwp)
	}

	// Recovery controls are normally hidden; use the same no-redraw policy so an
	// open recovery panel also survives maximize/restore without trails.
	authLayoutRecoveryPanel(clientW, clientH)
}

// WELCOME_444_LAYOUT_END

// RC581 welcome hotfix: do not redesign the protected welcome page. The only
// allowed post-lock adjustment is extra horizontal breathing room for the four
// approved feature headings. It fixes DPI clipping (most visible on item 04)
// without changing wording, fonts, vertical rhythm, assets, or the locked blocks.
func authHardenFeatureTitleGeometry() {
	titles := map[string]bool{
		"ЛАБОРАТОРНАЯ АНАЛИТИКА":  true,
		"ОЦЕНКА ОСЛОЖНЕНИЙ":       true,
		"МЕТОДЫ И МЕРОПРИЯТИЯ":    true,
		"КОМПЛЕКСНАЯ ДИАГНОСТИКА": true,
	}
	for i := range authCenteredItems {
		item := &authCenteredItems[i]
		if item.h == 0 || !titles[strings.TrimSpace(getText(item.h))] {
			continue
		}
		// Keep a visible gap after the red 01–04 number while giving long
		// headings the full right-card width available at 125/150% DPI.
		item.x -= 32
		item.w += 44
	}
}

func authScheduleFullRedraw(hwnd uintptr) {
	if hwnd == 0 || authResizeRedrawPending {
		return
	}
	authResizeRedrawPending = true
	pPostMessageW.Call(hwnd, WM_AUTH_FULL_REDRAW, 0, 0)
}

func authFitHiddenWindowToWorkArea(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	// Do not rely on WS_MAXIMIZE while WM_CREATE is still constructing children.
	// On some Windows builds this delivers WM_SIZE before the full child list exists,
	// leaving part of the controls at pre-maximize coordinates and producing the
	// duplicated/smeared first frame seen in RC404. Resize the still-hidden parent
	// once, after WM_CREATE has completed, then run one final layout for every child.
	const SPI_GETWORKAREA = 0x0030
	var work RECT
	ok, _, _ := pSystemParametersInfoWAuth.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&work)), 0)
	if ok == 0 || work.Right <= work.Left || work.Bottom <= work.Top {
		return
	}
	const SWP_NOZORDER = 0x0004
	const SWP_NOACTIVATE = 0x0010
	pSetWindowPos.Call(hwnd, 0,
		uintptr(work.Left), uintptr(work.Top),
		uintptr(work.Right-work.Left), uintptr(work.Bottom-work.Top),
		SWP_NOZORDER|SWP_NOACTIVATE)

	var rc RECT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	clientW, clientH := rc.Right-rc.Left, rc.Bottom-rc.Top
	if clientW <= 0 || clientH <= 0 {
		return
	}
	// WM_SIZE may already have run during the hidden SetWindowPos, but send a final
	// size message now that authCenteredItems is complete. This guarantees that no
	// child remains at its construction-time coordinates.
	lp := uintptr(uint32(clientW)&0xFFFF) | (uintptr(uint32(clientH)&0xFFFF) << 16)
	pSendMessageW.Call(hwnd, WM_SIZE, 0, lp)
}

func authActionLayout() (loginX, loginW, createX, createW int32) {
	// RC381 reference lock: proportions are taken from the approved login visual.
	// The primary action is wider; the secondary action stays visually quieter.
	return 32, 266, 312, 216
}

func authRoundWindow(h syscall.Handle, w, hgt, radius int32) {
	if h == 0 || w <= 0 || hgt <= 0 || radius <= 0 {
		return
	}
	rgn, _, _ := pCreateRoundRectRgnAuth.Call(0, 0, uintptr(w+1), uintptr(hgt+1), uintptr(radius), uintptr(radius))
	if rgn != 0 {
		pSetWindowRgnAuth.Call(uintptr(h), rgn, 1)
	}
}

func drawAuthInfoNumber(di *DRAWITEMSTRUCT) {
	if di == nil || di.HDC == 0 {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(authBrushWhite))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	font := authNumberFont
	if font == 0 {
		font = authBoldFont
	}
	oldFont, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	text := p(getText(di.HwndItem))
	shadow := di.RcItem
	shadow.Left += 1
	shadow.Top += 1
	shadow.Right += 1
	shadow.Bottom += 1
	pSetTextColor.Call(uintptr(di.HDC), rgb(233, 218, 221))
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&shadow)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	r := di.RcItem
	pSetTextColor.Call(uintptr(di.HDC), rgb(203, 27, 43))
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	if oldFont != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldFont)
	}
}

func authSetStatus(s string) {
	if authStatus != 0 {
		pSetWindowTextW.Call(uintptr(authStatus), uintptr(unsafe.Pointer(p(s))))
	}
}

func authSetPasswordMask(masked bool) {
	if authPasswordEdit == 0 {
		return
	}
	ch := uintptr(0)
	if masked {
		ch = uintptr('*')
	}
	pSendMessageW.Call(uintptr(authPasswordEdit), EM_SETPASSWORDCHAR, ch, 0)
	pInvalidateRect.Call(uintptr(authPasswordEdit), 0, 1)
	authPasswordVisible = !masked
	if authPasswordShowButton != 0 {
		caption := "Показать"
		if authPasswordVisible {
			caption = "Скрыть"
		}
		pSetWindowTextW.Call(uintptr(authPasswordShowButton), uintptr(unsafe.Pointer(p(caption))))
	}
}

func authShowControl(h syscall.Handle, show bool) {
	if h == 0 {
		return
	}
	if show {
		pShowWindow.Call(uintptr(h), SW_SHOW)
	} else {
		pShowWindow.Call(uintptr(h), SW_HIDE)
	}
}

func authRestoreNormalFlow(password, status string) {
	authFlowMode = authFlowNormal
	authRecoveryLogin = ""
	authPendingCreate = authPendingProfile{}
	if authPasswordLabel != 0 {
		pSetWindowTextW.Call(uintptr(authPasswordLabel), uintptr(unsafe.Pointer(p("ПАРОЛЬ"))))
	}
	authSetPasswordMask(true)
	if authPasswordEdit != 0 {
		pSetWindowTextW.Call(uintptr(authPasswordEdit), uintptr(unsafe.Pointer(p(password))))
	}
	if authLoginButton != 0 {
		pSetWindowTextW.Call(uintptr(authLoginButton), uintptr(unsafe.Pointer(p("ВОЙТИ  →"))))
	}
	if authCreateButton != 0 {
		pSetWindowTextW.Call(uintptr(authCreateButton), uintptr(unsafe.Pointer(p("Создать профиль"))))
	}
	authShowControl(authRemember, true)
	authShowControl(authForgotButton, true)
	authShowControl(authGuestButton, true)
	authShowControl(authPasswordShowButton, true)
	authSetStatus(status)
}

func authEnterControlWordFlow(mode int, login, status string) {
	authFlowMode = mode
	authRecoveryLogin = login
	if authPasswordLabel != 0 {
		pSetWindowTextW.Call(uintptr(authPasswordLabel), uintptr(unsafe.Pointer(p("КОНТРОЛЬНОЕ СЛОВО"))))
	}
	authSetPasswordMask(false)
	if authPasswordEdit != 0 {
		pSetWindowTextW.Call(uintptr(authPasswordEdit), uintptr(unsafe.Pointer(p(""))))
		pSetFocus.Call(uintptr(authPasswordEdit))
	}
	authShowControl(authRemember, false)
	authShowControl(authForgotButton, false)
	authShowControl(authGuestButton, false)
	authShowControl(authPasswordShowButton, false)
	switch mode {
	case authFlowCreateSecurity, authFlowLegacySecuritySetup:
		if authLoginButton != 0 {
			pSetWindowTextW.Call(uintptr(authLoginButton), uintptr(unsafe.Pointer(p("НАЗАД"))))
		}
		if authCreateButton != 0 {
			pSetWindowTextW.Call(uintptr(authCreateButton), uintptr(unsafe.Pointer(p("СОХРАНИТЬ ЗАЩИТУ"))))
		}
	case authFlowRecoverSecurity:
		if authLoginButton != 0 {
			pSetWindowTextW.Call(uintptr(authLoginButton), uintptr(unsafe.Pointer(p("ПРОВЕРИТЬ ОТВЕТ"))))
		}
		if authCreateButton != 0 {
			pSetWindowTextW.Call(uintptr(authCreateButton), uintptr(unsafe.Pointer(p("НАЗАД"))))
		}
	}
	authSetStatus(status)
}
func authGetText(h syscall.Handle) string {
	n, _, _ := pGetWindowTextLengthW.Call(uintptr(h))
	buf := make([]uint16, n+1)
	if n >= 0 {
		pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), n+1)
	}
	return string(utf16.Decode(buf[:n]))
}

const (
	authPlaceholderNGDO     = "Выберите НГДО"
	authPlaceholderCITS     = "Выберите ЦИТС/ТПП"
	authPlaceholderCDNG     = "Выберите ЦДНГ"
	authPlaceholderPosition = "Выберите должность"
)

func authPlaceholderForID(id int) string {
	switch id {
	case ID_AUTH_NGDO:
		return authPlaceholderNGDO
	case ID_AUTH_CITS:
		return authPlaceholderCITS
	case ID_AUTH_CDNG:
		return authPlaceholderCDNG
	case ID_AUTH_POSITION:
		return authPlaceholderPosition
	}
	return ""
}

func authApplyReferencePlaceholders() {
	for _, item := range []struct {
		h syscall.Handle
		v string
	}{
		{authNGDOEdit, authPlaceholderNGDO},
		{authCITSEdit, authPlaceholderCITS},
		{authCDNGEdit, authPlaceholderCDNG},
		{authPositionEdit, authPlaceholderPosition},
	} {
		if item.h != 0 && strings.TrimSpace(authGetText(item.h)) == "" {
			pSetWindowTextW.Call(uintptr(item.h), uintptr(unsafe.Pointer(p(item.v))))
		}
	}
}

func authFieldValue(h syscall.Handle, placeholder string) string {
	v := strings.TrimSpace(authGetText(h))
	if v == placeholder {
		return ""
	}
	return v
}

func authClearPlaceholderOnFocus(id int) {
	ph := authPlaceholderForID(id)
	if ph == "" {
		return
	}
	var h syscall.Handle
	switch id {
	case ID_AUTH_NGDO:
		h = authNGDOEdit
	case ID_AUTH_CITS:
		h = authCITSEdit
	case ID_AUTH_CDNG:
		h = authCDNGEdit
	case ID_AUTH_POSITION:
		h = authPositionEdit
	}
	if h != 0 && strings.TrimSpace(authGetText(h)) == ph {
		pSetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(p(""))))
	}
}

func authPopulateOrgCombos() {
	add := func(h syscall.Handle, items []string) {
		if h == 0 {
			return
		}
		pSendMessageW.Call(uintptr(h), CB_RESETCONTENT, 0, 0)
		for _, item := range items {
			pSendMessageW.Call(uintptr(h), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(item))))
		}
	}
	add(authNGDOEdit, []string{"—", "РИТЭК"})
	add(authCITSEdit, []string{"—", "АУ", "Волгограднефтегаз", "ТатРИТЭКнефть", "РИТЭК-Самара-Нафта"})
	add(authCDNGEdit, []string{"-", "ЦДНГ-1", "ЦДНГ-2", "Север", "Юг", "ЦДНГ-3"})
	add(authPositionEdit, []string{"—", "ведущий инженер", "инженер 1 категории", "инженер 2 категории", "инженер-технолог 1 категории", "инженер-технолог 2 категории"})
}

func authPopulateUsers() {
	if authUsersList == 0 {
		return
	}
	pSendMessageW.Call(uintptr(authUsersList), LB_RESETCONTENT, 0, 0)
	st := loadAuthStore()
	for _, a := range st.Accounts {
		if a.Disabled {
			continue
		}
		name := a.DisplayName
		if name == "" {
			name = a.Login
		}
		pSendMessageW.Call(uintptr(authUsersList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(name))))
	}
	visible := make([]LocalAccount, 0, len(st.Accounts))
	for _, a := range st.Accounts {
		if !a.Disabled {
			visible = append(visible, a)
		}
	}
	if len(visible) == 1 {
		pSendMessageW.Call(uintptr(authUsersList), LB_SETCURSEL, 0, 0)
		authFillProfileFields(visible[0])
	}
	if len(st.Accounts) == 0 {
		// Prefill the owner's name for first-time setup; do not create an
		// unauthenticated account or invent a password or department.
		authFillingProfile = true
		for _, field := range []struct {
			h     syscall.Handle
			value string
		}{
			{authLastNameEdit, "Ромадина"},
			{authFirstNameEdit, "Екатерина"},
			{authMiddleNameEdit, "Викторовна"},
		} {
			if field.h != 0 {
				pSetWindowTextW.Call(uintptr(field.h), uintptr(unsafe.Pointer(p(field.value))))
			}
		}
		authFillingProfile = false
		authSetStatus("Первый запуск: проверьте ФИО, заполните НГДО, ЦИТС/ТПП, ЦДНГ, должность и пароль, затем нажмите «Создать профиль».")
	}
}

func authApplyRememberedCredential() {
	login, password, ok := loadRememberedCredential()
	if !ok {
		return
	}
	pSetWindowTextW.Call(uintptr(authPasswordEdit), uintptr(unsafe.Pointer(p(password))))
	if authRemember != 0 {
		pSendMessageW.Call(uintptr(authRemember), BM_SETCHECK, BST_CHECKED, 0)
	}
	st := loadAuthStore()
	visibleIndex := 0
	for _, a := range st.Accounts {
		if a.Disabled {
			continue
		}
		if normalizeLogin(a.Login) == normalizeLogin(login) {
			pSendMessageW.Call(uintptr(authUsersList), LB_SETCURSEL, uintptr(visibleIndex), 0)
			authFillProfileFields(a)
			break
		}
		visibleIndex++
	}
}

func authRememberChecked() bool {
	if authRemember == 0 {
		return false
	}
	r, _, _ := pSendMessageW.Call(uintptr(authRemember), BM_GETCHECK, 0, 0)
	return r == BST_CHECKED
}

func authFillProfileFields(a LocalAccount) {
	authFillingProfile = true
	defer func() { authFillingProfile = false }()
	if authCITSEdit != 0 {
		pSetWindowTextW.Call(uintptr(authCITSEdit), uintptr(unsafe.Pointer(p(a.CITS))))
	}
	if authCDNGEdit != 0 {
		pSetWindowTextW.Call(uintptr(authCDNGEdit), uintptr(unsafe.Pointer(p(a.CDNG))))
	}
	if authNGDOEdit != 0 {
		pSetWindowTextW.Call(uintptr(authNGDOEdit), uintptr(unsafe.Pointer(p(a.NGDO))))
	}
	if authPositionEdit != 0 {
		pSetWindowTextW.Call(uintptr(authPositionEdit), uintptr(unsafe.Pointer(p(a.Position))))
	}
	lastName, firstName, middleName := accountPersonNames(a)
	if authLastNameEdit != 0 {
		pSetWindowTextW.Call(uintptr(authLastNameEdit), uintptr(unsafe.Pointer(p(lastName))))
	}
	if authFirstNameEdit != 0 {
		pSetWindowTextW.Call(uintptr(authFirstNameEdit), uintptr(unsafe.Pointer(p(firstName))))
	}
	if authMiddleNameEdit != 0 {
		pSetWindowTextW.Call(uintptr(authMiddleNameEdit), uintptr(unsafe.Pointer(p(middleName))))
	}
	authSelectedLogin = a.Login
}

func authSelectUserFromList() {
	r, _, _ := pSendMessageW.Call(uintptr(authUsersList), LB_GETCURSEL, 0, 0)
	idx := int(int32(r))
	if idx < 0 {
		return
	}
	st := loadAuthStore()
	visible := make([]LocalAccount, 0, len(st.Accounts))
	for _, a := range st.Accounts {
		if !a.Disabled {
			visible = append(visible, a)
		}
	}
	if idx >= len(visible) {
		return
	}
	selectedLogin := visible[idx].Login
	authFillProfileFields(visible[idx])
	rememberedLogin, rememberedPassword, remembered := loadRememberedCredential()
	if remembered && normalizeLogin(rememberedLogin) == normalizeLogin(selectedLogin) {
		pSetWindowTextW.Call(uintptr(authPasswordEdit), uintptr(unsafe.Pointer(p(rememberedPassword))))
		pSendMessageW.Call(uintptr(authRemember), BM_SETCHECK, BST_CHECKED, 0)
	} else {
		pSetWindowTextW.Call(uintptr(authPasswordEdit), uintptr(unsafe.Pointer(p(""))))
		pSendMessageW.Call(uintptr(authRemember), BM_SETCHECK, 0, 0)
	}
	pSetFocus.Call(uintptr(authPasswordEdit))
}

func authCardTextControl(h syscall.Handle) bool {
	for _, z := range authCardText {
		if z == h {
			return true
		}
	}
	return false
}

func drawAuthSegments(di *DRAWITEMSTRUCT, parts []struct {
	text string
	red  bool
}, font syscall.Handle, bg syscall.Handle) {
	if di == nil || di.HDC == 0 {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(bg))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	availableW := di.RcItem.Right - di.RcItem.Left
	candidates := []syscall.Handle{font}
	if font == authBrandFont {
		// The header slogan may be enlarged, but must never be clipped on smaller
		// screens or at 125-150% Windows scaling.
		candidates = append(candidates, authBoldFont, authButtonFont)
	}
	var oldFont uintptr
	widths := make([]int32, len(parts))
	totalW := int32(0)
	for ci, candidate := range candidates {
		if candidate == 0 {
			continue
		}
		prev, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(candidate))
		if ci == 0 {
			oldFont = prev
		}
		totalW = 0
		for i, part := range parts {
			buf, _ := syscall.UTF16FromString(part.text)
			r := RECT{Left: 0, Top: 0, Right: 2600, Bottom: 140}
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX)
			widths[i] = r.Right - r.Left
			totalW += widths[i]
		}
		if totalW <= availableW-16 || ci == len(candidates)-1 {
			break
		}
	}
	x := di.RcItem.Left + max32(8, (availableW-totalW)/2)
	for i, part := range parts {
		if part.red {
			pSetTextColor.Call(uintptr(di.HDC), rgb(203, 27, 43))
		} else {
			pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
		}
		buf, _ := syscall.UTF16FromString(part.text)
		r := RECT{Left: x, Top: di.RcItem.Top, Right: min32(di.RcItem.Right-4, x+widths[i]+4), Bottom: di.RcItem.Bottom}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		x += widths[i]
	}
	if oldFont != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldFont)
	}
}

// WELCOME_444_DRAW_BEGIN
func drawAuthProductLabel(di *DRAWITEMSTRUCT) {
	if di == nil || di.HDC == 0 {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(authBrushWhite))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	type headerWord struct{ initial, rest string }
	words := []headerWord{
		{"Р", "АСЧЁТ"},
		{"О", "ЦЕНКА"},
		{"С", "ОПРОВОЖДЕНИЕ"},
		{"А", "ВТОМАТИЗАЦИЯ"},
	}
	availableW := di.RcItem.Right - di.RcItem.Left
	if availableW <= 0 {
		return
	}
	leftPad := int32(18)
	rightPad := int32(18)
	usableW := availableW - leftPad - rightPad
	if usableW <= 0 {
		return
	}

	// Pick the largest corporate type size that lets the four words live as one
	// centered group with equal *visible* edge-to-edge gaps.  Equal cell centers
	// look uneven here because the word lengths differ strongly.
	candidates := []syscall.Handle{authBrandFont, authBoldFont, authButtonFont, authProfileCaptionFont}
	font := authProfileCaptionFont
	var oldFont uintptr
	measure := func(text string) int32 {
		buf, _ := syscall.UTF16FromString(text)
		r := RECT{Left: 0, Top: 0, Right: 1600, Bottom: 120}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX)
		return r.Right - r.Left
	}
	const minGap int32 = 24
	for _, candidate := range candidates {
		if candidate == 0 {
			continue
		}
		prev, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(candidate))
		totalWordsW := int32(0)
		for _, w := range words {
			totalWordsW += measure(w.initial + w.rest)
		}
		pSelectObject.Call(uintptr(di.HDC), prev)
		font = candidate
		if totalWordsW+int32(len(words)-1)*minGap <= usableW {
			break
		}
	}
	if font != 0 {
		oldFont, _, _ = pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	}

	wordWidths := make([]int32, len(words))
	totalWordsW := int32(0)
	for i, w := range words {
		wordWidths[i] = measure(w.initial + w.rest)
		totalWordsW += wordWidths[i]
	}
	gap := minGap
	if len(words) > 1 && usableW > totalWordsW {
		gap = (usableW - totalWordsW) / int32(len(words)-1)
		// Equal spacing should still read as one typographic lock-up. On very wide
		// screens an uncapped gap makes the words look scattered even though the
		// mathematical spacing is correct.
		if gap > 58 {
			gap = 58
		}
	}
	groupW := totalWordsW + gap*int32(len(words)-1)
	x := di.RcItem.Left + leftPad + max32(0, (usableW-groupW)/2)

	for i, w := range words {
		initialW := measure(w.initial)
		restW := measure(w.rest)
		pSetTextColor.Call(uintptr(di.HDC), rgb(203, 27, 43))
		bufInitial, _ := syscall.UTF16FromString(w.initial)
		rInitial := RECT{Left: x, Top: di.RcItem.Top - 1, Right: x + initialW + 2, Bottom: di.RcItem.Bottom}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bufInitial[0])), uintptr(len(bufInitial)-1), uintptr(unsafe.Pointer(&rInitial)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		x += initialW

		pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
		bufRest, _ := syscall.UTF16FromString(w.rest)
		rRest := RECT{Left: x, Top: di.RcItem.Top - 1, Right: x + restW + 2, Bottom: di.RcItem.Bottom}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bufRest[0])), uintptr(len(bufRest)-1), uintptr(unsafe.Pointer(&rRest)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		x += restW

		if i < len(words)-1 {
			// A small red dot sits exactly at the centre of the equal inter-word gap.
			// It is intentionally understated so the header reads as typography,
			// not as four separate buttons.
			pSetTextColor.Call(uintptr(di.HDC), rgb(203, 27, 43))
			bullet, _ := syscall.UTF16FromString("•")
			mid := x + gap/2
			rBullet := RECT{Left: mid - 12, Top: di.RcItem.Top - 2, Right: mid + 12, Bottom: di.RcItem.Bottom}
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bullet[0])), 1, uintptr(unsafe.Pointer(&rBullet)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			x += gap
		}
	}
	if oldFont != 0 {
		pSelectObject.Call(uintptr(di.HDC), oldFont)
	}
}

func drawAuthEntryTitle(di *DRAWITEMSTRUCT) {
	parts := []struct {
		text string
		red  bool
	}{
		{"Вход в ", false}, {"РОСА", true},
	}
	drawAuthSegments(di, parts, authTitleFont, authBrushWhite)
}

// The three-line analytical card heading follows the supplied 7.3.11.444-RC
// executable: red РОСА on the first line, then two centered blue lines.
func drawAuthInfoLead(di *DRAWITEMSTRUCT) {
	if di == nil || di.HDC == 0 {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(authBrushWhite))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	font := authInfoLeadFont
	if font == 0 {
		font = authTitleFont
	}
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	measure := func(s string) int32 {
		buf, _ := syscall.UTF16FromString(s)
		r := RECT{Left: 0, Top: 0, Right: 1200, Bottom: 120}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX)
		return r.Right - r.Left
	}
	first, rest := "РОСА", " — ваш инструмент"
	firstW, restW := measure(first), measure(rest)
	x := di.RcItem.Left + max32(0, (di.RcItem.Right-di.RcItem.Left-firstW-restW)/2)
	paint := func(s string, color uintptr, r RECT) {
		pSetTextColor.Call(uintptr(di.HDC), color)
		buf, _ := syscall.UTF16FromString(s)
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)
	}
	paint(first, rgb(203, 27, 43), RECT{Left: x, Top: di.RcItem.Top, Right: x + firstW + 2, Bottom: di.RcItem.Top + 38})
	paint(rest, rgb(15, 48, 82), RECT{Left: x + firstW, Top: di.RcItem.Top, Right: x + firstW + restW + 2, Bottom: di.RcItem.Top + 38})
	paint("для принятия технологических", rgb(15, 48, 82), RECT{Left: di.RcItem.Left, Top: di.RcItem.Top + 38, Right: di.RcItem.Right, Bottom: di.RcItem.Top + 72})
	paint("решений", rgb(15, 48, 82), RECT{Left: di.RcItem.Left, Top: di.RcItem.Top + 70, Right: di.RcItem.Right, Bottom: di.RcItem.Bottom})
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
}

// WELCOME_444_DRAW_END

func authResolvedLoginFromFields() string {
	if strings.TrimSpace(authSelectedLogin) != "" {
		return authSelectedLogin
	}
	cits := authFieldValue(authCITSEdit, authPlaceholderCITS)
	cdng := authFieldValue(authCDNGEdit, authPlaceholderCDNG)
	ngdo := authFieldValue(authNGDOEdit, authPlaceholderNGDO)
	position := authFieldValue(authPositionEdit, authPlaceholderPosition)
	lastName := strings.TrimSpace(authGetText(authLastNameEdit))
	firstName := strings.TrimSpace(authGetText(authFirstNameEdit))
	middleName := strings.TrimSpace(authGetText(authMiddleNameEdit))
	for _, a := range loadAuthStore().Accounts {
		if a.Disabled {
			continue
		}
		al, af, am := accountPersonNames(a)
		if strings.EqualFold(a.CITS, cits) && strings.EqualFold(a.CDNG, cdng) && strings.EqualFold(a.NGDO, ngdo) && strings.EqualFold(a.Position, position) && strings.EqualFold(al, lastName) && strings.EqualFold(af, firstName) && strings.EqualFold(am, middleName) {
			return a.Login
		}
	}
	return ""
}

func authLayoutRecoveryPanel(clientW, clientH int32) {
	if authRecoveryPanel == 0 || clientW <= 0 || clientH <= 0 {
		return
	}
	// Keep password help compact and local to the login card. The previous
	// 700x330 overlay covered most of the right information card and looked like
	// a blocking dialog rather than a short informational plaque.
	panelW, panelH := int32(462), int32(248)
	offset := authCenterOffset(clientW)
	x := offset + 35 // visually centered inside the 500 px left card
	if x < 18 {
		x = 18
	}
	y := authHeaderHeight + 270
	if y+panelH > clientH-20 {
		y = max32(authHeaderHeight+20, clientH-panelH-20)
	}
	pSetWindowPos.Call(uintptr(authRecoveryPanel), 0, uintptr(x), uintptr(y), uintptr(panelW), uintptr(panelH), 0x001C)
	pSetWindowPos.Call(uintptr(authRecoveryAccent), 0, uintptr(x), uintptr(y), uintptr(panelW), 5, 0x001C)
	pSetWindowPos.Call(uintptr(authRecoveryTitle), 0, uintptr(x+22), uintptr(y+18), uintptr(panelW-44), 34, 0x001C)
	pSetWindowPos.Call(uintptr(authRecoveryText), 0, uintptr(x+28), uintptr(y+60), uintptr(panelW-56), 122, 0x001C)
	buttonW := int32(168)
	pSetWindowPos.Call(uintptr(authRecoveryClose), 0, uintptr(x+(panelW-buttonW)/2), uintptr(y+194), uintptr(buttonW), 38, 0x001C)
}

func authShowRecoveryPanel(text string) {
	if authRecoveryPanel == 0 {
		return
	}
	pSetWindowTextW.Call(uintptr(authRecoveryText), uintptr(unsafe.Pointer(p(text))))
	var rc RECT
	if authWnd != 0 {
		pGetClientRect.Call(uintptr(authWnd), uintptr(unsafe.Pointer(&rc)))
		authLayoutRecoveryPanel(rc.Right-rc.Left, rc.Bottom-rc.Top)
	}
	for _, h := range []syscall.Handle{authRecoveryPanel, authRecoveryAccent, authRecoveryTitle, authRecoveryText, authRecoveryClose} {
		if h != 0 {
			pShowWindow.Call(uintptr(h), SW_SHOW)
			pSetWindowPos.Call(uintptr(h), 0, 0, 0, 0, 0, 0x0001|0x0002|0x0010) // NOSIZE|NOMOVE|NOACTIVATE; HWND_TOP
		}
	}
	if authRecoveryClose != 0 {
		pSetFocus.Call(uintptr(authRecoveryClose))
	}
}

func authHideRecoveryPanel() {
	for _, h := range []syscall.Handle{authRecoveryClose, authRecoveryText, authRecoveryTitle, authRecoveryAccent, authRecoveryPanel} {
		if h != 0 {
			pShowWindow.Call(uintptr(h), SW_HIDE)
		}
	}
}

func authHandleForgotPassword(hwnd uintptr) {
	login := authResolvedLoginFromFields()
	if login == "" {
		authSetStatus("Сначала выберите профиль или заполните подразделение, фамилию и имя.")
		return
	}
	a, ok := authAccountByLogin(login)
	if !ok {
		authSetStatus("Профиль не найден.")
		return
	}
	if strings.TrimSpace(a.SecurityAnswerHash) != "" {
		authEnterControlWordFlow(authFlowRecoverSecurity, login, "Введите контрольное слово, заданное при создании профиля.")
		return
	}
	// Legacy profiles created before RC406 may not have a control word. If this
	// computer already has a protected Windows credential for the same user, use
	// it only to let the owner configure the new protection first. The password is
	// not shown until the control word has been saved.
	rememberedLogin, rememberedPassword, remembered := loadRememberedCredential()
	if remembered && normalizeLogin(rememberedLogin) == normalizeLogin(login) && rememberedPassword != "" {
		authPendingCreate.password = rememberedPassword
		authEnterControlWordFlow(authFlowLegacySecuritySetup, login, "Старый профиль: задайте контрольное слово. После сохранения РОСА восстановит пароль из защищённого хранилища Windows.")
		return
	}
	authSetStatus("Для этого старого профиля контрольный вопрос ещё не настроен. Обратитесь к администратору для сброса доступа без удаления истории.")
}

func authStartGuestSession() (*UserSession, error) {
	tempDir, err := os.MkdirTemp("", "ROSA_guest_")
	if err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	return &UserSession{UserID: "guest", Login: "guest", DisplayName: "Гостевой пользователь", LastName: "Гостевой", FirstName: "Пользователь", MiddleName: "", NGDO: "Не указано", CITS: "Гостевой доступ", CDNG: "Не указано", Position: "Неавторизованный пользователь", IsGuest: true, TempDir: tempDir, masterKey: key}, nil
}

func authWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND_AUTH:
		// Paint the oilfield artwork strictly below the complete fixed header.
		// RC394 used 104 px here while the real header is 112 px, so the image
		// leaked eight pixels into the header and looked diagonally/unevenly cut.
		// Keep a single authoritative header height and use a bottom/right anchored
		// aspect-cover crop so the lower industrial silhouettes and RITEK mark survive.
		hdc := wParam
		var rc RECT
		if hdc != 0 && pGetClientRect != nil {
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			cw, ch := int32(rc.Right-rc.Left), int32(rc.Bottom-rc.Top)
			if authBrushWhite != 0 {
				pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(authBrushWhite))
			}
			if authBackgroundBitmap != 0 && cw > 0 && ch > authHeaderHeight {
				dstW, dstH := cw, ch-authHeaderHeight
				sx, sy, cropW, cropH := authBackgroundCoverCrop(dstW, dstH)
				memDC, _, _ := pCreateCompatibleDC.Call(hdc)
				if memDC != 0 {
					old, _, _ := pSelectObject.Call(memDC, uintptr(authBackgroundBitmap))
					pSetStretchBltMode.Call(hdc, HALFTONE_AUTH)
					pStretchBlt.Call(hdc, 0, uintptr(authHeaderHeight), uintptr(dstW), uintptr(dstH), memDC, uintptr(sx), uintptr(sy), uintptr(cropW), uintptr(cropH), SRCCOPY_AUTH)
					if old != 0 {
						pSelectObject.Call(memDC, old)
					}
					pDeleteDC.Call(memDC)
				}
			}
		}
		return 1
	case WM_CREATE:
		// RC386: native login screen.  The central cards are built from real
		// Windows controls and sharp standalone icons; no full-card screenshots.
		authCenteredItems = authCenteredItems[:0]
		authCardText = authCardText[:0]
		authInfoNumbers = [4]syscall.Handle{}
		authFeatureIconControls = [4]syscall.Handle{}
		authNavIconControls = [4]syscall.Handle{}
		authFeatureIconBitmaps = [4]syscall.Handle{}
		authNavIconBitmaps = [4]syscall.Handle{}

		// WELCOME_444_CREATE_BEGIN
		// Header.
		authBrand = authCreate("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, 0, 0, 1670, authHeaderHeight, syscall.Handle(hwnd), 0)
		authRosaLogo = authCreate("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP|SS_CENTERIMAGE, 28, 6, 256, 96, syscall.Handle(hwnd), 0)
		authLukoilLogo = authCreate("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP|SS_CENTERIMAGE, 1402, 0, 258, 74, syscall.Handle(hwnd), 0)
		authRosaBitmap, _ = buildHeaderBrandBitmap()
		authLukoilBitmap, _ = buildHeaderLukoilBitmap()
		if authRosaBitmap != 0 {
			pSendMessageW.Call(uintptr(authRosaLogo), STM_SETIMAGE, IMAGE_BITMAP, uintptr(authRosaBitmap))
		}
		if authLukoilBitmap != 0 {
			pSendMessageW.Call(uintptr(authLukoilLogo), STM_SETIMAGE, IMAGE_BITMAP, uintptr(authLukoilBitmap))
		}
		authRosaSubtitle = authCreate("STATIC", "ИНФОРМАЦИОННО-АНАЛИТИЧЕСКАЯ СИСТЕМА\r\nДЛЯ ФОНДА СКВАЖИН", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, 0, 88, 312, 28, syscall.Handle(hwnd), 0)
		if authBrandCaptionFont != 0 {
			pSendMessageW.Call(uintptr(authRosaSubtitle), WM_SETFONT, uintptr(authBrandCaptionFont), 1)
		}
		authLukoilTagline = authCreate("STATIC", "ВСЕГДА В ДВИЖЕНИИ", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, 1402, 74, 258, 22, syscall.Handle(hwnd), 0)
		if authFooterFont != 0 {
			pSendMessageW.Call(uintptr(authLukoilTagline), WM_SETFONT, uintptr(authFooterFont), 1)
		}
		authProductLabel = authCreate("STATIC", "", WS_CHILD|WS_VISIBLE|SS_OWNERDRAW, 292, 30, 1086, 72, syscall.Handle(hwnd), 0)
		authHeaderRule = authCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 128, 1670, 4, syscall.Handle(hwnd), 0)

		// The user-provided oilfield image is painted directly by the parent window
		// below the fixed header. This avoids SS_BITMAP sizing/margin artefacts.
		authTitlePanel = 0
		authBackgroundBitmap, _ = buildLoginBackgroundBitmap(int(authBackgroundSourceWidth), int(authBackgroundSourceHeight))
		authSideLeft = 0
		authSideRight = 0

		// RC401 compact two-card proportions follow the user's marked reference:
		// both cards are about 500 px wide with a clear ~28 px gap between them.
		// The height stays generous so no labels or controls are squeezed vertically.
		const leftCardX int32 = 16
		const leftCardW int32 = 500
		const rightCardX int32 = 544
		const rightCardW int32 = 500
		const cardY int32 = 121
		const cardH int32 = 760
		const leftInnerX int32 = 50
		const leftInnerW int32 = 432
		const rightInnerX int32 = 578
		const rightInnerW int32 = 432

		// Red accents for the left card: matching top and bottom stripes.
		// Cards first, accents second: the red rails must stay visible above the white surfaces.
		authLeftCard = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, leftCardX, cardY, leftCardW, cardH, syscall.Handle(hwnd), 0)
		authRightCard = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, rightCardX, cardY, rightCardW, cardH, syscall.Handle(hwnd), 0)
		authTitleAccent = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, leftCardX, cardY, leftCardW, 7, syscall.Handle(hwnd), 0)
		authLeftBottomAccent = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, leftCardX, cardY+cardH-7, leftCardW, 7, syscall.Handle(hwnd), 0)
		authRoundWindow(authLeftCard, leftCardW, cardH, 12)
		authRoundWindow(authRightCard, rightCardW, cardH, 12)

		// Left card — all native text and inputs.
		authEntryTitle = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_OWNERDRAW, leftInnerX, 142, leftInnerW, 58, syscall.Handle(hwnd), 0)
		authEntryUnderline = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, leftInnerX+(leftInnerW-76)/2, 207, 76, 3, syscall.Handle(hwnd), 0)
		subtitle := authCreateCentered("STATIC", "Используйте корпоративные данные\r\nдля входа в систему", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, leftInnerX, 214, leftInnerW, 58, syscall.Handle(hwnd), 0)
		authCardText = append(authCardText, subtitle)
		if authBodyFont != 0 {
			pSendMessageW.Call(uintptr(subtitle), WM_SETFONT, uintptr(authBodyFont), 1)
		}

		// Keep account list available to auth engine, hidden from the approved UI.
		authUsersList = authCreate("LISTBOX", "", WS_CHILD|LBS_NOTIFY|LBS_HASSTRINGS, 0, 0, 1, 1, syscall.Handle(hwnd), ID_AUTH_USERS)

		labelH := int32(28)
		// Win32 CBS_DROPDOWN uses the CREATE/SetWindowPos height for the
		// entire dropdown, not only the closed edit field. A 42px height
		// makes the popup appear empty. Keep the native closed height, but
		// reserve enough underlying height for 5-7 selectable rows.
		comboH := int32(258)
		fieldCombo := func(caption string, x, y, w int32, id int) syscall.Handle {
			lab := authCreateCentered("STATIC", caption, WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE, x, y, w, labelH, syscall.Handle(hwnd), 0)
			authCardText = append(authCardText, lab)
			if authFieldLabelFont != 0 {
				pSendMessageW.Call(uintptr(lab), WM_SETFONT, uintptr(authFieldLabelFont), 1)
			}
			h := authCreateCentered("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|WS_BORDER|CBS_DROPDOWN_AUTH, x, y+31, w, comboH, syscall.Handle(hwnd), id)
			if h != 0 && authInputFont != 0 {
				pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(authInputFont), 1)
			}
			if h != 0 {
				// Let long department/position names remain readable when open.
				pSendMessageW.Call(uintptr(h), CB_SETDROPPEDWIDTH, uintptr(max32(w, 305)), 0)
			}
			return h
		}
		orgGap := int32(16)
		colW := (leftInnerW - orgGap) / 2
		leftX, rightX := leftInnerX, leftInnerX+colW+orgGap
		authNGDOEdit = fieldCombo("НГДО", leftX, 282, colW, ID_AUTH_NGDO)
		authCITSEdit = fieldCombo("ЦИТС/ТПП", rightX, 282, colW, ID_AUTH_CITS)
		authCDNGEdit = fieldCombo("ЦДНГ", leftX, 372, colW, ID_AUTH_CDNG)
		authPositionEdit = fieldCombo("ДОЛЖНОСТЬ", rightX, 372, colW, ID_AUTH_POSITION)

		nameGap := int32(8)
		nameColW := (leftInnerW - 2*nameGap) / 3
		nameX1 := leftInnerX
		nameX2 := leftInnerX + nameColW + nameGap
		nameX3 := nameX2 + nameColW + nameGap
		nameField := func(caption string, x int32, id int) syscall.Handle {
			lab := authCreateCentered("STATIC", caption, WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE, x, 462, nameColW, labelH, syscall.Handle(hwnd), 0)
			authCardText = append(authCardText, lab)
			if authFieldLabelFont != 0 {
				pSendMessageW.Call(uintptr(lab), WM_SETFONT, uintptr(authFieldLabelFont), 1)
			}
			// A multiline edit with an explicit formatting rectangle is used only
			// to obtain reliable vertical centring; Enter remains the dialog action.
			// Native single-line EDIT controls pin glyphs to the upper edge at
			// 125–150% DPI even when ES_CENTER is set.
			h := authCreateCentered("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL|ES_CENTER|ES_MULTILINE, x, 495, nameColW, 42, syscall.Handle(hwnd), id)
			if h != 0 && authInputFont != 0 {
				pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(authInputFont), 1)
				textRect := RECT{Left: 6, Top: 8, Right: nameColW - 6, Bottom: 36}
				pSendMessageW.Call(uintptr(h), 0x00B3, 0, uintptr(unsafe.Pointer(&textRect))) // EM_SETRECT
			}
			return h
		}
		authLastNameEdit = nameField("ФАМИЛИЯ", nameX1, ID_AUTH_LASTNAME)
		authFirstNameEdit = nameField("ИМЯ", nameX2, ID_AUTH_FIRSTNAME)
		authMiddleNameEdit = nameField("ОТЧЕСТВО", nameX3, ID_AUTH_MIDDLENAME)
		showPasswordW := int32(104)
		passwordGap := int32(8)
		passwordEditW := leftInnerW - showPasswordW - passwordGap
		// The service button is not part of the field: centre the caption over the
		// password input itself so both vertical axes coincide.
		authPasswordLabel = authCreateCentered("STATIC", "ПАРОЛЬ", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE, leftInnerX, 550, passwordEditW, labelH, syscall.Handle(hwnd), 0)
		authCardText = append(authCardText, authPasswordLabel)
		if authFieldLabelFont != 0 {
			pSendMessageW.Call(uintptr(authPasswordLabel), WM_SETFONT, uintptr(authFieldLabelFont), 1)
		}
		// Multiline formatting rectangle gives the password the same reliable
		// vertical centring as the FIO fields at 125–200% Windows scaling.
		authPasswordEdit = authCreateCentered("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL|ES_PASSWORD|ES_CENTER|ES_MULTILINE, leftInnerX, 583, passwordEditW, 42, syscall.Handle(hwnd), ID_AUTH_PASSWORD)
		if authPasswordEdit != 0 && authInputFont != 0 {
			pSendMessageW.Call(uintptr(authPasswordEdit), WM_SETFONT, uintptr(authInputFont), 1)
			textRect := RECT{Left: 8, Top: 8, Right: passwordEditW - 8, Bottom: 36}
			pSendMessageW.Call(uintptr(authPasswordEdit), 0x00B3, 0, uintptr(unsafe.Pointer(&textRect))) // EM_SETRECT
		}
		authPasswordShowButton = authCreateCentered("BUTTON", "Показать", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, leftInnerX+passwordEditW+passwordGap, 583, showPasswordW, 42, syscall.Handle(hwnd), ID_AUTH_SHOW_PASSWORD)
		authRoundWindow(authPasswordShowButton, showPasswordW, 42, 6)
		// Every authentication window starts masked, including a remembered value.
		authSetPasswordMask(true)
		authRemember = authCreateCentered("BUTTON", "Запомнить пользователя", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, leftInnerX, 633, leftInnerW, 28, syscall.Handle(hwnd), ID_AUTH_REMEMBER)
		authLoginButton = authCreateCentered("BUTTON", "ВОЙТИ  →", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, leftInnerX, 670, leftInnerW, 52, syscall.Handle(hwnd), ID_AUTH_LOGIN)
		authCreateButton = authCreateCentered("BUTTON", "Создать профиль", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, leftInnerX, 732, leftInnerW, 48, syscall.Handle(hwnd), ID_AUTH_CREATE)
		authRoundWindow(authLoginButton, leftInnerW, 52, 6)
		authRoundWindow(authCreateButton, leftInnerW, 48, 6)
		forgotW := (leftInnerW - 10) / 2
		authForgotButton = authCreateCentered("BUTTON", "Забыли пароль?", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, leftInnerX, 786, forgotW, 34, syscall.Handle(hwnd), ID_AUTH_FORGOT)
		authGuestButton = authCreateCentered("BUTTON", "Войти без авторизации", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, leftInnerX+forgotW+10, 786, forgotW, 34, syscall.Handle(hwnd), ID_AUTH_GUEST)
		authCardText = append(authCardText, authForgotButton, authGuestButton)
		if authFooterFont != 0 {
			pSendMessageW.Call(uintptr(authForgotButton), WM_SETFONT, uintptr(authFooterFont), 1)
			pSendMessageW.Call(uintptr(authGuestButton), WM_SETFONT, uintptr(authFooterFont), 1)
		}
		authStatus = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, leftInnerX, 824, leftInnerW, 42, syscall.Handle(hwnd), ID_AUTH_STATUS)
		if authFooterFont != 0 {
			pSendMessageW.Call(uintptr(authStatus), WM_SETFONT, uintptr(authFooterFont), 1)
		}

		// Right card — red top line, sharp standalone icons, native labels.
		authInfoAccent = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, rightCardX, cardY, rightCardW, 7, syscall.Handle(hwnd), 0)
		authInfoBottomAccent = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, rightCardX, cardY+cardH-7, rightCardW, 7, syscall.Handle(hwnd), 0)
		authInfoLead = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_OWNERDRAW, rightInnerX, 148, rightInnerW, 108, syscall.Handle(hwnd), 0)
		authInfoLeadUnderline = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, rightInnerX+(rightInnerW-76)/2, 258, 76, 3, syscall.Handle(hwnd), 0)

		featureTitles := []string{"ЛАБОРАТОРНАЯ АНАЛИТИКА", "ОЦЕНКА ОСЛОЖНЕНИЙ", "МЕТОДЫ И МЕРОПРИЯТИЯ", "КОМПЛЕКСНАЯ ДИАГНОСТИКА"}
		featureDetails := []string{
			"История, контроль качества,\r\nтренды и сопоставление результатов.",
			"Классификация технолога,\r\nлабораторное предположение и сверка.",
			"Кандидаты с ограничениями, проверкой\r\nприменимости и контролем результата.",
			"Один объект, группа или фильтруемая\r\nвыборка с инфографикой.",
		}
		featureY := []int32{280, 390, 500, 610}
		// Compact icon + number marker.  The pictogram adds meaning; the red two-digit
		// number preserves the strict ordered rhythm of the approved reference.
		// Feature pictograms were marked for removal in the approved visual review.
		// Keep the numbered rhythm and center all captions without a decorative icon column.
		featureNumX := rightCardX + 28
		featureNumW := int32(32)
		featureTextX := rightCardX + 98
		featureTextW := rightCardW - 118
		for i := 0; i < 4; i++ {
			num := authCreateCentered("STATIC", fmt.Sprintf("%02d", i+1), WS_CHILD|WS_VISIBLE|SS_OWNERDRAW, featureNumX, featureY[i]-2, featureNumW, 40, syscall.Handle(hwnd), 0)
			authInfoNumbers[i] = num
			t := authCreateCentered("STATIC", featureTitles[i], WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE, featureTextX, featureY[i], featureTextW, 38, syscall.Handle(hwnd), 0)
			authCardText = append(authCardText, t)
			if authBoldFont != 0 {
				pSendMessageW.Call(uintptr(t), WM_SETFONT, uintptr(authBoldFont), 1)
			}
			d := authCreateCentered("STATIC", featureDetails[i], WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, featureTextX, featureY[i]+38, featureTextW, 62, syscall.Handle(hwnd), 0)
			authCardText = append(authCardText, d)
			if authBodyFont != 0 {
				pSendMessageW.Call(uintptr(d), WM_SETFONT, uintptr(authBodyFont), 1)
			}
		}
		navLabels := []string{"ДАННЫЕ", "ОЦЕНКА", "СЦЕНАРИЙ", "РЕШЕНИЕ"}
		navGap := int32(6)
		navW := (rightInnerW - 3*navGap) / 4
		for i := 0; i < 4; i++ {
			navX := rightInnerX + int32(i)*(navW+navGap)
			card := authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, navX, 730, navW, 102, syscall.Handle(hwnd), 0)
			_ = card
			// Horizontal row uses ready-made Lucide icons sourced from the public icon set.
			authNavIconBitmaps[i], _ = buildLoginStandaloneIconBitmap(true, i, 58, 58)
			iconX := navX + (navW-58)/2
			authNavIconControls[i] = authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP|SS_CENTERIMAGE, iconX, 736, 58, 58, syscall.Handle(hwnd), 0)
			if authNavIconBitmaps[i] != 0 {
				pSendMessageW.Call(uintptr(authNavIconControls[i]), STM_SETIMAGE, IMAGE_BITMAP, uintptr(authNavIconBitmaps[i]))
			}
			lab := authCreateCentered("STATIC", navLabels[i], WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX|SS_CENTERIMAGE, navX+2, 794, navW-4, 30, syscall.Handle(hwnd), 0)
			authCardText = append(authCardText, lab)
			if authNavLabelFont != 0 {
				pSendMessageW.Call(uintptr(lab), WM_SETFONT, uintptr(authNavLabelFont), 1)
			} else if authFieldLabelFont != 0 {
				pSendMessageW.Call(uintptr(lab), WM_SETFONT, uintptr(authFieldLabelFont), 1)
			}
		}

		// WELCOME_444_CREATE_END
		authHardenFeatureTitleGeometry()
		// No separate footer band: the approved background must remain visible to the
		// bottom edge. The RITEK mark is already part of the supplied source image.
		authFooterCard = 0
		authFooterText = 0
		authCancelButton = 0
		authRitekLogo = 0
		authRitekBitmap = 0

		// Password recovery is now inline: the password field temporarily becomes
		// the control-word field. No floating plaque can cover the cards.
		authRecoveryPanel, authRecoveryAccent, authRecoveryTitle, authRecoveryText, authRecoveryClose = 0, 0, 0, 0, 0

		authPopulateOrgCombos()
		authPopulateUsers()
		authApplyRememberedCredential()
		authApplyReferencePlaceholders()
		return 0
	case WM_CTLCOLOREDIT:
		pSetBkColor.Call(wParam, rgb(255, 255, 255))
		pSetTextColor.Call(wParam, rgb(15, 48, 82))
		return uintptr(authBrushWhite)
	case WM_CTLCOLORSTATIC:
		control := syscall.Handle(lParam)
		if control == authTitleAccent || control == authLeftBottomAccent || control == authInfoAccent || control == authInfoBottomAccent || control == authEntryUnderline || control == authInfoLeadUnderline || control == authHeaderRule {
			pSetBkColor.Call(wParam, rgb(203, 27, 43))
			return uintptr(authBrushRed)
		}
		if control == authTitlePanel {
			pSetBkColor.Call(wParam, rgb(238, 243, 248))
			return uintptr(authBrushPanel)
		}
		if control == authBrand || control == authRosaLogo || control == authLukoilLogo || control == authLeftCard || control == authRightCard || control == authRecoveryPanel || control == authRecoveryTitle || control == authRecoveryText {
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(authBrushWhite)
		}
		if control == authRecoveryAccent {
			pSetBkColor.Call(wParam, rgb(203, 27, 43))
			return uintptr(authBrushRed)
		}
		if control == authRosaSubtitle {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(18, 70, 135))
			return uintptr(authBrushWhite)
		}
		if control == authFooterText {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(70, 119, 171))
			return uintptr(authBrushPanel)
		}
		if control == authStatus {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(203, 27, 43))
			return uintptr(authBrushWhite)
		}
		if control == authLukoilTagline {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(24, 24, 24))
			return uintptr(authBrushWhite)
		}
		if authCardTextControl(control) {
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			if control != 0 {
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
			}
			return uintptr(authBrushWhite)
		}
		if control == authRitekLogo {
			pSetBkColor.Call(wParam, rgb(238, 243, 248))
			return uintptr(authBrushPanel)
		}
		if control == authFooterCard {
			pSetBkColor.Call(wParam, rgb(238, 243, 248))
			return uintptr(authBrushPanel)
		}
		pSetBkColor.Call(wParam, rgb(246, 248, 251))
		if control == authProductLabel || control == authEntryTitle {
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
		} else {
			pSetTextColor.Call(wParam, rgb(43, 46, 50))
		}
		return uintptr(authBrushLight)
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		// RC379: owner-drawn text must be handled explicitly. Without this branch
		// Windows leaves both the central product slogan and "Вход в РОСА" blank.
		if di != nil && di.CtlType == ODT_STATIC {
			if int(di.CtlID) == ID_AUTH_LOGIN || int(di.CtlID) == ID_AUTH_CREATE {
				// Reference bitmap already contains the exact button artwork.
				return 1
			}
			item := syscall.Handle(di.HwndItem)
			switch item {
			case authProductLabel:
				drawAuthProductLabel(di)
				return 1
			case authEntryTitle:
				drawAuthEntryTitle(di)
				return 1
			case authInfoLead:
				drawAuthInfoLead(di)
				return 1
			}
			for _, n := range authInfoNumbers {
				if n != 0 && item == n {
					drawAuthInfoNumber(di)
					return 1
				}
			}
		}
		if di != nil && di.CtlType == ODT_LISTBOX && int(di.CtlID) == ID_AUTH_USERS && di.ItemID != ^uint32(0) {
			// RC378: the saved-profile selector uses the same calm muted fill as
			// the secondary "Создать профиль" action. The user name stays bold
			// corporate blue even when the profile is selected.
			bg := authBrushMuted
			fg := rgb(15, 48, 82)
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(bg))
			buf := make([]uint16, 512)
			n, _, _ := pSendMessageW.Call(uintptr(di.HwndItem), LB_GETTEXT, uintptr(di.ItemID), uintptr(unsafe.Pointer(&buf[0])))
			if n > 0 && int(n) < len(buf) {
				pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
				pSetTextColor.Call(uintptr(di.HDC), fg)
				old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(authBoldFont))
				r := di.RcItem
				pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buf[0])), n, uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
				if old != 0 {
					pSelectObject.Call(uintptr(di.HDC), old)
				}
			}
			return 1
		}
		if di != nil && di.CtlType == ODT_BUTTON {
			brush := authBrushBlue
			textColor := rgb(255, 255, 255)
			if int(di.CtlID) == ID_AUTH_CREATE || int(di.CtlID) == ID_AUTH_CANCEL || int(di.CtlID) == ID_AUTH_FORGOT || int(di.CtlID) == ID_AUTH_GUEST || int(di.CtlID) == ID_AUTH_SHOW_PASSWORD {
				brush = authBrushWhite
				textColor = rgb(15, 48, 82)
			}
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brush))
			if int(di.CtlID) == ID_AUTH_CREATE || int(di.CtlID) == ID_AUTH_SHOW_PASSWORD {
				pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(authBrushBlue))
			}
			pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
			pSetTextColor.Call(uintptr(di.HDC), textColor)
			buttonFont := authButtonFont
			if int(di.CtlID) == ID_AUTH_FORGOT || int(di.CtlID) == ID_AUTH_GUEST || int(di.CtlID) == ID_AUTH_SHOW_PASSWORD {
				buttonFont = authProfileCaptionFont
			}
			old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(buttonFont))
			text := p(getText(di.HwndItem))
			rect := di.RcItem
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			if old != 0 {
				pSelectObject.Call(uintptr(di.HDC), old)
			}
			return 1
		}
	case WM_GETMINMAXINFO:
		// Keep the restored login window large enough for both 760 px cards.
		// Without this guard a 1360x840 restore could clip the bottom controls even
		// though the maximized layout was correct. This also prevents repeated
		// maximize/restore cycles from entering unsupported geometry.
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			if mmi != nil {
				mmi.PtMinTrackSize.X = 1080
				mmi.PtMinTrackSize.Y = 930
			}
		}
		return 0
	case WM_SIZE:
		// Never lay out the child tree while Windows is minimizing the parent. The
		// minimized client rectangle is transient and moving controls into it was the
		// main source of duplicated text/controls after restore.
		if wParam == SIZE_MINIMIZED_AUTH {
			return 0
		}
		clientW := int32(lParam & 0xFFFF)
		clientH := int32((lParam >> 16) & 0xFFFF)
		if clientW > 0 && clientH > 0 {
			// Move every child with redraw disabled, commit the whole geometry as one
			// batch, then repaint the parent+all children once on the queued message.
			authApplyResizeLayout(clientW, clientH)
			authScheduleFullRedraw(hwnd)
		}
		return 0
	case WM_AUTH_FULL_REDRAW:
		authResizeRedrawPending = false
		const redrawFlags = 0x0001 | 0x0004 | 0x0080 | 0x0100 | 0x0400 // INVALIDATE|ERASE|ALLCHILDREN|UPDATENOW|FRAME
		pRedrawWindow.Call(hwnd, 0, 0, redrawFlags)
		return 0
	case WM_COMMAND:
		id := int(loWord(wParam))
		notify := int(hiWord(wParam))
		if id == ID_AUTH_NGDO || id == ID_AUTH_CITS || id == ID_AUTH_CDNG || id == ID_AUTH_POSITION || id == ID_AUTH_LASTNAME || id == ID_AUTH_FIRSTNAME || id == ID_AUTH_MIDDLENAME {
			if notify == EN_SETFOCUS || notify == CBN_SETFOCUS_AUTH {
				authClearPlaceholderOnFocus(id)
			}
			if notify == EN_KILLFOCUS {
				authApplyReferencePlaceholders()
			}
			if notify == EN_CHANGE && !authFillingProfile {
				authSelectedLogin = ""
			}
		}
		if id == ID_AUTH_USERS && notify == LBN_SELCHANGE {
			authSelectUserFromList()
			return 0
		}
		switch id {
		case ID_AUTH_SHOW_PASSWORD:
			if authFlowMode == authFlowNormal {
				authSetPasswordMask(authPasswordVisible)
			}
			return 0
		case ID_AUTH_LOGIN:
			if authFlowMode == authFlowCreateSecurity || authFlowMode == authFlowLegacySecuritySetup {
				pw := authPendingCreate.password
				authRestoreNormalFlow(pw, "")
				return 0
			}
			if authFlowMode == authFlowRecoverSecurity {
				a, ok := authAccountByLogin(authRecoveryLogin)
				if !ok {
					authSetStatus("Профиль не найден.")
					return 0
				}
				if !verifyAccountSecurityAnswer(a, authGetText(authPasswordEdit)) {
					authSetStatus("Неверное контрольное слово.")
					return 0
				}
				rememberedLogin, rememberedPassword, remembered := loadRememberedCredential()
				if remembered && normalizeLogin(rememberedLogin) == normalizeLogin(authRecoveryLogin) && rememberedPassword != "" {
					authRestoreNormalFlow(rememberedPassword, "Контрольное слово принято. Пароль восстановлен из защищённого хранилища Windows.")
					pSendMessageW.Call(uintptr(authRemember), BM_SETCHECK, BST_CHECKED, 0)
				} else {
					authRestoreNormalFlow("", "Контрольное слово принято. Пароль на этом компьютере не сохранён — обратитесь к администратору для сброса доступа без удаления истории.")
				}
				return 0
			}
			cits := authFieldValue(authCITSEdit, authPlaceholderCITS)
			cdng := authFieldValue(authCDNGEdit, authPlaceholderCDNG)
			ngdo := authFieldValue(authNGDOEdit, authPlaceholderNGDO)
			position := authFieldValue(authPositionEdit, authPlaceholderPosition)
			lastName := strings.TrimSpace(authGetText(authLastNameEdit))
			firstName := strings.TrimSpace(authGetText(authFirstNameEdit))
			middleName := strings.TrimSpace(authGetText(authMiddleNameEdit))
			password := authGetText(authPasswordEdit)
			var s *UserSession
			var err error
			login := authSelectedLogin
			if login != "" {
				s, err = authenticateLocal(login, password)
			} else {
				s, err = authenticateLocalNamedProfile(cits, cdng, ngdo, position, lastName, firstName, middleName, password)
			}
			if err != nil {
				authSetStatus(err.Error())
				return 0
			}
			currentSession = s
			login = s.Login
			if authRememberChecked() {
				if err := saveRememberedCredential(login, password); err != nil {
					startupLog("LOGIN remember-user save failed: %v", err)
				}
			} else {
				clearRememberedCredential()
			}
			authAccepted = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case ID_AUTH_CREATE:
			if authFlowMode == authFlowRecoverSecurity {
				authRestoreNormalFlow("", "")
				return 0
			}
			if authFlowMode == authFlowCreateSecurity {
				answer := authGetText(authPasswordEdit)
				if len([]rune(strings.TrimSpace(answer))) < 4 {
					authSetStatus("Контрольное слово должно содержать не менее 4 символов.")
					return 0
				}
				pnd := authPendingCreate
				first := len(loadAuthStore().Accounts) == 0
				a, err := createLocalAccountNamedProfile(pnd.cits, pnd.cdng, pnd.ngdo, pnd.position, pnd.lastName, pnd.firstName, pnd.middleName, pnd.password, first)
				if err != nil {
					authSetStatus(err.Error())
					return 0
				}
				if err := setAccountSecurityAnswer(a.ID, answer); err != nil {
					authSetStatus("Профиль создан, но не удалось сохранить контрольное слово: " + err.Error())
					return 0
				}
				migrateLegacyDataForFirstUser(a.ID)
				authPopulateUsers()
				authRestoreNormalFlow(pnd.password, "Профиль создан и защищён контрольным словом. Теперь нажмите «Войти».")
				return 0
			}
			if authFlowMode == authFlowLegacySecuritySetup {
				answer := authGetText(authPasswordEdit)
				a, ok := authAccountByLogin(authRecoveryLogin)
				if !ok {
					authSetStatus("Профиль не найден.")
					return 0
				}
				if err := setAccountSecurityAnswer(a.ID, answer); err != nil {
					authSetStatus(err.Error())
					return 0
				}
				pw := authPendingCreate.password
				authRestoreNormalFlow(pw, "Контрольное слово сохранено. Пароль восстановлен из защищённого хранилища Windows.")
				pSendMessageW.Call(uintptr(authRemember), BM_SETCHECK, BST_CHECKED, 0)
				return 0
			}
			cits := authFieldValue(authCITSEdit, authPlaceholderCITS)
			cdng := authFieldValue(authCDNGEdit, authPlaceholderCDNG)
			ngdo := authFieldValue(authNGDOEdit, authPlaceholderNGDO)
			position := authFieldValue(authPositionEdit, authPlaceholderPosition)
			lastName := strings.TrimSpace(authGetText(authLastNameEdit))
			firstName := strings.TrimSpace(authGetText(authFirstNameEdit))
			middleName := strings.TrimSpace(authGetText(authMiddleNameEdit))
			password := authGetText(authPasswordEdit)
			if cits == "" || cdng == "" || ngdo == "" || position == "" || lastName == "" || firstName == "" {
				authSetStatus("Заполните НГДО, ЦИТС/ТПП, ЦДНГ, должность, фамилию и имя.")
				return 0
			}
			if len([]rune(password)) < 6 {
				authSetStatus("Пароль должен содержать не менее 6 символов.")
				return 0
			}
			authPendingCreate = authPendingProfile{cits: cits, cdng: cdng, ngdo: ngdo, position: position, lastName: lastName, firstName: firstName, middleName: middleName, password: password}
			authEnterControlWordFlow(authFlowCreateSecurity, "", "Придумайте контрольное слово (не менее 4 символов). Оно потребуется при восстановлении доступа.")
			return 0
		case ID_AUTH_FORGOT:
			authHandleForgotPassword(hwnd)
			return 0
		case ID_AUTH_RECOVERY_CLOSE:
			authHideRecoveryPanel()
			return 0
		case ID_AUTH_GUEST:
			s, err := authStartGuestSession()
			if err != nil {
				authSetStatus("Не удалось открыть гостевой режим: " + err.Error())
				return 0
			}
			currentSession = s
			authAccepted = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case ID_AUTH_CANCEL:
			authCancelled = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case 0x0010: // WM_CLOSE
		if !authAccepted {
			authCancelled = true
		}
		pDestroyWindowAuth.Call(hwnd)
		return 0
	case WM_DESTROY:
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func runOfflineLogin() bool {
	authAccepted, authCancelled = false, false
	startupLog("LOGIN initialization")
	authFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(18-1), 0, 0, 0, 400, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authInputFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(15-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authTitleFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(38-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authBrandFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(38-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authBoldFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(26-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authButtonFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(21-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authFieldLabelFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(17-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authProfileCaptionFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(18-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authFooterFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(17-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authBrandCaptionFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(12-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authBodyFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(18-1), 0, 0, 0, 500, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authNavLabelFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(16-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authNumberFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(28-1), 0, 0, 0, 600, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authInfoLeadFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(26-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	authBrushRed = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(203, 27, 43))))
	authBrushBlue = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(15, 48, 82))))
	authBrushLight = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(246, 248, 251))))
	authBrushWhite = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 255, 255))))
	authBrushMuted = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(229, 235, 241))))
	authBrushCard = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(255, 255, 255))))
	authBrushPanel = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(238, 243, 248))))
	authBrushCircle = syscall.Handle(mustCall(pCreateSolidBrush.Call(rgb(232, 241, 248))))
	authPenCircle = syscall.Handle(mustCall(pCreatePenAuth.Call(0, 1, rgb(232, 241, 248))))
	cls := p("ROSAOfflineAuthV1")
	hInstRaw, _, hInstErr := pGetModuleHandleW.Call(0)
	if hInstRaw == 0 {
		startupLog("LOGIN GetModuleHandleW failed: %v", hInstErr)
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(p("Не удалось инициализировать окно входа. Откройте ROSA_startup.log рядом с программой или в %APPDATA%\\OFModel."))), uintptr(unsafe.Pointer(p("РОСА — ошибка запуска"))), MB_OK|MB_ICONERROR)
		return false
	}
	loadEmbeddedAppIcons()
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(authWndProc), HInstance: syscall.Handle(hInstRaw), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	atom, _, regErr := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	// The native welcome window may be reopened with «Выход» to change user.
	// Win32 reports ERROR_CLASS_ALREADY_EXISTS (1410) for the registered class.
	if atom == 0 && regErr != syscall.Errno(1410) {
		startupLog("LOGIN RegisterClassExW failed: %v", regErr)
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(p("Windows не удалось зарегистрировать окно входа. Откройте ROSA_startup.log рядом с программой или в %APPDATA%\\OFModel."))), uintptr(unsafe.Pointer(p("РОСА — ошибка запуска"))), MB_OK|MB_ICONERROR)
		return false
	}
	// Create the login window hidden. WM_CREATE builds a fairly rich native UI and
	// prepares several raster assets. Showing the parent before WM_CREATE finishes
	// lets Windows expose half-painted child controls (combo fragments, missing
	// cards/header) on slower first launches. Reveal it only after the whole tree
	// and all bitmaps are ready.
	// Build the hidden window directly in final maximized geometry. RC402/403 used
	// WS_EX_COMPOSITED, which on some Windows/GPU combinations left native and
	// owner-drawn children permanently blank after the hidden first frame.
	authDeferredVisible = authDeferredVisible[:0]
	authConstructing = true
	hwnd, _, createErr := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — вход"))), WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN_AUTH, 0x80000000, 0x80000000, 1360, 960, 0, 0, hInstRaw, 0)
	authConstructing = false
	if hwnd == 0 {
		startupLog("LOGIN CreateWindowExW failed: %v", createErr)
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(p("Windows не удалось создать окно входа. Откройте ROSA_startup.log рядом с программой или в %APPDATA%\\OFModel."))), uintptr(unsafe.Pointer(p("РОСА — ошибка запуска"))), MB_OK|MB_ICONERROR)
		return false
	}
	startupLog("LOGIN window created hwnd=%d", hwnd)
	authWnd = syscall.Handle(hwnd)
	// Set the approved full ROSA application icon explicitly on the login window.
	// This prevents Windows from showing the compact brand-R mark or a cached generic icon.
	if hIconBig != 0 {
		pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
	}
	if hIconSmall != 0 {
		pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
	}
	// Finalize the parent geometry only after WM_CREATE has completed and every
	// centered child has been registered. This is the critical RC405 ordering fix:
	// no visible control is ever moved from construction coordinates to final ones.
	authFitHiddenWindowToWorkArea(hwnd)

	// Reveal the complete hierarchy while the parent is still hidden, then show one
	// fully laid-out frame. This avoids both blank cards and duplicated paint trails.
	authRevealDeferredChildren()
	pInvalidateRect.Call(hwnd, 0, 1)
	pShowWindow.Call(hwnd, SW_SHOW)
	const authFirstFrameRedraw = 0x0001 | 0x0004 | 0x0080 | 0x0100 | 0x0400 // INVALIDATE|ERASE|ALLCHILDREN|UPDATENOW|FRAME
	pRedrawWindow.Call(hwnd, 0, 0, authFirstFrameRedraw)
	pUpdateWindow.Call(hwnd)
	var m MSG
	for !authAccepted && !authCancelled {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	if authFont != 0 {
		pDeleteObject.Call(uintptr(authFont))
		authFont = 0
	}
	for _, resource := range []*syscall.Handle{&authInputFont, &authTitleFont, &authBrandFont, &authBoldFont, &authButtonFont, &authFieldLabelFont, &authProfileCaptionFont, &authFooterFont, &authBrandCaptionFont, &authBodyFont, &authNavLabelFont, &authNumberFont, &authInfoLeadFont, &authBrushRed, &authBrushBlue, &authBrushLight, &authBrushWhite, &authBrushMuted, &authBrushCard, &authBrushPanel, &authBrushCircle, &authPenCircle} {
		if *resource != 0 {
			pDeleteObject.Call(uintptr(*resource))
			*resource = 0
		}
	}
	for _, bitmap := range []*syscall.Handle{&authRosaBitmap, &authLukoilBitmap, &authRitekBitmap, &authThemeBitmap, &authLeftCardBitmap, &authRightCardBitmap, &authSideLeftBitmap, &authSideRightBitmap, &authBackgroundBitmap} {
		if *bitmap != 0 {
			pDeleteObject.Call(uintptr(*bitmap))
			*bitmap = 0
		}
	}
	for i := 0; i < 4; i++ {
		for _, bitmap := range []*syscall.Handle{&authFeatureIconBitmaps[i], &authNavIconBitmaps[i]} {
			if *bitmap != 0 {
				pDeleteObject.Call(uintptr(*bitmap))
				*bitmap = 0
			}
		}
	}
	ok := authAccepted && currentSession != nil
	startupLog("LOGIN finished accepted=%t cancelled=%t session=%t", authAccepted, authCancelled, currentSession != nil)
	return ok
}

func currentUserLabel() string {
	if currentSession == nil {
		return ""
	}
	if short := shortPersonName(currentSession.LastName, currentSession.FirstName, currentSession.MiddleName); short != "" {
		return short
	}
	if strings.TrimSpace(currentSession.DisplayName) != "" {
		lastName, firstName, middleName := splitPersonName(currentSession.DisplayName)
		if short := shortPersonName(lastName, firstName, middleName); short != "" {
			return short
		}
		return currentSession.DisplayName
	}
	return currentSession.Login
}

type UserPreferences struct {
	SchemaVersion          int      `json:"schema_version"`
	SelectedObjectKey      string   `json:"selected_object_key,omitempty"`
	RecentObjectKeys       []string `json:"recent_object_keys,omitempty"`
	TeachingExampleRelease string   `json:"teaching_example_release,omitempty"`
}

var teachingExampleReleaseSeen string

func loadUserPreferences() {
	var pref UserPreferences
	loadJSON(filepath.Join(dataDir, "preferences.json"), &pref)
	if pref.SchemaVersion > 1 {
		diagLog("PREFERENCES newer schema=%d", pref.SchemaVersion)
		return
	}
	selectedObjectKey = strings.TrimSpace(pref.SelectedObjectKey)
	recentObjectKeys = append([]string(nil), pref.RecentObjectKeys...)
	teachingExampleReleaseSeen = pref.TeachingExampleRelease
}

func saveUserPreferences() {
	_ = saveJSON(filepath.Join(dataDir, "preferences.json"), UserPreferences{SchemaVersion: 1, SelectedObjectKey: selectedObjectKey, RecentObjectKeys: recentObjectKeys, TeachingExampleRelease: teachingExampleReleaseSeen})
}
