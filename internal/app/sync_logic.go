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
	s = b.String()
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func (c *Core) SyncAccount(ctx context.Context, acc mail.Account) {
	if ctx != nil {
		wailsRuntime.EventsEmit(ctx, "sync_start", acc.Email)
	}

	msgs, err := c.FetchInbox(acc.Email, 50)
	if err != nil {
		if ctx != nil {
			wailsRuntime.EventsEmit(ctx, "sync_error", acc.Email)
		}
		return
	}

	c.Mu.Lock()
	oldMessages := c.Cache[acc.Email]
	c.Cache[acc.Email] = msgs
	c.Mu.Unlock()

	// Check for new messages
	if len(msgs) > 0 {
		if len(oldMessages) > 0 {
			newCount := 0
			latestOldUID := oldMessages[0].UID
			for _, m := range msgs {
				if m.UID > latestOldUID {
					newCount++
				}
			}

			if newCount > 0 {
				if c.Settings.Notifications {
					msg := msgs[0]
					suffix := ""
					if newCount > 1 {
						suffix = fmt.Sprintf(" (+%d more)", newCount-1)
					}
					// Cleaner title: "Inbox: [Label]"
					title := fmt.Sprintf("Inbox: %s", acc.Label)
					body := fmt.Sprintf("%s: %s%s", msg.From, msg.Subject, suffix)
					c.Notify(ctx, title, body)
				}
			}
		} else {
			// fmt.Printf("Sync: Initial sync for %s, %d messages loaded into cache.\n", acc.Email, len(msgs))
		}
	}

	// Phase 9: Save to persistent storage
	for _, m := range msgs {
		data, err := json.Marshal(m)
		if err == nil {
			encrypted, err := c.Crypto.Encrypt(data)
			if err == nil {
				c.Storage.SaveMail(acc.Email, m.UID, encrypted)
			}
		}
	}

	if ctx != nil {
		wailsRuntime.EventsEmit(ctx, "sync_complete", acc.Email)
	}

	// Phase 31: Sync Deletions
	localUIDs, err := c.Storage.ListUIDs(acc.Email)
	if err == nil {
		serverUIDMap := make(map[uint32]bool)
		var minServerUID uint32
		hasMsgs := len(msgs) > 0

		if hasMsgs {
			minServerUID = msgs[0].UID
			for _, m := range msgs {
				serverUIDMap[m.UID] = true
				if m.UID < minServerUID {
					minServerUID = m.UID
				}
			}
		}

		deletedAny := false
		for _, luid := range localUIDs {
			// If we have messages, sync deletions within the current window
			// If we have NO messages, it means the server is empty (for the last 50 window),
			// so we delete everything we previously thought was there.
			if !hasMsgs || luid >= minServerUID {
				if !serverUIDMap[luid] {
					c.Storage.DeleteMail(acc.Email, luid)
					deletedAny = true
				}
			}
		}

		if deletedAny {
			// Refresh cache after deletions
			c.Mu.Lock()
			currentCache := c.Cache[acc.Email]
			newCache := make([]mail.Message, 0, len(currentCache))
			for _, m := range currentCache {
				if hasMsgs {
					if m.UID < minServerUID || serverUIDMap[m.UID] {
						newCache = append(newCache, m)
					}
				}
				// if !hasMsgs, newCache remains empty (all deleted)
			}
			c.Cache[acc.Email] = newCache
			c.Mu.Unlock()
		}
	}
}

func (c *Core) Notify(ctx context.Context, title, message string) {
	if c.Settings.Notifications {
		// OS-specific sanitization
		var safeTitle, safeMsg string
		if runtime.GOOS == "windows" {
			safeTitle = escapeXML(title)
			safeMsg = escapeXML(message)
		} else {
			// On Linux/others, just strip control characters but don't escape XML
			safeTitle = stripControlChars(title)
			safeMsg = stripControlChars(message)
		}
		
		// System notification
		err := beeep.Notify(safeTitle, safeMsg, "")
		if err != nil {
			// fmt.Printf("System Notification error: %v\n", err)
			// fmt.Printf("GOOS: %s\n", runtime.GOOS)
			// fmt.Printf("Attempted Title: [%s]\n", safeTitle)
			// fmt.Printf("Attempted Msg: [%s]\n", safeMsg)
		}
		
		// In-app notification
		if ctx != nil {
			wailsRuntime.EventsEmit(ctx, "notification", map[string]interface{}{
				"title": title,
				"msg":   message,
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
