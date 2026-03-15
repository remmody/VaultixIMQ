package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/remmody/VaultixIMQ/internal/account"
	"github.com/remmody/VaultixIMQ/internal/backup"
	"github.com/remmody/VaultixIMQ/internal/crypto"
	"github.com/remmody/VaultixIMQ/internal/mail"
	"github.com/remmody/VaultixIMQ/internal/storage"
	"github.com/remmody/VaultixIMQ/internal/syncmgr"
	"github.com/remmody/VaultixIMQ/internal/totp"

	"github.com/emersion/go-imap/client"
)

var Version = "1.0.0"

type Settings struct {
	SyncInterval         int    `json:"sync_interval"`
	AutoLogin            bool   `json:"auto_login"`
	Notifications        bool   `json:"notifications"`
	Sound                bool   `json:"sound"`
	AppPasswordHash      string `json:"app_password_hash,omitempty"`
	AppPasswordSalt      string `json:"app_password_salt,omitempty"`
	AutoLockInterval     int    `json:"auto_lock_interval,omitempty"` // in minutes
	AppPasswordSetupDone bool   `json:"app_password_setup_done"`
}

type Vault struct {
	Accounts []mail.Account `json:"accounts"`
	TOTP     []totp.Entry   `json:"totp"`
	Settings Settings       `json:"settings"`
}

type Core struct {
	Accounts      *account.Manager
	TOTP          *totp.Manager
	Sync          *syncmgr.SyncManager
	Crypto        *crypto.Manager
	Storage       *storage.MailStorage
	Clients       map[string]*client.Client
	Cache         map[string]map[string][]mail.Message
	Settings      Settings
	ConfigDir     string
	EncryptionKey []byte
	Mu            sync.Mutex             // For overall state (Cache, Clients map)
	AccountMus    map[string]*sync.Mutex // For per-account IMAP client access
	Batcher       *Batcher               // For throttled UI updates
}

func NewCore(configDir string, encryptionKey []byte) *Core {
	c := &Core{
		ConfigDir:     configDir,
		EncryptionKey: encryptionKey,
		Clients:       make(map[string]*client.Client),
		AccountMus:    make(map[string]*sync.Mutex),
		Cache:         make(map[string]map[string][]mail.Message),
		Settings: Settings{ // Default settings
			SyncInterval:  10,
			AutoLogin:     true,
			Notifications: true,
			Sound:         true,
		},
	}
	c.Storage = storage.NewMailStorage(configDir)
	c.Accounts = account.NewManager(configDir)
	c.TOTP = totp.NewManager(configDir)
	c.Batcher = NewBatcher(context.TODO(), 800*time.Millisecond) // Global throttler
	return c
}

func (c *Core) GetAccountMutex(email string) *sync.Mutex {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if mu, ok := c.AccountMus[email]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	c.AccountMus[email] = mu
	return mu
}

func (c *Core) FinalizeInit() {
	if c.EncryptionKey != nil {
		c.Crypto = crypto.NewManager(c.EncryptionKey)
		c.Accounts.SetCrypto(c.Crypto)
		c.TOTP.SetCrypto(c.Crypto)
	}
}

func (c *Core) LoadAll() {
	if err := c.LoadVault(); err != nil {
		c.MigrateLegacy()
	}
}

func (c *Core) LoadVault() error {
	path := filepath.Join(c.ConfigDir, "vault.vxc")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var v Vault
	if err := c.Crypto.DecryptJSON(data, &v); err != nil {
		return err
	}

	c.applyVault(v)
	return nil
}

func (c *Core) applyVault(v Vault) {
	changed := false
	for i := range v.Accounts {
		oldHost := v.Accounts[i].Host
		v.Accounts[i].Finalize()
		if v.Accounts[i].Host != oldHost {
			changed = true
		}
	}

	c.Settings = v.Settings
	c.Accounts.SetAccounts(v.Accounts)
	c.TOTP.SetEntries(v.TOTP)

	c.loadCache(v.Accounts)

	if changed {
		c.SaveVault()
	}
}

func (c *Core) loadCache(accounts []mail.Account) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	for _, acc := range accounts {
		if c.Cache[acc.Email] == nil {
			c.Cache[acc.Email] = make(map[string][]mail.Message)
		}
		// Load INBOX by default
		for _, folder := range []string{"INBOX", "SPAM"} {
			uids, err := c.Storage.ListUIDs(acc.Email, folder)
			if err != nil || len(uids) == 0 {
				continue
			}
			sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })

			limit := 50
			if len(uids) < limit {
				limit = len(uids)
			}

			msgs := make([]mail.Message, 0, limit)
			for i := 0; i < limit; i++ {
				if m, err := c.loadMessage(acc.Email, folder, uids[i]); err == nil {
					msgs = append(msgs, m)
				}
			}
			c.Cache[acc.Email][folder] = msgs
		}
	}
}

func (c *Core) loadMessage(email string, folder string, uid uint32) (mail.Message, error) {
	data, err := c.Storage.LoadMail(email, folder, uid)
	if err != nil {
		return mail.Message{}, err
	}
	decrypted, err := c.Crypto.Decrypt(data)
	if err != nil {
		return mail.Message{}, err
	}
	var m mail.Message
	err = json.Unmarshal(decrypted, &m)
	return m, err
}

func (c *Core) SaveVault() error {
	if c.Crypto == nil || c.EncryptionKey == nil {
		return nil
	}
	v := Vault{
		Accounts: c.Accounts.GetAccounts(),
		TOTP:     c.TOTP.GetEntries(),
		Settings: c.Settings,
	}
	data, err := c.Crypto.EncryptJSON(v)
	if err != nil {
		return err
	}
	path := filepath.Join(c.ConfigDir, "vault.vxc")
	return os.WriteFile(path, data, 0600)
}

func (c *Core) MigrateLegacy() {
	v := Vault{Settings: c.Settings}
	migrated := []string{}

	migrated = append(migrated, c.migrateAccounts(&v)...)
	migrated = append(migrated, c.migrateTOTP(&v)...)
	migrated = append(migrated, c.migrateSettings(&v)...)

	if len(migrated) == 0 {
		return
	}

	c.Settings = v.Settings
	c.Accounts.SetAccounts(v.Accounts)
	c.TOTP.SetEntries(v.TOTP)

	if err := c.SaveVault(); err == nil {
		c.cleanupLegacy(migrated)
	}
}

func (c *Core) tryLoad(data []byte, target interface{}) bool {
	if c.Crypto != nil && c.Crypto.DecryptJSON(data, target) == nil {
		return true
	}
	return json.Unmarshal(data, target) == nil
}

func (c *Core) migrateAccounts(v *Vault) []string {
	paths := []string{
		filepath.Join(c.ConfigDir, "accounts.vxc"),
		filepath.Join(c.ConfigDir, "accounts.json"),
		filepath.Join(c.ConfigDir, "account.json"),
	}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var accs []mail.Account
			if c.tryLoad(data, &accs) && len(accs) > 0 {
				for i := range accs {
					accs[i].Finalize()
				}
				v.Accounts = accs
				return []string{p}
			}
		}
	}
	return nil
}

func (c *Core) migrateTOTP(v *Vault) []string {
	paths := []string{
		filepath.Join(c.ConfigDir, "totp.vxc"),
		filepath.Join(c.ConfigDir, "totp.json"),
		filepath.Join(c.ConfigDir, "totp_secrets.json"),
	}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var entries []totp.Entry
			if c.tryLoad(data, &entries) && len(entries) > 0 {
				v.TOTP = entries
				return []string{p}
			}
		}
	}
	return nil
}

func (c *Core) migrateSettings(v *Vault) []string {
	paths := []string{
		filepath.Join(c.ConfigDir, "settings.vxc"),
		filepath.Join(c.ConfigDir, "settings.json"),
	}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var s Settings
			if c.tryLoad(data, &s) && s.SyncInterval > 0 {
				v.Settings = s
				return []string{p}
			}
		}
	}
	return nil
}

func (c *Core) cleanupLegacy(migrated []string) {
	for _, p := range migrated {
		os.Remove(p)
	}
}

func (c *Core) SaveSettings() {
	c.SaveVault()
}

func (c *Core) SetAppPassword(password string) {
	c.Settings.AppPasswordSalt = base64.StdEncoding.EncodeToString(generateSalt())
	hash := crypto.DeriveKey(password, c.DecodeSalt(c.Settings.AppPasswordSalt))
	c.Settings.AppPasswordHash = base64.StdEncoding.EncodeToString(hash)
	c.Settings.AppPasswordSetupDone = true
	c.SaveVault()
}

func (c *Core) ChangeAppPassword(old, new string) error {
	if !c.VerifyAppPassword(old) {
		return fmt.Errorf("invalid current password")
	}
	newSalt := generateSalt()
	newHash := crypto.DeriveKey(new, newSalt)

	c.Settings.AppPasswordSalt = base64.StdEncoding.EncodeToString(newSalt)
	c.Settings.AppPasswordHash = base64.StdEncoding.EncodeToString(newHash)
	c.Settings.AppPasswordSetupDone = true

	return c.SaveVault()
}

func (c *Core) SkipAppPasswordSetup() {
	c.Settings.AppPasswordSetupDone = true
	c.Settings.AppPasswordHash = ""
	c.Settings.AppPasswordSalt = ""
	c.SaveVault()
}

func (c *Core) VerifyAppPassword(password string) bool {
	if c.Settings.AppPasswordHash == "" {
		return true
	}
	salt := c.DecodeSalt(c.Settings.AppPasswordSalt)
	hash := crypto.DeriveKey(password, salt)
	return base64.StdEncoding.EncodeToString(hash) == c.Settings.AppPasswordHash
}

func generateSalt() []byte {
	s := make([]byte, 16)
	_, _ = rand.Read(s)
	return s
}

func (c *Core) DecodeSalt(s string) []byte {
	d, _ := base64.StdEncoding.DecodeString(s)
	return d
}

func (c *Core) ExportProfile(password string, outputPath string) error {
	v := Vault{
		Accounts: c.Accounts.GetAccounts(),
		TOTP:     c.TOTP.GetEntries(),
		Settings: c.Settings,
	}
	v.Settings.AppPasswordHash = ""
	v.Settings.AppPasswordSalt = ""
	v.Settings.AppPasswordSetupDone = false

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return backup.ExportProfile(c.EncryptionKey, data, password, outputPath)
}

func (c *Core) ImportProfile(password string, inputPath string) error {
	_, vaultData, err := backup.ImportProfile(password, inputPath)
	if err != nil {
		return err
	}

	var v Vault
	if err := json.Unmarshal(vaultData, &v); err != nil {
		return err
	}

	// Сбрасываем старые настройки пароля из бэкапа
	v.Settings.AppPasswordSetupDone = false
	v.Settings.AppPasswordHash = ""
	v.Settings.AppPasswordSalt = ""

	c.Settings = v.Settings
	c.Accounts.SetAccounts(v.Accounts)
	c.TOTP.SetEntries(v.TOTP)

	// Мы убрали отсюда c.FinalizeInit() и c.SaveVault(),
	// потому что app.go и так вызывает их сразу после этой функции
	return nil
}
