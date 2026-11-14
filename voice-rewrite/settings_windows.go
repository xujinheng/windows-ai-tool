//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// A small, native settings surface; it does not host a browser or intercept
// the editor focus until deliberately opened from the tray menu.
const (
	uxWMCommand      = 0x0111
	uxWMClose        = 0x0010
	uxWMEraseBkgnd   = 0x0014
	uxWMPaint        = 0x000F
	uxWMNCDestroy    = 0x0082
	uxWMColorStatic  = 0x0138
	uxWMColorEdit    = 0x0133
	uxWMProbeDone    = 0x8000 + 104
	uxSaveButton     = 9101
	uxCloseButton    = 9102
	uxAdvancedButton = 9103
	uxProbeASRButton = 9104
	uxProbeLLMButton = 9105
	uxWSChild        = 0x40000000
	uxWSVisible      = 0x10000000
	uxWSTabStop      = 0x00010000
	uxESAutoScroll   = 0x0080
	uxESPassword     = 0x0020
	uxWSBorder       = 0x00800000
	uxWSCaption      = 0x00C00000
	uxWSSysMenu      = 0x00080000
	uxWSMinimizeBox  = 0x00020000
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
	hint            uintptr
	font, titleFont uintptr
	background      uintptr
	cardBrush       uintptr
	cardStatics     map[uintptr]bool
	probeMu         sync.Mutex
	probeResult     string
	scale           float64
}

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

func uxSettingsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	p := uxActiveSettings
	switch uint32(msg) {
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
				for _, top := range []int32{99, 308} {
					sc := func(v int32) uintptr { return uintptr(int32(float64(v)*p.scale + 0.5)) }
					uxRoundRect.Call(hdc, sc(17), sc(top), sc(657), sc(top+196), sc(18), sc(18))
				}
				uxSelectObject.Call(hdc, old)
				uxSelectObject.Call(hdc, oldPen)
				procEndPaintHandy.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			}
			return 0
		}
	case uxWMColorStatic:
		if p != nil && p.window == hwnd && p.background != 0 {
			uxSetTextColor.Call(wParam, uintptr(0x002D2734))
			uxSetBkMode.Call(wParam, TRANSPARENT)
			if p.cardStatics[lParam] {
				return p.cardBrush
			}
			return p.background
		}
	case uxWMColorEdit:
		// Preserve the standard high-contrast system EDIT background.
	case uxWMProbeDone:
		if p != nil && p.window == hwnd {
			p.probeMu.Lock()
			result := p.probeResult
			p.probeMu.Unlock()
			uxSetText(p.hint, result)
		}
		return 0
	case uxWMCommand:
		if p != nil && p.window == hwnd {
			switch wParam & 0xffff {
			case uxSaveButton:
				if err := p.save(); err != nil {
					uxSetText(p.hint, "保存失败："+err.Error())
				} else {
					uxSetText(p.hint, "✓ 已保存，模型地址和名称立即生效")
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
				p.check("ASR", uxWinText(p.inputs[0]), uxWinText(p.inputs[2]))
				return 0
			case uxProbeLLMButton:
				p.check("LLM", uxWinText(p.inputs[3]), uxWinText(p.inputs[5]))
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
	blob, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(configPath(), append(blob, '\n'), 0600); err != nil {
		return err
	}
	p.app.cfg = cfg
	p.app.logger.Printf("settings updated: ASR=%s LLM=%s asr_model=%s text_model=%s", cfg.ASRBaseURL, cfg.LLMBaseURL, cfg.ASRModel, cfg.TextModel)
	return nil
}

func (p *uxSettings) check(kind, base, key string) {
	if p == nil || p.window == 0 {
		return
	}
	uxSetText(p.hint, "正在检测 "+kind+" API…")
	h := p.window
	go func() {
		result := uxProbeWithKey(context.Background(), kind, base, key)
		p.probeMu.Lock()
		p.probeResult = result
		p.probeMu.Unlock()
		procPostMessage.Call(h, uxWMProbeDone, 0, 0)
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
	className := uxPtr("VoiceRewrite.Settings.ClientOnly.v19")
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
	x, y := uxCenteredWindow(int32(sc(694)), int32(sc(626)))
	if y < 0 {
		y = 60
	}
	h, _, createErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		uxWSCaption|uxWSSysMenu|uxWSMinimizeBox, uintptr(x), uintptr(y), sc(694), sc(626), 0, 0, module, 0)
	if h == 0 {
		return fmt.Errorf("创建设置窗口失败: %w", createErr)
	}
	p := &uxSettings{window: h, app: app, scale: scale, cardStatics: make(map[uintptr]bool)}
	uxActiveSettings = p
	p.background, _, _ = procCreateSolidBrush.Call(uintptr(0x00F7F6F4))
	p.cardBrush, _, _ = procCreateSolidBrush.Call(uintptr(0x00FFFFFF))
	fontName := uxPtr("Microsoft YaHei UI")
	p.font, _, _ = procCreateFont.Call(sc(-15), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.titleFont, _, _ = procCreateFont.Call(sc(-21), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	makeOne := func(cls, t string, style uintptr, x, y, w, hh int32, id uintptr, head bool) uintptr {
		useFont := p.font
		if head {
			useFont = p.titleFont
		}
		control := uxCreateControl(h, cls, t, style, int32(sc(x)), int32(sc(y)), int32(sc(w)), int32(sc(hh)), id, useFont)
		if cls == "STATIC" && y >= 99 && y < 504 {
			p.cardStatics[control] = true
		}
		return control
	}
	stat := uintptr(uxWSChild | uxWSVisible)
	edit := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxWSBorder | uxESAutoScroll)
	btn := uintptr(uxWSChild | uxWSVisible | uxWSTabStop)
	makeOne("STATIC", "VoiceRewrite", stat, 28, 19, 360, 32, 0, true)
	makeOne("STATIC", "纯客户端  ·  Mouse4 听写  /  Mouse5 改写与解释", stat, 30, 57, 595, 24, 0, false)
	// Both required connections are visible at once. No tabs or server lifecycle controls.
	makeOne("STATIC", "01    语音识别 API   ·   两种功能共用", stat, 34, 109, 400, 24, 0, true)
	makeOne("STATIC", "服务地址", stat, 34, 141, 100, 20, 0, false)
	p.inputs[0] = makeOne("EDIT", app.cfg.ASRBaseURL, edit, 130, 137, 385, 29, 0, false)
	makeOne("BUTTON", "检测", btn, 527, 137, 96, 29, uxProbeASRButton, false)
	makeOne("STATIC", "模型 ID", stat, 34, 185, 100, 20, 0, false)
	p.inputs[1] = makeOne("EDIT", app.cfg.ASRModel, edit, 130, 180, 493, 29, 0, false)
	makeOne("STATIC", "API Key（可选）", stat, 34, 228, 100, 20, 0, false)
	p.inputs[2] = makeOne("EDIT", app.cfg.ASRAPIKey, edit|uxESPassword, 130, 222, 493, 29, 0, false)
	makeOne("STATIC", "用于音频转写的 OpenAI 兼容接口", stat, 132, 265, 435, 19, 0, false)

	makeOne("STATIC", "02    文本处理 API   ·   仅 Mouse5", stat, 34, 317, 400, 24, 0, true)
	makeOne("STATIC", "服务地址", stat, 34, 350, 100, 20, 0, false)
	p.inputs[3] = makeOne("EDIT", app.cfg.LLMBaseURL, edit, 130, 345, 385, 29, 0, false)
	makeOne("BUTTON", "检测", btn, 527, 345, 96, 29, uxProbeLLMButton, false)
	makeOne("STATIC", "模型 ID", stat, 34, 393, 100, 20, 0, false)
	p.inputs[4] = makeOne("EDIT", app.cfg.TextModel, edit, 130, 388, 493, 29, 0, false)
	makeOne("STATIC", "API Key（可选）", stat, 34, 436, 100, 20, 0, false)
	p.inputs[5] = makeOne("EDIT", app.cfg.LLMAPIKey, edit|uxESPassword, 130, 430, 493, 29, 0, false)
	makeOne("STATIC", "用于改写 / 翻译 / 问答的 Chat Completions 接口", stat, 132, 472, 495, 19, 0, false)
	makeOne("BUTTON", "保存并应用", btn, 29, 519, 133, 32, uxSaveButton, false)
	makeOne("BUTTON", "高级 JSON", btn, 178, 519, 119, 32, uxAdvancedButton, false)
	makeOne("BUTTON", "关闭", btn, 550, 519, 93, 32, uxCloseButton, false)
	p.hint = makeOne("STATIC", "连接检测只检查服务可达性；真正转写请按 Mouse4 试用", stat, 30, 557, 615, 21, 0, false)
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
