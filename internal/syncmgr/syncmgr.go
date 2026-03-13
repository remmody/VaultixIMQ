package syncmgr

import (
	"context"
	"sync"
	"time"

	"github.com/remmody/VaultixIMQ/internal/mail"
)

type SyncManager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	nextSync   time.Time
	interval   int
	mu         sync.Mutex
	syncFunc   func(context.Context, mail.Account)
	getAccs    func() []mail.Account
}

func NewSyncManager(syncFunc func(context.Context, mail.Account), getAccs func() []mail.Account) *SyncManager {
	return &SyncManager{
		syncFunc: syncFunc,
		getAccs:  getAccs,
		interval: 10,
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
	
	// Trigger immediate sync on start by setting nextSync to now
	m.nextSync = time.Now()
	m.mu.Unlock()

	// Polling loop
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.mu.Lock()
				isTime := time.Now().After(m.nextSync)
				currInterval := time.Duration(m.interval) * time.Second
				m.mu.Unlock()

				if isTime {
					m.SyncAll()
					m.mu.Lock()
					m.nextSync = time.Now().Add(currInterval)
					m.mu.Unlock()
				}
			case <-sCtx.Done():
				return
			}
		}
	}()
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
