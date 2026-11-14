package main

// Geometry shared by the native Win32 EDIT controls and their Direct2D/GDI
// backplates. Keeping one source of truth prevents either side of a focus
// border from being covered by a child HWND at 100%-200% DPI.
const (
	settingsFieldFrameX      int32 = 345
	settingsFieldFrameWidth  int32 = 447
	settingsFieldFrameHeight int32 = 34
	settingsFieldFirstY      int32 = 169
	settingsFieldRowStep     int32 = 54
	settingsFieldInsetX      int32 = 14
	settingsFieldInsetY      int32 = 5
	settingsFieldEditHeight  int32 = 24
)

type settingsFieldRect struct{ X, Y, W, H int32 }

func settingsFieldFrame(row int) settingsFieldRect {
	return settingsFieldRect{settingsFieldFrameX, settingsFieldFirstY + int32(row)*settingsFieldRowStep, settingsFieldFrameWidth, settingsFieldFrameHeight}
}

func settingsFieldEdit(row int) settingsFieldRect {
	frame := settingsFieldFrame(row)
	return settingsFieldRect{frame.X + settingsFieldInsetX, frame.Y + settingsFieldInsetY,
		frame.W - 2*settingsFieldInsetX, settingsFieldEditHeight}
}

func settingsFieldLabel(row int) settingsFieldRect {
	frame := settingsFieldFrame(row)
	return settingsFieldRect{224, frame.Y + 5, 119, 24}
}

// Shortcut uses the same text baseline/inset with a shorter field.
func settingsShortcutFrame() settingsFieldRect {
	frame := settingsFieldFrame(0)
	frame.W = 226
	return frame
}

func settingsShortcutEdit() settingsFieldRect {
	frame := settingsShortcutFrame()
	return settingsFieldRect{frame.X + settingsFieldInsetX, frame.Y + settingsFieldInsetY,
		frame.W - 2*settingsFieldInsetX, settingsFieldEditHeight}
}
