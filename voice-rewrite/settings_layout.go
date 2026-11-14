package main

// Width-dependent controls stretch or remain right anchored. The layout is
// expressed exclusively in 96-DPI logical pixels to avoid repeated scaling.
type uxLayoutSpec struct {
	x, y, w, h     int32
	stretch, right bool
}

func layoutSettingsRect(c uxLayoutSpec, clientWidth int32) (x, y, w, h int32) {
	delta := clientWidth - 816
	if delta < 0 {
		delta = 0
	}
	x, y, w, h = c.x, c.y, c.w, c.h
	if c.right {
		x += delta
	}
	if c.stretch {
		w += delta
	}
	return
}
