package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var singleFunctionKey = regexp.MustCompile(`(?i)^F([1-9]|1[0-9]|2[0-4])$`)

// uxDictationShortcut is independent of native Windows calls so the same
// validation can be exercised in CI. A lone F key supports hold-to-talk;
// modifier combinations do not reliably expose their physical release.
func uxDictationShortcut(candidate, rewrite, exit string) (string, error) {
	key := strings.TrimSpace(candidate)
	normalized := key
	switch strings.ToLower(key) {
	case "mouse4", "xbutton1":
		normalized = "Mouse4"
	case "mouse5", "xbutton2":
		normalized = "Mouse5"
	default:
		match := singleFunctionKey.FindStringSubmatch(key)
		if match == nil {
			return "", errors.New("听写快捷键须为 Mouse4、Mouse5 或 F1–F24")
		}
		n, _ := strconv.Atoi(match[1])
		normalized = fmt.Sprintf("F%d", n)
	}
	normalizedRewrite, _ := canonicalShortcut(rewrite)
	normalizedExit, _ := canonicalShortcut(exit)
	if normalized == normalizedRewrite || normalized == normalizedExit {
		return "", errors.New("听写快捷键不能与改写或退出快捷键重复")
	}
	return normalized, nil
}

func canonicalShortcut(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mouse4", "xbutton1":
		return "Mouse4", true
	case "mouse5", "xbutton2":
		return "Mouse5", true
	}
	if m := singleFunctionKey.FindStringSubmatch(strings.TrimSpace(raw)); m != nil {
		n, _ := strconv.Atoi(m[1])
		return fmt.Sprintf("F%d", n), true
	}
	return "", false
}
