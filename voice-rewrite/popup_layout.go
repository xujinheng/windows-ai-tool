package main

// Compact by default; expand only for longer explanations or rewrite previews.
// The Win32 result window retains native buttons and keyboard accessibility.
func popupSizeForText(runes int) (width, height int32) {
	width, height = 576, 420
	if runes > 100 {
		height = 470
	}
	if runes > 320 {
		height = 540
	}
	if runes > 850 {
		height = 620
	}
	return
}
