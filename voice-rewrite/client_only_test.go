package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientOnlyConfigHasNoServerManagement(t *testing.T) {
	b, err := json.Marshal(defaults())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"llama_server_path", "_service_mode", "mmproj_path", "gpu_layers", "model_path", "context_size", "auto_start"} {
		if strings.Contains(string(b), key) {
			t.Errorf("server-only config field remains: %s", key)
		}
	}
	var cfg Config
	// Historical configuration keys must not re-enable local process startup.
	if err := json.Unmarshal([]byte(`{"asr_service_mode":"launch","llama_server_path":"server.exe","asr_model_path":"model.gguf","asr_base_url":"https://example.org/v1","llm_base_url":"https://llm.example.org/v1"}`), &cfg); err != nil {
		t.Fatal(err)
	}
	result, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), "launch") || strings.Contains(string(result), "gguf") || strings.Contains(string(result), "server.exe") {
		t.Fatal("deprecated fields were preserved in config")
	}
}

func TestClientOnlyAPIProbeAuthAndNonstandardModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected probe path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if got := uxProbeWithKey(context.Background(), "ASR", srv.URL+"/v1", "token123"); !strings.Contains(got, "204") {
		t.Fatalf("got %q", got)
	}
	if got := uxProbeWithKey(context.Background(), "ASR", srv.URL+"/v1", ""); !strings.Contains(got, "API Key") {
		t.Fatalf("got %q", got)
	}
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer fallback.Close()
	if got := uxProbe(context.Background(), "ASR", fallback.URL+"/v1"); !strings.Contains(got, "请实际试用") {
		t.Fatalf("got %q", got)
	}
}

func TestClientOnlySettingsPersistCredentials(t *testing.T) {
	cfg, err := uxUpdatedConfig(defaults(), [6]string{"https://asr.example.net/v1", "speech-v2", "asr-token", "https://llm.example.net/v1", "writer-v3", "text-token"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ASRBaseURL != "https://asr.example.net/v1" || cfg.LLMBaseURL != "https://llm.example.net/v1" || cfg.ASRAPIKey != "asr-token" || cfg.LLMAPIKey != "text-token" || cfg.TextModel != "writer-v3" {
		t.Fatalf("unexpected fields: %#v", cfg)
	}
}
