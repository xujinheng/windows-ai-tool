//go:build !windows

package main

import "log"

type serviceManager struct{}

func newServiceManager(l *log.Logger) *serviceManager       { return &serviceManager{} }
func (m *serviceManager) Apply(cfg Config) map[string]error { return map[string]error{} }
func (m *serviceManager) Close()                            {}
