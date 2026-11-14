package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActualASRProbeUsesAudioTranscription(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   string
	}{
		{200, "静音测试成功"},
		{415, "HTTP 415"},
		{404, "不支持 POST"},
		{401, "API Key"},
		{400, "请试录真实语音"},
		{503, "尚未就绪"},
	} {
		t.Run(fmt.Sprintf("HTTP%d", tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/transcriptions" {
					t.Errorf("wrong path and method: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer testkey" {
					t.Errorf("auth header not passed")
				}
				mr, err := r.MultipartReader()
				if err != nil {
					t.Errorf("missing multipart: %v", err)
					return
				}
				fields := map[string]bool{}
				for {
					p, err := mr.NextPart()
					if err != nil {
						break
					}
					fields[p.FormName()] = true
					if p.FormName() == "file" && p.FileName() != "voice.wav" {
						t.Errorf("unexpected file name: %q", p.FileName())
					}
				}
				for _, name := range []string{"file", "model", "response_format"} {
					if !fields[name] {
						t.Errorf("missing field %s", name)
					}
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			got := uxProbeModel(context.Background(), "ASR", server.URL+"/v1", "testkey", "whisper-1")
			if !strings.Contains(got, tc.want) {
				t.Fatalf("status=%d: %s", tc.status, got)
			}
		})
	}
}
