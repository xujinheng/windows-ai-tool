//go:build windows

package main

import (
	"testing"
	"unsafe"
)

func TestWinMMInputCapsLayout(t *testing.T) {
	if got := unsafe.Sizeof(waveInCapsW{}); got != 80 {
		t.Fatalf("WAVEINCAPSW layout mismatch: %d", got)
	}
}
