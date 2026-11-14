//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// UI colors live in one place so native windows do not drift into unrelated
// hard-coded themes. COLORREF uses BGR; Windows DWM attributes use RGB.
type uiPalette struct {
	dark                                                                                               bool
	background, surface, field, text, muted, border, primary, primaryText, secondary, secondaryPressed uint32
}

func systemPalette() uiPalette {
	if os.Getenv("VOICEREWRITE_VISUAL_PREVIEW") != "" {
		switch os.Getenv("VOICEREWRITE_VISUAL_THEME") {
		case "dark":
			return darkUIPalette()
		case "light":
			return lightUIPalette()
		}
	}
	if windowsPrefersDark() {
		return darkUIPalette()
	}
	return lightUIPalette()
}
func darkUIPalette() uiPalette {
	return uiPalette{true, 0x171B23, 0x232934, 0x2C3340, 0xF4F6F9, 0xA8B4C3, 0x3B4654, 0xE477A0, 0x21141B, 0x2C3441, 0x3B4654}
}
func lightUIPalette() uiPalette {
	return uiPalette{false, 0xF6F7FA, 0xFFFFFF, 0xF8F9FB, 0x1F2937, 0x6B7889, 0xE1E6ED, 0xB84B78, 0xFFFFFF, 0xF1F3F7, 0xE4E8EF}
}

func (p uiPalette) color(rgb uint32) uintptr {
	return uintptr(((rgb & 0x0000ff) << 16) | (rgb & 0x00ff00) | ((rgb >> 16) & 0x0000ff))
}

// Windows Personalization: AppsUseLightTheme == 0 means dark. Failure is a
// system-default light theme, not an app startup error.
func windowsPrefersDark() bool {
	var flag uint32 = 1
	size := uint32(unsafe.Sizeof(flag))
	sub, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	name, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
	regGetValue := syscall.NewLazyDLL("advapi32.dll").NewProc("RegGetValueW")
	code, _, _ := regGetValue.Call(0x80000001, uintptr(unsafe.Pointer(sub)), uintptr(unsafe.Pointer(name)), 0x18, 0, uintptr(unsafe.Pointer(&flag)), uintptr(unsafe.Pointer(&size)))
	return code == 0 && flag == 0
}

func applyWindowPalette(hwnd uintptr, p uiPalette) {
	var enabled uint32
	if p.dark {
		enabled = 1
	}
	const dwmUseImmersiveDarkMode = 20
	procDwmSetWindowAttribute.Call(hwnd, dwmUseImmersiveDarkMode, uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))
	// The background keeps the same contrast on Windows 10, where the DWM
	// attribute may not exist or may return E_INVALIDARG.
}

// refreshPaletteBrush allocates the new brush first, so a transient GDI
// allocation failure never leaves a live window without its old brush.
func refreshPaletteBrush(old uintptr, palette uiPalette, rgb uint32) uintptr {
	fresh, _, _ := procCreateSolidBrush.Call(palette.color(rgb))
	if fresh == 0 {
		return old
	}
	if old != 0 {
		procDeleteObject.Call(old)
	}
	return fresh
}

func refreshNativeWindow(hwnd uintptr) {
	procInvalidateRectSettings.Call(hwnd, 0, 1)
}
