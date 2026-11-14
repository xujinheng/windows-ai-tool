package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Windows RegisterHotKey modifier values are kept here so the configuration
// parser and its unit tests work on Linux as well as Windows.
const (
	shortcutAlt      uint32 = 0x0001
	shortcutCtrl     uint32 = 0x0002
	shortcutShift    uint32 = 0x0004
	shortcutNoRepeat uint32 = 0x4000
)

// Shortcut input is captured as physical VKs in the native settings UI.
// Bare letters/digits are deliberately forbidden: they would hijack typing
// in every application. Win+key is reserved for Windows/system shortcuts.
var shortcutNamedKeys = map[string]uint32{
	"SPACE": 0x20, "TAB": 0x09, "ENTER": 0x0D,
	"LEFT": 0x25, "UP": 0x26, "RIGHT": 0x27, "DOWN": 0x28,
	"HOME": 0x24, "END": 0x23, "PAGEUP": 0x21, "PAGEDOWN": 0x22,
	"INSERT": 0x2D, "DELETE": 0x2E,
}

func shortcutKeyName(vk uint32) string {
	if vk >= 0x70 && vk <= 0x87 {
		return fmt.Sprintf("F%d", vk-0x70+1)
	}
	if vk >= 'A' && vk <= 'Z' || vk >= '0' && vk <= '9' {
		return string(rune(vk))
	}
	for name, key := range shortcutNamedKeys {
		if vk == key {
			switch name {
			case "PAGEUP":
				return "PageUp"
			case "PAGEDOWN":
				return "PageDown"
			default:
				return strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
			}
		}
	}
	return ""
}

func shortcutFromKey(vk, modifiers uint32) (string, error) {
	name := shortcutKeyName(vk)
	if name == "" {
		return "", errors.New("该按键不能用作全局录音快捷键")
	}
	if modifiers & ^(shortcutAlt|shortcutCtrl|shortcutShift) != 0 {
		return "", errors.New("Windows 系统键组合不支持录音快捷键")
	}
	if modifiers == 0 && (vk < 0x70 || vk > 0x87) {
		return "", errors.New("单键仅支持 F1–F24；字母、数字和方向键请搭配 Ctrl 或 Alt")
	}
	if modifiers == shortcutShift && (vk < 0x70 || vk > 0x87) {
		return "", errors.New("Shift+字母会干扰打字，请同时使用 Ctrl 或 Alt")
	}
	parts := []string{}
	if modifiers&shortcutCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if modifiers&shortcutAlt != 0 {
		parts = append(parts, "Alt")
	}
	if modifiers&shortcutShift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, name), "+"), nil
}

func parseShortcutParts(raw string) (string, uint32, uint32, bool, error) {
	key := strings.TrimSpace(raw)
	switch strings.ToLower(key) {
	case "mouse4", "xbutton1":
		return "Mouse4", 0, 0, true, nil
	case "mouse5", "xbutton2":
		return "Mouse5", 0, 0, true, nil
	case "pause", "break":
		return "Pause", shortcutNoRepeat, 0x13, false, nil
	}
	parts := strings.Split(key, "+")
	if len(parts) == 0 {
		return "", 0, 0, false, errors.New("请按下快捷键")
	}
	var modifiers uint32
	for _, token := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(token)) {
		case "ctrl", "control":
			if modifiers&shortcutCtrl != 0 {
				return "", 0, 0, false, errors.New("Ctrl 重复")
			}
			modifiers |= shortcutCtrl
		case "alt":
			if modifiers&shortcutAlt != 0 {
				return "", 0, 0, false, errors.New("Alt 重复")
			}
			modifiers |= shortcutAlt
		case "shift":
			if modifiers&shortcutShift != 0 {
				return "", 0, 0, false, errors.New("Shift 重复")
			}
			modifiers |= shortcutShift
		default:
			return "", 0, 0, false, fmt.Errorf("未知修饰键 %q", token)
		}
	}
	last := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
	var vk uint32
	if strings.HasPrefix(last, "F") {
		if n, err := strconv.Atoi(strings.TrimPrefix(last, "F")); err == nil && n >= 1 && n <= 24 {
			vk = 0x70 + uint32(n) - 1
		}
	}
	if vk == 0 && len(last) == 1 {
		if last[0] >= 'A' && last[0] <= 'Z' || last[0] >= '0' && last[0] <= '9' {
			vk = uint32(last[0])
		}
	}
	if vk == 0 {
		vk = shortcutNamedKeys[last]
	}
	normalized, err := shortcutFromKey(vk, modifiers)
	if err != nil {
		return "", 0, 0, false, err
	}
	return normalized, modifiers | shortcutNoRepeat, vk, false, nil
}

func validateVoiceShortcut(candidate, other, exit, label string) (string, error) {
	normalized, _, vk, mouse, err := parseShortcutParts(candidate)
	if err != nil {
		return "", err
	}
	if !mouse && vk == 0x13 {
		return "", errors.New("Pause/Break 没有可靠的松开事件")
	}
	otherValue, _, _, _, _ := parseShortcutParts(other)
	exitValue, _, _, _, _ := parseShortcutParts(exit)
	if normalized == otherValue || normalized == exitValue {
		return "", fmt.Errorf("%s不能与另一语音功能或退出快捷键相同", label)
	}
	return normalized, nil
}

func uxDictationShortcut(candidate, rewrite, exit string) (string, error) {
	return validateVoiceShortcut(candidate, rewrite, exit, "听写快捷键")
}
func uxRewriteShortcut(candidate, dictation, exit string) (string, error) {
	return validateVoiceShortcut(candidate, dictation, exit, "语音指令快捷键")
}

func canonicalShortcut(raw string) (string, bool) {
	v, _, _, _, err := parseShortcutParts(raw)
	return v, err == nil
}
