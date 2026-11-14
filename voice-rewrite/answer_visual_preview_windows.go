//go:build windows

package main

import (
	"os"
	"runtime"
	"strings"
	"unsafe"
)

// Visual-only preview: uses the real Win32 result card, not a mockup.
// No model, microphone, clipboard, or selected text is accessed.
func runAnswerVisualPreview() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	body := "这是 VoiceRewrite 的真实结果卡片。\n\n使用本地模型可以完成：\n• 改写与润色\n• 翻译与总结\n• 对选中文本提问\n\n所有内容均为视觉验收用的固定示例文字。"
	if os.Getenv("VOICEREWRITE_VISUAL_LONG") == "1" {
		body = strings.Repeat("这一段用于验证长文本滚动、换行、阅读区域和底部操作按钮。\n", 80)
	}
	// Render the real Apply/Copy action combinations without a target editor,
	// model request or text replacement. Preview Apply is intentionally inert.
	previewApply := os.Getenv("VOICEREWRITE_VISUAL_APPLY") == "1"
	var selection *selectedText
	if previewApply {
		selection = &selectedText{}
	}
	title := "AI 回答"
	if previewApply {
		title = "AI 改写结果"
	}
	err := showAnswerPopup(popupMessage{
		title:      title,
		body:       body,
		anchor:     trayPoint{X: 250, Y: 160},
		selection:  selection,
		allowApply: previewApply,
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
		dispatchNativeMessage(&msg)
	}
}
