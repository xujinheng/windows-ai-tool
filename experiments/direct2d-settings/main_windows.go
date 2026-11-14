//go:build windows

// VoiceRewrite Direct2D + DirectWrite experiment. No browser, Node, Wails,
// external Go packages or persisted configuration. Windows amd64 only.
package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type guid struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}

var iidD2DFactory = guid{0x06152247, 0x6f50, 0x465a, [8]byte{0x92, 0x45, 0x11, 0x8b, 0xfd, 0x3b, 0x60, 0x07}}
var iidDWriteFactory = guid{0xb859ee5a, 0xd838, 0x4b5b, [8]byte{0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48}}

type rect struct{ Left, Top, Right, Bottom int32 }
type point struct{ X, Y int32 }
type msg struct {
	HWND           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type wndClass struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSmall                          uintptr
}
type d2dColor struct{ R, G, B, A float32 }
type d2dRect struct{ Left, Top, Right, Bottom float32 }
type d2dRound struct {
	Rect   d2dRect
	RX, RY float32
}
type d2dMatrix struct{ M11, M12, M21, M22, DX, DY float32 }
type renderProps struct {
	Kind                   uint32
	PixelFormat, AlphaMode uint32
	DpiX, DpiY             float32
	Usage, MinLevel        uint32
}
type hwndRenderProps struct {
	HWND          uintptr
	Width, Height uint32
	Present       uint32
	_             uint32
}

const (
	wsOverlappedWindow   = 0x00cf0000
	wsVisible            = 0x10000000
	wsChild              = 0x40000000
	wsTabStop            = 0x00010000
	wsVScroll            = 0x00200000
	esAutoHScroll        = 0x0080
	esPassword           = 0x0020
	cbDropDownList       = 0x0003
	wmDestroy            = 0x0002
	wmClose              = 0x0010
	wmPaint              = 0x000f
	wmSize               = 0x0005
	wmCommand            = 0x0111
	wmLButtonDown        = 0x0201
	wmMouseMove          = 0x0200
	wmEraseBkgnd         = 0x0014
	wmCtlColorEdit       = 0x0133
	wmGetMinMaxInfo      = 0x0024
	wmKeyDown            = 0x0100
	wmSetFont            = 0x0030
	wmSetText            = 0x000c
	wmThemeChanged       = 0x031a
	swShow               = 5
	swHide               = 0
	dtCenter             = 1
	d2dErrRecreateTarget = 0x8899000c
	emSetMargins         = 0x00d3
)

var (
	user            = syscall.NewLazyDLL("user32.dll")
	gdi             = syscall.NewLazyDLL("gdi32.dll")
	kern            = syscall.NewLazyDLL("kernel32.dll")
	dwm             = syscall.NewLazyDLL("dwmapi.dll")
	d2d             = syscall.NewLazyDLL("d2d1.dll")
	dw              = syscall.NewLazyDLL("dwrite.dll")
	procRegister    = user.NewProc("RegisterClassExW")
	procCreate      = user.NewProc("CreateWindowExW")
	procDefault     = user.NewProc("DefWindowProcW")
	procGetClient   = user.NewProc("GetClientRect")
	procInvalidate  = user.NewProc("InvalidateRect")
	procMove        = user.NewProc("MoveWindow")
	procShow        = user.NewProc("ShowWindow")
	procSend        = user.NewProc("SendMessageW")
	procSetFocus    = user.NewProc("SetFocus")
	procGetText     = user.NewProc("GetWindowTextW")
	procSetText     = user.NewProc("SetWindowTextW")
	procTextLength  = user.NewProc("GetWindowTextLengthW")
	procBeginPaint  = user.NewProc("BeginPaint")
	procEndPaint    = user.NewProc("EndPaint")
	procGetMessage  = user.NewProc("GetMessageW")
	procTranslate   = user.NewProc("TranslateMessage")
	procDispatch    = user.NewProc("DispatchMessageW")
	procDialog      = user.NewProc("IsDialogMessageW")
	procPostQuit    = user.NewProc("PostQuitMessage")
	procDestroy     = user.NewProc("DestroyWindow")
	procModule      = kern.NewProc("GetModuleHandleW")
	procD2DFactory  = d2d.NewProc("D2D1CreateFactory")
	procDWFactory   = dw.NewProc("DWriteCreateFactory")
	procFont        = gdi.NewProc("CreateFontW")
	procDelete      = gdi.NewProc("DeleteObject")
	procCreateSolid = gdi.NewProc("CreateSolidBrush")
	procTextColor   = gdi.NewProc("SetTextColor")
	procBkColor     = gdi.NewProc("SetBkColor")
	procDwmAttr     = dwm.NewProc("DwmSetWindowAttribute")
	procGetDpi      = user.NewProc("GetDpiForSystem")
)

type paintStruct struct {
	HDC                uintptr
	Erase              int32
	_                  int32
	Paint              rect
	Restore, Increment int32
	Reserved           [32]byte
}
type screen struct {
	hwnd                          uintptr
	factory, writeFactory, target uintptr
	brushes                       [9]uintptr
	formats                       [5]uintptr
	edits                         [3]uintptr
	combo                         uintptr
	font                          uintptr
	editBrush                     uintptr
	tab                           int
	scale                         float32
	width, height                 float32
	dark                          bool
	changed                       bool
	updating                      bool
	values                        [3][3]string
	status                        string
}

var active *screen
var callback = syscall.NewCallback(windowProc)

// Calling a COM slot by its native vtable offset avoids any third-party library.
func com(obj uintptr, slot int, params ...uintptr) uintptr {
	if obj == 0 {
		return 0x80004003
	}
	fn := *(*uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(obj)) + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	args := make([]uintptr, 0, len(params)+1)
	args = append(args, obj)
	args = append(args, params...)
	rv, _, _ := syscall.SyscallN(fn, args...)
	return rv
}
func release(obj uintptr) {
	if obj != 0 {
		com(obj, 2)
	}
}
func utf16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func bits(n float32) uintptr { return uintptr(math.Float32bits(n)) }
func clamp(n, lo, hi float32) float32 {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
func rgb(n uint32) d2dColor {
	return d2dColor{float32((n>>16)&0xff) / 255, float32((n>>8)&0xff) / 255, float32(n&0xff) / 255, 1}
}
func bgr(n uint32) uintptr { return uintptr(((n & 0xff) << 16) | (n & 0xff00) | ((n >> 16) & 0xff)) }

// 0=background, 1=sidebar, 2=card, 3=field, 4=text, 5=muted,
// 6=border, 7=accent, 8=selection background.
func (s *screen) colors() [9]uint32 {
	if s.dark {
		return [9]uint32{0x101116, 0x16171e, 0x1b1d25, 0x242630, 0xf1f1f6, 0x9498aa, 0x30333e, 0xb2a3ff, 0x282637}
	}
	return [9]uint32{0xf8f9fb, 0xf1f2f6, 0xffffff, 0xf8f9fc, 0x242735, 0x767d8e, 0xe6e8ee, 0x6b55c7, 0xe9e5f7}
}
func (s *screen) disposeTarget() {
	for i, b := range s.brushes {
		release(b)
		s.brushes[i] = 0
	}
	release(s.target)
	s.target = 0
}
func (s *screen) close() {
	s.disposeTarget()
	for i, v := range s.formats {
		release(v)
		s.formats[i] = 0
	}
	release(s.writeFactory)
	release(s.factory)
	s.writeFactory = 0
	s.factory = 0
	if s.font != 0 {
		procDelete.Call(s.font)
		s.font = 0
	}
	if s.editBrush != 0 {
		procDelete.Call(s.editBrush)
		s.editBrush = 0
	}
}
func (s *screen) initialize() error {
	if hr, _, _ := procD2DFactory.Call(0, uintptr(unsafe.Pointer(&iidD2DFactory)), 0, uintptr(unsafe.Pointer(&s.factory))); int32(hr) < 0 {
		return fmt.Errorf("D2D1CreateFactory: 0x%x", uint32(hr))
	}
	if hr, _, _ := procDWFactory.Call(0, uintptr(unsafe.Pointer(&iidDWriteFactory)), uintptr(unsafe.Pointer(&s.writeFactory))); int32(hr) < 0 {
		return fmt.Errorf("DWriteCreateFactory: 0x%x", uint32(hr))
	}
	for i, v := range []struct {
		size   float32
		weight uint32
	}{{12, 400}, {14, 400}, {15, 500}, {26, 600}, {16, 600}} {
		fam := utf16("Microsoft YaHei UI")
		loc := utf16("zh-cn")
		hr := com(s.writeFactory, 15, uintptr(unsafe.Pointer(fam)), 0, uintptr(v.weight), 0, 5, bits(v.size), uintptr(unsafe.Pointer(loc)), uintptr(unsafe.Pointer(&s.formats[i])))
		if int32(hr) < 0 {
			return fmt.Errorf("IDWriteFactory.CreateTextFormat(%d): 0x%x", i, uint32(hr))
		}
	}
	s.font, _, _ = procFont.Call(uintptr(uint32(-int32(15*s.scale))), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(utf16("Microsoft YaHei UI"))))
	colors := s.colors()
	s.editBrush, _, _ = procCreateSolid.Call(bgr(colors[3]))
	return s.createTarget()
}
func (s *screen) createTarget() error {
	var rc rect
	procGetClient.Call(s.hwnd, uintptr(unsafe.Pointer(&rc)))
	s.width = float32(rc.Right) / s.scale
	s.height = float32(rc.Bottom) / s.scale
	props := renderProps{DpiX: 96, DpiY: 96}
	hwndProps := hwndRenderProps{HWND: s.hwnd, Width: uint32(rc.Right), Height: uint32(rc.Bottom)}
	hr := com(s.factory, 14, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&hwndProps)), uintptr(unsafe.Pointer(&s.target)))
	if int32(hr) < 0 {
		return fmt.Errorf("CreateHwndRenderTarget: 0x%x", uint32(hr))
	}
	colors := s.colors()
	for i, c := range colors {
		v := rgb(c)
		// ID2D1RenderTarget::CreateSolidColorBrush, brushProperties = NULL.
		hr = com(s.target, 8, uintptr(unsafe.Pointer(&v)), 0, uintptr(unsafe.Pointer(&s.brushes[i])))
		if int32(hr) < 0 {
			return fmt.Errorf("CreateSolidColorBrush(%d): 0x%x", i, uint32(hr))
		}
	}
	return nil
}
func (s *screen) round(x, y, w, h, r float32, fill int) {
	rr := d2dRound{d2dRect{x, y, x + w, y + h}, r, r}
	com(s.target, 19, uintptr(unsafe.Pointer(&rr)), s.brushes[fill])
}
func (s *screen) stroke(x, y, w, h, r float32, brush int) {
	rr := d2dRound{d2dRect{x, y, x + w, y + h}, r, r}
	com(s.target, 18, uintptr(unsafe.Pointer(&rr)), s.brushes[brush], bits(1), 0)
}
func (s *screen) fill(x, y, w, h float32, brush int) {
	rr := d2dRect{x, y, x + w, y + h}
	com(s.target, 17, uintptr(unsafe.Pointer(&rr)), s.brushes[brush])
}
func (s *screen) text(t string, x, y, w, h float32, size int, clr int) {
	if size < 0 || size >= len(s.formats) {
		return
	}
	u := syscall.StringToUTF16(t)
	rc := d2dRect{x, y, x + w, y + h}
	// DrawText is ID2D1RenderTarget vtable index 27, UTF-16 code unit length.
	com(s.target, 27, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), s.formats[size], uintptr(unsafe.Pointer(&rc)), s.brushes[clr], 0, 0)
	runtime.KeepAlive(u)
}
func (s *screen) draw() {
	if s.target == 0 {
		if err := s.createTarget(); err != nil {
			return
		}
	}
	// D2D drawing coordinates are logical 96-DPI points, scaled once on device.
	tr := d2dMatrix{M11: s.scale, M22: s.scale}
	com(s.target, 30, uintptr(unsafe.Pointer(&tr)))
	com(s.target, 48)
	bg := rgb(s.colors()[0])
	com(s.target, 47, uintptr(unsafe.Pointer(&bg)))
	w, h := s.width, s.height
	s.fill(0, 0, 222, h, 1)
	s.fill(221, 0, 1, h, 6)
	// Identity and slim navigation are intentionally modest, not a dashboard.
	s.round(28, 27, 33, 33, 11, 7)
	s.text("V", 37, 31, 24, 30, 4, 4)
	s.text("VoiceRewrite", 73, 29, 143, 25, 4, 4)
	s.text("偏好设置", 73, 56, 138, 18, 0, 5)
	s.text("设置", 29, 115, 160, 20, 0, 5)
	tabs := [3]string{"语音识别", "AI 文字处理", "快捷键与设备"}
	for i, label := range tabs {
		y := float32(143 + i*48)
		if s.tab == i {
			s.round(16, y, 190, 40, 9, 8)
			s.round(16, y+8, 3, 24, 1, 7)
		}
		s.text(label, 35, y+11, 166, 24, 2, func() int {
			if s.tab == i {
				return 4
			}
			return 5
		}())
	}
	s.text("纯原生窗口", 27, h-67, 176, 18, 0, 5)
	s.text("Direct2D · DirectWrite", 27, h-46, 186, 18, 0, 5)
	// Main sheet
	x := float32(262)
	maxW := w - x - 34
	if maxW < 280 {
		maxW = 280
	}
	titles := [3]string{"语音识别", "AI 文字处理", "快捷键与设备"}
	subtitles := [3]string{"连接你的语音识别服务，将语音自然转为文字。", "让每一次改写与润色，都更贴近你的表达。", "只配置你真正需要的录音快捷键和麦克风。"}
	s.text("VOICE REWRITE / SETTINGS", x, 37, maxW, 20, 0, 5)
	s.text(titles[s.tab], x, 79, maxW, 46, 3, 4)
	s.text(subtitles[s.tab], x, 132, maxW, 28, 1, 5)
	contentY := float32(192)
	bottom := h - 100
	if bottom < 435 {
		bottom = 435
	}
	s.round(x-8, contentY, maxW+12, bottom-contentY, 15, 2)
	s.stroke(x-8, contentY, maxW+12, bottom-contentY, 15, 6)
	labels := [3][3]string{{"服务地址", "模型 ID", "API 密钥"}, {"服务地址", "模型 ID", "API 密钥"}, {"录音快捷键", "麦克风", ""}}
	hints := [3][3]string{{"识别 API 的地址", "服务端使用的模型标识", "可留空"}, {"兼容 OpenAI 的文本 API", "用于改写、翻译等任务", "可留空"}, {"使用你熟悉的组合键", "选择系统输入设备", ""}}
	for row := 0; row < 3; row++ {
		if s.tab == 2 && row == 2 {
			continue
		}
		y := contentY + 20 + float32(row)*90
		if y+58 > bottom-1 {
			continue
		}
		s.text(labels[s.tab][row], x+14, y, maxW-100, 24, 2, 4)
		s.text(hints[s.tab][row], x+maxW-150, y, maxW/3, 22, 0, 5)
		s.round(x+12, y+30, maxW-30, 38, 8, 3)
		s.stroke(x+12, y+30, maxW-30, 38, 8, 6)
	}
	s.fill(x-8, h-77, maxW+12, 1, 6)
	status := s.status
	if status == "" {
		status = "更改仅在此预览中生效 · 不会写入原配置"
	}
	s.text(status, x, h-60, maxW-158, 30, 0, 5)
	saveW := float32(112)
	saveX := w - saveW - 47
	s.round(saveX, h-68, saveW, 36, 9, 7)
	s.text("保存设置", saveX+17, h-60, 95, 23, 2, 2)
	// Focus region: small text underneath the render target layout.
	if s.changed {
		s.text("● 预览已修改", x, h-92, 180, 18, 0, 7)
	}
	hr := com(s.target, 49, 0, 0)
	if uint32(hr) == d2dErrRecreateTarget {
		s.disposeTarget()
	}
}
func (s *screen) captureValues() {
	for i, e := range s.edits {
		if e != 0 {
			s.values[s.tab][i] = readWindowText(e)
		}
	}
	if s.tab == 2 && s.combo != 0 {
		sel, _, _ := procSend.Call(s.combo, 0x0147, 0, 0)
		if sel == 1 {
			s.values[2][1] = "麦克风阵列"
		} else {
			s.values[2][1] = "系统默认"
		}
	}
}
func readWindowText(hwnd uintptr) string {
	n, _, _ := procTextLength.Call(hwnd)
	buf := make([]uint16, n+1)
	if len(buf) == 0 {
		return ""
	}
	procGetText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func (s *screen) applyTab(tab int) {
	if tab < 0 || tab > 2 || tab == s.tab {
		return
	}
	s.captureValues()
	s.tab = tab
	s.layoutControls()
	procInvalidate.Call(s.hwnd, 0, 0)
}
func (s *screen) layoutControls() {
	s.updating = true
	defer func() { s.updating = false }()
	x := int32(274 * s.scale)
	width := int32((s.width - 262 - 34 - 30) * s.scale)
	if width < 100 {
		width = 100
	}
	for i, e := range s.edits {
		if e == 0 {
			continue
		}
		if s.tab == 2 && i >= 1 {
			procShow.Call(e, swHide)
			continue
		}
		y := int32((242 + float32(i)*90) * s.scale)
		procMove.Call(e, uintptr(x+8), uintptr(y), uintptr(width-16), uintptr(29*s.scale), 1)
		procSetText.Call(e, uintptr(unsafe.Pointer(utf16(s.values[s.tab][i]))))
		procShow.Call(e, swShow)
	}
	if s.combo != 0 {
		if s.tab == 2 {
			y := int32(332 * s.scale)
			procMove.Call(s.combo, uintptr(x+8), uintptr(y), uintptr(width-16), uintptr(170*s.scale), 1)
			procSend.Call(s.combo, 0x014e, 0, 0)
			if s.values[2][1] == "麦克风阵列" {
				procSend.Call(s.combo, 0x014e, 1, 0)
			}
			procShow.Call(s.combo, swShow)
		} else {
			procShow.Call(s.combo, swHide)
		}
	}
}
func (s *screen) resize() {
	var rc rect
	procGetClient.Call(s.hwnd, uintptr(unsafe.Pointer(&rc)))
	s.width = float32(rc.Right) / s.scale
	s.height = float32(rc.Bottom) / s.scale
	if s.target != 0 {
		size := [2]uint32{uint32(rc.Right), uint32(rc.Bottom)}
		hr := com(s.target, 58, uintptr(unsafe.Pointer(&size[0])))
		if int32(hr) < 0 {
			s.disposeTarget()
		}
	}
	s.layoutControls()
}
func windowProc(hwnd, message, wParam, lParam uintptr) uintptr {
	s := active
	switch uint32(message) {
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		var p paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&p)))
		if hdc != 0 && s != nil && s.hwnd == hwnd {
			s.draw()
		}
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&p)))
		return 0
	case wmSize:
		if s != nil && s.hwnd == hwnd {
			s.resize()
			procInvalidate.Call(hwnd, 0, 0)
		}
		return 0
	case wmLButtonDown:
		if s != nil && s.hwnd == hwnd {
			x := float32(int16(lParam&0xffff)) / s.scale
			y := float32(int16((lParam>>16)&0xffff)) / s.scale
			if x >= 15 && x < 206 && y >= 143 && y < 279 {
				s.applyTab(int((y - 143) / 48))
				return 0
			}
			if x >= s.width-159 && x < s.width-47 && y >= s.height-68 && y < s.height-32 {
				s.captureValues()
				s.changed = false
				s.status = "演示已完成 · 没有写入任何设置"
				procInvalidate.Call(hwnd, 0, 0)
				return 0
			}
		}
	case wmKeyDown:
		if s != nil && s.hwnd == hwnd {
			if wParam == 0x1b {
				procDestroy.Call(hwnd)
				return 0
			}
			if wParam == '1' || wParam == '2' || wParam == '3' {
				s.applyTab(int(wParam - '1'))
				return 0
			}
		}
	case wmCommand:
		if s != nil && s.hwnd == hwnd && !s.updating && ((wParam>>16) == 0x0300 || (s.tab == 2 && (wParam>>16) == 1)) {
			s.changed = true
			s.status = "设置有未保存的演示修改"
			procInvalidate.Call(hwnd, 0, 0)
		}
	case wmCtlColorEdit:
		if s != nil && s.hwnd == hwnd {
			procTextColor.Call(wParam, bgr(s.colors()[4]))
			procBkColor.Call(wParam, bgr(s.colors()[3]))
			return s.editBrush
		}
	case wmClose:
		procDestroy.Call(hwnd)
		return 0
	case wmDestroy:
		if s != nil && s.hwnd == hwnd {
			s.close()
			active = nil
		}
		procPostQuit.Call(0)
		return 0
	}
	rv, _, _ := procDefault.Call(hwnd, message, wParam, lParam)
	return rv
}
func makeChild(s *screen, class, text string, style uintptr, x, y, w, h int32) uintptr {
	classPtr := utf16(class)
	valuePtr := utf16(text)
	instance, _, _ := procModule.Call(0)
	c, _, _ := procCreate.Call(0, uintptr(unsafe.Pointer(classPtr)), uintptr(unsafe.Pointer(valuePtr)), style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), s.hwnd, 0, instance, 0)
	if c != 0 && s.font != 0 {
		procSend.Call(c, wmSetFont, s.font, 1)
	}
	return c
}
func run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// The demo uses real Win32 controls; no browser engine is loaded.
	user.NewProc("SetProcessDPIAware").Call()
	scale := float32(1)
	if z := os.Getenv("VOICEREWRITE_D2D_SCALE"); z != "" {
		if n, err := strconv.ParseFloat(z, 32); err == nil {
			scale = clamp(float32(n), 1, 2)
		}
	} else {
		if dpi, _, _ := procGetDpi.Call(); dpi >= 96 && dpi <= 288 {
			scale = float32(dpi) / 96
		}
	}
	dark := os.Getenv("VOICEREWRITE_D2D_THEME") != "light"
	s := &screen{dark: dark, scale: scale, values: [3][3]string{{"http://127.0.0.1:8000/v1", "qwen3-asr", ""}, {"http://127.0.0.1:11434/v1", "qwen3:8b", ""}, {"Ctrl + Shift + Space", "系统默认", ""}}}
	className := utf16("VoiceRewrite.Direct2D.Settings.v1")
	instance, _, _ := procModule.Call(0)
	wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), WndProc: callback, Instance: instance, ClassName: className}
	if ok, _, e := procRegister.Call(uintptr(unsafe.Pointer(&wc))); ok == 0 {
		return fmt.Errorf("RegisterClassEx: %v", e)
	}
	title := utf16("VoiceRewrite · Direct2D 原生设置预览")
	width, height := int32(820*scale), int32(618*scale)
	hwnd, _, e := procCreate.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), wsOverlappedWindow,
		uintptr(int32(210)), uintptr(int32(80)), uintptr(width), uintptr(height), 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %v", e)
	}
	s.hwnd = hwnd
	active = s
	s.updating = true // suppress native EN_CHANGE during control initialization
	if err := s.initialize(); err != nil {
		procDestroy.Call(hwnd)
		return err
	}
	if s.dark {
		flag := uint32(1)
		procDwmAttr.Call(hwnd, 20, uintptr(unsafe.Pointer(&flag)), 4)
	}
	editStyle := uintptr(wsVisible | wsChild | wsTabStop | esAutoHScroll)
	for i := range s.edits {
		extra := uintptr(0)
		if i == 2 {
			extra = esPassword
		}
		s.edits[i] = makeChild(s, "EDIT", s.values[0][i], editStyle|extra, 0, 0, 0, 0)
		if s.edits[i] != 0 {
			procSend.Call(s.edits[i], emSetMargins, 3, uintptr(6|(6<<16)))
		}
	}
	s.combo = makeChild(s, "COMBOBOX", "", wsChild|wsTabStop|cbDropDownList|wsVScroll, 0, 0, 0, 0)
	if s.combo != 0 {
		procSend.Call(s.combo, 0x0143, 0, uintptr(unsafe.Pointer(utf16("系统默认"))))
		procSend.Call(s.combo, 0x0143, 0, uintptr(unsafe.Pointer(utf16("麦克风阵列（示例）"))))
	}
	s.resize()
	s.updating = false
	procShow.Call(hwnd, swShow)
	user.NewProc("UpdateWindow").Call(hwnd)
	for {
		var m msg
		rv, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(rv) <= 0 {
			break
		}
		if handled, _, _ := procDialog.Call(hwnd, uintptr(unsafe.Pointer(&m))); handled != 0 {
			continue
		}
		procTranslate.Call(uintptr(unsafe.Pointer(&m)))
		procDispatch.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Avoid claiming that this experimental renderer changes any live config.
var _ = strings.TrimSpace
var _ = dtCenter
var _ = wmThemeChanged
var _ = wmGetMinMaxInfo
var _ = wmMouseMove
var _ = wmSetText
var _ = procSetFocus