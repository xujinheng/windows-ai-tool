//go:build windows

package main

import (
	"fmt"
	"runtime"
	"unicode/utf16"
	"unsafe"
)

const keyeventfUnicode = 0x0004

// sendUnicodeText avoids touching clipboard formats that cannot be safely
// snapshotted. SendInput uses UTF-16 code units, including surrogate pairs.
// Unlike Ctrl+V, some editors reject this input; a failed batch surfaces an
// error and the result remains available from VoiceRewrite's result card.
func sendUnicodeText(text string, stillValid func() bool) error {
	units := utf16.Encode([]rune(text))
	for pos := 0; pos < len(units); {
		if stillValid != nil && !stillValid() {
			return fmt.Errorf("录入中焦点改变，已停止键盘输入（已输入 %d/%d 个 UTF-16 单元）", pos, len(units))
		}
		end := pos + 32
		if end > len(units) {
			end = len(units)
		}
		keys := make([]winINPUT, 0, (end-pos)*2)
		for _, ch := range units[pos:end] {
			keys = append(keys,
				winINPUT{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Scan: ch, Flags: keyeventfUnicode}},
				winINPUT{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Scan: ch, Flags: keyeventfUnicode | KEYEVENTF_KEYUP}},
			)
		}
		n, _, err := procSendInput.Call(uintptr(len(keys)), uintptr(unsafe.Pointer(&keys[0])), unsafe.Sizeof(keys[0]))
		runtime.KeepAlive(keys)
		if n != uintptr(len(keys)) {
			return fmt.Errorf("Unicode 键盘输入失败（%d/%d）：%v", n, len(keys), err)
		}
		pos = end
	}
	return nil
}
