//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// A STATIC class has its own window procedure and can discard Shell_NotifyIcon
// SendMessage notifications before our GetMessage loop observes them. A
// registered window class forwards tray callbacks into the normal message queue.
type trayWindowClass struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSmall  uintptr
}

var (
	procRegisterClassEx = user32.NewProc("RegisterClassExW")
	procDefWindowProc   = user32.NewProc("DefWindowProcW")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	// Keep Go's callback allocated for the lifetime of the process.
	trayWndProcCallback = syscall.NewCallback(func(hwnd, message, wParam, lParam uintptr) uintptr {
		if uint32(message) == WM_TRAY_NOTIFY {
			// Short callback: queue work; don't open modal menus inside WndProc.
			procPostMessage.Call(hwnd, WM_TRAY_EVENT, wParam, lParam)
			return 0
		}
		result, _, _ := procDefWindowProc.Call(hwnd, message, wParam, lParam)
		return result
	})
)

func createTrayCallbackWindow() (uintptr, error) {
	className, _ := syscall.UTF16PtrFromString("VoiceRewrite.TrayCallbackWindow.v4")
	title, _ := syscall.UTF16PtrFromString("VoiceRewriteTraySink")
	module, _, _ := procGetModuleHandle.Call(0)
	class := trayWindowClass{
		Size:      uint32(unsafe.Sizeof(trayWindowClass{})),
		WndProc:   trayWndProcCallback,
		Instance:  module,
		ClassName: className,
	}
	atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))
	runtime.KeepAlive(class)
	if atom == 0 {
		return 0, fmt.Errorf("RegisterClassExW: %w", err)
	}
	hwnd, _, createErr := procCreateWindow.Call(0,
		uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		0, 0, 0, 0, 0, 0, 0, module, 0)
	runtime.KeepAlive(className)
	runtime.KeepAlive(title)
	if hwnd == 0 {
		if createErr != nil {
			return 0, fmt.Errorf("CreateWindowExW: %w", createErr)
		}
		return 0, errors.New("CreateWindowExW failed")
	}
	return hwnd, nil
}
