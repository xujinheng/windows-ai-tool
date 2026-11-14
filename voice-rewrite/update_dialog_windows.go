//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)
func askViewUpdate(version string) bool {
	text, _ := syscall.UTF16PtrFromString("发现新版本 " + version + "。\n\n当前程序不会自动下载或运行未经签名的安装文件。\n是否打开 GitHub 官方发布页面？")
	caption, _ := syscall.UTF16PtrFromString("VoiceRewrite · 更新可用")
	answer, _, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), 0x00000004|0x00000040|0x00040000)
	runtime.KeepAlive(text)
	runtime.KeepAlive(caption)
	return answer == 6
}
