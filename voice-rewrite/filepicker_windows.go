//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

// Native Win32 common file picker; no WebView, browser or console required.
type uxOpenFileName struct {
	Size            uint32
	Owner           uintptr
	Instance        uintptr
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustomFilter uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExt          *uint16
	CustData        uintptr
	Hook            uintptr
	TemplateName    *uint16
	Reserved        uintptr
	Reserved2       uint32
	FlagsEx         uint32
}

var uxGetOpenFileName = syscall.NewLazyDLL("comdlg32.dll").NewProc("GetOpenFileNameW")

func uxBrowseFile(owner uintptr, title, initial string) (string, bool) {
	buf := make([]uint16, 32768)
	if initial != "" {
		sample := syscall.StringToUTF16(initial)
		if len(sample) < len(buf) {
			copy(buf, sample)
		}
	}
	titleStr := syscall.StringToUTF16(title)
	filter := make([]uint16, 0, 100)
	for _, part := range []string{"支持的文件 (*.exe;*.gguf)", "*.exe;*.gguf", "所有文件 (*.*)", "*.*", ""} {
		filter = append(filter, syscall.StringToUTF16(part)...)
	}
	ofn := uxOpenFileName{Size: uint32(unsafe.Sizeof(uxOpenFileName{})), Owner: owner, Title: &titleStr[0], Filter: &filter[0], File: &buf[0], MaxFile: uint32(len(buf)), Flags: 0x00080000 | 0x00001000 | 0x00000800 | 0x00000008}
	ok, _, _ := uxGetOpenFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	runtime.KeepAlive(buf)
	runtime.KeepAlive(titleStr)
	runtime.KeepAlive(filter)
	if ok == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}
