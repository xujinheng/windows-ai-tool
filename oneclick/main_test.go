package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCleanTranscriptAllLanguages(t *testing.T) {
	cases := []struct{ in, out string }{
		{"language Chinese<asr_text>你好！", "你好！"},
		{"language English<asr_text>What's wrong with you?", "What's wrong with you?"},
		{"language English<asr_text>Hello. language Chinese<asr_text>你好。", "Hello. 你好。"},
		{"language German<asr_text>Guten Morgen.", "Guten Morgen."},
		{"language Japanese<asr_text>こんにちは", "こんにちは"},
		{"language Chinese,English<asr_text>Hi 你好", "Hi 你好"},
		{"language None<asr_text>", ""},
		{"Normal recognized text", "Normal recognized text"},
		{"language Chinese<asr_text>1. language English<asr_text>2. language Chinese<asr_text>3.", "1. 2. 3."},
	}
	for _, tc := range cases {
		if got := cleanTranscript(tc.in); got != tc.out {
			t.Errorf("input %q: got %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestProxyWithMultipartAndAllLanguages(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Error("lost multipart")
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "pretend WAVE") {
			t.Error("lost WAV")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"text":"language English<asr_text>Hello. language Chinese<asr_text>你好。","language":"en"}`)
	}))
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	ready := &atomic.Bool{}
	proxy := httptest.NewServer(makeHandler(u, backend.Client(), ready))
	defer proxy.Close()
	req, _ := http.NewRequest("POST", proxy.URL+"/v1/audio/transcriptions", strings.NewReader("pretend WAVE"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("not-ready status: %d", resp.StatusCode)
	}
	ready.Store(true)
	req, _ = http.NewRequest("POST", proxy.URL+"/v1/audio/transcriptions", strings.NewReader("pretend WAVE"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	resp, err = proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	got := map[string]string{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["text"] != "Hello. 你好。" {
		t.Fatalf("text: %#v", got)
	}
	if got["language"] != "en" {
		t.Fatalf("other field modified %#v", got)
	}
}

func TestConfigAndPaths(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.json")
	cfg, err := loadConfig(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BackendPort != 8080 || cfg.ListenAddress != "127.0.0.1:8081" {
		t.Fatal(cfg)
	}
	for _, name := range []string{"Qwen3-ASR-1.7B-Q8_0.gguf", "mmproj-Qwen3-ASR-1.7B-bf16.gguf"} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	opts := cfg.toOptions(dir)
	if err = findModelPaths(dir, &opts); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(opts.mmprojPath, "bf16.gguf") {
		t.Fatal(opts.mmprojPath)
	}
	data, _ := os.ReadFile(cfgFile)
	if strings.Contains(string(data), "Users\\") {
		t.Fatal("private path in config")
	}
}

func TestLlamaArgs(t *testing.T) {
	args := llamaArgs(options{modelPath: "model.gguf", mmprojPath: "mmproj.gguf", gpuLayers: 99, contextSize: 8192, upstreamHost: "127.0.0.1", llamaPort: 8080})
	if strings.Join(args, " ") != "-m model.gguf --mmproj mmproj.gguf -ngl 99 -c 8192 -np 1 --host 127.0.0.1 --port 8080" {
		t.Fatalf("args mismatch %q", args)
	}
}
