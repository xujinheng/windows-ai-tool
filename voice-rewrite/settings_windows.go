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
	window          uintptr
	app             *appState
	inputs          [6]uintptr
	shortcut        uintptr
	microphone      uintptr
	microphoneIDs   []int
	hint            uintptr
	font, titleFont uintptr
	background      uintptr
	cardBrush       uintptr
	editBrush       uintptr
	palette         uiPalette
	cardStatics     map[uintptr]bool
	probeMu         sync.Mutex
	probeResult     [2]string
	probeStatus     [2]uintptr
	probeSerial     [2]uint64
	controls        []uxControl
	scale           float64
}

// Every child control is reflowed after a resize. No hard-coded physical
// coordinates may escape uxLayoutRect; the data is in 96-DPI logical units.
type uxControl struct {
	hwnd       uintptr
	x, y, w, h int32
	stretch    bool
	right      bool
}

func uxLayoutRect(c uxControl, clientWidth int32) (x, y, w, h int32) {
	return layoutSettingsRect(uxLayoutSpec{x: c.x, y: c.y, w: c.w, h: c.h, stretch: c.stretch, right: c.right}, clientWidth)
}
func (p *uxSettings) layout() {
	if p == nil || p.window == 0 {
		return
	}
	var rect rect32
	procGetClientRect.Call(p.window, uintptr(unsafe.Pointer(&rect)))
	logical := int32(float64(rect.Right-rect.Left) / p.scale)
	for _, control := range p.controls {
		x, y, w, h := uxLayoutRect(control, logical)
		sc := func(v int32) uintptr { return uintptr(int32(float64(v)*p.scale + 0.5)) }
		procMoveWindow.Call(control.hwnd, sc(x), sc(y), sc(w), sc(h), 1)
	}
	procInvalidateRectSettings.Call(p.window, 0, 1)
}

var (
	procMoveWindow             = user32.NewProc("MoveWindow")
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
	palette := systemPalette()
	if palette == p.palette {
		return
	}
	p.background = refreshPaletteBrush(p.background, palette, palette.background)
	p.cardBrush = refreshPaletteBrush(p.cardBrush, palette, palette.surface)
	p.editBrush = refreshPaletteBrush(p.editBrush, palette, palette.field)
	p.palette = palette
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
	case uxWMSize:
		if p != nil && p.window == hwnd {
			p.layout()
		}
		return 0
	case uxWMGetMinMaxInfo:
		if p != nil && lParam != 0 {
			bounds := (*uxMinMaxInfo)(unsafe.Pointer(lParam))
			bounds.MinSize = trayPoint{X: int32(680 * p.scale), Y: int32(690 * p.scale)}
		}
		return 0
	case uxWMClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case uxWMNCDestroy:
		if p != nil && p.window == hwnd {
			uxActiveSettings = nil
			if p.font != 0 {
				procDeleteObject.Call(p.font)
			}
			if p.titleFont != 0 {
				procDeleteObject.Call(p.titleFont)
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
				old, _, _ := uxSelectObject.Call(hdc, p.cardBrush)
				nullPen, _, _ := uxGetStockObject.Call(8) // NULL_PEN
				oldPen, _, _ := uxSelectObject.Call(hdc, nullPen)
				for _, top := range []int32{93, 285} {
					sc := func(v int32) uintptr { return uintptr(int32(float64(v)*p.scale + 0.5)) }
					var rc rect32
					procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
					uxRoundRect.Call(hdc, sc(17), sc(top), uintptr(rc.Right)-sc(17), sc(top+185), sc(18), sc(18))
				}
				sc := func(v int32) uintptr { return uintptr(int32(float64(v)*p.scale + 0.5)) }
				var rc rect32
				procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
				uxRoundRect.Call(hdc, sc(17), sc(477), uintptr(rc.Right)-sc(17), sc(595), sc(18), sc(18))
				// Accent rails establish a consistent hierarchy across sections.
				accentBrush, _, _ := procCreateSolidBrush.Call(p.palette.color(p.palette.primary))
				if accentBrush != 0 {
					for _, r := range []rect32{
						{Left: int32(sc(25)), Top: int32(sc(23)), Right: int32(sc(30)), Bottom: int32(sc(51))},
						{Left: int32(sc(25)), Top: int32(sc(106)), Right: int32(sc(29)), Bottom: int32(sc(128))},
						{Left: int32(sc(25)), Top: int32(sc(298)), Right: int32(sc(29)), Bottom: int32(sc(320))},
						{Left: int32(sc(25)), Top: int32(sc(491)), Right: int32(sc(29)), Bottom: int32(sc(513))},
					} {
						procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), accentBrush)
					}
					procDeleteObject.Call(accentBrush)
				}
				uxSelectObject.Call(hdc, old)
				uxSelectObject.Call(hdc, oldPen)
				procEndPaintHandy.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			}
			return 0
		}
	case uxWMDrawItem:
		if p != nil && p.window == hwnd && wParam == uxSaveButton && lParam != 0 {
			item := (*drawItemStruct)(unsafe.Pointer(lParam))
			brush, _, _ := procCreateSolidBrush.Call(p.palette.color(p.palette.primary))
			if brush != 0 {
				oldBrush, _, _ := uxSelectObject.Call(item.Hdc, brush)
				pen, _, _ := uxGetStockObject.Call(8)
				oldPen, _, _ := uxSelectObject.Call(item.Hdc, pen)
				r := item.Rect
				uxRoundRect.Call(item.Hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 12, 12)
				uxSelectObject.Call(item.Hdc, oldPen)
				uxSelectObject.Call(item.Hdc, oldBrush)
				procDeleteObject.Call(brush)
			}
			uxSetBkMode.Call(item.Hdc, TRANSPARENT)
			uxSetTextColor.Call(item.Hdc, p.palette.color(p.palette.primaryText))
			oldFont, _, _ := uxSelectObject.Call(item.Hdc, p.font)
			label := uxPtr("保存并应用")
			procDrawText.Call(item.Hdc, uintptr(unsafe.Pointer(label)), ^uintptr(0), uintptr(unsafe.Pointer(&item.Rect)), 0x25)
			uxSelectObject.Call(item.Hdc, oldFont)
			return 1
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

func showNativeSettings(app *appState) error {
	restoreDPI := handyAwareWindowThread()
	defer restoreDPI()
	if uxActiveSettings != nil && uxActiveSettings.window != 0 {
		procShowWindow.Call(uxActiveSettings.window, SW_SHOW)
		procSetForegroundWindow.Call(uxActiveSettings.window)
		return nil
	}
	className := uxPtr("VoiceRewrite.Settings.v20")
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
	x, y := uxCenteredWindow(int32(sc(714)), int32(sc(720)))
	if y < 0 {
		y = 60
	}
	h, _, createErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		uxWSCaption|uxWSSysMenu|uxWSMinimizeBox|uxWSThickFrame, uintptr(x), uintptr(y), sc(714), sc(720), 0, 0, module, 0)
	if h == 0 {
		return fmt.Errorf("创建设置窗口失败: %w", createErr)
	}
	p := &uxSettings{window: h, app: app, scale: scale, cardStatics: make(map[uintptr]bool), palette: systemPalette()}
	uxActiveSettings = p
	p.background, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.background))
	p.cardBrush, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.surface))
	p.editBrush, _, _ = procCreateSolidBrush.Call(p.palette.color(p.palette.field))
	applyWindowPalette(h, p.palette)
	fontName := uxPtr("Segoe UI Variable")
	p.font, _, _ = procCreateFont.Call(sc(-16), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.titleFont, _, _ = procCreateFont.Call(sc(-22), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	makeOne := func(cls, t string, style uintptr, x, y, w, hh int32, id uintptr, head bool) uintptr {
		useFont := p.font
		if head {
			useFont = p.titleFont
		}
		control := uxCreateControl(h, cls, t, style, int32(sc(x)), int32(sc(y)), int32(sc(w)), int32(sc(hh)), id, useFont)
		if control == 0 {
			return 0
		}
		if cls == "EDIT" {
			procSendMessage.Call(control, uxEMSetMargins, 3, uintptr(8|(8<<16)))
		}
		if cls == "STATIC" && y >= 99 && y < 596 {
			p.cardStatics[control] = true
		}
		p.controls = append(p.controls, uxControl{
			hwnd: control, x: x, y: y, w: w, h: hh,
			stretch: (x == 130 || (cls == "STATIC" && w >= 380)),
			right:   (x >= 527),
		})
		return control
	}
	stat := uintptr(uxWSChild | uxWSVisible)
	edit := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxWSBorder | uxESAutoScroll)
	btn := uintptr(uxWSChild | uxWSVisible | uxWSTabStop)
	makeOne("STATIC", "VoiceRewrite", stat, 42, 18, 400, 34, 0, true)
	makeOne("STATIC", "语音听写  /  智能改写  /  本地 AI", stat, 42, 59, 610, 23, 0, false)
	makeOne("STATIC", "01   语音识别 ASR", stat, 34, 101, 410, 28, 0, true)
	makeOne("STATIC", "API 地址", stat, 34, 141, 100, 20, 0, false)
	p.inputs[0] = makeOne("EDIT", app.cfg.ASRBaseURL, edit, 130, 135, 385, 30, 0, false)
	makeOne("BUTTON", "检测", btn, 527, 135, 96, 30, uxProbeASRButton, false)
	makeOne("STATIC", "模型 ID", stat, 34, 179, 100, 20, 0, false)
	p.inputs[1] = makeOne("EDIT", app.cfg.ASRModel, edit, 130, 173, 493, 30, 0, false)
	makeOne("STATIC", "API Key", stat, 34, 217, 100, 20, 0, false)
	p.inputs[2] = makeOne("EDIT", app.cfg.ASRAPIKey, edit|uxESPassword, 130, 211, 493, 30, 0, false)
	p.probeStatus[0] = makeOne("STATIC", "● 尚未检测 · 可在设置后试录音验证", stat, 34, 251, 594, 24, 0, false)

	makeOne("STATIC", "02   AI 文本处理 LLM", stat, 34, 295, 410, 28, 0, true)
	makeOne("STATIC", "API 地址", stat, 34, 331, 100, 20, 0, false)
	p.inputs[3] = makeOne("EDIT", app.cfg.LLMBaseURL, edit, 130, 325, 385, 30, 0, false)
	makeOne("BUTTON", "检测", btn, 527, 325, 96, 30, uxProbeLLMButton, false)
	makeOne("STATIC", "模型 ID", stat, 34, 372, 100, 20, 0, false)
	p.inputs[4] = makeOne("EDIT", app.cfg.TextModel, edit, 130, 366, 493, 30, 0, false)
	makeOne("STATIC", "API Key", stat, 34, 413, 100, 20, 0, false)
	p.inputs[5] = makeOne("EDIT", app.cfg.LLMAPIKey, edit|uxESPassword, 130, 407, 493, 30, 0, false)
	p.probeStatus[1] = makeOne("STATIC", "● 尚未检测 · 用于改写 / 翻译 / 问答", stat, 34, 451, 594, 24, 0, false)

	makeOne("STATIC", "03   输入与快捷键", stat, 34, 490, 300, 26, 0, true)
	makeOne("STATIC", "听写快捷键", stat, 34, 527, 108, 22, 0, false)
	p.shortcut = makeOne("EDIT", app.cfg.DictationHotkey, edit, 130, 521, 493, 30, uxShortcutInput, false)
	makeOne("STATIC", "麦克风", stat, 34, 562, 90, 22, 0, false)
	p.microphone = makeOne("COMBOBOX", "", uxWSChild|uxWSVisible|uxWSTabStop|uxCBSDropDownList|WS_VSCROLL, 130, 557, 493, 192, uxMicrophoneInput, false)
	p.populateMicrophones(app.cfg.AudioDeviceID)
	makeOne("BUTTON", "保存并应用", btn|uxBSOwnerDraw, 29, 607, 150, 36, uxSaveButton, false)
	makeOne("BUTTON", "高级设置", btn, 195, 607, 119, 36, uxAdvancedButton, false)
	makeOne("BUTTON", "关闭", btn, 550, 607, 93, 36, uxCloseButton, false)
	p.hint = makeOne("STATIC", "本地优先  ·  API Key 已加密  ·  按住 "+app.cfg.DictationHotkey+" 开始听写", stat, 30, 656, 610, 22, 0, false)
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
