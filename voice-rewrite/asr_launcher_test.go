package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASRPortAndLaunchDefaults(t *testing.T) {
	c := defaults()
	if c.ASRBaseURL != "http://127.0.0.1:8080/v1" || !c.AutoStartASR {
		t.Fatalf("defaults: %+v", c)
	}
}
func TestASRLauncherPlan(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"llama-server.exe", "Qwen3-ASR-1.7B-Q8_0.gguf", "mmproj-Qwen3-ASR-1.7B-bf16.gguf"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := defaults()
	p, e := asrPlan(c, root)
	if e != nil {
		t.Fatal(e)
	}
	if p.Port != "8080" || p.Host != "127.0.0.1" {
		t.Fatal(p)
	}
	if !strings.Contains(strings.Join(asrPlanArgs(p), " "), "--port 8080") {
		t.Fatal(asrPlanArgs(p))
	}
	c.ASRBaseURL = "http://192.168.0.2:8080/v1"
	if _, err := asrPlan(c, root); err == nil {
		t.Fatal("remote automatic server launch allowed")
	}
	c = defaults()
	c.AutoStartASR = false
	if _, err := asrPlan(c, root); err == nil {
		t.Fatal("disabled auto-start allowed")
	}
}
