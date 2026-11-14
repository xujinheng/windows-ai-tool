package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func uxValidURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && u.Fragment == "" && u.RawQuery == ""
}

func uxUpdatedConfig(old Config, values [6]string) (Config, error) {
	cfg := old
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	if !uxValidURL(values[0]) || !uxValidURL(values[3]) {
		return cfg, errors.New("请输入有效的 http:// 或 https:// 服务地址")
	}
	if len(values[0]) > 500 || len(values[3]) > 500 || len(values[1]) > 150 || len(values[4]) > 150 || len(values[2]) > 4000 || len(values[5]) > 4000 {
		return cfg, errors.New("输入内容过长")
	}
	if values[1] == "" {
		return cfg, errors.New("ASR 模型名称不能留空")
	}
	if values[4] == "" {
		values[4] = "auto"
	}
	cfg.ASRBaseURL = strings.TrimRight(values[0], "/")
	cfg.ASRModel = values[1]
	cfg.ASRAPIKey = values[2]
	cfg.LLMBaseURL = strings.TrimRight(values[3], "/")
	cfg.TextModel = values[4]
	cfg.LLMAPIKey = values[5]
	return cfg, nil
}

// Probe only discovers whether an OpenAI-compatible API is reachable.
// No process launch, no server-specific /health dependency.
func uxProbe(ctx context.Context, kind, base string) string {
	return uxProbeWithKey(ctx, kind, base, "")
}

func uxProbeWithKey(ctx context.Context, kind, base, apiKey string) string {
	return uxProbeModel(ctx, kind, base, apiKey, "")
}

// A service health response and a compatible model are two separate facts.
// This probe never uploads microphone audio or sends text prompts.
func uxProbeModel(ctx context.Context, kind, base, apiKey, configuredModel string) string {
	if kind == "ASR" && strings.TrimSpace(configuredModel) != "" {
		return uxProbeASRAudio(ctx, base, apiKey, configuredModel)
	}
	if !uxValidURL(base) {
		return kind + "：服务地址无效"
	}
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(base), "/"))
	if err != nil {
		return kind + "：服务地址无效"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/models"
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return kind + "：检测请求无效"
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := requestClient(4 * time.Second).Do(req)
	if err != nil {
		return kind + "：无法连接，请检查 API 地址及网络"
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		var response struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if e := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&response); e != nil {
			return kind + "：API 可达 · HTTP 200，但模型列表格式无法识别；请试用实际功能"
		}
		if len(response.Data) == 0 {
			return kind + "：API 可达 · HTTP 200，模型列表为空；请检查是否已加载模型"
		}
		chosen := strings.TrimSpace(configuredModel)
		if chosen != "" && chosen != "auto" {
			for _, model := range response.Data {
				if model.ID == chosen {
					return kind + "：模型 " + chosen + " 已在服务中发现（未验证实际任务）"
				}
			}
			return kind + "：服务可达，但模型 " + chosen + " 未列出；请核对 ID 或服务别名"
		}
		return fmt.Sprintf("%s：已连接 · 可发现 %d 个模型（未验证实际任务）", kind, len(response.Data))
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return kind + "：服务可达；此接口不提供模型列表，请实际试用"
	case http.StatusUnauthorized, http.StatusForbidden:
		return kind + "：API Key 无效或没有权限"
	case http.StatusServiceUnavailable:
		return kind + "：服务暂时不可用（503）"
	default:
		return fmt.Sprintf("%s：API 返回 HTTP %d", kind, res.StatusCode)
	}
}

// uxProbeASRAudio exercises the actual endpoint and request format used by
// VoiceRewrite. A models-list success cannot establish support for uploading
// multipart WAV, as some text-only OpenAI-compatible APIs demonstrate.
// The probe sends only locally generated silence, never microphone recordings.
func uxProbeASRAudio(ctx context.Context, base, apiKey, model string) string {
	const prefix = "ASR："
	if !uxValidURL(base) || strings.TrimSpace(model) == "" {
		return prefix + "请填写有效的服务地址和模型 ID"
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cfg := Config{ASRBaseURL: strings.TrimRight(strings.TrimSpace(base), "/"), ASRModel: strings.TrimSpace(model), ASRAPIKey: apiKey}
	// Exactly one second of 16-kHz 16-bit mono silence. Valid WAV and multipart
	// headers, but contains no actual voice or private information.
	req, err := newASRRequest(ctx, cfg, make([]byte, 16000*2))
	if err != nil {
		return prefix + "无法生成测试音频请求"
	}
	res, err := requestClient(15 * time.Second).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return prefix + "音频接口检测超时；请检查模型是否加载完成"
		}
		return prefix + "无法连接音频接口；请检查服务地址及网络"
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return prefix + "音频转写接口接受 WAV 上传（静音测试成功；请试录真实语音）"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return fmt.Sprintf("%s音频接口收到请求但拒绝测试音频（HTTP %d）；请试录真实语音", prefix, res.StatusCode)
	case http.StatusUnsupportedMediaType:
		return prefix + "HTTP 415：服务不接受当前 WAV 音频上传格式，无法直接用于听写"
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return fmt.Sprintf("%s服务不支持 POST /v1/audio/transcriptions（HTTP %d）", prefix, res.StatusCode)
	case http.StatusUnauthorized, http.StatusForbidden:
		return prefix + "认证失败：API Key 无效或无访问权限"
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return fmt.Sprintf("%s模型服务尚未就绪（HTTP %d）", prefix, res.StatusCode)
	default:
		return fmt.Sprintf("%s音频接口返回 HTTP %d；请检查服务器日志", prefix, res.StatusCode)
	}
}
