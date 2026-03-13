package account

import (
	"sync"

	"github.com/remmody/VaultixIMQ/internal/crypto"
	"github.com/remmody/VaultixIMQ/internal/mail"
)

type Manager struct {
	accounts  []mail.Account
	configDir string
	crypto    *crypto.Manager
	mu        sync.Mutex
}

func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
		accounts:  []mail.Account{},
	}
}

func (m *Manager) SetCrypto(c *crypto.Manager) {
	m.crypto = c
}

func (m *Manager) SetAccounts(accs []mail.Account) {
	m.mu.Lock()
	m.accounts = accs
	m.mu.Unlock()
}

func (m *Manager) Add(acc mail.Account) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.accounts {
		if existing.Email == acc.Email {
			return false
		}
	}

	acc.Finalize()
	m.accounts = append(m.accounts, acc)
	return true
}

func (m *Manager) Delete(email string) {
	m.mu.Lock()
	newAccs := []mail.Account{}
	for _, acc := range m.accounts {
		if acc.Email != email {
			newAccs = append(newAccs, acc)
		}
	}
	m.accounts = newAccs
	m.mu.Unlock()
}

func (m *Manager) GetAccounts() []mail.Account {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]mail.Account, len(m.accounts))
	copy(res, m.accounts)
	return res
}

func (m *Manager) Find(email string) (mail.Account, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acc := range m.accounts {
		if acc.Email == email {
			return acc, true
		}
	}
	return mail.Account{}, false
}
