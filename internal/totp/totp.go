package totp

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/remmody/VaultixIMQ/internal/crypto"
	"github.com/pquerna/otp/totp"
)

type Entry struct {
	AccountName string `json:"account_name"`
	Issuer      string `json:"issuer"`
	Secret      string `json:"secret"`
	LinkedAccount string `json:"account"` // Added to link with mail account
}

type Response struct {
	Code     string `json:"code"`
	TimeLeft int    `json:"timeLeft"`
}

type Manager struct {
	entries   []Entry
	configDir string
	crypto    *crypto.Manager
	mu        sync.Mutex
}

func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
		entries:   []Entry{},
	}
}

func (m *Manager) SetCrypto(c *crypto.Manager) {
	m.crypto = c
}

func (m *Manager) SetEntries(entries []Entry) {
	m.mu.Lock()
	m.entries = entries
	m.mu.Unlock()
}

func (m *Manager) Add(entry Entry) {
	m.mu.Lock()
	m.entries = append(m.entries, entry)
	m.mu.Unlock()
}

func (m *Manager) Delete(accountName string) {
	m.mu.Lock()
	newEntries := []Entry{}
	for _, e := range m.entries {
		if e.AccountName != accountName {
			newEntries = append(newEntries, e)
		}
	}
	m.entries = newEntries
	m.mu.Unlock()
}

func (m *Manager) GetEntries() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries
}

func (m *Manager) Get(accountName string) (Response, error) {
	m.mu.Lock()
	var secret string
	for _, e := range m.entries {
		if e.AccountName == accountName {
			secret = e.Secret
			break
		}
	}
	m.mu.Unlock()

	if secret == "" {
		return Response{}, os.ErrNotExist
	}

	return m.Generate(secret)
}

func (m *Manager) Generate(secret string) (Response, error) {
	// Robust normalization
	normalizedSecret := normalizeBase32(secret)
	
	code, err := totp.GenerateCode(normalizedSecret, time.Now())
	if err != nil {
		log.Printf("[TOTP Error] Failed to generate code for secret [%s] (normalized: [%s]): %v", secret, normalizedSecret, err)
		return Response{}, err
	}

	timeLeft := 30 - (time.Now().Unix() % 30)
	return Response{
		Code:     code,
		TimeLeft: int(timeLeft),
	}, nil
}

func normalizeBase32(secret string) string {
	// 1. Strip whitespace, dashes, and common delimiters
	s := strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == '_' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, secret)

	// 2. Convert to uppercase
	s = strings.ToUpper(s)

	// 3. Fix common typos
	// Standard Base32: A-Z, 2-7
	replacer := strings.NewReplacer(
		"0", "O",
		"1", "I", 
		"8", "B",
	)
	s = replacer.Replace(s)

	// 4. Strip any remaining invalid characters
	validBase32 := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	var sb strings.Builder
	for _, r := range s {
		if strings.ContainsRune(validBase32, r) {
			sb.WriteRune(r)
		}
	}
	s = sb.String()

	// 5. Add padding if missing
	if len(s) > 0 && len(s)%8 != 0 {
		padding := 8 - (len(s) % 8)
		s += strings.Repeat("=", padding)
	}

	return s
}
