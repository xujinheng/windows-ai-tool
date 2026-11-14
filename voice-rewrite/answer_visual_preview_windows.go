//go:build windows

package main

import (
	"runtime"
	"unsafe"
)

// Visual-only preview: uses the real Win32 result card, not a mockup.
// No model, microphone, clipboard, or selected text is accessed.
func runAnswerVisualPreview() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := showAnswerPopup(popupMessage{
		title:  "AI 回答",
		body:   "这是 VoiceRewrite 的真实结果卡片。\n\n使用本地模型可以完成：\n• 改写与润色\n• 翻译与总结\n• 对选中文本提问\n\n所有内容均为视觉验收用的固定示例文字。",
		anchor: trayPoint{X: 250, Y: 160},
	})
	if err != nil {
		return
	}
	defer closeActiveAnswer()
	for {
		var msg winMSG
		rv, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(rv) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
