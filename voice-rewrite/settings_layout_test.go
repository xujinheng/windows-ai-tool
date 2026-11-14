package main

import "testing"

func TestResponsiveSettingsRect(t *testing.T) {
	input := uxLayoutSpec{x: 350, y: 173, w: 325, h: 34, stretch: true}
	right := uxLayoutSpec{x: 687, y: 173, w: 92, h: 34, right: true}
	for _, width := range []int32{816, 896, 1056} {
		x, y, w, h := layoutSettingsRect(input, width)
		if x != 350 || y != 173 || h != 34 || w != 325+(width-816) {
			t.Fatalf("edit at %d: %d %d %d %d", width, x, y, w, h)
		}
		bx, _, bw, _ := layoutSettingsRect(right, width)
		if bx != 687+(width-816) || bw != 92 || x+w+12 != bx {
			t.Fatalf("misaligned at %d", width)
		}
	}
}
