package main

import (
	"errors"
	"strings"
	"testing"
)

func TestFriendlyErrorClassification(t *testing.T) {
	for _, tt := range []struct{ input, expected string }{
		{"ASR HTTP 415: unsupported", "音频格式"},
		{"LLM HTTP 401", "API 验证失败"},
		{"Post http://127.0.0.1:8080: connect: connection refused", "无法连接"},
		{"context deadline exceeded", "响应超时"},
		{"原选区已更改", "选区"},
	} {
		if !strings.Contains(friendlyError(errors.New(tt.input)), tt.expected) {
			t.Errorf("%q", tt.input)
		}
	}
}
