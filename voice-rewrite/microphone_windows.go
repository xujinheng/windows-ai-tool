//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

type waveInCapsW struct {
	ManufacturerID uint16
	ProductID      uint16
	DriverVersion  uint32
	ProductName    [32]uint16
	Formats        uint32
	Channels       uint16
	Reserved       uint16
}

type inputDevice struct {
	id   int
	name string
}

func availableMicrophones() []inputDevice {
	count, _, _ := procWaveInGetNumDevs.Call()
	if count > 256 {
		count = 256
	}
	result := make([]inputDevice, 0, int(count)+1)
	result = append(result, inputDevice{-1, "系统默认麦克风"})
	for i := uintptr(0); i < count; i++ {
		var caps waveInCapsW
		ok, _, _ := procWaveInGetDevCaps.Call(i, uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
		if ok != 0 {
			continue
		}
		name := syscall.UTF16ToString(caps.ProductName[:])
		if name == "" {
			name = "录音设备"
		}
		result = append(result, inputDevice{int(i), name})
	}
	return result
}

func (p *uxSettings) populateMicrophones(current int) {
	if p == nil || p.microphone == 0 {
		return
	}
	devices := availableMicrophones()
	p.microphoneIDs = make([]int, 0, len(devices))
	currentIndex := 0
	for _, device := range devices {
		label, err := syscall.UTF16PtrFromString(device.name)
		if err != nil {
			continue
		}
		item, _, _ := procSendMessage.Call(p.microphone, uxCBAddString, 0, uintptr(unsafe.Pointer(label)))
		runtime.KeepAlive(label)
		if int32(item) < 0 {
			continue
		}
		p.microphoneIDs = append(p.microphoneIDs, device.id)
		if device.id == current {
			currentIndex = len(p.microphoneIDs) - 1
		}
	}
	procSendMessage.Call(p.microphone, uxCBSetCurSel, uintptr(currentIndex), 0)
}
