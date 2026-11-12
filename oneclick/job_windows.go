//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const jobObjectExtendedLimitInformation = 9
const jobObjectLimitKillOnJobClose = 0x00002000
const processSetQuota = 0x0100
const processTerminate = 0x0001

type jobBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}
type jobIoCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}
type jobExtendedLimitInformation struct {
	BasicLimitInformation jobBasicLimitInformation
	IoInfo                jobIoCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func attachKillOnParentExit(pid int) (func(), error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	createJob := kernel.NewProc("CreateJobObjectW")
	setJob := kernel.NewProc("SetInformationJobObject")
	openProcess := kernel.NewProc("OpenProcess")
	assignJob := kernel.NewProc("AssignProcessToJobObject")
	closeHandle := kernel.NewProc("CloseHandle")
	job, _, e := createJob.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("CreateJobObjectW failed: %w", e)
	}
	cleanup := func() { closeHandle.Call(job) }
	cfg := jobExtendedLimitInformation{}
	cfg.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	ok, _, e := setJob.Call(job, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&cfg)), unsafe.Sizeof(cfg))
	if ok == 0 {
		cleanup()
		return nil, fmt.Errorf("SetInformationJobObject failed: %w", e)
	}
	h, _, e := openProcess.Call(processSetQuota|processTerminate, 0, uintptr(pid))
	if h == 0 {
		cleanup()
		return nil, fmt.Errorf("OpenProcess failed: %w", e)
	}
	defer closeHandle.Call(h)
	ok, _, e = assignJob.Call(job, h)
	if ok == 0 {
		cleanup()
		return nil, fmt.Errorf("AssignProcessToJobObject failed: %w", e)
	}
	return cleanup, nil
}
