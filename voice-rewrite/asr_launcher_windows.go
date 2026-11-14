//go:build windows

package main

import (
	"log"
	"net"
	"net/url"
	"os/exec"
	"syscall"
	"time"
)

// Launches ONLY an explicitly discoverable or configured local server. Never
// kills a preexisting ASR process (it may be owned by the user or another app).
func startManagedASR(cfg Config, l *log.Logger) func() {
	noop := func() {}
	if !cfg.AutoStartASR {
		return noop
	}
	u, err := url.Parse(cfg.ASRBaseURL)
	if err != nil {
		return noop
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "80"
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return noop
	}
	if asrServerAlreadyRunning(net.JoinHostPort(host, port)) {
		l.Printf("ASR: connected to existing local server on %s", u.Host)
		return noop
	}
	plan, err := asrPlan(cfg, asrExecRoot())
	if err != nil {
		l.Printf("ASR autostart unavailable: %v", err)
		return noop
	}
	cmd := exec.Command(plan.Server, asrPlanArgs(plan)...)
	cmd.Dir = asrExecRoot()
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		l.Printf("ASR autostart failed: %v", err)
		return noop
	}
	l.Printf("ASR: started bundled llama-server, pid=%d, port=%s", cmd.Process.Pid, plan.Port)
	go func() { err := cmd.Wait(); l.Printf("ASR: managed llama-server stopped: %v", err) }()
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		time.Sleep(10 * time.Millisecond)
	}
}
