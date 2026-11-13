//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Windows notification-area (system tray) indicator. The icon is painted in
// memory; no icon files, third-party dependencies, or background services.
const (
	WM_TRAY_NOTIFY      = 0x8000 + 47 // WM_APP + 47
	WM_TRAY_UPDATE      = 0x8000 + 48
	WM_TRAY_EXIT        = 0x8000 + 49
	TRAY_RESET_TIMER_ID = 3002
	WM_RBUTTONUP        = 0x0205
	WM_CONTEXTMENU      = 0x007B
	NIM_ADD             = 0
	NIM_MODIFY          = 1
	NIM_DELETE          = 2
	NIF_MESSAGE         = 1
	NIF_ICON            = 2
	NIF_TIP             = 4
	TPM_RETURNCMD       = 0x0100
	TPM_RIGHTBUTTON     = 0x0002
	MF_STRING           = 0
	DIB_RGB_COLORS      = 0
	BI_RGB              = 0
	TRAY_ICON_ID        = 1
)

type trayState int32

const (
	trayIdle trayState = iota
	trayRecording
	trayTranscribing
	trayRewriting
	traySuccess
	trayError
)

type trayStateInfo struct {
	label string
	color uint32 // 0xRRGGBB
}

func trayInfo(s trayState) trayStateInfo {
	switch s {
	case trayRecording:
		return trayStateInfo{"正在录音 · 按键松开后提交", 0xe04444}
	case trayTranscribing:
		return trayStateInfo{"正在识别语音", 0xe7ae39}
	case trayRewriting:
		return trayStateInfo{"正在改写文字", 0x3987d9}
	case traySuccess:
		return trayStateInfo{"改写成功", 0x29a568}
	case trayError:
		return trayStateInfo{"操作失败 · 查看错误提示", 0xed8438}
	default:
		return trayStateInfo{"待机 · 选中文字后按录音快捷键", 0x7c8794}
	}
}

type notifyIconData struct {
	Size            uint32
	Hwnd            uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Timeout         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            [16]byte
	BalloonIcon     uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type iconInfo struct {
	IsIcon   int32
	HotspotX uint32
	HotspotY uint32
	Mask     uintptr
	Color    uintptr
}
type trayPoint struct{ X, Y int32 }

var (
	shell32                   = syscall.NewLazyDLL("shell32.dll")
	gdi32                     = syscall.NewLazyDLL("gdi32.dll")
	procShellNotifyIcon       = shell32.NewProc("Shell_NotifyIconW")
	procCreateDIBSection      = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap          = gdi32.NewProc("CreateBitmap")
	procDeleteObject          = gdi32.NewProc("DeleteObject")
	procCreateIconIndirect    = user32.NewProc("CreateIconIndirect")
	procDestroyIcon           = user32.NewProc("DestroyIcon")
	procPostMessage           = user32.NewProc("PostMessageW")
	procCreatePopupMenu       = user32.NewProc("CreatePopupMenu")
	procAppendMenu            = user32.NewProc("AppendMenuW")
	procTrackPopupMenu        = user32.NewProc("TrackPopupMenu")
	procDestroyMenu           = user32.NewProc("DestroyMenu")
	procGetCursorPos          = user32.NewProc("GetCursorPos")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
)

type trayIndicator struct {
	hwnd           uintptr
	icons          [6]uintptr
	taskbarCreated uint32
	created        bool
}

func colorIcon(rgb uint32) (uintptr, error) {
	const n = 32
	bmi := bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: n, Height: -n, Planes: 1, BitCount: 32, Compression: BI_RGB}
	var bits unsafe.Pointer
	colorBmp, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if colorBmp == 0 || bits == nil {
		return 0, errors.New("CreateDIBSection failed")
	}
	defer procDeleteObject.Call(colorBmp)
	pixels := unsafe.Slice((*uint32)(bits), n*n)
	maskBits := make([]byte, n*n/8) // 4 bytes/row, 1bpp
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			d2 := dx*dx + dy*dy
			if d2 > 14.7*14.7 {
				maskBits[y*(n/8)+x/8] |= 0x80 >> uint(x%8)
				pixels[y*n+x] = 0
				continue
			}
			c := rgb
			if d2 > 13.1*13.1 {
				c = 0x29323b
			}
			// Simple white microphone silhouette: rounded capsule, cradle, stem.
			capsule := (x >= 13 && x <= 18 && y >= 8 && y <= 18) || ((y == 7 || y == 19) && x >= 14 && x <= 17)
			cradle := (y >= 17 && y <= 23 && (x == 10 || x == 21)) || (y == 24 && x >= 12 && x <= 19) || (y == 23 && (x == 11 || x == 20))
			stem := (x == 15 || x == 16) && (y >= 24 && y <= 27)
			stand := (y == 28 && x >= 12 && x <= 19)
			if capsule || cradle || stem || stand {
				c = 0xffffff
			}
			pixels[y*n+x] = 0xff000000 | c
		}
	}
	maskBmp, _, _ := procCreateBitmap.Call(n, n, 1, 1, uintptr(unsafe.Pointer(&maskBits[0])))
	runtime.KeepAlive(maskBits)
	if maskBmp == 0 {
		return 0, errors.New("CreateBitmap failed")
	}
	defer procDeleteObject.Call(maskBmp)
	ii := iconInfo{IsIcon: 1, Mask: maskBmp, Color: colorBmp}
	h, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	runtime.KeepAlive(ii)
	if h == 0 {
		return 0, errors.New("CreateIconIndirect failed")
	}
	return h, nil
}

func newTrayIndicator(hwnd uintptr) (*trayIndicator, error) {
	t := &trayIndicator{hwnd: hwnd}
	for i := range t.icons {
		h, e := colorIcon(trayInfo(trayState(i)).color)
		if e != nil {
			t.close()
			return nil, fmt.Errorf("tray icon: %w", e)
		}
		t.icons[i] = h
	}
	taskbarName, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	n, _, _ := procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(taskbarName)))
	runtime.KeepAlive(taskbarName)
	t.taskbarCreated = uint32(n)
	if e := t.notify(trayIdle, true); e != nil {
		t.close()
		return nil, e
	}
	return t, nil
}
func (t *trayIndicator) notify(status trayState, add bool) error {
	if t == nil {
		return nil
	}
	if status < trayIdle || status > trayError {
		status = trayIdle
	}
	data := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Hwnd: t.hwnd, ID: TRAY_ICON_ID, Flags: NIF_MESSAGE | NIF_ICON | NIF_TIP, CallbackMessage: WM_TRAY_NOTIFY, Icon: t.icons[status]}
	tip := syscall.StringToUTF16("VoiceRewrite - " + trayInfo(status).label)
	copy(data.Tip[:], tip)
	op := uintptr(NIM_MODIFY)
	if add {
		op = NIM_ADD
	}
	ok, _, _ := procShellNotifyIcon.Call(op, uintptr(unsafe.Pointer(&data)))
	runtime.KeepAlive(data)
	if ok == 0 {
		return errors.New("Shell_NotifyIconW failed")
	}
	t.created = true
	return nil
}
func (t *trayIndicator) close() {
	if t == nil {
		return
	}
	if t.created {
		d := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Hwnd: t.hwnd, ID: TRAY_ICON_ID}
		procShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&d)))
		t.created = false
	}
	for i, h := range t.icons {
		if h != 0 {
			procDestroyIcon.Call(h)
			t.icons[i] = 0
		}
	}
}
func (t *trayIndicator) contextMenu() {
	m, _, _ := procCreatePopupMenu.Call()
	if m == 0 {
		return
	}
	defer procDestroyMenu.Call(m)
	label, _ := syscall.UTF16PtrFromString("退出 VoiceRewrite")
	procAppendMenu.Call(m, MF_STRING, 1, uintptr(unsafe.Pointer(label)))
	runtime.KeepAlive(label)
	var p trayPoint
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p))); ok == 0 {
		return
	}
	procSetForegroundWindow.Call(t.hwnd)
	selected, _, _ := procTrackPopupMenu.Call(m, TPM_RETURNCMD|TPM_RIGHTBUTTON, int32ptr(p.X), int32ptr(p.Y), 0, t.hwnd, 0)
	if selected == 1 {
		procPostMessage.Call(t.hwnd, WM_TRAY_EXIT, 0, 0)
	}
}
func int32ptr(v int32) uintptr { return uintptr(uint32(v)) }