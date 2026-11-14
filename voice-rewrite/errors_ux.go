package main

import "strings"

// Human-readable recovery guidance. Never include request bodies or API keys.
func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "http 415"):
		return "语音接口不支持上传的音频格式（HTTP 415）。请确认 ASR 服务提供 /v1/audio/transcriptions，而不只是 /v1/chat/completions。"
	case strings.Contains(lower, "401") || strings.Contains(lower, "403"):
		return "API 验证失败。检查服务地址、模型 ID 和 API Key。"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "connectex"):
		return "无法连接模型服务。请启动 ASR / LLM 服务并检查端口，然后在设置中测试连接。"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return "模型服务响应超时。检查模型是否仍在加载、GPU 内存与网络连接。"
	case strings.Contains(message, "选区") || strings.Contains(message, "窗口已"):
		return "原选区或编辑窗口发生变化，自动写入已取消。请重新选中文字再试。"
	default:
		return message
	}
}
