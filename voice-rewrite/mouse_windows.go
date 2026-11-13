//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// WH_MOUSE_LL is installed on the UI thread and uses its GetMessage loop.
// It intercepts only the selected XBUTTON; all other mouse actions continue normally.
const (
	WH_MOUSE_LL      = 14
	WM_XBUTTONDOWN   = 0x020B
	WM_XBUTTONUP     = 0x020C
	WM_XBUTTONDBLCLK = 0x020D
	WM_SIDE_DOWN     = 0x8000 + 52
	WM_SIDE_UP       = 0x8000 + 53
)

var (
	procSetWindowsHookEx   = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx     = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
)

type lowLevelMouseData struct {
	X, Y       int32
	MouseData  uint32
	Flags      uint32
	Time       uint32
	ExtraInfo  uintptr
}

type sideMouseHook struct {
	hook       uintptr
	callback   uintptr
	window     uintptr
	button     uint16
	pressed    bool
}

func installSideMouseHook(hwnd uintptr, button uint16) (*sideMouseHook, error) {
	if hwnd == 0 || (button != 1 && button != 2) {
		return nil, errors.New("鼠标钩子参数无效")
	}
	h := &sideMouseHook{window: hwnd, button: button}
	h.callback = syscall.NewCallback(func(code, message, data uintptr) uintptr {
		if int32(code) >= 0 && data != 0 {
			event := uint32(message)
			if event == WM_XBUTTONDOWN || event == WM_XBUTTONUP || event == WM_XBUTTONDBLCLK {
				info := (*lowLevelMouseData)(unsafe.Pointer(data))
				which := uint16(info.MouseData >> 16)
				if which == h.button {
					// Only post to the window; never run recording or clipboard
					// operations inside a time-critical system hook callback.
					msg := uint32(0)
					if event == WM_XBUTTONDOWN && !h.pressed {
						msg = WM_SIDE_DOWN
					} else if event == WM_XBUTTONUP && h.pressed {
						msg = WM_SIDE_UP
					}
					if msg != 0 {
						if ok, _, _ := procPostMessage.Call(h.window, uintptr(msg), 0, 0); ok == 0 {
							// If we cannot deliver this event, don't swallow it.
							return callNextMouseHook(h.hook, code, message, data)
						}
					}
					if event == WM_XBUTTONDOWN { h.pressed = true }
					if event == WM_XBUTTONUP { h.pressed = false }
					// Suppress browser forward/back for the configured button.
					return 1
				}
			}
		}
		return callNextMouseHook(h.hook, code, message, data)
	})
	hook, _, e := procSetWindowsHookEx.Call(WH_MOUSE_LL, h.callback, 0, 0)
	if hook == 0 {
		return nil, fmt.Errorf("无法注册鼠标侧键监听: %w", e)
	}
	h.hook = hook
	runtime.KeepAlive(h)
	return h, nil
}

func callNextMouseHook(hook, code, message, data uintptr) uintptr {
	value, _, _ := procCallNextHookEx.Call(hook, code, message, data)
	return value
}

func (h *sideMouseHook) close() {
	if h != nil && h.hook != 0 {
		procUnhookWindowsHookEx.Call(h.hook)
		h.hook = 0
		runtime.KeepAlive(h)
	}
}
