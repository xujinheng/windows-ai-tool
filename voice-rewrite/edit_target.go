package main

import "strings"

const nativePasswordStyle = 0x0020

// A native password field is always a protected target for global dictation
// and selection editing. Browser/Electron password widgets need separate UIA
// support; don't infer their safety from an arbitrary HWND style bit.
func nativePasswordControl(className string, style uintptr) bool {
	className = strings.ToLower(className)
	nativeEdit := className == "edit" || strings.HasPrefix(className, "richedit") || strings.HasPrefix(className, "windowsforms10.edit")
	return nativeEdit && style&nativePasswordStyle != 0
}
