package main

// Compact by default; expand only for longer explanations or rewrite previews.
// The Win32 result window retains native buttons and keyboard accessibility.
func v14PopupSize(runes int) (width, height int32) {
	width, height = 480, 224
	if runes > 100 {
		height = 280
	}
	if runes > 320 {
		height = 360
	}
	if runes > 850 {
		height = 450
	}
	return
}
