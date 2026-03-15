package app

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/remmody/VaultixIMQ/internal/config"
	"github.com/remmody/VaultixIMQ/internal/mail"
	"github.com/remmody/VaultixIMQ/internal/syncmgr"
	"github.com/remmody/VaultixIMQ/internal/totp"

	"github.com/atotto/clipboard"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	core    *Core
	locked  bool
	version string
}

func NewApp() *App {
	a := &App{}
	a.version = Version

	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "VaultixIMQ")
	os.MkdirAll(configDir, 0700)

	a.core = NewCore(configDir, nil)
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.core.Batcher.SetContext(ctx)
	a.core.InitEncryption()
	a.core.FinalizeInit()
	a.core.LoadAll()
	if a.core.Settings.SyncInterval <= 0 {
		a.core.Settings.SyncInterval = 60
	}
	if a.core.Settings.AutoLockInterval <= 0 {
		a.core.Settings.AutoLockInterval = 15
	}
	exists := config.IsVaultSet(a.core.ConfigDir)
	hasHash := a.core.Settings.AppPasswordHash != ""
	a.locked = exists && hasHash
	a.core.Sync = syncmgr.NewSyncManager(a.core.SyncAccount, a.core.Accounts.GetAccounts)
	if !a.locked && a.core.Settings.AutoLogin {
		go a.core.Sync.Start(ctx, a.core.Settings.SyncInterval)
	}
}

// --- Lock & Password Bindings ---

func (a *App) IsLocked() bool {
	return a.locked
}

func (a *App) IsPasswordSet() bool {
	return a.core.Settings.AppPasswordHash != ""
}

func (a *App) NeedsSetup() bool {
	return !config.IsVaultSet(a.core.ConfigDir)
}

func (a *App) VerifyAppPassword(password string) bool {
	return a.core.VerifyAppPassword(password)
}

func (a *App) SetAppPassword(password string) {
	a.core.SetAppPassword(password)
	a.locked = false
	if a.core.Settings.AutoLogin {
		go a.core.Sync.Start(a.ctx, a.core.Settings.SyncInterval)
	}
}

func (a *App) SkipAppPasswordSetup() {
	a.core.SkipAppPasswordSetup()
}

func (a *App) UnlockApp(password string) bool {
	if a.core.VerifyAppPassword(password) {
		a.locked = false
		if a.core.Settings.AutoLogin {
			go a.core.Sync.Start(a.ctx, a.core.Settings.SyncInterval)
		}
		return true
	}
	return false
}

func (a *App) LockVault() { a.locked = true }
func (a *App) LockApp()   { a.locked = true }

// --- Bindings ---

func (a *App) GetAccountsLight() []mail.AccountLight {
	accs := a.core.Accounts.GetAccounts()
	a.core.Mu.Lock()
	defer a.core.Mu.Unlock()

	light := make([]mail.AccountLight, len(accs))
	for i := range accs {
		count := 0
		if folderCache, ok := a.core.Cache[accs[i].Email]; ok {
			if msgs, ok := folderCache["INBOX"]; ok {
				for _, m := range msgs {
					if !m.Seen {
						count++
					}
				}
			}
		}
		light[i] = mail.AccountLight{
			Email:           accs[i].Email,
			Label:           accs[i].Label,
			Status:          accs[i].Status,
			UnreadCount:     count,
			LastMessageTime: accs[i].LastMessageTime,
		}
	}
	return light
}

func (a *App) GetAccounts() []mail.Account {
	return a.core.Accounts.GetAccounts()
}

func (a *App) GetAccountDetails(email string) mail.Account {
	accs := a.core.Accounts.GetAccounts()
	for _, acc := range accs {
		if acc.Email == email {
			return acc
		}
	}
	return mail.Account{Email: email}
}

func (a *App) SetVisibleAccounts(emails []string) {
	if a.core.Sync != nil {
		a.core.Sync.SetVisibleAccounts(emails)
	}
}

func (a *App) AddAccount(email, password, host, port, label string) bool {
	success := a.core.Accounts.Add(mail.Account{
		Email:    email,
		Password: password,
		Host:     host,
		Port:     port,
		Label:    label,
	})
	if success {
		a.core.SaveVault()
		if acc, ok := a.core.Accounts.Find(email); ok && a.core.Sync != nil {
			a.core.Sync.TriggerImmediate(acc)
		}
	}
	return success
}

func (a *App) DeleteAccount(email string) {
	a.core.Accounts.Delete(email)
	a.core.SaveVault()
}

func (a *App) BulkAddAccounts(filePath string, imapServer string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	host := imapServer
	port := "993"
	if strings.Contains(imapServer, ":") {
		parts := strings.Split(imapServer, ":")
		host = parts[0]
		port = parts[1]
	}

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		acc := mail.Account{
			Email:    parts[0],
			Password: parts[1],
			Host:     host,
			Port:     port,
		}
		if a.core.Accounts.Add(acc) {
			count++
		}
	}

	if count > 0 {
		a.core.SaveVault()
	}

	return count, scanner.Err()
}

func (a *App) GetCachedMessages(email string, folder string) []mail.Message {
	a.core.Mu.Lock()
	defer a.core.Mu.Unlock()
	if folder == "" {
		folder = "INBOX"
	}
	if folderCache, ok := a.core.Cache[email]; ok {
		if msgs, ok := folderCache[folder]; ok {
			return msgs
		}
	}
	return []mail.Message{}
}

func (a *App) FetchInbox(emailAddress string, limit int) ([]mail.Message, error) {
	return a.core.FetchInbox(emailAddress, limit)
}

func (a *App) FetchEmails(emailAddress string, folder string, limit int) ([]mail.Message, error) {
	return a.core.FetchEmails(emailAddress, folder, limit)
}

func (a *App) DiscoverSpamFolder(email string) (string, error) {
	return a.core.DiscoverSpamFolder(email)
}

func (a *App) FetchBody(emailAddress string, folder string, uid uint32) ([]interface{}, error) {
	return a.core.FetchBody(emailAddress, folder, uid)
}

func (a *App) MarkAsRead(email string, folder string, uid uint32) error {
	return a.core.MarkAsRead(email, folder, uid)
}

func (a *App) MarkAllAsRead(email string, folder string) error {
	return a.core.MarkAllAsRead(email, folder)
}

func (a *App) ExportProfile(password string, outputPath string) error {
	return a.core.ExportProfile(password, outputPath)
}

func (a *App) ImportProfile(password string, inputPath string) error {
	currentHash := a.core.Settings.AppPasswordHash
	currentSalt := a.core.Settings.AppPasswordSalt
	currentSetup := a.core.Settings.AppPasswordSetupDone

	err := a.core.ImportProfile(password, inputPath)
	if err != nil {
		return err
	}

	a.core.Settings.AppPasswordHash = currentHash
	a.core.Settings.AppPasswordSalt = currentSalt
	a.core.Settings.AppPasswordSetupDone = currentSetup

	a.core.FinalizeInit()
	return a.core.SaveVault()
}

func (a *App) GenerateTOTP(secret string) (totp.Response, error) {
	return a.core.TOTP.Generate(secret)
}

func (a *App) AddTOTP(accountName, secret, issuer, account string) {
	a.core.TOTP.Add(totp.Entry{
		AccountName:   accountName,
		Issuer:        issuer,
		Secret:        secret,
		LinkedAccount: account,
	})
	a.core.SaveVault()
}

func (a *App) DeleteTOTP(accountName string) {
	a.core.TOTP.Delete(accountName)
	a.core.SaveVault()
}

func (a *App) GetTOTPList() []totp.Entry {
	return a.core.TOTP.GetEntries()
}

func (a *App) GetSettings() Settings {
	return a.core.Settings
}

func (a *App) UpdateSettings(s Settings) {
	a.core.Settings = s
	a.core.SaveVault()
	if a.core.Sync != nil {
		a.core.Sync.Start(a.ctx, s.SyncInterval)
	}
}

func (a *App) GetNextSync() int {
	if a.core.Sync == nil {
		return 0
	}
	next := a.core.Sync.GetNextSync()
	diff := time.Until(next)
	if diff < 0 {
		return 0
	}
	return int(diff.Seconds())
}

func (a *App) SelectSavePath() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "profile_backup.vaultix",
		Title:           "Save Profile Backup",
		Filters: []runtime.FileFilter{
			{DisplayName: "Vaultix Backup (*.vaultix)", Pattern: "*.vaultix"},
		},
	})
}

func (a *App) SelectOpenPath() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Profile Backup",
		Filters: []runtime.FileFilter{
			{DisplayName: "Vaultix Backup (*.vaultix)", Pattern: "*.vaultix"},
		},
	})
}

func (a *App) SelectBulkImportPath() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Account List (.txt)",
		Filters: []runtime.FileFilter{
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
}

func (a *App) CopyToClipboard(text string) bool {
	err := clipboard.WriteAll(text)
	return err == nil
}

type AboutInfo struct {
	Version string `json:"version"`
	Author  string `json:"author"`
	License string `json:"license"`
	GitHub  string `json:"github"`
}

func (a *App) GetAboutInfo() AboutInfo {
	return AboutInfo{
		Version: Version,
		Author:  "RemmoDY",
		License: "MIT License",
		GitHub:  "https://github.com/remmody/VaultixIMQ",
	}
}

func (a *App) CheckForUpdates() (*UpdateInfo, error) {
	return a.core.CheckForUpdates()
}

func (a *App) StartEngine() {
	if !a.locked && a.core.Sync != nil {
		go a.core.Sync.Start(a.ctx, a.core.Settings.SyncInterval)
	}
}

func (a *App) ChangeAppPassword(old, new string) error {
	return a.core.ChangeAppPassword(old, new)
}

func (a *App) IsUnsecuredImport() bool {
	return false
}

func (a *App) IsVaultSet() bool {
	_, err := os.Stat(filepath.Join(a.core.ConfigDir, "vault.vxc"))
	return err == nil
}
