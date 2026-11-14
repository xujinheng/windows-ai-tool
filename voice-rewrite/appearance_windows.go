//go:build windows

package main

import (
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
	if windowsPrefersDark() {
		return uiPalette{true, 0x17191E, 0x24272D, 0x30343C, 0xF5F6FA, 0xB6BFCD, 0x414753, 0x9F7AEA, 0xFFFFFF, 0x323741, 0x434A56}
	}
	return uiPalette{false, 0xF6F7F9, 0xFFFFFF, 0xFFFFFF, 0x263142, 0x697586, 0xDDE1E8, 0xB84B78, 0xFFFFFF, 0xF0F2F5, 0xE2E6EB}
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
