package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOneClickLanguagePrefixAllLanguages(t *testing.T) {
	cases := []struct{ input, expected string }{
		{"language Chinese<asr_text>你好", "你好"},
		{"language English<asr_text>Hello world", "Hello world"},
		{"language German<asr_text>Hallo!", "Hallo!"},
		{"language Japanese<asr_text>こんにちは", "こんにちは"},
		{"language Chinese,English<asr_text>OK 你好", "OK 你好"},
		{"language None<asr_text>", ""},
		{"language English<asr_text>Hello language Chinese<asr_text>世界", "Hello 世界"},
		{"language Chinese<asr_text>1. language English<asr_text>2.", "1. 2."},
		{"<asr_text>plain", "plain"},
		{"A language learning example", "A language learning example"},
		{"language 正常的中文<asr_text>不删除前面的语言", "language 正常的中文<asr_text>不删除前面的语言"},
		{"I said language Chinese without a marker", "I said language Chinese without a marker"},
	}
	for _, tc := range cases {
		if got := cleanASR(tc.input); got != tc.expected {
			t.Errorf("cleanASR(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestParseASRResponseFormats(t *testing.T) {
	cases := []struct {
		body, ct, expect string
		bad              bool
	}{
		{`{"text":"language Chinese<asr_text>测试"}`, "application/json", "language Chinese<asr_text>测试", false},
		{`language English<asr_text>Hello`, "text/plain; charset=utf-8", "language English<asr_text>Hello", false},
		{`"language German<asr_text>Hallo"`, "application/json", "language German<asr_text>Hallo", false},
		{`{"text":`, "application/json", "", true},
		{`{"oops":`, "text/plain", "", true},
		{`<html>Error</html>`, "text/html", "", true},
	}
	for _, c := range cases {
		got, e := parseASRResponse([]byte(c.body), c.ct)
		if (e != nil) != c.bad || (!c.bad && got != c.expect) {
			t.Errorf("parse %q: %q / %v", c.body, got, e)
		}
	}
}
func TestConfigValuesValidation(t *testing.T) {
	cfg := defaults()
	cfg2, e := uxUpdatedConfig(cfg, [4]string{" http://127.0.0.1:8081/v1/ ", " qwen3-asr ", "http://127.0.0.1:1234/v1", ""})
	if e != nil || cfg2.ASRModel != "qwen3-asr" || cfg2.TextModel != "auto" || cfg2.ASRBaseURL != "http://127.0.0.1:8081/v1" {
		t.Fatalf("updated=%+v, error=%v", cfg2, e)
	}
	_, e = uxUpdatedConfig(cfg, [4]string{"ftp://user:pw@bad/v1", "m", "http://localhost:1234/v1", "auto"})
	if e == nil {
		t.Fatal("must reject unsupported API URL")
	}
}
func TestNativeServiceProbe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/v1/models" {
			w.WriteHeader(200)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	if got := uxProbe(context.Background(), "ASR", srv.URL+"/v1"); !strings.Contains(got, "HTTP 200") {
		t.Fatal(got)
	}
	if got := uxProbe(context.Background(), "LLM", srv.URL+"/v1"); !strings.Contains(got, "HTTP 200") {
		t.Fatal(got)
	}
}
