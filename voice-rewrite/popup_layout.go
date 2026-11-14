package main

// Keep results close to the mouse: short answers should not need a giant
// reading sheet, while long AI responses still have a scrollable native EDIT.
func popupSizeForText(runes int) (width, height int32) {
	width, height = 510, 300
	switch {
	case runes > 1200:
		height = 530
	case runes > 600:
		height = 475
	case runes > 260:
		height = 405
	case runes > 100:
		height = 345
	}
	return
}

// Use screen coordinates for the low-level pointer hook so a click can
// dismiss a non-activating result even when the user clicks another process.
// Clicking inside the native read-only editor must always be kept alive:
// users need an initial mouse-down to start selecting/copying the AI text.
type popupHitRect struct{ Left, Top, Right, Bottom int32 }

func (r popupHitRect) contains(x, y int32) bool {
	return r.Right > r.Left && r.Bottom > r.Top &&
		x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

// The only protected regions are the response text and actual action buttons.
// Title, decoration, empty panel, and all points outside the window dismiss it.
func popupClickDismisses(x, y int32, popup, editor popupHitRect, buttons []popupHitRect) bool {
	if !popup.contains(x, y) {
		return true
	}
	if editor.contains(x, y) {
		return false
	}
	for _, b := range buttons {
		if b.contains(x, y) {
			return false
		}
	}
	return true
}
