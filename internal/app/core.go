package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/remmody/VaultixIMQ/internal/account"
	"github.com/remmody/VaultixIMQ/internal/backup"
	"github.com/remmody/VaultixIMQ/internal/crypto"
	"github.com/remmody/VaultixIMQ/internal/mail"
	"github.com/remmody/VaultixIMQ/internal/storage"
	"github.com/remmody/VaultixIMQ/internal/syncmgr"
	"github.com/remmody/VaultixIMQ/internal/totp"

	"github.com/emersion/go-imap/client"
)

var Version = "1.0.0" // Default version, should be overridden by ldflags during build

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
	Cache         map[string][]mail.Message
	Settings      Settings
	ConfigDir     string
	EncryptionKey []byte
	Mu            sync.Mutex             // For overall state (Cache, Clients map)
	AccountMus    map[string]*sync.Mutex // For per-account IMAP client access
	WailsCtx      context.Context
}

func NewCore(configDir string, encryptionKey []byte) *Core {
	c := &Core{
		ConfigDir:     configDir,
		EncryptionKey: encryptionKey,
		Clients:       make(map[string]*client.Client),
		AccountMus:    make(map[string]*sync.Mutex),
		Cache:         make(map[string][]mail.Message),
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
	c.Crypto = crypto.NewManager(c.EncryptionKey)
	c.Accounts.SetCrypto(c.Crypto)
	c.TOTP.SetCrypto(c.Crypto)
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

	// Fix/Update accounts during load
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

	// Pre-load cache from storage
	c.Mu.Lock()
	for _, acc := range v.Accounts {
		uids, err := c.Storage.ListUIDs(acc.Email)
		if err == nil && len(uids) > 0 {
			// Sort descending
			sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })

			limit := 50
			if len(uids) < limit {
				limit = len(uids)
			}

			msgs := make([]mail.Message, 0, limit)
			for i := 0; i < limit; i++ {
				data, err := c.Storage.LoadMail(acc.Email, uids[i])
				if err == nil {
					var m mail.Message
					decrypted, err := c.Crypto.Decrypt(data)
					if err == nil {
						if err := json.Unmarshal(decrypted, &m); err == nil {
							msgs = append(msgs, m)
						}
					}
				}
			}
			c.Cache[acc.Email] = msgs
			// fmt.Printf("Cache loaded: %d messages for %s\n", len(msgs), acc.Email)
		}
	}
	c.Mu.Unlock()

	if changed {
		c.SaveVault()
	}
	return nil
}

func (c *Core) SaveVault() error {
	if c.Crypto == nil {
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
	v := Vault{
		Settings: c.Settings,
	}
	migratedPaths := []string{}

	// Helper to try decrypt then unmarshal
	tryLoad := func(data []byte, target interface{}) bool {
		if c.Crypto != nil {
			if err := c.Crypto.DecryptJSON(data, target); err == nil {
				return true
			}
		}
		return json.Unmarshal(data, target) == nil
	}

	// 1. Accounts
	accPaths := []string{
		filepath.Join(c.ConfigDir, "accounts.vxc"),
		filepath.Join(c.ConfigDir, "accounts.json"),
		filepath.Join(c.ConfigDir, "account.json"),
	}
	for _, p := range accPaths {
		if data, err := os.ReadFile(p); err == nil {
			var accs []mail.Account
			if tryLoad(data, &accs) {
				if len(accs) > 0 {
					for i := range accs {
						accs[i].Finalize()
					}
					v.Accounts = accs
					migratedPaths = append(migratedPaths, p)
					break
				}
			} else {
				// Try as single object
				var single mail.Account
				if tryLoad(data, &single) && single.Email != "" {
					single.Finalize()
					v.Accounts = []mail.Account{single}
					migratedPaths = append(migratedPaths, p)
					break
				}
			}
		}
	}

	// 2. TOTP
	totpPaths := []string{
		filepath.Join(c.ConfigDir, "totp.vxc"),
		filepath.Join(c.ConfigDir, "totp.json"),
		filepath.Join(c.ConfigDir, "totp_secrets.json"),
	}
	for _, p := range totpPaths {
		if data, err := os.ReadFile(p); err == nil {
			var entries []totp.Entry
			if tryLoad(data, &entries) && len(entries) > 0 {
				v.TOTP = entries
				migratedPaths = append(migratedPaths, p)
				break
			}
		}
	}

	// 3. Settings
	setPaths := []string{
		filepath.Join(c.ConfigDir, "settings.vxc"),
		filepath.Join(c.ConfigDir, "settings.json"),
	}
	for _, p := range setPaths {
		if data, err := os.ReadFile(p); err == nil {
			var s Settings
			if tryLoad(data, &s) && s.SyncInterval > 0 {
				v.Settings = s
				migratedPaths = append(migratedPaths, p)
				break
			}
		}
	}

	// Apply and Save
	c.Settings = v.Settings
	c.Accounts.SetAccounts(v.Accounts)
	c.TOTP.SetEntries(v.TOTP)

	if err := c.SaveVault(); err == nil {
		for _, p := range migratedPaths {
			os.Remove(p)
		}
		// Also clean up common variants
		for _, p := range accPaths {
			os.Remove(p)
		}
		for _, p := range totpPaths {
			os.Remove(p)
		}
		for _, p := range setPaths {
			os.Remove(p)
		}
	}
}

func (c *Core) SaveSettings() {
	c.SaveVault()
}

func (c *Core) SetAppPassword(password string) {
	c.Settings.AppPasswordSalt = base64.StdEncoding.EncodeToString(generateSalt())
	hash := crypto.DeriveKey(password, decodeSalt(c.Settings.AppPasswordSalt))
	c.Settings.AppPasswordHash = base64.StdEncoding.EncodeToString(hash)
	c.Settings.AppPasswordSetupDone = true
	c.SaveVault()
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
	salt := decodeSalt(c.Settings.AppPasswordSalt)
	hash := crypto.DeriveKey(password, salt)
	return base64.StdEncoding.EncodeToString(hash) == c.Settings.AppPasswordHash
}

func generateSalt() []byte {
	s := make([]byte, 16)
	_, _ = rand.Read(s)
	return s
}

func decodeSalt(s string) []byte {
	d, _ := base64.StdEncoding.DecodeString(s)
	return d
}

func (c *Core) ExportProfile(password string, outputPath string) error {
	// Note: The UI should check app password BEFORE calling this if set
	v := Vault{
		Accounts: c.Accounts.GetAccounts(),
		TOTP:     c.TOTP.GetEntries(),
		Settings: c.Settings,
	}
	// Strip sensitive app password fields for export
	v.Settings.AppPasswordHash = ""
	v.Settings.AppPasswordSalt = ""
	v.Settings.AppPasswordSetupDone = false // Allow recipient to set their own

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return backup.ExportProfile(c.EncryptionKey, data, password, outputPath)
}

func (c *Core) ImportProfile(password string, inputPath string) error {
	key, vaultData, err := backup.ImportProfile(password, inputPath)
	if err != nil {
		return err
	}
	c.EncryptionKey = key

	var v Vault
	if err := json.Unmarshal(vaultData, &v); err != nil {
		return err
	}

	// Reset password setup flag to ask the user on this new machine/installation
	v.Settings.AppPasswordSetupDone = false
	v.Settings.AppPasswordHash = ""
	v.Settings.AppPasswordSalt = ""

	c.Settings = v.Settings
	c.Accounts.SetAccounts(v.Accounts)
	c.TOTP.SetEntries(v.TOTP)

	c.FinalizeInit()
	return c.SaveVault()
}
