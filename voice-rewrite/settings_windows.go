//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// A small, native settings surface; it does not host a browser or intercept
// the editor focus until deliberately opened from the tray menu.
const (
	uxWMCommand         = 0x0111
	uxWMClose           = 0x0010
	uxWMEraseBkgnd      = 0x0014
	uxWMNCDestroy       = 0x0082
	uxWMColorStatic     = 0x0138
	uxWMColorEdit       = 0x0133
	uxWMProbeDone       = 0x8000 + 104
	uxSaveButton        = 9101
	uxCloseButton       = 9102
	uxAdvancedButton    = 9103
	uxProbeASRButton    = 9104
	uxProbeLLMButton    = 9105
	uxModelFolderButton = 9106
	uxWSChild           = 0x40000000
	uxWSVisible         = 0x10000000
	uxWSTabStop         = 0x00010000
	uxESAutoScroll      = 0x0080
	uxWSBorder          = 0x00800000
	uxWSCaption         = 0x00C00000
	uxWSSysMenu         = 0x00080000
	uxWSMinimizeBox     = 0x00020000
)

var (
	uxGetWindowText  = user32.NewProc("GetWindowTextW")
	uxSetTextColor   = gdi32.NewProc("SetTextColor")
	uxSetBkMode      = gdi32.NewProc("SetBkMode")
	uxWindowCallback = syscall.NewCallback(uxSettingsWndProc)
	uxActiveSettings *uxSettings
)

type uxSettings struct {
	window          uintptr
	app             *appState
	inputs          [4]uintptr
	hint            uintptr
	font, titleFont uintptr
	background      uintptr
	probeMu         sync.Mutex
	probeResult     string
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
		}
		return 0
	case uxWMEraseBkgnd:
		if p != nil && p.window == hwnd && p.background != 0 {
			var r rect32
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			procFillRect.Call(wParam, uintptr(unsafe.Pointer(&r)), p.background)
			return 1
		}
	case uxWMColorStatic:
		if p != nil && p.window == hwnd && p.background != 0 {
			uxSetTextColor.Call(wParam, uintptr(0x002D2734))
			uxSetBkMode.Call(wParam, TRANSPARENT)
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
				p.check("ASR", uxWinText(p.inputs[0]))
				return 0
			case uxProbeLLMButton:
				p.check("LLM", uxWinText(p.inputs[2]))
				return 0
			case uxModelFolderButton:
				folder := filepath.Join(asrExecRoot(), "models")
				if err := os.MkdirAll(folder, 0755); err != nil {
					uxSetText(p.hint, "无法创建模型目录："+err.Error())
				} else if err := shellOpenLocalFile(folder); err != nil {
					uxSetText(p.hint, "打开目录失败："+err.Error())
				} else {
					uxSetText(p.hint, "已打开 models 文件夹。放入 llama-server、GGUF 后重启本程序。")
				}
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
	var fields [4]string
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

func (p *uxSettings) check(kind, base string) {
	if p == nil || p.window == 0 {
		return
	}
	uxSetText(p.hint, "正在检测 "+kind+" 服务…")
	h := p.window
	go func() {
		result := uxProbe(context.Background(), kind, base)
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
	className := uxPtr("VoiceRewrite.Settings.v16")
	title := uxPtr("VoiceRewrite · 设置")
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
	x, y := uxCenteredWindow(int32(sc(600)), int32(sc(665)))
	h, _, createErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)),
		uxWSCaption|uxWSSysMenu|uxWSMinimizeBox, uintptr(x), uintptr(y), sc(600), sc(665), 0, 0, module, 0)
	if h == 0 {
		return fmt.Errorf("创建设置窗口失败: %w", createErr)
	}
	p := &uxSettings{window: h, app: app}
	uxActiveSettings = p
	p.background, _, _ = procCreateSolidBrush.Call(uintptr(0x00FBF8F9))
	fontName := uxPtr("Microsoft YaHei UI")
	p.font, _, _ = procCreateFont.Call(sc(-17), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.titleFont, _, _ = procCreateFont.Call(sc(-23), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	makeOne := func(cls, t string, style uintptr, x, y, w, hh int32, id uintptr, head bool) uintptr {
		useFont := p.font
		if head {
			useFont = p.titleFont
		}
		return uxCreateControl(h, cls, t, style, int32(sc(x)), int32(sc(y)), int32(sc(w)), int32(sc(hh)), id, useFont)
	}
	stat := uintptr(uxWSChild | uxWSVisible)
	edit := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxWSBorder | uxESAutoScroll)
	btn := uintptr(uxWSChild | uxWSVisible | uxWSTabStop)
	makeOne("STATIC", "欢迎使用 VoiceRewrite", stat, 28, 18, 440, 36, 0, true)
	makeOne("STATIC", "Mouse4 直接听写    ·    Mouse5 选区改写 / 问答", stat, 30, 60, 540, 26, 0, false)
	makeOne("STATIC", "语音识别 · Qwen3-ASR / llama.cpp", stat, 30, 110, 520, 28, 0, true)
	makeOne("STATIC", "ASR 服务地址", stat, 30, 150, 180, 24, 0, false)
	p.inputs[0] = makeOne("EDIT", app.cfg.ASRBaseURL, edit, 30, 180, 417, 34, 0, false)
	makeOne("BUTTON", "测试连接", btn, 455, 180, 108, 34, uxProbeASRButton, false)
	makeOne("STATIC", "ASR 模型名称", stat, 30, 220, 180, 25, 0, false)
	p.inputs[1] = makeOne("EDIT", app.cfg.ASRModel, edit, 30, 245, 533, 34, 0, false)
	makeOne("STATIC", "文本处理 · LM Studio", stat, 30, 293, 520, 30, 0, true)
	makeOne("STATIC", "LLM 服务地址", stat, 30, 330, 180, 24, 0, false)
	p.inputs[2] = makeOne("EDIT", app.cfg.LLMBaseURL, edit, 30, 356, 417, 34, 0, false)
	makeOne("BUTTON", "测试连接", btn, 455, 356, 108, 34, uxProbeLLMButton, false)
	makeOne("STATIC", "文本模型名称（auto = 自动选择）", stat, 30, 396, 400, 24, 0, false)
	p.inputs[3] = makeOne("EDIT", app.cfg.TextModel, edit, 30, 419, 533, 34, 0, false)
	makeOne("BUTTON", "保存并应用", btn, 30, 470, 150, 36, uxSaveButton, false)
	makeOne("BUTTON", "高级配置 JSON", btn, 194, 470, 146, 36, uxAdvancedButton, false)
	makeOne("BUTTON", "关闭", btn, 461, 470, 102, 36, uxCloseButton, false)
	p.hint = makeOne("STATIC", "请先分别测试 ASR 和 LLM 连接，再尝试 Mouse4 / Mouse5", stat, 30, 517, 545, 26, 0, false)
	makeOne("STATIC", "ASR 默认 8080；将 CUDA 版 llama-server 和 GGUF 放到 models/", stat, 30, 553, 393, 26, 0, false)
	makeOne("BUTTON", "打开模型目录", btn, 438, 550, 128, 31, uxModelFolderButton, false)
	makeOne("STATIC", "复制模型后重启程序；以后可右键托盘打开设置。", stat, 30, 589, 530, 25, 0, false)
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
