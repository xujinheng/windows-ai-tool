package main

import "testing"

func TestV14PopupCompactAdaptiveSizing(t *testing.T) {
	for _, tt := range []struct {
		n int
		h int32
	}{{0, 420}, {80, 420}, {101, 470}, {321, 540}, {900, 620}} {
		w, h := v14PopupSize(tt.n)
		if w != 576 || h != tt.h {
			t.Errorf("length %d -> %dx%d", tt.n, w, h)
		}
	}
}
