package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDualMouseFlows(t *testing.T) {
	for _, tc := range []struct {
		button uint16
		want   string
	}{
		{1, "dictation"}, {2, "rewrite"}, {3, "ignore"}, {0, "ignore"},
	} {
		if got := sideButtonFlow(tc.button, 2, 1); got != tc.want {
			t.Fatalf("%d: %q != %q", tc.button, got, tc.want)
		}
	}
	if got := sideButtonFlow(2, 0, 1); got != "ignore" {
		t.Fatalf("unexpected: %s", got)
	}
}
func TestRMSLevel(t *testing.T) {
	if pcmRMSLevel(nil) != 0 || pcmRMSLevel(make([]byte, 6400)) != 0 {
		t.Fatal("silence must be 0")
	}
	loud := make([]byte, 6400)
	for i := 0; i < len(loud); i += 2 {
		loud[i] = 0xff
		loud[i+1] = 0x7f
	}
	if got := pcmRMSLevel(loud); got < 990 || got > 1000 {
		t.Fatalf("loud=%d", got)
	}
}

func TestDictationTranscribesWithoutLLM(t *testing.T) {
	var asrCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		asrCalls++
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("multipart: %v", err)
		}
		if r.FormValue("model") != "whisper-1" {
			t.Errorf("model=%s", r.FormValue("model"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"早上好，今天的会议开始。"}`))
	}))
	defer server.Close()
	cfg := defaults()
	cfg.ASRBaseURL = server.URL + "/v1"
	cfg.LLMBaseURL = "http://127.0.0.1:1/v1" // must not be called
	pcm := make([]byte, 6400)
	result, err := transcribe(context.Background(), cfg, pcm)
	if err != nil || result != "早上好，今天的会议开始。" || asrCalls != 1 {
		t.Fatalf("result=%q err=%v calls=%d", result, err, asrCalls)
	}
}
