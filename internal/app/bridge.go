package app

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/remmody/VaultixIMQ/internal/mail"
	"github.com/remmody/VaultixIMQ/internal/syncmgr"
	"github.com/remmody/VaultixIMQ/internal/totp"

	"github.com/atotto/clipboard"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx  context.Context
	core *Core
	locked bool
	version string
}

func NewApp() *App {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "VaultixIMQ")
	os.MkdirAll(configDir, 0700)

	a := &App{}
	a.core = NewCore(configDir, nil)
	a.core.InitEncryption()
	a.core.FinalizeInit() // Important: Initialize crypto manager with the retrieved key
	a.core.LoadAll()      // Now that key is ready, load data (with migration)
	
	// Initialize Sync Manager
	a.core.Sync = syncmgr.NewSyncManager(a.core.SyncAccount, a.core.Accounts.GetAccounts)
	
	a.locked = a.core.Settings.AppPasswordHash != ""
	a.version = Version
	
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	
	if a.core.Settings.AutoLogin {
		a.core.Sync.Start(ctx, a.core.Settings.SyncInterval)
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
	return !a.core.Settings.AppPasswordSetupDone
}

func (a *App) VerifyAppPassword(password string) bool {
	return a.core.VerifyAppPassword(password)
}

func (a *App) SetAppPassword(password string) {
	a.core.SetAppPassword(password)
}

func (a *App) SkipAppPasswordSetup() {
	a.core.SkipAppPasswordSetup()
}

func (a *App) UnlockApp(password string) bool {
	if a.core.VerifyAppPassword(password) {
		a.locked = false
		return true
	}
	return false
}

func (a *App) LockApp() {
	if a.IsPasswordSet() {
		a.locked = true
	}
}

// --- Bindings ---

func (a *App) GetAccounts() []mail.Account {
	accs := a.core.Accounts.GetAccounts()
	a.core.Mu.Lock()
	defer a.core.Mu.Unlock()

	for i := range accs {
		count := 0
		if msgs, ok := a.core.Cache[accs[i].Email]; ok {
			for _, m := range msgs {
				if !m.Seen {
					count++
				}
			}
		}
		accs[i].UnreadCount = count
	}
	return accs
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

func (a *App) GetCachedMessages(email string) []mail.Message {
	a.core.Mu.Lock()
	defer a.core.Mu.Unlock()
	msgs := a.core.Cache[email]
	if msgs == nil { return []mail.Message{} }
	return msgs
}

func (a *App) FetchInbox(emailAddress string, limit int) ([]mail.Message, error) {
	return a.core.FetchInbox(emailAddress, limit)
}

func (a *App) FetchBody(emailAddress string, uid uint32) ([]interface{}, error) {
	return a.core.FetchBody(emailAddress, uid)
}

func (a *App) MarkAsRead(email string, uid uint32) error {
	return a.core.MarkAsRead(email, uid)
}

func (a *App) MarkAllAsRead(email string) error {
	return a.core.MarkAllAsRead(email)
}

func (a *App) ExportProfile(password string, outputPath string) error {
	return a.core.ExportProfile(password, outputPath)
}

func (a *App) ImportProfile(password string, inputPath string) error {
	return a.core.ImportProfile(password, inputPath)
}

func (a *App) GenerateTOTP(secret string) (totp.Response, error) {
	return a.core.TOTP.Generate(secret)
}

func (a *App) AddTOTP(accountName, secret, issuer, account string) {
	a.core.TOTP.Add(totp.Entry{
		AccountName: accountName,
		Issuer:      issuer,
		Secret:      secret,
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
	// Restart sync loop with new interval
	a.core.Sync.Start(a.ctx, s.SyncInterval)
}

func (a *App) GetNextSync() int {
	next := a.core.Sync.GetNextSync()
	diff := time.Until(next)
	if diff < 0 {
		return 0
	}
	return int(diff.Seconds())
}

func (a *App) SelectSavePath() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename:      "profile_backup.vaultix",
		Title:                "Save Profile Backup",
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
