package totp

import (
	"os"
	"sync"
	"time"

	"github.com/remmody/VaultixIMQ/internal/crypto"
	"github.com/pquerna/otp/totp"
)

type Entry struct {
	AccountName string `json:"account_name"`
	Issuer      string `json:"issuer"`
	Secret      string `json:"secret"`
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

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return Response{}, err
	}

	timeLeft := 30 - (time.Now().Unix() % 30)
	return Response{
		Code:     code,
		TimeLeft: int(timeLeft),
	}, nil
}
func (m *Manager) Generate(secret string) (Response, error) {
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return Response{}, err
	}

	timeLeft := 30 - (time.Now().Unix() % 30)
	return Response{
		Code:     code,
		TimeLeft: int(timeLeft),
	}, nil
}
