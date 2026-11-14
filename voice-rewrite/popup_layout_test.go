package main

import "testing"

func TestV14PopupCompactAdaptiveSizing(t *testing.T) {
	for _, tt := range []struct {
		n int
		h int32
	}{{0, 224}, {80, 224}, {101, 280}, {321, 360}, {900, 450}} {
		w, h := v14PopupSize(tt.n)
		if w != 480 || h != tt.h {
			t.Errorf("length %d -> %dx%d", tt.n, w, h)
		}
	}
}
