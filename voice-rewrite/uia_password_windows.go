//go:build windows

package main

import (
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Query UI Automation off the mouse hook and GUI threads. An inaccessible
// provider can hang: use one COM worker, bounded caller time and fail-closed
// browser behavior when the provider cannot be inspected.
var (
	ole32              = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx     = ole32.NewProc("CoInitializeEx")
	coCreateInstance   = ole32.NewProc("CoCreateInstance")
	coUninitialize     = ole32.NewProc("CoUninitialize")
	passwordWorkerOnce sync.Once
	passwordChecks     = make(chan chan passwordProbe)
)
type passwordProbe struct {
	isPassword bool
	available  bool
}
var (
	clsidCUIAutomation = syscall.GUID{Data1: 0xff48dba4, Data2: 0x60ef, Data3: 0x4201, Data4: [8]byte{0xaa, 0x87, 0x54, 0x10, 0x3e, 0xef, 0x59, 0x4e}}
	iidIUIAutomation   = syscall.GUID{Data1: 0x30cbe57d, Data2: 0xd9d0, Data3: 0x452a, Data4: [8]byte{0xab, 0x13, 0x7a, 0xc5, 0xac, 0x48, 0x25, 0xee}}
)
func comMethod(obj uintptr, index uintptr, args ...uintptr) uintptr {
	vt := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vt + index*unsafe.Sizeof(uintptr(0))))
	parameters := append([]uintptr{obj}, args...)
	hr, _, _ := syscall.SyscallN(fn, parameters...)
	return hr
}
func comFailed(hr uintptr) bool { return int32(hr) < 0 }
func passwordWorker() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := coInitializeEx.Call(0, 0) // COINIT_MULTITHREADED
	if comFailed(hr) {
		for ch := range passwordChecks {
			ch <- passwordProbe{}
		}
		return
	}
	defer coUninitialize.Call()
	var automation uintptr
	hr, _, _ = coCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidCUIAutomation)), 0, 1,
		uintptr(unsafe.Pointer(&iidIUIAutomation)), uintptr(unsafe.Pointer(&automation)),
	)
	if comFailed(hr) || automation == 0 {
		for ch := range passwordChecks {
			ch <- passwordProbe{}
		}
		return
	}
	defer comMethod(automation, 2)
	for ch := range passwordChecks {
		var element uintptr
		hr = comMethod(automation, 8, uintptr(unsafe.Pointer(&element)))
		if comFailed(hr) || element == 0 {
			ch <- passwordProbe{}
			continue
		}
		var isPassword int32
		hr = comMethod(element, 35, uintptr(unsafe.Pointer(&isPassword)))
		comMethod(element, 2)
		ch <- passwordProbe{isPassword: isPassword != 0, available: !comFailed(hr)}
	}
}
func inspectFocusedPassword(timeout time.Duration) passwordProbe {
	passwordWorkerOnce.Do(func() { go passwordWorker() })
	reply := make(chan passwordProbe, 1)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case passwordChecks <- reply:
	case <-timer.C:
		return passwordProbe{}
	}
	select {
	case result := <-reply:
		return result
	case <-timer.C:
		return passwordProbe{}
	}
}
func browserLikeWindow(class string) bool {
	class = strings.ToLower(class)
	return strings.Contains(class, "chrome_widgetwin") ||
		strings.Contains(class, "mozillawindowclass") ||
		strings.Contains(class, "cefclientwindow") ||
		strings.Contains(class, "electron")
}
func uiaSensitiveFocus(target uintptr) bool {
	if target == 0 || !isSameForegroundWindow(target) {
		return false
	}
	result := inspectFocusedPassword(800 * time.Millisecond)
	if !isSameForegroundWindow(target) {
		return true
	}
	if result.available {
		return result.isPassword
	}
	return browserLikeWindow(windowClassName(target))
}
