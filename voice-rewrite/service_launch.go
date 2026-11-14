package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// An endpoint is either connected to a user-managed HTTP server or started
// as a dedicated llama-server subprocess. Each role is independently chosen.
const (
	serviceConnect = "connect"
	serviceLaunch  = "launch"
)

type servicePlan struct {
	Role, Exe, Model, MMProj, Alias, Host, Port, APIKey string
	GPULayers, ContextSize                              int
}

func managedAddress(base string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" {
		return "", "", errors.New("自启动只支持本机 HTTP 地址，例如 http://127.0.0.1:8080/v1；远程或 HTTPS 请使用连接模式")
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return "", "", errors.New("自启动不允许监听远程地址，请改用连接模式")
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	num, err := strconv.Atoi(port)
	if err != nil || num < 1 || num > 65535 {
		return "", "", errors.New("端口号必须在 1–65535 之间")
	}
	// Always bind loopback, avoiding localhost IPv6 vs IPv4 ambiguities.
	return "127.0.0.1", strconv.Itoa(num), nil
}
func requiredFile(root, value, label string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("请填写%s路径", label)
	}
	abs := asrResolveFile(root, strings.TrimSpace(value))
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() {
		return "", fmt.Errorf("%s不存在或不是文件：%s", label, abs)
	}
	return abs, nil
}
func servicePlanFor(cfg Config, role, root string) (servicePlan, error) {
	var mode, base, modelPath, mmproj, alias, apiKey string
	var gpu, ctx int
	switch role {
	case "ASR":
		mode, base, modelPath, mmproj, alias, apiKey, gpu, ctx = cfg.ASRServiceMode, cfg.ASRBaseURL, cfg.ASRModelPath, cfg.ASRMMProjPath, cfg.ASRModel, cfg.ASRAPIKey, cfg.ASRGPULayers, cfg.ASRContextSize
	case "LLM":
		mode, base, modelPath, alias, apiKey, gpu, ctx = cfg.LLMServiceMode, cfg.LLMBaseURL, cfg.LLMModelPath, cfg.TextModel, cfg.LLMAPIKey, cfg.LLMGPULayers, cfg.LLMContextSize
	default:
		return servicePlan{}, fmt.Errorf("unknown role: %s", role)
	}
	if mode != serviceLaunch {
		return servicePlan{}, fmt.Errorf("%s 当前选择连接已有服务，不需要启动", role)
	}
	host, port, err := managedAddress(base)
	if err != nil {
		return servicePlan{}, fmt.Errorf("%s：%w", role, err)
	}
	executable := strings.TrimSpace(cfg.LLAMAServerPath)
	if executable == "" {
		executable = asrFirstFile(filepath.Join(root, "llama-server.exe"), filepath.Join(root, "models", "llama-server.exe"))
		if executable == "" {
			return servicePlan{}, errors.New("请在设置中选择 llama-server.exe，或把它放到程序目录")
		}
	}
	exe, err := requiredFile(root, executable, "llama-server.exe")
	if err != nil {
		return servicePlan{}, err
	}
	model, err := requiredFile(root, modelPath, role+" GGUF 模型")
	if err != nil {
		return servicePlan{}, err
	}
	if !strings.EqualFold(filepath.Ext(model), ".gguf") {
		return servicePlan{}, fmt.Errorf("%s 自启动需要 llama.cpp 支持的 GGUF 模型；其他格式请连接对应推理服务", role)
	}
	projector := ""
	if mmproj != "" {
		projector, err = requiredFile(root, mmproj, "ASR mmproj")
		if err != nil {
			return servicePlan{}, err
		}
	}
	if strings.Contains(strings.ToLower(filepath.Base(model)), "qwen3-asr") && projector == "" {
		return servicePlan{}, errors.New("Qwen3-ASR GGUF 还需要配套的 mmproj 模型文件")
	}
	if gpu < 0 || gpu > 1000 {
		return servicePlan{}, fmt.Errorf("%s GPU 层数超出范围 0–1000", role)
	}
	if ctx < 256 || ctx > 262144 {
		return servicePlan{}, fmt.Errorf("%s 上下文大小超出范围 256–262144", role)
	}
	// "auto" is only the client's discovery mode, never an API model alias.
	if strings.EqualFold(alias, "auto") {
		alias = ""
	}
	if strings.ContainsAny(alias, "\r\n") || strings.ContainsAny(apiKey, "\r\n") {
		return servicePlan{}, errors.New("模型 ID/API Key 不允许换行")
	}
	return servicePlan{Role: role, Exe: exe, Model: model, MMProj: projector, Alias: alias, Host: host, Port: port, APIKey: apiKey, GPULayers: gpu, ContextSize: ctx}, nil
}
func serviceArgs(p servicePlan) []string {
	args := []string{"-m", p.Model, "--host", p.Host, "--port", p.Port, "-ngl", strconv.Itoa(p.GPULayers), "-c", strconv.Itoa(p.ContextSize), "-np", "1", "--no-webui"}
	if p.MMProj != "" {
		args = append(args, "--mmproj", p.MMProj)
	}
	if p.Alias != "" {
		args = append(args, "--alias", p.Alias)
	}
	return args
}
func validateServiceModes(cfg Config) error {
	for _, pair := range []struct{ role, mode string }{{"ASR", cfg.ASRServiceMode}, {"LLM", cfg.LLMServiceMode}} {
		if pair.mode != serviceConnect && pair.mode != serviceLaunch {
			return fmt.Errorf("%s 模式必须是 connect 或 launch", pair.role)
		}
	}
	if cfg.ASRServiceMode == serviceLaunch && cfg.LLMServiceMode == serviceLaunch {
		ah, ap, e := managedAddress(cfg.ASRBaseURL)
		if e != nil {
			return e
		}
		lh, lp, e := managedAddress(cfg.LLMBaseURL)
		if e != nil {
			return e
		}
		if net.JoinHostPort(ah, ap) == net.JoinHostPort(lh, lp) {
			return errors.New("ASR 与文本模型不能在同一端口分别启动，请为其中一个改用其他端口（如 8082）")
		}
	}
	return nil
}
