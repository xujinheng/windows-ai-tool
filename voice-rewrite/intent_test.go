package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntentsRouting(t *testing.T) {
	for _, tc := range []struct {
		instruction, intent string
		editable            bool
		want                string
	}{
		{"改得简洁一点", "edit", true, "replace"},
		{"改得简洁一点", "edit", false, "popup"},
		{"这是什么意思？", "answer", true, "popup"},
		{"为什么这样写？", "edit", true, "popup"},
		{"总结这段话", "summarize", true, "popup"},
		{"翻译成中文", "translate", true, "popup"},
		{"翻译成中文并替换原文", "edit", true, "replace"},
		{"replace this with a shorter version", "edit", true, "replace"},
		{"总结这段话", "edit", true, "popup"},
	} {
		got := routeTask(AssistantResult{Intent: tc.intent, Result: "text"}, tc.instruction, tc.editable)
		if got != tc.want {
			t.Errorf("%s intent=%s editable=%t: got %s want %s", tc.instruction, tc.intent, tc.editable, got, tc.want)
		}
	}
}
func TestParseAssistantResult(t *testing.T) {
	for _, s := range []string{
		`{"intent":"answer","result":"解释"}`,
		"```json\n{\"intent\":\"answer\",\"result\":\"解释\"}\n```",
		`<think>analysis</think>{"intent":"answer","result":"解释"}`,
	} {
		got, err := parseAssistantResult(s)
		if err != nil || got.Intent != "answer" || got.Result != "解释" {
			t.Fatalf("%s: %#v %v", s, got, err)
		}
	}
	for _, s := range []string{`{"intent":"hack","result":"test"}`, `{"intent":"edit","result":""}`, `not json`} {
		if _, err := parseAssistantResult(s); err == nil {
			t.Errorf("accepted invalid %q", s)
		}
	}
}
func TestUnifiedAssistantOneCall(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"qwen3-4b-instruct"}]}`)
		case "/v1/chat/completions":
			count++
			var req struct {
				Messages []struct{ Role, Content string } `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
			}
			if len(req.Messages) != 2 || !strings.Contains(req.Messages[0].Content, "intent") || !strings.Contains(req.Messages[1].Content, "量子纠缠") || !strings.Contains(req.Messages[1].Content, "什么意思") {
				t.Errorf("bad prompt: %#v", req.Messages)
			}
			fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"intent\":\"answer\",\"result\":\"量子纠缠是...\"}"}}]}`)
		default:
			t.Errorf("wrong path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	cfg := defaults()
	cfg.LLMBaseURL = server.URL + "/v1"
	result, err := completeSelectedTask(context.Background(), cfg, "量子纠缠", "这是什么意思")
	if err != nil || result.Intent != "answer" || count != 1 {
		t.Fatalf("result=%#v err=%v count=%d", result, err, count)
	}
}
