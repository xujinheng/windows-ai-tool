package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestConnectAndLaunchAreIndependent(t *testing.T) {
	c := defaults()
	if c.ASRServiceMode != serviceConnect || c.LLMServiceMode != serviceConnect {
		t.Fatalf("unexpected defaults: %#v", c)
	}
	c.ASRBaseURL = "https://speech.example/v1"
	c.LLMBaseURL = "https://text.example/v1"
	if err := validateServiceModes(c); err != nil {
		t.Fatal(err)
	}
	c.ASRServiceMode = serviceLaunch
	if _, err := servicePlanFor(c, "ASR", t.TempDir()); err == nil || !strings.Contains(err.Error(), "自启动") {
		t.Fatalf("must reject remote process launch: %v", err)
	}
	c.ASRServiceMode = serviceConnect
	c.LLMServiceMode = serviceLaunch
	c.LLMBaseURL = "http://127.0.0.1:8080/v1"
	if err := validateServiceModes(c); err != nil {
		t.Fatal(err)
	}
	c.ASRServiceMode = serviceLaunch
	c.ASRBaseURL = c.LLMBaseURL
	if err := validateServiceModes(c); err == nil {
		t.Fatal("two managed models cannot bind same port")
	}
}
func TestTwoIndependentLaunchPlans(t *testing.T) {
	root := t.TempDir()
	exe := fakeFile(t, root, "llama-server.exe")
	asr := fakeFile(t, root, "speech.gguf")
	proj := fakeFile(t, root, "mmproj.gguf")
	llm := fakeFile(t, root, "general model.gguf")
	c := defaults()
	c.ASRServiceMode = serviceLaunch
	c.LLMServiceMode = serviceLaunch
	c.LLAMAServerPath = exe
	c.ASRModelPath = asr
	c.ASRMMProjPath = proj
	c.LLMModelPath = llm
	c.LLMBaseURL = "http://127.0.0.1:8082/v1"
	c.ASRModel = "speech-id"
	c.TextModel = "text-id"
	if err := validateServiceModes(c); err != nil {
		t.Fatal(err)
	}
	ap, ae := servicePlanFor(c, "ASR", root)
	lp, le := servicePlanFor(c, "LLM", root)
	if ae != nil || le != nil {
		t.Fatalf("ASR:%v LLM:%v", ae, le)
	}
	if ap.Port != "8080" || lp.Port != "8082" || ap.MMProj != proj || lp.MMProj != "" || ap.Exe != lp.Exe {
		t.Fatalf("incorrect plan: %v | %v", ap, lp)
	}
	as := strings.Join(serviceArgs(ap), "|")
	ls := strings.Join(serviceArgs(lp), "|")
	for _, want := range []string{"--mmproj", proj, "--alias|speech-id"} {
		if !strings.Contains(as, want) {
			t.Errorf("missing %q in %s", want, as)
		}
	}
	if strings.Contains(ls, "--mmproj") || !strings.Contains(ls, "--port|8082") {
		t.Fatalf("text server flags: %s", ls)
	}
}
func TestLauncherValidationAndSecretHandling(t *testing.T) {
	root := t.TempDir()
	fakeFile(t, root, "llama-server.exe")
	fakeFile(t, root, "bad-model.bin")
	fakeFile(t, root, "qwen3-asr-0.6b.gguf")
	c := defaults()
	c.ASRServiceMode = serviceLaunch
	c.LLAMAServerPath = "llama-server.exe"
	c.ASRModelPath = "bad-model.bin"
	if _, err := servicePlanFor(c, "ASR", root); err == nil {
		t.Fatal("non-GGUF must be rejected")
	}
	c.ASRModelPath = "qwen3-asr-0.6b.gguf"
	if _, err := servicePlanFor(c, "ASR", root); err == nil {
		t.Fatal("Qwen ASR missing projector must be rejected")
	}
	c.ASRMMProjPath = "mmproj.gguf"
	fakeFile(t, root, "mmproj.gguf")
	c.ASRAPIKey = "super-secret"
	p, err := servicePlanFor(c, "ASR", root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(serviceArgs(p), " "), "super-secret") {
		t.Fatal("API key must never be passed on CLI")
	}
}
func TestManagedAddressSecurity(t *testing.T) {
	for _, bad := range []string{"https://localhost:8080/v1", "http://0.0.0.0:8080/v1", "http://192.168.1.10:8080/v1", "http://localhost:99999/v1", "http://user:pw@localhost:8080/v1"} {
		if _, _, err := managedAddress(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	for _, good := range []string{"http://127.0.0.1:8080/v1", "http://localhost:8082/v1"} {
		if _, _, err := managedAddress(good); err != nil {
			t.Errorf("rejected %q: %v", good, err)
		}
	}
}
