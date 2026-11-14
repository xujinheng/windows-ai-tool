package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ASR launch planning is platform-neutral, so its behavior can be tested
// without depending on Windows, CUDA drivers, or a real GGUF.
type asrLaunchPlan struct {
	Server, Model, MMProj, Host, Port string
	GPULayers, Context                int
}

func asrExecRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func asrResolveFile(root, value string) string {
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, value)
}

func asrFirstFile(candidates ...string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		fi, err := os.Stat(candidate)
		if err == nil && !fi.IsDir() {
			return candidate
		}
	}
	return ""
}

func asrDiscoverFile(root, configured string, fallback ...string) string {
	if configured != "" {
		return asrFirstFile(asrResolveFile(root, configured))
	}
	names := make([]string, 0, len(fallback)*2)
	for _, name := range fallback {
		names = append(names, filepath.Join(root, name), filepath.Join(root, "models", name))
	}
	return asrFirstFile(names...)
}

func asrPlan(cfg Config, root string) (asrLaunchPlan, error) {
	if !cfg.AutoStartASR {
		return asrLaunchPlan{}, fmt.Errorf("ASR 自动启动未启用")
	}
	u, err := url.Parse(cfg.ASRBaseURL)
	if err != nil || u.Scheme != "http" || u.Hostname() == "" {
		return asrLaunchPlan{}, fmt.Errorf("ASR 只允许在本机 HTTP 地址自动启动")
	}
	host := strings.ToLower(u.Hostname())
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return asrLaunchPlan{}, fmt.Errorf("ASR 只支持自动启动到本机回环地址")
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	if _, err = net.LookupPort("tcp", port); err != nil {
		return asrLaunchPlan{}, fmt.Errorf("ASR 端口无效")
	}
	server := asrDiscoverFile(root, cfg.LLAMAServerPath, "llama-server.exe")
	model := asrDiscoverFile(root, cfg.ASRModelPath, "Qwen3-ASR-1.7B-Q8_0.gguf", "Qwen3-ASR-0.6B-Q8_0.gguf")
	// Match the projector family to the selected model, rather than silently
	// pairing a 0.6B model with a 1.7B projector.
	family := "1.7B"
	if strings.Contains(strings.ToLower(filepath.Base(model)), "0.6b") {
		family = "0.6B"
	}
	mmproj := asrDiscoverFile(root, cfg.ASRMMProjPath,
		"mmproj-Qwen3-ASR-"+family+"-bf16.gguf",
		"mmproj-Qwen3-ASR-"+family+"-Q8_0.gguf")
	if model != "" && mmproj != "" && strings.Contains(strings.ToLower(filepath.Base(mmproj)), "qwen3-asr-") && !strings.Contains(strings.ToLower(filepath.Base(mmproj)), strings.ToLower(family)) {
		return asrLaunchPlan{}, fmt.Errorf("Qwen3-ASR 主模型与 mmproj 尺寸不匹配")
	}
	if server == "" || model == "" || mmproj == "" {
		return asrLaunchPlan{}, fmt.Errorf("未找到 llama-server.exe 或 Qwen3-ASR GGUF / mmproj；请在设置中检测服务，或将文件放在 EXE 同级 models 文件夹")
	}
	gpu := cfg.ASRGPULayers
	if gpu < 0 || gpu > 1000 {
		gpu = 99
	}
	context := cfg.ASRContextSize
	if context < 1024 || context > 131072 {
		context = 8192
	}
	return asrLaunchPlan{Server: server, Model: model, MMProj: mmproj, Host: "127.0.0.1", Port: port, GPULayers: gpu, Context: context}, nil
}

func asrPlanArgs(p asrLaunchPlan) []string {
	return []string{"-m", p.Model, "--mmproj", p.MMProj, "-ngl", fmt.Sprint(p.GPULayers), "-c", fmt.Sprint(p.Context), "-np", "1", "--host", p.Host, "--port", p.Port}
}

func asrServerAlreadyRunning(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
