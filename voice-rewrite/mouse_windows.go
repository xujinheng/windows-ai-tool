//go:build windows

package main

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
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
	WM_SIDE_ARM      = 0x8000 + 72
	WM_SIDE_CANCEL   = 0x8000 + 73
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

// Short presses are replayed as XBUTTON clicks; a held side button starts
// VoiceRewrite after this threshold.
// Used by the settings recorder across UI/mouse-hook threads.
var sideShortcutCapture atomic.Uint32

const sideHoldThreshold = 250 * time.Millisecond
const sideReplayExtraInfo = uintptr(0x56525842)

const (
	mouseInputType    = 0
	mouseXDownFlag    = 0x0080
	mouseXUpFlag      = 0x0100
	mouseInjectedFlag = 0x0001
)

// The MOUSEINPUT union occupies 32 bytes in a 64-bit Win32 INPUT (40 bytes).
type sideMouseInput struct {
	Type      uint32
	_         uint32
	DX        int32
	DY        int32
	Data      uint32
	Flags     uint32
	Time      uint32
	_         uint32
	ExtraInfo uintptr
}

func replaySideClick(button uint16) error {
	if button != 1 && button != 2 {
		return fmt.Errorf("invalid X button: %d", button)
	}
	inputs := [2]sideMouseInput{
		{Type: mouseInputType, Data: uint32(button), Flags: mouseXDownFlag, ExtraInfo: sideReplayExtraInfo},
		{Type: mouseInputType, Data: uint32(button), Flags: mouseXUpFlag, ExtraInfo: sideReplayExtraInfo},
	}
	if unsafe.Sizeof(inputs[0]) != 40 {
		return fmt.Errorf("unexpected Win32 INPUT size: %d", unsafe.Sizeof(inputs[0]))
	}
	n, _, err := procSendInput.Call(2, uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	runtime.KeepAlive(inputs)
	if n != 2 {
		return fmt.Errorf("SendInput XBUTTON %d: %v (%d/2)", button, err, n)
	}
	return nil
}

func installSideMouseHook(hwnd uintptr, rewriteButton, dictationButton uint16, paused *atomic.Bool) (*sideMouseHook, error) {
	if hwnd == 0 || (rewriteButton != 0 && rewriteButton == dictationButton) {
		return nil, errors.New("鼠标侧键钩子参数无效")
	}
	ready := make(chan mouseHookReady, 1)
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		tid, _, _ := procGetCurrentThreadID.Call()
		var mu sync.Mutex
		var buttons [3]sideButtonState
		var capturedDown [3]bool
		var timers [3]*time.Timer
		closed := false
		replay := make(chan uint16, 32)
		var replayDone sync.WaitGroup
		replayDone.Add(1)
		go func() {
			defer replayDone.Done()
			for button := range replay {
				if err := replaySideClick(button); err != nil {
					log.Printf("side-button navigation click: %v", err)
				}
			}
		}()
		hook := uintptr(0)
		callback := syscall.NewCallback(func(code, message, data uintptr) uintptr {
			if int32(code) < 0 || data == 0 {
				return callNextMouseHook(hook, code, message, data)
			}
			event := uint32(message)
			if event != WM_XBUTTONDOWN && event != WM_XBUTTONUP && event != WM_XBUTTONDBLCLK {
				return callNextMouseHook(hook, code, message, data)
			}
			info := (*lowLevelMouseData)(unsafe.Pointer(data))
			if info.Flags&mouseInjectedFlag != 0 && info.ExtraInfo == sideReplayExtraInfo {
				return callNextMouseHook(hook, code, message, data)
			}
			button := uint16(info.MouseData >> 16)
			if button != 1 && button != 2 {
				return callNextMouseHook(hook, code, message, data)
			}
			mu.Lock()
			// Capture the next physical mouse side-button in Settings before
			// the user's normal hold/navigation binding can consume it.
			if capturedDown[button] {
				if event == WM_XBUTTONUP {
					capturedDown[button] = false
				}
				mu.Unlock()
				return 1
			}
			if target := sideShortcutCapture.Load(); target != 0 && (event == WM_XBUTTONDOWN || event == WM_XBUTTONDBLCLK) {
				capturedDown[button] = true
				procPostMessage.Call(hwnd, WM_SETTINGS_CAPTURE_MOUSE, uintptr(button), uintptr(target))
				mu.Unlock()
				return 1
			}
			if button != rewriteButton && button != dictationButton {
				mu.Unlock()
				return callNextMouseHook(hook, code, message, data)
			}
			if event == WM_XBUTTONDOWN || event == WM_XBUTTONDBLCLK {
				if buttons[button].down {
					mu.Unlock()
					return 1
				}
				if closed || (paused != nil && paused.Load()) {
					mu.Unlock()
					return callNextMouseHook(hook, code, message, data)
				}
				token, _ := buttons[button].press()
				// Start a local audio buffer at physical button-down. The hold
				// timer only decides whether that buffer is used.
				if ok, _, _ := procPostMessage.Call(hwnd, WM_SIDE_ARM, uintptr(button), 0); ok == 0 {
					buttons[button].release()
					mu.Unlock()
					return callNextMouseHook(hook, code, message, data)
				}
				timers[button] = time.AfterFunc(sideHoldThreshold, func() {
					mu.Lock()
					defer mu.Unlock()
					if closed || (paused != nil && paused.Load()) || !buttons[button].canActivate(token) {
						return
					}
					if ok, _, _ := procPostMessage.Call(hwnd, WM_SIDE_DOWN, uintptr(button), 0); ok != 0 {
						buttons[button].activate(token)
					}
				})
				mu.Unlock()
				return 1
			}
			// A matching press was intercepted. Either finish the recording
			// or replay the navigation click, never both.
			wasHeld, active := buttons[button].release()
			if !wasHeld {
				mu.Unlock()
				return callNextMouseHook(hook, code, message, data)
			}
			if timers[button] != nil {
				timers[button].Stop()
				timers[button] = nil
			}
			if active {
				procPostMessage.Call(hwnd, WM_SIDE_UP, uintptr(button), 0)
			} else {
				// A short click must stop its provisional recording before
				// the app can process it as a voice command.
				procPostMessage.Call(hwnd, WM_SIDE_CANCEL, uintptr(button), 0)
				select {
				case replay <- button:
				default:
					log.Printf("side-button navigation queue full for XBUTTON%d", button)
				}
			}
			mu.Unlock()
			return 1
		})
		var lastErr error
		hook, _, lastErr = procSetWindowsHookEx.Call(WH_MOUSE_LL, callback, 0, 0)
		if hook == 0 {
			close(replay)
			replayDone.Wait()
			ready <- mouseHookReady{err: fmt.Errorf("注册鼠标侧键失败: %w", lastErr)}
			return
		}
		ready <- mouseHookReady{threadID: tid}
		for {
			var msg winMSG
			rc, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(rc) <= 0 {
				break
			}
		}
		procUnhookWindowsHookEx.Call(hook)
		mu.Lock()
		closed = true
		for i := range timers {
			if timers[i] != nil {
				timers[i].Stop()
			}
		}
		mu.Unlock()
		close(replay)
		replayDone.Wait()
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
