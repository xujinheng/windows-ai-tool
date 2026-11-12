//go:build !windows

package main

func attachKillOnParentExit(pid int) (func(), error) { return nil, nil }
