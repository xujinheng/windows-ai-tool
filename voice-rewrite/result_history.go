package main

import (
	"strings"
	"sync"
	"time"
)

// Result history is deliberately ephemeral. It never writes speech, prompts,
// answers, or other potentially sensitive content to disk. Menus display only
// the result category and timestamp (not a transcript preview).
const (
	resultHistoryLimit    = 10
	resultHistoryMaxBytes = 4 << 20
)

type resultEntry struct {
	ID        uint64
	Title     string
	Text      string
	CreatedAt time.Time
}

type resultHistory struct {
	mu      sync.Mutex
	nextID  uint64
	entries []resultEntry // newest first
}

func (h *resultHistory) remember(title, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	// Keep the result available in the original card even if it is too large
	// for an in-memory recovery history. Never retain a partial transcript.
	if len(value) > resultHistoryMaxBytes {
		return
	}
	if strings.TrimSpace(title) == "" {
		title = "语音结果"
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	h.entries = append([]resultEntry{{ID: h.nextID, Title: title, Text: value, CreatedAt: time.Now()}}, h.entries...)
	bytes := 0
	keep := 0
	for _, entry := range h.entries {
		if keep >= resultHistoryLimit || bytes+len(entry.Text) > resultHistoryMaxBytes {
			break
		}
		bytes += len(entry.Text)
		keep++
	}
	// Clear discarded backing references before shrinking to avoid holding on
	// to large copies of sensitive text in the slice's unused capacity.
	for i := keep; i < len(h.entries); i++ {
		h.entries[i] = resultEntry{}
	}
	h.entries = h.entries[:keep]
}

func (h *resultHistory) snapshot() []resultEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]resultEntry(nil), h.entries...)
}

func (h *resultHistory) get(id uint64) (resultEntry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.entries {
		if e.ID == id {
			return e, true
		}
	}
	return resultEntry{}, false
}

func (h *resultHistory) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.entries {
		h.entries[i] = resultEntry{}
	}
	h.entries = nil
}
