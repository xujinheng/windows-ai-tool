//go:build windows

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type ownedService struct {
	cmd      *exec.Cmd
	done     chan struct{}
	jobClose func()
	plan     servicePlan
}
type serviceManager struct {
	mu     sync.Mutex
	owned  map[string]*ownedService
	logger *log.Logger
	closed bool
}

func newServiceManager(l *log.Logger) *serviceManager {
	return &serviceManager{owned: map[string]*ownedService{}, logger: l}
}
func (m *serviceManager) stop(role string) {
	existing := m.owned[role]
	if existing == nil {
		return
	}
	delete(m.owned, role)
	if existing.cmd.Process != nil {
		_ = existing.cmd.Process.Kill()
	}
	if existing.jobClose != nil {
		existing.jobClose()
	}
	select {
	case <-existing.done:
	case <-time.After(3 * time.Second):
		m.logger.Printf("%s server: process exit still pending", role)
	}
}
func (m *serviceManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.stop("ASR")
	m.stop("LLM")
}

// Apply is always called from the UI message thread. An unchanged subprocess
// is kept alive. A new configuration starts or stops only subprocesses owned by
// VoiceRewrite; existing servers and user processes are never killed.
func (m *serviceManager) Apply(cfg Config) map[string]error {
	m.mu.Lock()
	defer m.mu.Unlock()
	failures := map[string]error{}
	if m.closed {
		return map[string]error{"system": fmt.Errorf("service manager closed")}
	}
	if err := validateServiceModes(cfg); err != nil {
		return map[string]error{"config": err}
	}
	for _, role := range []string{"ASR", "LLM"} {
		mode := cfg.ASRServiceMode
		if role == "LLM" {
			mode = cfg.LLMServiceMode
		}
		if mode != serviceLaunch {
			m.stop(role)
			continue
		}
		plan, err := servicePlanFor(cfg, role, asrExecRoot())
		if err != nil {
			m.stop(role)
			failures[role] = err
			continue
		}
		// Do not restart an identical, still-alive process when user saves settings.
		if old := m.owned[role]; old != nil && old.plan == plan {
			select {
			case <-old.done:
				m.stop(role)
			default:
				continue
			}
		}
		m.stop(role)
		addr := net.JoinHostPort(plan.Host, plan.Port)
		if asrServerAlreadyRunning(addr) {
			failures[role] = fmt.Errorf("端口 %s 已被其他进程使用。若是你启动的模型服务，请切换到连接已有服务模式", addr)
			continue
		}
		cmd := exec.Command(plan.Exe, serviceArgs(plan)...)
		cmd.Dir = filepath.Dir(plan.Exe)
		// Do not expose API tokens in the command-line argument list.
		if plan.APIKey != "" {
			cmd.Env = append(os.Environ(), "LLAMA_API_KEY="+plan.APIKey)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		// Externally supplied llama.cpp is intentionally not bundled or assumed.
		// Keep stderr to local log for diagnosing missing CUDA DLLs/model errors.
		cmd.Stdout = m.logger.Writer()
		cmd.Stderr = m.logger.Writer()
		if err := cmd.Start(); err != nil {
			failures[role] = fmt.Errorf("启动失败：%w", err)
			continue
		}
		jobClose, jobErr := attachKillOnParentExit(cmd.Process.Pid)
		if jobErr != nil {
			m.logger.Printf("%s server warning: process job guard not available: %v", role, jobErr)
		}
		owned := &ownedService{cmd: cmd, done: make(chan struct{}), plan: plan, jobClose: jobClose}
		m.owned[role] = owned
		m.logger.Printf("started managed %s llama-server pid=%d port=%s model=%s", role, cmd.Process.Pid, plan.Port, filepath.Base(plan.Model))
		go func(name string, s *ownedService) {
			err := s.cmd.Wait()
			m.logger.Printf("%s llama-server exited: %v", name, err)
			close(s.done)
		}(role, owned)
	}
	return failures
}
