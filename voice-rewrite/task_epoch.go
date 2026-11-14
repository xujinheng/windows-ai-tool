package main

import (
	"context"
	"sync"
)

// taskEpoch is the authoritative owner of the currently active operation.
// Starting another generation cancels the previous one's I/O immediately.
// The target window, selection, and clipboard ownership still require their
// independent checks before any external editor is modified.
type taskEpoch struct {
	mu        sync.Mutex
	currentID uint64
	closed    bool
	ctx       context.Context
	cancel    context.CancelFunc
}

func (g *taskEpoch) start() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return 0
	}
	if g.cancel != nil {
		g.cancel()
	}
	g.currentID++
	g.ctx, g.cancel = context.WithCancel(context.Background())
	return g.currentID
}

func (g *taskEpoch) valid(id uint64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return id != 0 && !g.closed && g.currentID == id
}

// contextFor returns a canceled context for a stale generation. This makes
// even a delayed network request terminate without contacting the provider.
func (g *taskEpoch) contextFor(id uint64) context.Context {
	g.mu.Lock()
	defer g.mu.Unlock()
	if id != 0 && !g.closed && g.currentID == id && g.ctx != nil {
		return g.ctx
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func (g *taskEpoch) current() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.currentID
}

func (g *taskEpoch) shutdown() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
	g.currentID++
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	g.ctx = nil
}

// Clipboard ownership is checked independently from task cancellation.
// Plain-text restoration must never overwrite a newer user clipboard value.
func mayRestoreTextClipboard(hasOriginalText bool, lastOwnedSeq, currentSeq uintptr) bool {
	return hasOriginalText && lastOwnedSeq != 0 && currentSeq == lastOwnedSeq
}
