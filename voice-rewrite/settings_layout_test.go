package main

import "testing"

func TestResponsiveSettingsRect(t *testing.T) {
	input := uxLayoutSpec{x: 130, y: 157, w: 385, h: 30, stretch: true}
	right := uxLayoutSpec{x: 527, y: 157, w: 96, h: 30, right: true}
	for _, width := range []int32{694, 774, 1034} {
		x, y, w, h := layoutSettingsRect(input, width)
		if x != 130 || y != 157 || h != 30 || w != 385+(width-694) {
			t.Fatalf("edit at %d: %d %d %d %d", width, x, y, w, h)
		}
		bx, _, bw, _ := layoutSettingsRect(right, width)
		if bx != 527+(width-694) || bw != 96 || x+w+12 != bx {
			t.Fatalf("misaligned at %d", width)
		}
	}
}
