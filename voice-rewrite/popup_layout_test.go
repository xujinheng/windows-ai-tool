package main

import "testing"

func TestV14PopupCompactAdaptiveSizing(t *testing.T) {
	for _, tt := range []struct {
		n int
		h int32
	}{{0, 360}, {80, 360}, {101, 400}, {321, 460}, {900, 540}} {
		w, h := v14PopupSize(tt.n)
		if w != 540 || h != tt.h {
			t.Errorf("length %d -> %dx%d", tt.n, w, h)
		}
	}
}
