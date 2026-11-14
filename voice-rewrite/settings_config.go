package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func uxValidURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && u.Fragment == ""
}

func uxUpdatedConfig(old Config, values [4]string) (Config, error) {
	cfg := old
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	if !uxValidURL(values[0]) || !uxValidURL(values[2]) {
		return cfg, errors.New("请输入有效的 http:// 或 https:// 服务地址")
	}
	if len(values[0]) > 500 || len(values[2]) > 500 || len(values[1]) > 150 || len(values[3]) > 150 {
		return cfg, errors.New("输入内容过长")
	}
	if values[1] == "" {
		return cfg, errors.New("ASR 模型名称不能留空")
	}
	if values[3] == "" {
		values[3] = "auto"
	}
	cfg.ASRBaseURL = strings.TrimRight(values[0], "/")
	cfg.ASRModel = values[1]
	cfg.LLMBaseURL = strings.TrimRight(values[2], "/")
	cfg.TextModel = values[3]
	return cfg, nil
}

func uxProbe(ctx context.Context, kind, base string) string {
	if !uxValidURL(base) {
		return kind + "：服务地址无效"
	}
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(base), "/"))
	if err != nil {
		return kind + "：服务地址无效"
	}
	if kind == "ASR" {
		u.Path = "/health" // llama.cpp health endpoint.
	} else {
		u.Path = strings.TrimRight(u.Path, "/") + "/models" // LM Studio /v1/models
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return kind + "：检测请求无效"
	}
	res, err := requestClient(4 * time.Second).Do(req)
	if err != nil {
		return kind + "：连接失败（请检查服务是否运行）"
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return kind + "：服务已连接 · HTTP 200"
	}
	if kind == "ASR" && res.StatusCode == http.StatusNotFound {
		return "ASR：服务可达，但没有 /health；需用实际听写验证"
	}
	if res.StatusCode == http.StatusServiceUnavailable {
		return kind + "：服务正在启动（HTTP 503）"
	}
	return fmt.Sprintf("%s：服务返回 HTTP %d", kind, res.StatusCode)
}
