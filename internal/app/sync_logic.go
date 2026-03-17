package app

import (
	"context"
	"encoding/json"
	"fmt"

	"strings"

	"github.com/remmody/VaultixIMQ/internal/mail"
	"github.com/gen2brain/beeep"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"runtime"
)

func escapeXML(s string) string {
	s = stripControlChars(s)
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func (c *Core) SyncAccount(ctx context.Context, acc mail.Account) {
	c.Batcher.Update("account_status:"+acc.Email, "syncing")
	c.Accounts.UpdateStatus(acc.Email, "syncing")

	c.syncFolder(ctx, acc, "INBOX")

	spamFolder, err := c.DiscoverSpamFolder(acc.Email)
	if err == nil && spamFolder != "" {
		c.syncFolder(ctx, acc, spamFolder)
	}

	c.Batcher.Update("account_status:"+acc.Email, "connected")
	c.Accounts.UpdateStatus(acc.Email, "connected")
}

func (c *Core) countUnread(email string) int {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	count := 0
	if folderCache, ok := c.Cache[email]; ok {
		if msgs, ok := folderCache["INBOX"]; ok {
			for _, m := range msgs {
				if !m.Seen {
					count++
				}
			}
		}
	}
	return count
}

func (c *Core) syncFolder(ctx context.Context, acc mail.Account, folderName string) {
	cacheFolder := "INBOX"
	if strings.Contains(strings.ToUpper(folderName), "SPAM") || strings.Contains(strings.ToUpper(folderName), "JUNK") {
		cacheFolder = "SPAM"
	}

	msgs, err := c.FetchEmails(acc.Email, folderName, 50)
	if err != nil {
		return
	}

	c.Mu.Lock()
	if c.Cache[acc.Email] == nil {
		c.Cache[acc.Email] = make(map[string][]mail.Message)
	}
	oldMessages := c.Cache[acc.Email][cacheFolder]
	c.Cache[acc.Email][cacheFolder] = msgs
	c.Mu.Unlock()

	if len(msgs) > 0 {
		if len(oldMessages) > 0 {
			newCount := 0
			oldUIDs := make(map[uint32]bool)
			for _, m := range oldMessages {
				oldUIDs[m.UID] = true
			}
			
			for _, m := range msgs {
				if !oldUIDs[m.UID] {
					newCount++
				}
			}

			if newCount > 0 {
				msg := msgs[0]
				if cacheFolder == "INBOX" {
					c.Accounts.UpdateLastMessageTime(acc.Email, msg.DateUnix)
				}

				if c.Settings.Notifications {
					suffix := ""
					if newCount > 1 {
						suffix = fmt.Sprintf(" (+%d more)", newCount-1)
					}
					title := fmt.Sprintf("%s: %s", cacheFolder, acc.Label)
					body := fmt.Sprintf("%s: %s%s", msg.From, msg.Subject, suffix)
					c.NotifyWithEmail(ctx, title, body, acc.Email)
					
					c.Batcher.EmitEvent("NewEmailNotification", map[string]interface{}{
						"email":   acc.Email,
						"label":   acc.Label,
						"folder":  cacheFolder,
						"subject": msg.Subject,
						"from":    msg.From,
					})
				}
			}
		}
	}

	c.Mu.Lock()
	hasKey := c.EncryptionKey != nil
	c.Mu.Unlock()

	if hasKey {
		for _, m := range msgs {
			data, err := json.Marshal(m)
			if err == nil {
				encrypted, err := c.Crypto.Encrypt(data)
				if err == nil {
					c.Storage.SaveMail(acc.Email, cacheFolder, m.UID, encrypted)
				}
			}
		}
	}

	localUIDs, err := c.Storage.ListUIDs(acc.Email, cacheFolder)
	if err == nil {
		serverUIDMap := make(map[uint32]bool)
		for _, m := range msgs {
			serverUIDMap[m.UID] = true
		}

		for _, luid := range localUIDs {
			if !serverUIDMap[luid] {
				c.Storage.DeleteMail(acc.Email, cacheFolder, luid)
			}
		}
	}

	// Update unread count for UI consistency after every sync
	if cacheFolder == "INBOX" {
		c.Batcher.Update("unread_count:"+acc.Email, c.countUnread(acc.Email))
	}
}

func (c *Core) Notify(ctx context.Context, title, message string) {
	c.NotifyWithEmail(ctx, title, message, "")
}

func (c *Core) NotifyWithEmail(ctx context.Context, title, message string, email string) {
	if c.Settings.Notifications {
		var safeTitle, safeMsg string
		if runtime.GOOS == "windows" {
			safeTitle = escapeXML(title)
			safeMsg = escapeXML(message)
		} else {
			safeTitle = stripControlChars(title)
			safeMsg = stripControlChars(message)
		}

		beeep.AppName = "VaultixIMQ"
		_ = beeep.Notify(safeTitle, safeMsg, "")
		if ctx != nil {
			wailsRuntime.EventsEmit(ctx, "notification", map[string]interface{}{
				"title": title,
				"msg":   message,
				"email": email,
			})
		}
	}
}

func stripControlChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 32 {
			if r == '\t' || r == '\n' || r == '\r' {
				b.WriteRune(r)
			}
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
