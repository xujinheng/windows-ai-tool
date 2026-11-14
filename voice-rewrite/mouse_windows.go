//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// Mouse4/Mouse5 are XBUTTON1/XBUTTON2. A low-level hook MUST run on a dedicated
// message-pump thread: a slow clipboard read or network request on the UI thread
// must not block the OS mouse input path (Windows can silently remove slow hooks).
const (
	WH_MOUSE_LL      = 14
	WM_XBUTTONDOWN   = 0x020B
	WM_XBUTTONUP     = 0x020C
	WM_XBUTTONDBLCLK = 0x020D
	WM_SIDE_DOWN     = 0x8000 + 52
	WM_SIDE_UP       = 0x8000 + 53
	WM_QUIT          = 0x0012
)

var (
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetCurrentThreadID  = kernel32.NewProc("GetCurrentThreadId")
	procPostThreadMessage   = user32.NewProc("PostThreadMessageW")
)

type lowLevelMouseData struct {
	X, Y      int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type sideMouseHook struct {
	threadID uintptr
	done     chan struct{}
}

type mouseHookReady struct {
	threadID uintptr
	err      error
}

func installSideMouseHook(hwnd uintptr, rewriteButton, dictationButton uint16, paused *atomic.Bool) (*sideMouseHook, error) {
	if hwnd == 0 || (rewriteButton == 0 && dictationButton == 0) || (rewriteButton != 0 && rewriteButton == dictationButton) {
		return nil, errors.New("鼠标侧键钩子参数无效")
	}
	ready := make(chan mouseHookReady, 1)
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		tid, _, _ := procGetCurrentThreadID.Call()
		var pressed [3]bool
		hook := uintptr(0)
		callback := syscall.NewCallback(func(code, message, data uintptr) uintptr {
			// A paused background assistant must not swallow normal Back/Forward
			// mouse navigation events in the user's browser or other apps.
			if paused != nil && paused.Load() {
				return callNextMouseHook(hook, code, message, data)
			}
			if int32(code) >= 0 && data != 0 {
				event := uint32(message)
				if event == WM_XBUTTONDOWN || event == WM_XBUTTONUP || event == WM_XBUTTONDBLCLK {
					info := (*lowLevelMouseData)(unsafe.Pointer(data))
					button := uint16(info.MouseData >> 16)
					if sideButtonFlow(button, rewriteButton, dictationButton) != "ignore" {
						notification := uintptr(0)
						switch event {
						case WM_XBUTTONDOWN:
							if !pressed[button] {
								notification = WM_SIDE_DOWN
							}
						case WM_XBUTTONUP:
							if pressed[button] {
								notification = WM_SIDE_UP
							}
						}
						if notification != 0 {
							if ok, _, _ := procPostMessage.Call(hwnd, notification, uintptr(button), 0); ok == 0 {
								// Never block this callback or swallow an undeliverable click.
								return callNextMouseHook(hook, code, message, data)
							}
						}
						if event == WM_XBUTTONDOWN {
							pressed[button] = true
						}
						if event == WM_XBUTTONUP {
							pressed[button] = false
						}
						// Suppress the browser's back/forward action only for this button.
						return 1
					}
				}
			}
			return callNextMouseHook(hook, code, message, data)
		})
		var lastErr error
		hook, _, lastErr = procSetWindowsHookEx.Call(WH_MOUSE_LL, callback, 0, 0)
		if hook == 0 {
			ready <- mouseHookReady{err: fmt.Errorf("注册鼠标侧键失败: %w", lastErr)}
			return
		}
		ready <- mouseHookReady{threadID: tid}
		// This is ONLY the hook's message loop; no user I/O, UI work or COM.
		for {
			var msg winMSG
			rc, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(rc) <= 0 {
				break
			}
		}
		procUnhookWindowsHookEx.Call(hook)
		runtime.KeepAlive(callback)
	}()
	result := <-ready
	if result.err != nil {
		return nil, result.err
	}
	return &sideMouseHook{threadID: result.threadID, done: done}, nil
}

func callNextMouseHook(hook, code, message, data uintptr) uintptr {
	value, _, _ := procCallNextHookEx.Call(hook, code, message, data)
	return value
}

func (h *sideMouseHook) close() {
	if h == nil || h.threadID == 0 {
		return
	}
	procPostThreadMessage.Call(h.threadID, WM_QUIT, 0, 0)
	select {
	case <-h.done:
	case <-time.After(1500 * time.Millisecond): // don't hang shutdown
	}
	h.threadID = 0
}
