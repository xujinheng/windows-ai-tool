//go:build windows

package main

import (
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

var (
	procPeekMessageForTest = user32.NewProc("PeekMessageW")
	procPostMessageForTest = user32.NewProc("PostMessageW")
)

// Enable with VOICE_REWRITE_NATIVE_POPUP_TEST=1 on an interactive Windows desktop.
// The isolated Linux tests and Windows cross-build cannot exercise GUI messages.
func TestNativePopupPaintAndClose(t *testing.T) {
	if os.Getenv("VOICE_REWRITE_NATIVE_POPUP_TEST") != "1" {
		t.Skip("requires an interactive Windows desktop; VOICE_REWRITE_NATIVE_POPUP_TEST=1")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer closeActiveAnswer()
	if err := showAnswerPopup(popupMessage{title: "测试", body: "这是一段正常显示的中文答案。", anchor: trayPoint{X: 350, Y: 350}}); err != nil {
		t.Fatal(err)
	}
	if activeAnswer == nil || activeAnswer.window == 0 || activeAnswer.edit == 0 {
		t.Fatal("popup not created")
	}
	if n, _, _ := procGetWindowTextLength.Call(activeAnswer.edit); n == 0 {
		t.Fatal("popup content is blank")
	}
	hwnd := activeAnswer.window
	if ok, _, err := procPostMessageForTest.Call(hwnd, WM_CLOSE, 0, 0); ok == 0 {
		t.Fatalf("PostMessage close: %v", err)
	}
	for i := 0; i < 100 && activeAnswer != nil; i++ {
		var msg winMSG
		if have, _, _ := procPeekMessageForTest.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1); have != 0 {
			dispatchNativeMessage(&msg)
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if activeAnswer != nil {
		t.Fatal("WM_CLOSE was not dispatched to the window procedure")
	}
	// Button WM_COMMAND must also close the window.
	if err := showAnswerPopup(popupMessage{title: "翻译", body: "Hello, world!", anchor: trayPoint{X: 350, Y: 350}}); err != nil {
		t.Fatal(err)
	}
	hwnd = activeAnswer.window
	procSendMessage.Call(hwnd, WM_COMMAND, BUTTON_CLOSE, 0)
	if activeAnswer != nil {
		t.Fatal("Close button failed")
	}
}

// Verify Win32 DRAWITEMSTRUCT's x64 layout before owner-draw button callbacks.
func TestDrawItemLayoutX64(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 8 && unsafe.Sizeof(drawItemStruct{}) != 64 {
		t.Fatalf("DRAWITEMSTRUCT layout invalid: %d", unsafe.Sizeof(drawItemStruct{}))
	}
}
