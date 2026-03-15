package app

import (
	"context"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Batcher struct {
	ctx      context.Context
	mu       sync.Mutex
	updates  map[string]interface{}
	interval time.Duration
}

func NewBatcher(ctx context.Context, interval time.Duration) *Batcher {
	if ctx == nil {
		ctx = context.Background()
	}
	b := &Batcher{
		ctx:      ctx,
		updates:  make(map[string]interface{}),
		interval: interval,
	}
	go b.run()
	return b
}

func (b *Batcher) SetContext(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ctx != nil {
		b.ctx = ctx
	}
}

func (b *Batcher) Update(key string, value interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.updates[key] = value
}

func (b *Batcher) run() {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.getContext().Done():
			return
		}
	}
}

func (b *Batcher) getContext() context.Context {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

func (b *Batcher) flush() {
	b.mu.Lock()
	if len(b.updates) == 0 {
		b.mu.Unlock()
		return
	}
	
	// Copy and clear
	toEmit := b.updates
	b.updates = make(map[string]interface{})
	ctx := b.ctx
	b.mu.Unlock()

	if ctx != nil && ctx != context.Background() {
		wailsRuntime.EventsEmit(ctx, "batch_update", toEmit)
	}
}

func (b *Batcher) EmitEvent(name string, data interface{}) {
	b.mu.Lock()
	ctx := b.ctx
	b.mu.Unlock()
	if ctx != nil && ctx != context.Background() {
		wailsRuntime.EventsEmit(ctx, name, data)
	}
}
