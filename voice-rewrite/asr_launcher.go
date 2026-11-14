package main

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

func asrExecRoot() string {
	p, e := os.Executable()
	if e != nil {
		return "."
	}
	return filepath.Dir(p)
}
func asrResolveFile(root, value string) string {
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, value)
}
func asrFirstFile(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if s, e := os.Stat(p); e == nil && !s.IsDir() {
			return p
		}
	}
	return ""
}
func asrServerAlreadyRunning(addr string) bool {
	c, e := net.DialTimeout("tcp", addr, 250*time.Millisecond)
	if e != nil {
		return false
	}
	_ = c.Close()
	return true
}
