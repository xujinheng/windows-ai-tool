package main

import "testing"

func TestPopupAdaptiveSizing(t *testing.T) {
	for _, tt := range []struct {
		n int
		h int32
	}{{0, 258}, {80, 258}, {200, 258}, {201, 315}, {600, 315}, {601, 430}, {1201, 500}} {
		w, h := popupSizeForText(tt.n)
		if w != 476 || h != tt.h {
			t.Errorf("length %d -> %dx%d, wanted 476x%d", tt.n, w, h, tt.h)
		}
	}
}

func TestPopupClickDismissesExceptResponseAndActions(t *testing.T) {
	window := popupHitRect{10, 10, 486, 268}
	editor := popupHitRect{30, 66, 466, 202}
	buttons := []popupHitRect{{258, 214, 375, 252}, {383, 214, 468, 252}}
	for _, tc := range []struct {
		name string
		x, y int32
		dismiss bool
	}{
		{"outside left", 9, 125, true},
		{"outside right", 486, 125, true},
		{"outside top", 190, 9, true},
		{"outside bottom", 190, 268, true},
		{"header", 45, 32, true},
		{"blank margin", 18, 100, true},
		{"empty footer", 80, 230, true},
		{"body selection", 50, 92, false},
		{"body scrollbar", 465, 115, false},
		{"apply", 300, 233, false},
		{"copy", 420, 233, false},
	} {
		if got := popupClickDismisses(tc.x, tc.y, window, editor, buttons); got != tc.dismiss {
			t.Errorf("%s: dismiss=%v, want %v", tc.name, got, tc.dismiss)
		}
	}
}
