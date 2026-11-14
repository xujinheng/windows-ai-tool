//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// A small, native settings surface; it does not host a browser or intercept
// the editor focus until deliberately opened from the tray menu.
const (
	uxWMCommand       = 0x0111
	uxWMSize          = 0x0005
	uxWMVScroll       = 0x0115
	uxWMHScroll       = 0x0114
	uxWMMouseWheel    = 0x020A
	uxSIFRange        = 1
	uxSIFPage         = 2
	uxSIFPos          = 4
	uxSIFTrackPos     = 16
	uxSBHorz          = 0
	uxSBVert          = 1
	uxWMGetMinMaxInfo = 0x0024
	uxWSThickFrame    = 0x00040000
	uxWMSettingChange = 0x001A
	uxWMThemeChanged  = 0x031A
	uxWMClose         = 0x0010
	uxWMEraseBkgnd    = 0x0014
	uxWMPaint         = 0x000F
	uxWMNCDestroy     = 0x0082
	uxWMColorStatic   = 0x0138
	uxWMColorEdit     = 0x0133
	uxWMDrawItem      = 0x002B
	uxBSOwnerDraw     = 0x000B
	uxEMSetMargins    = 0x00D3
	uxWMProbeDone     = 0x8000 + 104
	uxSaveButton      = 9101
	uxCloseButton     = 9102
	uxAdvancedButton  = 9103
	uxProbeASRButton  = 9104
	uxProbeLLMButton  = 9105
	uxShortcutInput   = 9106
	uxMicrophoneInput = 9107
	uxCBAddString     = 0x0143
	uxCBSetCurSel     = 0x014E
	uxCBGetCurSel     = 0x0147
	uxCBSDropDownList = 0x0003
	uxWSChild         = 0x40000000
	uxWSVisible       = 0x10000000
	uxWSTabStop       = 0x00010000
	uxESAutoScroll    = 0x0080
	uxESPassword      = 0x0020
	uxWSBorder        = 0x00800000
	uxWSCaption       = 0x00C00000
	uxWSSysMenu       = 0x00080000
	uxWSMinimizeBox   = 0x00020000
)

var (
	uxGetWindowText  = user32.NewProc("GetWindowTextW")
	uxSetTextColor   = gdi32.NewProc("SetTextColor")
	uxSelectObject   = gdi32.NewProc("SelectObject")
	uxRoundRect      = gdi32.NewProc("RoundRect")
	uxGetStockObject = gdi32.NewProc("GetStockObject")
	uxSetBkMode      = gdi32.NewProc("SetBkMode")
	uxWindowCallback = syscall.NewCallback(uxSettingsWndProc)
	uxActiveSettings *uxSettings
)

type uxSettings struct {
	window                                  uintptr
	app                                     *appState
	inputs                                  [6]uintptr
	shortcut                                uintptr
	microphone                              uintptr
	microphoneIDs                           []int
	hint                                    uintptr
	font, titleFont, sectionFont, mutedFont uintptr
	background                              uintptr
	cardBrush                               uintptr
	editBrush                               uintptr
	palette                                 uiPalette
	cardStatics                             map[uintptr]bool
	probeMu                                 sync.Mutex
	probeResult                             [2]string
	probeStatus                             [2]uintptr
	probeSerial                             [2]uint64
	controls                                []uxControl
	scale                                   float64
	scrollX, scrollY                        int32
	renderer                                *resultRenderer
}

// Every child control is reflowed after a resize. No hard-coded physical
// coordinates may escape uxLayoutRect; the data is in 96-DPI logical units.
type uxControl struct {
	hwnd       uintptr
	x, y, w, h int32
	stretch    bool
	right      bool
	id         uintptr
}

func uxLayoutRect(c uxControl, clientWidth int32) (x, y, w, h int32) {
	return layoutSettingsRect(uxLayoutSpec{x: c.x, y: c.y, w: c.w, h: c.h, stretch: c.stretch, right: c.right}, clientWidth)
}

// 96-DPI logical coordinate scrolling avoids inaccessible buttons on small
// work areas and 125%–200% display scaling.
const uxContentWidth, uxContentHeight int32 = 676, 520

type uxScrollInfo struct {
	Size, Mask    uint32
	Min, Max      int32
	Page          uint32
	Pos, TrackPos int32
}

func uxScrollClamp(n, limit int32) int32 {
	if n < 0 {
		return 0
	}
	if n > limit {
		return limit
	}
	return n
}
func (p *uxSettings) layout() {
	if p == nil || p.window == 0 {
		return
	}
	var rect rect32
	procGetClientRect.Call(p.window, uintptr(unsafe.Pointer(&rect)))
	viewW, viewH := int32(float64(rect.Right-rect.Left)/p.scale), int32(float64(rect.Bottom-rect.Top)/p.scale)
	if viewW < 1 {
		viewW = 1
	}
	if viewH < 1 {
		viewH = 1
	}
	maxX, maxY := uxContentWidth-viewW, uxContentHeight-viewH
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	p.scrollX = uxScrollClamp(p.scrollX, maxX)
	p.scrollY = uxScrollClamp(p.scrollY, maxY)
	for _, spec := range []struct {
		axis               uintptr
		content, page, pos int32
	}{
		{uxSBHorz, uxContentWidth, viewW, p.scrollX},
		{uxSBVert, uxContentHeight, viewH, p.scrollY},
	} {
		info := uxScrollInfo{Size: uint32(unsafe.Sizeof(uxScrollInfo{})), Mask: uxSIFRange | uxSIFPage | uxSIFPos,
			Min: 0, Max: spec.content - 1, Page: uint32(spec.page), Pos: spec.pos}
		procSetScrollInfoUX.Call(p.window, spec.axis, uintptr(unsafe.Pointer(&info)), 1)
	}
	contentW := viewW
	if contentW < uxContentWidth {
		contentW = uxContentWidth
	}
	sc := func(v int32) uintptr { return uintptr(int32(float64(v)*p.scale + 0.5)) }
	for _, ctrl := range p.controls {
		x, y, w, h := uxLayoutRect(ctrl, contentW)
		x -= p.scrollX
		// Keep the primary footer actions horizontally accessible even when
		// the form itself needs horizontal scrolling at high DPI.
		switch ctrl.id {
		case uxSaveButton:
			x = 16
		case uxAdvancedButton:
			x = 172
		case uxCloseButton:
			x = viewW - w - 16
			if x < 380 {
				x = 380
			}
		}
		if ctrl.hwnd == p.hint {
			x = 18
			if w > viewW-34 {
				w = viewW - 34
			}
		}
		procMoveWindow.Call(ctrl.hwnd, sc(x), sc(y-p.scrollY), sc(w), sc(h), 1)
	}
	procInvalidateRectSettings.Call(p.window, 0, 1)
}
func (p *uxSettings) scroll(axis, request uintptr) {
	var rect rect32
	procGetClientRect.Call(p.window, uintptr(unsafe.Pointer(&rect)))
	view := int32(float64(rect.Bottom-rect.Top) / p.scale)
	content := uxContentHeight
	pos := &p.scrollY
	if axis == uxSBHorz {
		view = int32(float64(rect.Right-rect.Left) / p.scale)
		content = uxContentWidth
		pos = &p.scrollX
	}
	max := content - view
	if max < 0 {
		max = 0
	}
	var delta int32
	switch request & 0xffff {
	case 0:
		delta = -24
	case 1:
		delta = 24
	case 2:
		delta = -view + 48
	case 3:
		delta = view - 48
	case 6:
		*pos = 0
	case 7:
		*pos = max
	case 4, 5:
		info := uxScrollInfo{Size: uint32(unsafe.Sizeof(uxScrollInfo{})), Mask: uxSIFTrackPos}
		procGetScrollInfoUX.Call(p.window, axis, uintptr(unsafe.Pointer(&info)))
		*pos = info.TrackPos
	}
	*pos = uxScrollClamp(*pos+delta, max)
	p.layout()
}

var (
	procMoveWindow             = user32.NewProc("MoveWindow")
	procSetScrollInfoUX        = user32.NewProc("SetScrollInfo")
	procGetScrollInfoUX        = user32.NewProc("GetScrollInfo")
	procInvalidateRectSettings = user32.NewProc("InvalidateRect")
)

type uxMinMaxInfo struct{ Reserved, MaxSize, MaxPos, MinSize, MaxTrack trayPoint }

func uxPtr(s string) *uint16 { ptr, _ := syscall.UTF16PtrFromString(s); return ptr }

func uxWinText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLength.Call(hwnd)
	if n > 4096 {
		n = 4096
	}
	buf := make([]uint16, n+1)
	uxGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}
func uxSetText(hwnd uintptr, text string) {
	if hwnd == 0 {
		return
	}
	ptr := uxPtr(text)
	procSetWindowText.Call(hwnd, uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(ptr)
}

func uxSettingsKeyMessage(msg winMSG) bool {
	p := uxActiveSettings
	if p == nil || p.window == 0 {
		return false
	}
	if msg.Message != WM_KEYDOWN || msg.WParam != VK_ESCAPE {
		return false
	}
	if msg.Hwnd != p.window {
		child, _, _ := procIsChild.Call(p.window, msg.Hwnd)
		if child == 0 {
			return false
		}
	}
	procDestroyWindow.Call(p.window)
	return true
}

func (p *uxSettings) refreshTheme() {
	if p == nil || p.window == 0 {
		return
	}
	palette := resultCardPalette()
	if palette == p.palette {
		return
	}
	p.background = refreshPaletteBrush(p.background, palette, palette.background)
	p.cardBrush = refreshPaletteBrush(p.cardBrush, palette, palette.surface)
	p.editBrush = refreshPaletteBrush(p.editBrush, palette, palette.field)
	p.palette = palette
	if p.renderer != nil {
		p.renderer.updateColors(palette.dark)
	}
	for _, edit := range p.inputs {
		resultStyleNativeEdit(edit, palette.dark)
	}
	resultStyleNativeEdit(p.microphone, palette.dark)
	applyWindowPalette(p.window, palette)
	refreshNativeWindow(p.window)
	for _, c := range p.controls {
		refreshNativeWindow(c.hwnd)
	}
}

func uxSettingsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	p := uxActiveSettings
	switch uint32(msg) {
	case uxWMSettingChange, uxWMThemeChanged:
		if p != nil && p.window == hwnd {
			p.refreshTheme()
		}
		return 0
	case uxWMVScroll, uxWMHScroll:
		if p != nil && p.window == hwnd {
			axis := uintptr(uxSBVert)
			if uint32(msg) == uxWMHScroll {
				axis = uxSBHorz
			}
			p.scroll(axis, wParam)
		}
		return 0
	case uxWMMouseWheel:
		if p != nil && p.window == hwnd {
			delta := int32(int16((wParam >> 16) & 0xffff))
			p.scrollY -= delta / 120 * 72
			p.layout()
		}
		return 0
	case uxWMSize:
		if p != nil && p.window == hwnd {
			if p.renderer != nil {
				p.renderer.resized()
			}
			p.layout()
		}
		return 0
	case uxWMGetMinMaxInfo:
		if p != nil && lParam != 0 {
			bounds := (*uxMinMaxInfo)(unsafe.Pointer(lParam))
			work := uxWorkAreaAtCursor()
			minW, minH := int32(480*p.scale), int32(420*p.scale)
			if minW > work.right-work.left-12 {
				minW = work.right - work.left - 12
			}
			if minH > work.bottom-work.top-12 {
				minH = work.bottom - work.top - 12
			}
			bounds.MinSize = trayPoint{X: minW, Y: minH}
		}
		return 0
	case uxWMClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case uxWMNCDestroy:
		if p != nil && p.window == hwnd {
			uxActiveSettings = nil
			if p.renderer != nil {
				p.renderer.close()
				p.renderer = nil
			}
			if p.font != 0 {
				procDeleteObject.Call(p.font)
			}
			if p.titleFont != 0 {
				procDeleteObject.Call(p.titleFont)
			}
			if p.sectionFont != 0 {
				procDeleteObject.Call(p.sectionFont)
			}
			if p.mutedFont != 0 {
				procDeleteObject.Call(p.mutedFont)
			}
			if p.background != 0 {
				procDeleteObject.Call(p.background)
			}
			if p.cardBrush != 0 {
				procDeleteObject.Call(p.cardBrush)
			}
			if p.editBrush != 0 {
				procDeleteObject.Call(p.editBrush)
			}
		}
		return 0
	case uxWMEraseBkgnd:
		if p != nil && p.window == hwnd && p.background != 0 {
			var r rect32
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			procFillRect.Call(wParam, uintptr(unsafe.Pointer(&r)), p.background)
			return 1
		}
	case uxWMPaint:
		if p != nil && p.window == hwnd {
			var ps handyPaintStruct
			hdc, _, _ := procBeginPaintHandy.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			if hdc != 0 {
				if p.renderer == nil || !p.renderer.drawSettings(p.scrollX, p.scrollY) {
					p.drawSettingsFallback(hdc)
				}
				procEndPaintHandy.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			}
			return 0
		}
	case uxWMDrawItem:
		if p != nil && p.window == hwnd && lParam != 0 {
			item := (*drawItemStruct)(unsafe.Pointer(lParam))
			label := ""
			primary := false
			switch item.CtlID {
			case uxSaveButton:
				label, primary = "保存设置", true
			case uxCloseButton:
				label = "关闭"
			case uxAdvancedButton:
				label = "高级配置"
			case uxProbeASRButton, uxProbeLLMButton:
				label = "测试连接"
			}
			if label != "" && uiDrawActionButton(item, p.palette, p.font, label, primary) {
				return 1
			}
		}
	case uxWMColorStatic:
		if p != nil && p.window == hwnd && p.background != 0 {
			color := p.palette.text
			if lParam == p.hint || lParam == p.probeStatus[0] || lParam == p.probeStatus[1] {
				color = p.palette.muted
			}
			uxSetTextColor.Call(wParam, p.palette.color(color))
			uxSetBkMode.Call(wParam, TRANSPARENT)
			if p.cardStatics[lParam] {
				return p.cardBrush
			}
			return p.background
		}
	case uxWMColorEdit:
		if p != nil && p.window == hwnd && p.editBrush != 0 {
			uxSetTextColor.Call(wParam, p.palette.color(p.palette.text))
			procSetBkColor.Call(wParam, p.palette.color(p.palette.field))
			return p.editBrush
		}
	case uxWMProbeDone:
		if p != nil && p.window == hwnd && wParam < 2 {
			idx := int(wParam)
			p.probeMu.Lock()
			serial := p.probeSerial[idx]
			result := p.probeResult[idx]
			p.probeMu.Unlock()
			if serial == uint64(lParam) {
				uxSetText(p.probeStatus[idx], result)
			}
		}
		return 0
	case uxWMCommand:
		if p != nil && p.window == hwnd {
			if (wParam>>16)&0xffff == 0x0300 { // EN_CHANGE
				for idx, controls := range [2][3]uintptr{{p.inputs[0], p.inputs[1], p.inputs[2]}, {p.inputs[3], p.inputs[4], p.inputs[5]}} {
					for _, edit := range controls {
						if lParam == edit {
							p.probeMu.Lock()
							p.probeSerial[idx]++ // reject in-flight stale response
							p.probeMu.Unlock()
							uxSetText(p.probeStatus[idx], "连接配置已修改 · 请重新检测")
							break
						}
					}
				}
			}
			switch wParam & 0xffff {
			case uxSaveButton:
				if err := p.save(); err != nil {
					uxSetText(p.hint, "保存失败："+err.Error())
				} else {
					// save() provides the precise message, including restart notices.
				}
				return 0
			case uxCloseButton:
				procDestroyWindow.Call(hwnd)
				return 0
			case uxAdvancedButton:
				if err := shellOpenLocalFile(configPath()); err != nil {
					uxSetText(p.hint, "打开高级配置失败："+err.Error())
				}
				return 0
			case uxProbeASRButton:
				p.check("ASR", uxWinText(p.inputs[0]), uxWinText(p.inputs[2]), uxWinText(p.inputs[1]))
				return 0
			case uxProbeLLMButton:
				p.check("LLM", uxWinText(p.inputs[3]), uxWinText(p.inputs[5]), uxWinText(p.inputs[4]))
				return 0
			}
		}
	}
	v, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return v
}

func (p *uxSettings) save() error {
	if p.app.busy.Load() {
		return errors.New("请先完成当前录音／改写，再保存")
	}
	var fields [6]string
	for i, h := range p.inputs {
		fields[i] = uxWinText(h)
	}
	cfg, err := uxUpdatedConfig(p.app.cfg, fields)
	if err != nil {
		return err
	}
	newShortcut := strings.TrimSpace(uxWinText(p.shortcut))
	var shortcutErr error
	newShortcut, shortcutErr = uxDictationShortcut(newShortcut, cfg.HotkeyRecord, cfg.HotkeyExit)
	if shortcutErr != nil {
		return shortcutErr
	}
	changedShortcut := cfg.DictationHotkey != newShortcut
	cfg.DictationHotkey = newShortcut
	selected, _, _ := procSendMessage.Call(p.microphone, uxCBGetCurSel, 0, 0)
	if p.microphone != 0 && selected != ^uintptr(0) && selected < uintptr(len(p.microphoneIDs)) {
		cfg.AudioDeviceID = p.microphoneIDs[selected]
	}
	changedAudioDevice := cfg.AudioDeviceID != p.app.cfg.AudioDeviceID
	if err = saveConfig(cfg); err != nil {
		return err
	}
	p.app.cfg = cfg
	if changedShortcut || changedAudioDevice {
		uxSetText(p.hint, "设置已保存。快捷键和录音设备将在重启 VoiceRewrite 后生效。")
	} else {
		uxSetText(p.hint, "✓ 已保存，API 地址和模型立即生效")
	}
	p.app.logger.Print("settings updated: connection configuration saved (endpoint/key values redacted)")
	return nil
}

func (p *uxSettings) check(kind, base, key, model string) {
	if p == nil || p.window == 0 {
		return
	}
	idx := 0
	if kind == "LLM" {
		idx = 1
	}
	p.probeMu.Lock()
	p.probeSerial[idx]++
	serial := p.probeSerial[idx]
	p.probeMu.Unlock()
	uxSetText(p.probeStatus[idx], "正在检测服务…")
	h := p.window
	go func() {
		result := uxProbeModel(context.Background(), kind, base, key, model)
		p.probeMu.Lock()
		if serial == p.probeSerial[idx] {
			p.probeResult[idx] = result
		}
		p.probeMu.Unlock()
		procPostMessage.Call(h, uxWMProbeDone, uintptr(idx), uintptr(serial))
	}()
}

func uxCreateControl(parent uintptr, class, text string, style uintptr, x, y, w, h int32, id uintptr, font uintptr) uintptr {
	cn := uxPtr(class)
	val := uxPtr(text)
	handle, _, _ := procCreateWindow.Call(0, uintptr(unsafe.Pointer(cn)), uintptr(unsafe.Pointer(val)), style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, 0, 0)
	runtime.KeepAlive(cn)
	runtime.KeepAlive(val)
	if font != 0 {
		setFont(handle, font)
	}
	return handle
}

// Center a settings window inside the monitor work area; the recording
// pill remains bottom-centered, but a full settings window should not be.
func uxCenteredWindow(w, h int32) (int32, int32) {
	pt := currentCursorPoint()
	packed := uint64(uint32(pt.X)) | uint64(uint32(pt.Y))<<32
	mon, _, _ := procMonitorFromPointHandy.Call(uintptr(packed), MONITOR_DEFAULTTONEAREST)
	if mon != 0 {
		info := handyMonitorInfo{Size: uint32(unsafe.Sizeof(handyMonitorInfo{}))}
		if ok, _, _ := procGetMonitorInfoHandy.Call(mon, uintptr(unsafe.Pointer(&info))); ok != 0 {
			area := info.Work
			x := area.Left + (area.Right-area.Left-w)/2
			y := area.Top + (area.Bottom-area.Top-h)/2
			if x < area.Left {
				x = area.Left
			}
			if y < area.Top {
				y = area.Top
			}
			return x, y
		}
	}
	width, _, _ := procGetSystemMetrics.Call(0)
	height, _, _ := procGetSystemMetrics.Call(1)
	return (int32(width) - w) / 2, (int32(height) - h) / 2
}

func uxWorkAreaAtCursor() desktopWorkArea { return uxWorkAreaForPoint(currentCursorPoint()) }

func showNativeSettings(app *appState) error {
	restoreDPI := handyAwareWindowThread()
	defer restoreDPI()
	if uxActiveSettings != nil && uxActiveSettings.window != 0 {
		procShowWindow.Call(uxActiveSettings.window, SW_SHOW)
		procSetForegroundWindow.Call(uxActiveSettings.window)
		return nil
	}
	className := uxPtr("VoiceRewrite.Settings")
	title := uxPtr("VoiceRewrite · 连接设置")
	module, _, _ := procGetModuleHandle.Call(0)
	klass := trayWindowClass{Size: uint32(unsafe.Sizeof(trayWindowClass{})), WndProc: uxWindowCallback, Instance: module, ClassName: className}
	atom, _, regErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&klass)))
	if atom == 0 && regErr != syscall.Errno(1410) {
		return fmt.Errorf("无法注册设置窗口: %w", regErr)
	}
	// 96-DPI logical layout; all children use the same scale as the parent.
	scale := handyMonitorScale()
	if scale < 1 {
		scale = 1
	}
	sc := func(v int32) uintptr { return uintptr(int32(float64(v)*scale + 0.5)) }
	work := uxWorkAreaAtCursor()
	winW, winH := desktopFitSize(int32(sc(710)), int32(sc(580)), work, 12)
	x, y := uxCenteredWindow(winW, winH)
	h, _, createErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		uxWSCaption|uxWSSysMenu|uxWSMinimizeBox|uxWSThickFrame|WS_VSCROLL|0x00100000, uintptr(x), uintptr(y), uintptr(winW), uintptr(winH), 0, 0, module, 0)
	if h == 0 {
		return fmt.Errorf("创建设置窗口失败: %w", createErr)
	}
	p := &uxSettings{window: h, app: app, scale: scale, cardStatics: make(map[uintptr]bool), palette: resultCardPalette()}
	uxActiveSettings = p
	p.background, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.background))
	p.cardBrush, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.surface))
	p.editBrush, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.field))
	applyWindowPalette(h, p.palette)
	if renderer, renderErr := newResultRenderer(h, scale, "设置", p.palette.dark, false); renderErr == nil {
		p.renderer = renderer
	} else if app != nil && app.logger != nil {
		app.logger.Printf("settings Direct2D unavailable; using GDI: %v", renderErr)
	}
	fontName := uxPtr("Segoe UI Variable")
	p.font, _, _ = procCreateFont.Call(sc(-16), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.titleFont, _, _ = procCreateFont.Call(sc(-24), 0, 0, 0, 650, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.sectionFont, _, _ = procCreateFont.Call(sc(-18), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.mutedFont, _, _ = procCreateFont.Call(sc(-14), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	makeOne := func(cls, t string, style uintptr, x, y, w, hh int32, id uintptr, head bool) uintptr {
		useFont := p.font
		if head {
			useFont = p.sectionFont
		}
		control := uxCreateControl(h, cls, t, style, int32(sc(x)), int32(sc(y)), int32(sc(w)), int32(sc(hh)), id, useFont)
		if control == 0 {
			return 0
		}
		if cls == "EDIT" {
			procSendMessage.Call(control, uxEMSetMargins, 3, uintptr(8|(8<<16)))
			resultStyleNativeEdit(control, p.palette.dark)
		}
		if cls == "STATIC" && y >= 54 && y < 463 {
			p.cardStatics[control] = true
		}
		p.controls = append(p.controls, uxControl{
			hwnd: control, x: x, y: y, w: w, h: hh,
			stretch: false,
			right:   false,
			id:      id,
		})
		return control
	}
	stat := uintptr(uxWSChild | uxWSVisible)
	edit := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxWSBorder | uxESAutoScroll)
	btn := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxBSOwnerDraw)
	// The visual heading is rendered by DirectWrite. A native static child
	// retains the title for Windows accessibility tools.
	heading := makeOne("STATIC", "设置", uxWSChild, 0, 0, 1, 1, 0, false)
	setFont(heading, p.titleFont)
	// Two side-by-side service cards eliminate three full-width stacked blocks.
	makeOne("STATIC", "语音识别", stat, 30, 68, 175, 26, 0, true)
	makeOne("BUTTON", "测试连接", btn, 235, 68, 85, 29, uxProbeASRButton, false)
	makeOne("STATIC", "API 地址", stat, 30, 105, 120, 20, 0, false)
	p.inputs[0] = makeOne("EDIT", app.cfg.ASRBaseURL, edit, 30, 128, 291, 31, 0, false)
	makeOne("STATIC", "模型 ID", stat, 30, 169, 120, 20, 0, false)
	p.inputs[1] = makeOne("EDIT", app.cfg.ASRModel, edit, 30, 191, 291, 31, 0, false)
	makeOne("STATIC", "API Key", stat, 30, 232, 120, 20, 0, false)
	p.inputs[2] = makeOne("EDIT", app.cfg.ASRAPIKey, edit|uxESPassword, 30, 254, 291, 31, 0, false)
	p.probeStatus[0] = makeOne("STATIC", "尚未检测", stat, 30, 294, 286, 52, 0, false)

	makeOne("STATIC", "AI 文本处理", stat, 364, 68, 175, 26, 0, true)
	makeOne("BUTTON", "测试连接", btn, 572, 68, 89, 29, uxProbeLLMButton, false)
	makeOne("STATIC", "API 地址", stat, 364, 105, 120, 20, 0, false)
	p.inputs[3] = makeOne("EDIT", app.cfg.LLMBaseURL, edit, 364, 128, 297, 31, 0, false)
	makeOne("STATIC", "模型 ID", stat, 364, 169, 120, 20, 0, false)
	p.inputs[4] = makeOne("EDIT", app.cfg.TextModel, edit, 364, 191, 297, 31, 0, false)
	makeOne("STATIC", "API Key", stat, 364, 232, 120, 20, 0, false)
	p.inputs[5] = makeOne("EDIT", app.cfg.LLMAPIKey, edit|uxESPassword, 364, 254, 297, 31, 0, false)
	p.probeStatus[1] = makeOne("STATIC", "尚未检测", stat, 364, 294, 298, 52, 0, false)

	makeOne("STATIC", "快捷键与输入设备", stat, 30, 373, 290, 24, 0, true)
	makeOne("STATIC", "听写快捷键", stat, 30, 412, 110, 22, 0, false)
	p.shortcut = makeOne("EDIT", app.cfg.DictationHotkey, edit, 130, 405, 158, 31, uxShortcutInput, false)
	makeOne("STATIC", "麦克风", stat, 318, 412, 85, 22, 0, false)
	p.microphone = makeOne("COMBOBOX", "", uxWSChild|uxWSVisible|uxWSTabStop|uxCBSDropDownList|WS_VSCROLL, 407, 404, 254, 190, uxMicrophoneInput, false)
	p.populateMicrophones(app.cfg.AudioDeviceID)
	resultStyleNativeEdit(p.microphone, p.palette.dark)
	makeOne("BUTTON", "保存设置", btn, 16, 461, 144, 34, uxSaveButton, false)
	makeOne("BUTTON", "高级配置", btn, 172, 461, 113, 34, uxAdvancedButton, false)
	makeOne("BUTTON", "关闭", btn, 580, 461, 96, 34, uxCloseButton, false)
	p.hint = makeOne("STATIC", "", stat, 18, 498, 630, 21, 0, false)
	setFont(p.hint, p.mutedFont)
	setFont(p.probeStatus[0], p.mutedFont)
	setFont(p.probeStatus[1], p.mutedFont)
	p.layout()
	// Windows 11 DWM-rounded frame and non-activating status pill are separate.
	rounding := uint32(2)
	procDwmSetWindowAttribute.Call(h, DWMWA_WINDOW_CORNER_PREFERENCE, uintptr(unsafe.Pointer(&rounding)), 4)
	procShowWindow.Call(h, SW_SHOW)
	procSetForegroundWindow.Call(h)
	return nil
}
func closeNativeSettings() {
	if uxActiveSettings != nil && uxActiveSettings.window != 0 {
		procDestroyWindow.Call(uxActiveSettings.window)
	}
}
