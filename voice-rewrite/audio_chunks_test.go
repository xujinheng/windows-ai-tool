package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLongAudioChunksNeverDropSamples(t *testing.T) {
	pcm := make([]byte, 62*pcmBytesPerSecond)
	for i := 0; i < len(pcm); i += 2 {
		binary.LittleEndian.PutUint16(pcm[i:i+2], uint16(i/2))
	}
	parts := splitPCMForASR(pcm)
	if len(parts) < 3 {
		t.Fatalf("expected several chunks; got %d", len(parts))
	}
	if !bytes.Equal(bytes.Join(parts, nil), pcm) {
		t.Fatal("lost or reordered samples")
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > asrChunkSeconds*pcmBytesPerSecond || len(part)%2 != 0 {
			t.Fatalf("invalid PCM chunk length: %d", len(part))
		}
	}
}

func TestASRChunksMergeAndRespectCancel(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := calls.Add(1)
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		fmt.Fprintf(w, `{"text":"segment %d"}`, i)
	}))
	defer srv.Close()
	cfg := defaults()
	cfg.ASRBaseURL = srv.URL + "/v1"
	got, err := transcribe(context.Background(), cfg, make([]byte, 55*pcmBytesPerSecond))
	if err != nil || !strings.Contains(got, "segment 1") || !strings.Contains(got, "segment 2") || calls.Load() < 2 {
		t.Fatalf("got=%q err=%v calls=%d", got, err, calls.Load())
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	before := calls.Load()
	if _, err = transcribe(canceled, cfg, make([]byte, 55*pcmBytesPerSecond)); err == nil || calls.Load() != before {
		t.Fatalf("canceled task contacted server: %v", err)
	}
}

func TestJoinASRChunksChineseAndEnglish(t *testing.T) {
	if got := joinASRChunks([]string{"你好", "世界。"}); got != "你好世界。" {
		t.Fatal(got)
	}
	if got := joinASRChunks([]string{"Hello", "world."}); got != "Hello world." {
		t.Fatal(got)
	}
}
