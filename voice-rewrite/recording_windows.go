//go:build windows

package main

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type audioResult struct {
	pcm []byte
	err error
}
type audioRecorder struct {
	handle     uintptr
	hdrs       []*waveHdr
	bufs       [][]byte
	pins       []runtime.Pinner
	done       chan struct{}
	result     chan audioResult
	once       sync.Once
	maxSeconds int
	level      atomic.Uint32 // latest captured PCM RMS, scaled 0-1000
}

func winmmCheck(rc uintptr, op string) error {
	if rc != 0 {
		return fmt.Errorf("%s 失败 (WinMM 错误码 %d)，检查 Windows 麦克风权限和默认录音设备", op, rc)
	}
	return nil
}
func startRecorder(maxSec, deviceID int) (*audioRecorder, error) {
	f := waveFormatEx{FormatTag: WAVE_FORMAT_PCM, Channels: 1, SamplesPerSec: 16000, AvgBytesPerSec: 32000, BlockAlign: 2, BitsPerSample: 16}
	var h uintptr
	device := WAVE_MAPPER
	if deviceID >= 0 {
		device = uintptr(deviceID)
	}
	rc, _, _ := procWaveInOpen.Call(uintptr(unsafe.Pointer(&h)), device, uintptr(unsafe.Pointer(&f)), 0, 0, 0)
	if e := winmmCheck(rc, "waveInOpen"); e != nil {
		return nil, e
	}
	r := &audioRecorder{handle: h, done: make(chan struct{}), result: make(chan audioResult, 1), maxSeconds: maxSec}
	const bufferBytes = 6400 // 200ms at 16 kHz, mono PCM16
	const nBuffers = 8
	r.pins = make([]runtime.Pinner, nBuffers)
	for i := 0; i < nBuffers; i++ {
		b := make([]byte, bufferBytes)
		hdr := &waveHdr{Data: uintptr(unsafe.Pointer(&b[0])), BufferLength: uint32(len(b))}
		r.pins[i].Pin(&b[0])
		r.pins[i].Pin(hdr)
		r.bufs = append(r.bufs, b)
		r.hdrs = append(r.hdrs, hdr)
		rc, _, _ = procWaveInPrepareHeader.Call(h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		if e := winmmCheck(rc, "waveInPrepareHeader"); e != nil {
			r.closeHardware()
			return nil, e
		}
		rc, _, _ = procWaveInAddBuffer.Call(h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		if e := winmmCheck(rc, "waveInAddBuffer"); e != nil {
			r.closeHardware()
			return nil, e
		}
	}
	rc, _, _ = procWaveInStart.Call(h)
	if e := winmmCheck(rc, "waveInStart"); e != nil {
		r.closeHardware()
		return nil, e
	}
	go r.collect()
	return r, nil
}
func (r *audioRecorder) Stop() { r.once.Do(func() { close(r.done) }) }
func (r *audioRecorder) closeHardware() {
	procWaveInStop.Call(r.handle)
	procWaveInReset.Call(r.handle)
	for _, hdr := range r.hdrs {
		procWaveInUnprepareHeader.Call(r.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
	}
	procWaveInClose.Call(r.handle)
	for i := range r.pins {
		r.pins[i].Unpin()
	}
	runtime.KeepAlive(r.bufs)
	runtime.KeepAlive(r.hdrs)
}
func (r *audioRecorder) collect() {
	var pcm bytes.Buffer
	pending := make([]bool, len(r.hdrs))
	for i := range pending {
		pending[i] = true
	}
	tick := time.NewTicker(18 * time.Millisecond)
	defer tick.Stop()
	deadline := time.NewTimer(time.Duration(r.maxSeconds) * time.Second)
	defer deadline.Stop()
	stopped := false
	var lastError error
	for !stopped {
		select {
		case <-r.done:
			stopped = true
		case <-deadline.C:
			r.Stop()
			stopped = true
		case <-tick.C:
		}
		if stopped {
			break
		}
		for i, hdr := range r.hdrs {
			if !pending[i] || atomic.LoadUint32(&hdr.Flags)&WHDR_DONE == 0 {
				continue
			}
			n := int(atomic.LoadUint32(&hdr.BytesRecorded))
			if n > len(r.bufs[i]) {
				n = len(r.bufs[i])
			}
			if n > 0 {
				r.level.Store(pcmRMSLevel(r.bufs[i][:n]))
				pcm.Write(r.bufs[i][:n])
			}
			atomic.StoreUint32(&hdr.BytesRecorded, 0)
			// Never overwrite hdr.Flags: waveInPrepareHeader sets WHDR_PREPARED.
			// Clearing it to 0 breaks waveInAddBuffer after the first cycle.
			rc, _, _ := procWaveInAddBuffer.Call(r.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
			if rc != 0 {
				pending[i] = false
				lastError = fmt.Errorf("waveInAddBuffer 重排录音缓冲区失败 (WinMM %d)", rc)
				stopped = true
				break
			}
		}
	}
	procWaveInStop.Call(r.handle)
	procWaveInReset.Call(r.handle)
	for i, hdr := range r.hdrs {
		if !pending[i] {
			continue
		}
		n := int(atomic.LoadUint32(&hdr.BytesRecorded))
		if n > len(r.bufs[i]) {
			n = len(r.bufs[i])
		}
		if n > 0 {
			pcm.Write(r.bufs[i][:n])
		}
	}
	r.closeHardware()
	r.result <- audioResult{pcm: pcm.Bytes(), err: lastError}
}
