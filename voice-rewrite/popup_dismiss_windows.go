//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// WH_MOUSE_LL observes mouse-downs outside VoiceRewrite's HWND without taking
// focus or consuming the click. The user can continue working in the editor
// they clicked; the result card only receives an asynchronous WM_CLOSE.
const (
	popupWHMouseLL   = 14
	popupHcAction    = 0
	popupLButtonDown = 0x0201
	popupRButtonDown = 0x0204
	popupMButtonDown = 0x0207
)

type popupMousePoint struct{ X, Y int32 }
type popupMouseEvent struct {
	Point     popupMousePoint
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

var (
	popupSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	popupUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	popupCallNextHookEx      = user32.NewProc("CallNextHookEx")
	popupGetWindowRect       = user32.NewProc("GetWindowRect")
	popupGetDlgItem          = user32.NewProc("GetDlgItem")
	popupMouseHook           uintptr
	popupMouseCallback       = syscall.NewCallback(func(code, event, data uintptr) uintptr {
		if code == popupHcAction && data != 0 &&
			(event == popupLButtonDown || event == popupRButtonDown || event == popupMButtonDown) {
			if p := activeAnswer; p != nil && p.window != 0 {
				point := (*popupMouseEvent)(unsafe.Pointer(data)).Point
				if popupDismissAt(p, point.X, point.Y) {
					// Never close inside the low-level hook. Do not swallow the
					// click either: it belongs to the application underneath.
					procPostMessage.Call(p.window, WM_CLOSE, 0, 0)
				}
			}
		}
		next, _, _ := popupCallNextHookEx.Call(popupMouseHook, code, event, data)
		return next
	})
)

func popupScreenRect(hwnd uintptr) popupHitRect {
	if hwnd == 0 {
		return popupHitRect{}
	}
	var rect rect32
	ok, _, _ := popupGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 {
		return popupHitRect{}
	}
	return popupHitRect{rect.Left, rect.Top, rect.Right, rect.Bottom}
}

func popupDismissAt(p *answerPopup, x, y int32) bool {
	window := popupScreenRect(p.window)
	if !window.contains(x, y) {
		return true
	}
	editor := popupScreenRect(p.edit)
	buttons := make([]popupHitRect, 0, 3)
	for _, id := range []uintptr{BUTTON_APPLY, BUTTON_COPY, BUTTON_CLOSE} {
		hwnd, _, _ := popupGetDlgItem.Call(p.window, id)
		if hwnd != 0 {
			buttons = append(buttons, popupScreenRect(hwnd))
		}
	}
	return popupClickDismisses(x, y, window, editor, buttons)
}

func installPopupClickAwayHook() error {
	removePopupClickAwayHook()
	module, _, _ := procGetModuleHandle.Call(0)
	hook, _, err := popupSetWindowsHookEx.Call(
		popupWHMouseLL, popupMouseCallback, module, 0,
	)
	if hook == 0 {
		return err
	}
	popupMouseHook = hook
	return nil
}

func removePopupClickAwayHook() {
	if popupMouseHook != 0 {
		popupUnhookWindowsHookEx.Call(popupMouseHook)
		popupMouseHook = 0
	}
}
