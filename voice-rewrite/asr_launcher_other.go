//go:build !windows

package main

import "log"

func startManagedASR(cfg Config, l *log.Logger) func() { return func() {} }
