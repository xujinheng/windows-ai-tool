package main

import "sync/atomic"

// taskEpoch prevents a completed older task from committing after a newer
// capture starts or shutdown begins. It does not authorize a paste: the target
// window and selected text must still be verified independently.
type taskEpoch struct {
	currentID atomic.Uint64
	closed    atomic.Bool
}

func (g *taskEpoch) start() uint64 {
	if g.closed.Load() {
		return 0
	}
	id := g.currentID.Add(1)
	if g.closed.Load() {
		return 0
	}
	return id
}

func (g *taskEpoch) valid(id uint64) bool {
	return id != 0 && !g.closed.Load() && g.currentID.Load() == id
}

func (g *taskEpoch) shutdown() {
	g.closed.Store(true)
	g.currentID.Add(1)
}

// A clipboard snapshot can only be restored if the clipboard sequence remains
// unchanged since the application's last known write. Never overwrite a newer
// user clipboard update. This is a guard for plain text only, not rich data.
func mayRestoreTextClipboard(hasOriginalText bool, lastOwnedSeq, currentSeq uintptr) bool {
	return hasOriginalText && lastOwnedSeq != 0 && currentSeq == lastOwnedSeq
}
