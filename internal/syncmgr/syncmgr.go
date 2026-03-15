package syncmgr

import (
	"context"
	"sync"
	"time"

	"github.com/remmody/VaultixIMQ/internal/mail"
)

type SyncManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	nextSync        time.Time
	interval        int
	mu              sync.Mutex
	syncFunc        func(context.Context, mail.Account)
	getAccs         func() []mail.Account
	visibleAccounts map[string]bool
}

func NewSyncManager(syncFunc func(context.Context, mail.Account), getAccs func() []mail.Account) *SyncManager {
	return &SyncManager{
		syncFunc:        syncFunc,
		getAccs:         getAccs,
		interval:        10,
		visibleAccounts: make(map[string]bool),
	}
}

func (m *SyncManager) SetVisibleAccounts(emails []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.visibleAccounts = make(map[string]bool)
	for _, email := range emails {
		m.visibleAccounts[email] = true
	}
}

func (m *SyncManager) Start(ctx context.Context, interval int) {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.interval = interval
	if m.interval < 5 { m.interval = 10 }
	
	sCtx, cancel := context.WithCancel(ctx)
	m.ctx = sCtx
	m.cancel = cancel
	m.nextSync = time.Now()
	m.mu.Unlock()

	// Polling loop
	go func() {
		ticker := time.NewTicker(2 * time.Second) // Check every 2s
		defer ticker.Stop()
		
		backgroundTicker := time.NewTicker(5 * time.Minute) // Background sync every 5m
		defer backgroundTicker.Stop()

		for {
			select {
			case <-ticker.C:
				m.mu.Lock()
				shouldSync := time.Now().After(m.nextSync)
				interval := m.interval
				m.mu.Unlock()

				if shouldSync {
					m.SyncVisible()
					m.mu.Lock()
					m.nextSync = time.Now().Add(time.Duration(interval) * time.Second)
					m.mu.Unlock()
				}
			case <-backgroundTicker.C:
				m.SyncBackground()
			case <-sCtx.Done():
				return
			}
		}
	}()
}

func (m *SyncManager) SyncVisible() {
	m.mu.Lock()
	visible := m.visibleAccounts
	m.mu.Unlock()

	accounts := m.getAccs()
	for _, acc := range accounts {
		if visible[acc.Email] {
			go m.syncFunc(m.ctx, acc)
		}
	}
}

func (m *SyncManager) SyncBackground() {
	m.mu.Lock()
	visible := m.visibleAccounts
	m.mu.Unlock()

	accounts := m.getAccs()
	for _, acc := range accounts {
		if !visible[acc.Email] {
			// Throttled background sync: small delay between accounts to avoid CPU spike
			time.Sleep(100 * time.Millisecond)
			go m.syncFunc(m.ctx, acc)
		}
	}
}

func (m *SyncManager) SyncAll() {
	accounts := m.getAccs()
	for _, acc := range accounts {
		go m.syncFunc(m.ctx, acc)
	}
}

func (m *SyncManager) SetInterval(interval int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interval = interval
	m.nextSync = time.Now().Add(time.Duration(interval) * time.Second)
}

func (m *SyncManager) GetNextSync() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nextSync
}

func (m *SyncManager) TriggerImmediate(acc mail.Account) {
	if m.ctx != nil && m.syncFunc != nil {
		go m.syncFunc(m.ctx, acc)
	}
}
