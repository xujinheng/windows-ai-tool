//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
)

func waitKeysReleased(max time.Duration) bool {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		keys := []uintptr{VK_CONTROL, VK_MENU, VK_SHIFT, recordVirtualKey}
		down := false
		for _, k := range keys {
			v, _, _ := procGetAsyncKeyState.Call(k)
			if v&0x8000 != 0 {
				down = true
				break
			}
		}
		if !down {
			time.Sleep(60 * time.Millisecond)
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func sendCombo(letter uint16) error {
	keys := [4]winINPUT{
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: VK_CONTROL}},
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: letter}},
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: letter, Flags: KEYEVENTF_KEYUP}},
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: VK_CONTROL, Flags: KEYEVENTF_KEYUP}},
	}
	if unsafe.Sizeof(keys[0]) != 40 {
		return fmt.Errorf("INPUT layout invalid: %d", unsafe.Sizeof(keys[0]))
	}
	n, _, e := procSendInput.Call(uintptr(len(keys)), uintptr(unsafe.Pointer(&keys[0])), unsafe.Sizeof(keys[0]))
	runtime.KeepAlive(keys)
	if n != 4 {
		return fmt.Errorf("SendInput 失败(%d/4): %v。管理员权限的目标程序可能不接受普通权限模拟键盘", n, e)
	}
	return nil
}
func clipboardSeq() uintptr { n, _, _ := procGetClipboardSequenceNumber.Call(); return n }
func openClipboard(hwnd uintptr) error {
	for i := 0; i < 20; i++ {
		ok, _, _ := procOpenClipboard.Call(hwnd)
		if ok != 0 {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("无法打开剪贴板（其他程序可能正在占用）")
}
func getClipboardText(hwnd uintptr) (string, bool, error) {
	if e := openClipboard(hwnd); e != nil {
		return "", false, e
	}
	defer procCloseClipboard.Call()
	h, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if h == 0 {
		return "", false, nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return "", false, errors.New("读取剪贴板失败")
	}
	defer procGlobalUnlock.Call(h)
	// Limit reads to the actual allocation instead of assuming a huge buffer.
	allocationSize, _, _ := procGlobalSize.Call(h)
	if allocationSize < 2 || allocationSize > 8*1024*1024 {
		return "", false, errors.New("剪贴板文本长度异常")
	}
	data := unsafe.Slice((*uint16)(unsafe.Pointer(p)), int(allocationSize/2))
	n := 0
	for n < len(data) && data[n] != 0 {
		n++
	}
	if n == len(data) {
		return "", false, errors.New("剪贴板文本长度异常")
	}
	text := string(utf16.Decode(data[:n]))
	return text, true, nil
}
func setClipboardText(hwnd uintptr, s string) error {
	if e := openClipboard(hwnd); e != nil {
		return e
	}
	defer procCloseClipboard.Call()
	chars := utf16.Encode([]rune(s))
	chars = append(chars, 0)
	size := uintptr(len(chars) * 2)
	h, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if h == 0 {
		return errors.New("剪贴板内存分配失败")
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return errors.New("无法锁定剪贴板内存")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(p)), len(chars)), chars)
	procGlobalUnlock.Call(h)
	if ok, _, _ := procEmptyClipboard.Call(); ok == 0 {
		procGlobalFree.Call(h)
		return errors.New("无法清空剪贴板")
	}
	if out, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, h); out == 0 {
		procGlobalFree.Call(h)
		return errors.New("无法写入剪贴板")
	}
	return nil
}

func clipboardCopy(hwnd uintptr, timeout time.Duration) (string, error) {
	old := clipboardSeq()
	if err := sendCombo('C'); err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if clipboardSeq() != old {
			s, ok, e := getClipboardText(hwnd)
			if e == nil && ok && strings.TrimSpace(s) != "" {
				return s, nil
			}
		}
		time.Sleep(35 * time.Millisecond)
	}
	return "", errors.New("未获取到选中文字。请先在原窗口选中文字再按快捷键；某些受保护或管理员窗口无法自动复制")
}
func isSameForegroundWindow(target uintptr) bool {
	if target == 0 {
		return false
	}
	foreground, _, _ := procGetForegroundWindow.Call()
	valid, _, _ := procIsWindow.Call(target)
	return valid != 0 && foreground == target
}

// Copy only while the original editor is foreground. Some applications update the
// clipboard asynchronously, so retry without crossing application boundaries.
func copySelectionWithRetry(hwnd, target uintptr, cfg Config) (string, error) {
	time.Sleep(35 * time.Millisecond) // Let mouse key-up and selection settle.
	var lastErr error
	for i := 0; i < cfg.SelectionCopyAttempts; i++ {
		if !isSameForegroundWindow(target) {
			return "", errors.New("编辑窗口已切换，已停止自动复制，避免读取其他窗口的文字")
		}
		text, err := clipboardCopy(hwnd, time.Duration(cfg.SelectionCopyTimeoutMS)*time.Millisecond)
		if !isSameForegroundWindow(target) {
			return "", errors.New("复制期间编辑窗口发生切换，已停止自动复制")
		}
		if err == nil && strings.TrimSpace(text) != "" {
			return text, nil
		}
		lastErr = err
		if i+1 < cfg.SelectionCopyAttempts {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
		}
	}
	return "", fmt.Errorf("复制选中文字失败（已尝试 %d 次）：%w", cfg.SelectionCopyAttempts, lastErr)
}

func captureSelection(hwnd uintptr, cfg Config) (selectedText, error) {
	var sel selectedText
	sel.TargetWindow, _, _ = procGetForegroundWindow.Call()
	if sel.TargetWindow == 0 {
		return sel, errors.New("找不到当前编辑窗口")
	}
	old, exists, e := getClipboardText(hwnd)
	if e != nil {
		return sel, e
	}
	sel.OriginalClipboard = old
	sel.HadOriginalClipboard = exists
	text, e := copySelectionWithRetry(hwnd, sel.TargetWindow, cfg)
	sel.CopyClipboardSeq = clipboardSeq()
	if e != nil {
		// Do not blindly overwrite the clipboard if another app/user changed it.
		return sel, e
	}
	sel.Text = text
	return sel, nil
}
func replaceSelection(hwnd uintptr, sel selectedText, replacement string, cfg Config) error {
	return replaceSelectionWithVerification(hwnd, sel, replacement, cfg, false)
}

// Unknown editor types (Chrome/Edge/WeChat/VS Code and more) cannot be
// classified reliably via Win32 HWND. First verify the selected text and
// foreground window; then paste and check whether the same original selection
// remains. If it does, the surface likely refused the paste, so show a preview.
// This remains a heuristic: it cannot prove an application's content changed.
func replaceSelectionWithVerification(hwnd uintptr, sel selectedText, replacement string, cfg Config, verifyBrowser bool) error {
	return replaceSelectionWithGuard(hwnd, sel, replacement, cfg, verifyBrowser, nil)
}

func replaceSelectionWithGuard(hwnd uintptr, sel selectedText, replacement string, cfg Config, verifyBrowser bool, valid func() bool) error {
	if valid != nil && !valid() {
		return errors.New("任务已过期，禁止覆盖原文")
	}
	if !waitKeysReleased(3 * time.Second) {
		return errors.New("请松开 Ctrl/Alt 等快捷键后再试")
	}
	fg, _, _ := procGetForegroundWindow.Call()
	windowValid, _, _ := procIsWindow.Call(sel.TargetWindow)
	if windowValid == 0 || fg != sel.TargetWindow {
		if e := setClipboardText(hwnd, replacement); e != nil {
			return e
		}
		return errors.New("编辑窗口已经切换；为了避免覆盖错误位置，改写内容已复制到剪贴板，请返回原窗口手动粘贴。")
	}
	// Re-check selection; otherwise Ctrl+V might insert into a different place.
	actual, e := copySelectionWithRetry(hwnd, sel.TargetWindow, cfg)
	if e != nil || actual != sel.Text {
		if e2 := setClipboardText(hwnd, replacement); e2 != nil {
			return e2
		}
		return errors.New("原来的选中文字已经变化；为避免误覆盖，结果已复制到剪贴板。请手动粘贴。")
	}
	if valid != nil && !valid() {
		restoreSelectionClipboardIfOwned(hwnd, selectedText{
			HadOriginalClipboard: sel.HadOriginalClipboard, OriginalClipboard: sel.OriginalClipboard,
			CopyClipboardSeq: clipboardSeq(),
		})
		return errors.New("任务已过期，禁止覆盖原文")
	}
	if !isSameForegroundWindow(sel.TargetWindow) {
		return errors.New("粘贴前编辑窗口已变化，已取消覆盖")
	}
	if e = setClipboardText(hwnd, replacement); e != nil {
		return e
	}
	ownedSeq := clipboardSeq()
	if valid != nil && !valid() {
		restoreSelectionClipboardIfOwned(hwnd, selectedText{HadOriginalClipboard: sel.HadOriginalClipboard, OriginalClipboard: sel.OriginalClipboard, CopyClipboardSeq: ownedSeq})
		return errors.New("任务已过期，禁止粘贴")
	}
	if e = sendCombo('V'); e != nil {
		return e
	}
	// Small grace interval for applications that read the clipboard asynchronously.
	time.Sleep(270 * time.Millisecond)
	if verifyBrowser {
		if !isSameForegroundWindow(sel.TargetWindow) {
			return errors.New("粘贴后窗口切换，无法验证结果")
		}
		// A post-paste Ctrl+C itself may overwrite the clipboard; track its
		// sequence as part of our own operation instead of losing the backup.
		beforeProbe := clipboardSeq()
		stillSelected, verifyErr := clipboardCopy(hwnd, 470*time.Millisecond)
		afterProbe := clipboardSeq()
		if verifyErr == nil && afterProbe != beforeProbe {
			ownedSeq = afterProbe
		}
		if verifyErr == nil && stillSelected == sel.Text {
			if mayRestoreTextClipboard(sel.HadOriginalClipboard, ownedSeq, clipboardSeq()) {
				_ = setClipboardText(hwnd, sel.OriginalClipboard)
			}
			return errors.New("原始文字仍被选中，可能是只读内容；改为预览")
		}
		if afterProbe != beforeProbe && verifyErr != nil {
			// Unknown clipboard writer: never overwrite that new value.
			ownedSeq = 0
		}
	}
	if mayRestoreTextClipboard(sel.HadOriginalClipboard, ownedSeq, clipboardSeq()) {
		if e := setClipboardText(hwnd, sel.OriginalClipboard); e != nil {
			return fmt.Errorf("改写已发送，但恢复纯文本剪贴板失败: %w", e)
		}
	}
	return nil
}
