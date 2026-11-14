package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VoiceRewrite never sends audio or selected text to a provider other than the
// two HTTP endpoints explicitly configured by the user.
type Config struct {
	ASRBaseURL             string `json:"asr_base_url"`
	LLMBaseURL             string `json:"llm_base_url"`
	TextModel              string `json:"text_model"`
	ASRAPIKey              string `json:"asr_api_key"`
	LLMAPIKey              string `json:"llm_api_key"`
	ASRModel               string `json:"asr_model"`
	AudioDeviceID          int    `json:"audio_device_id"`
	HotkeyRecord           string `json:"hotkey_record"`
	DictationHotkey        string `json:"dictation_hotkey"`
	RecordMode             string `json:"record_mode"`
	HotkeyExit             string `json:"hotkey_exit"`
	SystemPrompt           string `json:"system_prompt"`
	MaxResponseTokens      int    `json:"max_response_tokens"`
	MaxRecordSeconds       int    `json:"max_record_seconds"`
	MaxSelectedRunes       int    `json:"max_selected_characters"`
	SelectionCopyAttempts  int    `json:"selection_copy_attempts"`
	SelectionCopyTimeoutMS int    `json:"selection_copy_timeout_ms"`
}

func defaults() Config {
	return Config{
		ASRBaseURL:             "http://127.0.0.1:8080/v1",
		LLMBaseURL:             "http://127.0.0.1:1234/v1",
		TextModel:              "auto",
		ASRModel:               "whisper-1",
		AudioDeviceID:          -1,
		HotkeyRecord:           "Mouse5",
		DictationHotkey:        "Mouse4",
		RecordMode:             "hold",
		HotkeyExit:             "Ctrl+Alt+F10",
		MaxRecordSeconds:       60,
		MaxSelectedRunes:       12000,
		MaxResponseTokens:      1200,
		SelectionCopyAttempts:  2,
		SelectionCopyTimeoutMS: 750,
	}
}

// mouseSideButton reports the Windows XBUTTON identifier. Mouse4 is XBUTTON1
// and Mouse5 is XBUTTON2, independent of how the mouse is physically arranged.
func mouseSideButton(s string) (uint16, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mouse4", "xbutton1":
		return 1, true
	case "mouse5", "xbutton2":
		return 2, true
	default:
		return 0, false
	}
}

func configPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return filepath.Join(os.TempDir(), "VoiceRewrite", "config.json")
	}
	return filepath.Join(base, "VoiceRewrite", "config.json")
}

func loadConfig() (Config, error) {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return defaults(), fmt.Errorf("无法创建用户配置目录：%w", err)
	}
	cfg := defaults()
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		blob, _ := json.MarshalIndent(cfg, "", "  ")
		if e := os.WriteFile(p, append(blob, '\n'), 0600); e != nil {
			return cfg, fmt.Errorf("无法创建配置文件：%w", e)
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("配置文件 JSON 错误: %w", err)
	}
	if cfg, err = decryptConfigSecrets(cfg); err != nil {
		return cfg, fmt.Errorf("无法解锁 API 凭据（当前 Windows 用户）: %w", err)
	}
	if cfg.ASRModel == "" {
		cfg.ASRModel = "whisper-1"
	}
	if cfg.HotkeyRecord == "" {
		cfg.HotkeyRecord = "Mouse5"
	}
	if cfg.DictationHotkey == "" {
		cfg.DictationHotkey = "Mouse4"
	}
	if cfg.RecordMode == "" {
		cfg.RecordMode = "hold"
	}
	if cfg.RecordMode != "toggle" && cfg.RecordMode != "hold" {
		return cfg, fmt.Errorf("invalid record_mode %q: use toggle or hold", cfg.RecordMode)
	}
	if cfg.HotkeyExit == "" {
		cfg.HotkeyExit = "Ctrl+Alt+F10"
	}
	if cfg.AudioDeviceID < -1 || cfg.AudioDeviceID > 255 {
		return cfg, fmt.Errorf("录音设备编号无效：%d", cfg.AudioDeviceID)
	}
	if cfg.MaxResponseTokens < 128 || cfg.MaxResponseTokens > 4096 {
		cfg.MaxResponseTokens = 1200
	}
	if cfg.MaxRecordSeconds <= 0 || cfg.MaxRecordSeconds > 180 {
		cfg.MaxRecordSeconds = 60
	}
	if cfg.MaxSelectedRunes <= 0 || cfg.MaxSelectedRunes > 100000 {
		cfg.MaxSelectedRunes = 12000
	}
	if cfg.SelectionCopyAttempts < 1 || cfg.SelectionCopyAttempts > 5 {
		cfg.SelectionCopyAttempts = 3
	}
	if cfg.SelectionCopyTimeoutMS < 500 || cfg.SelectionCopyTimeoutMS > 5000 {
		cfg.SelectionCopyTimeoutMS = 1800
	}
	for _, x := range []string{cfg.ASRBaseURL, cfg.LLMBaseURL} {
		if err := validateServerBaseURL(x); err != nil {
			return cfg, fmt.Errorf("无效的 API 地址: %w", err)
		}
	}
	return cfg, nil
}
