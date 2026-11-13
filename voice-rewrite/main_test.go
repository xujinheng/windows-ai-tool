package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCleanASR(t *testing.T) {
	cases := []struct{ in, want string }{
		{"language Chinese<asr_text>帮我改得简洁一点", "帮我改得简洁一点"},
		{"language English<asr_text>Make this shorter.", "Make this shorter."},
		{"language Japanese<asr_text>短くしてください。", "短くしてください。"},
		{"language English<asr_text>Hello. language Chinese<asr_text>你好。", "Hello. 你好。"},
		{"这是一段正常文字", "这是一段正常文字"},
		{"language Chinese<asr_text>", ""},
	}
	for _, tc := range cases {
		if got := cleanASR(tc.in); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}
func TestWAV(t *testing.T) {
	pcm := make([]byte, 32000)
	wav, e := encodeWAV(pcm)
	if e != nil {
		t.Fatal(e)
	}
	if string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || string(wav[36:40]) != "data" {
		t.Fatalf("invalid WAV headers %q", wav[:12])
	}
	if binary.LittleEndian.Uint32(wav[24:28]) != 16000 {
		t.Fatal("sample rate incorrect")
	}
	if len(wav) != 32044 {
		t.Fatal("len", len(wav))
	}
}
func TestAudioAndLLMWorkflow(t *testing.T) {
	asr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("bad route %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Error("bad method")
		}
		if e := r.ParseMultipartForm(3 << 20); e != nil {
			t.Errorf("bad multipart: %v", e)
			return
		}
		if r.FormValue("model") != "whisper-1" {
			t.Error("ASR model parameter incorrect")
		}
		if r.FormValue("response_format") != "json" {
			t.Error("response format incorrect")
		}
		f, _, e := r.FormFile("file")
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		if string(b[:4]) != "RIFF" {
			t.Error("no WAV header")
		}
		fmt.Fprint(w, `{"text":"language Chinese<asr_text>请把这句话改得简洁一点。"}`)
	}))
	defer asr.Close()
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"qwen3-asr-1.7b"},{"id":"qwen3-4b-instruct"}]}`)
		case "/v1/chat/completions":
			var b struct {
				Model    string `json:"model"`
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
				t.Error(e)
			}
			if b.Model != "qwen3-4b-instruct" {
				t.Errorf("wrong model %s", b.Model)
			}
			if !strings.Contains(b.Messages[1].Content, "请把这句话改得简洁一点") {
				t.Error("instruction not sent")
			}
			if !strings.Contains(b.Messages[1].Content, "我觉得这个产品的功能其实是比较不错的") {
				t.Error("selected text not sent")
			}
			fmt.Fprint(w, `{"choices":[{"message":{"content":"这个产品功能不错。"}}]}`)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer llm.Close()
	cfg := defaults()
	cfg.ASRBaseURL = asr.URL + "/v1"
	cfg.LLMBaseURL = llm.URL + "/v1"
	instruction, e := transcribe(context.Background(), cfg, make([]byte, 32000))
	if e != nil {
		t.Fatal(e)
	}
	if instruction != "请把这句话改得简洁一点。" {
		t.Errorf("wrong instruction: %q", instruction)
	}
	result, e := rewrite(context.Background(), cfg, "我觉得这个产品的功能其实是比较不错的。", instruction)
	if e != nil {
		t.Fatal(e)
	}
	if result != "这个产品功能不错。" {
		t.Errorf("wrong result: %q", result)
	}
}
func TestNoASRAsTextModel(t *testing.T) {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"Qwen3-ASR-1.7B-Q8_0"},{"id":"text-embedding-3-small"}]}`)
	}))
	defer llm.Close()
	cfg := defaults()
	cfg.LLMBaseURL = llm.URL + "/v1"
	_, e := discoverModel(context.Background(), cfg)
	if e == nil {
		t.Fatal("must not use ASR or embeddings as text editing model")
	}
}
func TestThinkingTag(t *testing.T) {
	s := thinkingTag.ReplaceAllString("<think>分析...</think>更简洁。", "")
	if s != "更简洁。" {
		t.Fatal(s)
	}
}

func TestHoldReleaseDebounce(t *testing.T) {
	polls := 0
	for _, pressed := range []bool{true, true, false, true, true, false} {
		if releaseDebounced(pressed, &polls) { t.Fatalf("premature release") }
	}
	if !releaseDebounced(false, &polls) { t.Fatal("release not detected") }
}

func TestDefaultMouse5Hold(t *testing.T) {
	cfg := defaults()
	if cfg.RecordMode != "hold" || cfg.HotkeyRecord != "Mouse5" {
		t.Fatalf("wrong mouse-first defaults: mode=%q, hotkey=%q", cfg.RecordMode, cfg.HotkeyRecord)
	}
}

func TestMouseButtonMapping(t *testing.T) {
	for _, tc := range []struct{ text string; want uint16; ok bool }{
		{"Mouse4", 1, true}, {"xbutton1", 1, true},
		{"mOuSe5", 2, true}, {"XBUTTON2", 2, true},
		{" Mouse5 ", 2, true}, {"F9", 0, false},
		{"", 0, false}, {"Mouse6", 0, false},
	} {
		got, ok := mouseSideButton(tc.text)
		if got != tc.want || ok != tc.ok {
			t.Errorf("mouseSideButton(%q)=%d,%t; want %d,%t", tc.text, got, ok, tc.want, tc.ok)
		}
	}
}
