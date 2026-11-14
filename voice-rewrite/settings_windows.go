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
	"strings"
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
	uxTabASR            = 9107
	uxTabLLM            = 9108
	uxBrowseExe         = 9109
	uxBrowseASR         = 9110
	uxBrowseProjector   = 9111
	uxBrowseLLM         = 9112
	uxModeASRButton     = 9113
	uxModeLLMButton     = 9114
	uxBMGetCheck        = 0x00F0
	uxBMSetCheck        = 0x00F1
	uxBSAutoCheckbox    = 0x00000003
	uxESPassword        = 0x0020
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
	paths           [4]uintptr // shared executable, ASR GGUF, ASR mmproj, LLM GGUF
	keys            [2]uintptr
	launchChecks    [2]uintptr
	tabs            [2]uintptr
	pageControls    [2][]uintptr
	currentTab      int
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
				message, err := p.save()
				if err != nil {
					uxSetText(p.hint, "保存失败："+err.Error())
				} else {
					uxSetText(p.hint, message)
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
			case uxModeLLMButton:
				checked, _, _ := procSendMessage.Call(p.launchChecks[1], uxBMGetCheck, 0, 0)
				if checked != 0 && strings.TrimSpace(uxWinText(p.inputs[2])) == "http://127.0.0.1:1234/v1" {
					uxSetText(p.inputs[2], "http://127.0.0.1:8082/v1")
					uxSetText(p.hint, "已为本地文本模型选择 8082 端口；ASR 默认使用 8080")
				}
				return 0
			case uxTabASR:
				p.switchTab(0)
				return 0
			case uxTabLLM:
				p.switchTab(1)
				return 0
			case uxBrowseExe:
				p.browse(p.paths[0], "选择 llama-server.exe")
				return 0
			case uxBrowseASR:
				p.browse(p.paths[1], "选择 ASR 模型 GGUF")
				return 0
			case uxBrowseProjector:
				p.browse(p.paths[2], "选择音频编码 mmproj GGUF")
				return 0
			case uxBrowseLLM:
				p.browse(p.paths[3], "选择文本模型 GGUF")
				return 0
			case uxProbeASRButton:
				p.check("ASR", uxWinText(p.inputs[0]), uxWinText(p.keys[0]))
				return 0
			case uxProbeLLMButton:
				p.check("LLM", uxWinText(p.inputs[2]), uxWinText(p.keys[1]))
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

func (p *uxSettings) switchTab(tab int) {
	if tab < 0 || tab > 1 {
		return
	}
	p.currentTab = tab
	for i := 0; i < 2; i++ {
		for _, hwnd := range p.pageControls[i] {
			if i == tab {
				procShowWindow.Call(hwnd, SW_SHOW)
			} else {
				procShowWindow.Call(hwnd, 0)
			}
		}
	}
	label := "ASR：连接自己的转写 API，或选择本地 GGUF 由程序启动；需支持 /audio/transcriptions"
	if tab == 1 {
		label = "文本模型：支持 OpenAI /chat/completions；自启动需文本生成 GGUF"
	}
	uxSetText(p.hint, label)
}

func (p *uxSettings) save() (string, error) {
	if p.app.busy.Load() {
		return "", errors.New("正在录音／改写，请结束后再修改模型配置")
	}
	var fields [4]string
	for i, h := range p.inputs {
		fields[i] = uxWinText(h)
	}
	cfg, err := uxUpdatedConfig(p.app.cfg, fields)
	if err != nil {
		return "", err
	}
	cfg.LLAMAServerPath = uxWinText(p.paths[0])
	cfg.ASRModelPath = uxWinText(p.paths[1])
	cfg.ASRMMProjPath = uxWinText(p.paths[2])
	cfg.LLMModelPath = uxWinText(p.paths[3])
	cfg.ASRAPIKey = uxWinText(p.keys[0])
	cfg.LLMAPIKey = uxWinText(p.keys[1])
	cfg.ASRServiceMode = serviceConnect
	cfg.LLMServiceMode = serviceConnect
	if check, _, _ := procSendMessage.Call(p.launchChecks[0], uxBMGetCheck, 0, 0); check != 0 {
		cfg.ASRServiceMode = serviceLaunch
	}
	if check, _, _ := procSendMessage.Call(p.launchChecks[1], uxBMGetCheck, 0, 0); check != 0 {
		cfg.LLMServiceMode = serviceLaunch
	}
	if err := validateServiceModes(cfg); err != nil {
		return "", err
	}
	// Do not assume the user wants to download or silently replace any model.
	for _, role := range []string{"ASR", "LLM"} {
		mode := cfg.ASRServiceMode
		if role == "LLM" {
			mode = cfg.LLMServiceMode
		}
		if mode == serviceLaunch {
			if _, err := servicePlanFor(cfg, role, asrExecRoot()); err != nil {
				return "", fmt.Errorf("%s：%w", role, err)
			}
		}
	}
	blob, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(configPath(), append(blob, '\n'), 0600); err != nil {
		return "", err
	}
	p.app.cfg = cfg
	p.app.logger.Printf("settings: ASR=%s (%s), LLM=%s (%s)", cfg.ASRBaseURL, cfg.ASRServiceMode, cfg.LLMBaseURL, cfg.LLMServiceMode)
	if p.app.models != nil {
		failures := p.app.models.Apply(cfg)
		if len(failures) > 0 {
			for role, err := range failures {
				p.app.logger.Printf("%s launch failed: %v", role, err)
				return "配置已保存，但 " + role + " 启动失败：" + err.Error(), nil
			}
		}
	}
	if cfg.ASRServiceMode == serviceLaunch || cfg.LLMServiceMode == serviceLaunch {
		return "配置已保存，后台模型正在加载；稍后点击测试连接", nil
	}
	return "配置已保存。连接模式下请确保自己的 API 服务可用", nil
}

func (p *uxSettings) browse(target uintptr, title string) {
	filename, ok := uxBrowseFile(p.window, title, uxWinText(target))
	if ok {
		uxSetText(target, filename)
	}
}

func (p *uxSettings) check(kind, base, apiKey string) {
	if p == nil || p.window == 0 {
		return
	}
	uxSetText(p.hint, "正在检测 "+kind+" 服务…")
	h := p.window
	go func() {
		result := uxProbeWithKey(context.Background(), kind, base, apiKey)
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
	className := uxPtr("VoiceRewrite.Settings.v17")
	title := uxPtr("VoiceRewrite · 模型与连接")
	module, _, _ := procGetModuleHandle.Call(0)
	klass := trayWindowClass{Size: uint32(unsafe.Sizeof(trayWindowClass{})), WndProc: uxWindowCallback, Instance: module, ClassName: className}
	atom, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&klass)))
	if atom == 0 && err != syscall.Errno(1410) {
		return fmt.Errorf("无法注册设置窗口: %w", err)
	}
	scale := handyMonitorScale()
	if scale < 1 {
		scale = 1
	}
	sc := func(v int32) uintptr { return uintptr(int32(float64(v)*scale + 0.5)) }
	x, y := uxCenteredWindow(int32(sc(710)), int32(sc(746)))
	hwnd, _, createErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), uxWSCaption|uxWSSysMenu|uxWSMinimizeBox, uintptr(x), uintptr(y), sc(710), sc(746), 0, 0, module, 0)
	if hwnd == 0 {
		return fmt.Errorf("创建设置窗口失败: %w", createErr)
	}
	p := &uxSettings{window: hwnd, app: app}
	uxActiveSettings = p
	p.background, _, _ = procCreateSolidBrush.Call(0x00FBF8F9)
	fontName := uxPtr("Microsoft YaHei UI")
	p.font, _, _ = procCreateFont.Call(sc(-16), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	p.titleFont, _, _ = procCreateFont.Call(sc(-23), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(fontName)))
	stat := uintptr(uxWSChild | uxWSVisible)
	edit := uintptr(uxWSChild | uxWSVisible | uxWSTabStop | uxWSBorder | uxESAutoScroll)
	btn := uintptr(uxWSChild | uxWSVisible | uxWSTabStop)
	makeOne := func(class, text string, style uintptr, x, y, w, h int32, id uintptr, head bool) uintptr {
		font := p.font
		if head {
			font = p.titleFont
		}
		return uxCreateControl(hwnd, class, text, style, int32(sc(x)), int32(sc(y)), int32(sc(w)), int32(sc(h)), id, font)
	}
	paged := func(tab int, class, text string, style uintptr, x, y, w, h int32, id uintptr) uintptr {
		handle := makeOne(class, text, style, x, y, w, h, id, false)
		p.pageControls[tab] = append(p.pageControls[tab], handle)
		return handle
	}
	makeOne("STATIC", "VoiceRewrite · 模型与连接", stat, 24, 15, 610, 36, 0, true)
	makeOne("STATIC", "Mouse4 听写  ·  Mouse5 改写/问答  ·  两个服务可分别选择连接或由程序启动", stat, 26, 55, 660, 25, 0, false)
	makeOne("STATIC", "llama-server.exe 路径（两个托管模型共用；连接已有 API 时可留空）", stat, 26, 90, 660, 26, 0, false)
	p.paths[0] = makeOne("EDIT", app.cfg.LLAMAServerPath, edit, 26, 119, 535, 33, 0, false)
	makeOne("BUTTON", "浏览…", btn, 576, 119, 105, 33, uxBrowseExe, false)
	p.tabs[0] = makeOne("BUTTON", "语音识别 ASR", btn, 26, 172, 186, 36, uxTabASR, false)
	p.tabs[1] = makeOne("BUTTON", "文本处理 LLM", btn, 219, 172, 190, 36, uxTabLLM, false)
	// ASR page
	p.launchChecks[0] = paged(0, "BUTTON", "由 VoiceRewrite 自动启动本机 llama-server (不需要先开服务)", stat|uxBSAutoCheckbox, 28, 222, 600, 30, uxModeASRButton)
	paged(0, "STATIC", "ASR API 地址（连接已有服务时可以是其他电脑）", stat, 28, 264, 550, 24, 0)
	p.inputs[0] = paged(0, "EDIT", app.cfg.ASRBaseURL, edit, 28, 290, 516, 33, 0)
	paged(0, "STATIC", "ASR 模型 ID（HTTP multipart 的 model 参数）", stat, 28, 333, 520, 24, 0)
	p.inputs[1] = paged(0, "EDIT", app.cfg.ASRModel, edit, 28, 357, 647, 33, 0)
	paged(0, "STATIC", "ASR GGUF 文件（自启动模式需要）", stat, 28, 402, 510, 23, 0)
	p.paths[1] = paged(0, "EDIT", app.cfg.ASRModelPath, edit, 28, 426, 516, 32, 0)
	paged(0, "BUTTON", "浏览…", btn, 556, 426, 117, 32, uxBrowseASR)
	paged(0, "STATIC", "音频 mmproj 文件（例如 Qwen3-ASR，其他模型可选）", stat, 28, 465, 560, 23, 0)
	p.paths[2] = paged(0, "EDIT", app.cfg.ASRMMProjPath, edit, 28, 489, 516, 32, 0)
	paged(0, "BUTTON", "浏览…", btn, 556, 489, 117, 32, uxBrowseProjector)
	paged(0, "STATIC", "API Key（可留空）", stat, 28, 530, 270, 22, 0)
	p.keys[0] = paged(0, "EDIT", app.cfg.ASRAPIKey, edit|uxESPassword, 28, 554, 516, 32, 0)
	paged(0, "BUTTON", "测试 ASR", btn, 556, 554, 117, 32, uxProbeASRButton)
	// LLM page
	p.launchChecks[1] = paged(1, "BUTTON", "由 VoiceRewrite 自动启动本机 llama-server (不需要先开服务)", stat|uxBSAutoCheckbox, 28, 222, 600, 30, uxModeLLMButton)
	paged(1, "STATIC", "文本模型 API 地址（连接已有服务时支持远程接口）", stat, 28, 264, 550, 24, 0)
	p.inputs[2] = paged(1, "EDIT", app.cfg.LLMBaseURL, edit, 28, 290, 516, 33, 0)
	paged(1, "STATIC", "文本模型 ID（auto 将查询 /models）", stat, 28, 333, 520, 24, 0)
	p.inputs[3] = paged(1, "EDIT", app.cfg.TextModel, edit, 28, 357, 647, 33, 0)
	paged(1, "STATIC", "文本 GGUF 模型文件（自启动模式需要）", stat, 28, 402, 510, 23, 0)
	p.paths[3] = paged(1, "EDIT", app.cfg.LLMModelPath, edit, 28, 426, 516, 32, 0)
	paged(1, "BUTTON", "浏览…", btn, 556, 426, 117, 32, uxBrowseLLM)
	paged(1, "STATIC", "ASR 和 LLM 可以共用 llama-server.exe，但必须使用不同端口。", stat, 28, 465, 630, 28, 0)
	paged(1, "STATIC", "例如 ASR 127.0.0.1:8080，LLM 127.0.0.1:8082。", stat, 28, 501, 630, 28, 0)
	paged(1, "STATIC", "API Key（可留空）", stat, 28, 530, 270, 22, 0)
	p.keys[1] = paged(1, "EDIT", app.cfg.LLMAPIKey, edit|uxESPassword, 28, 554, 516, 32, 0)
	paged(1, "BUTTON", "测试 LLM", btn, 556, 554, 117, 32, uxProbeLLMButton)
	if app.cfg.ASRServiceMode == serviceLaunch {
		procSendMessage.Call(p.launchChecks[0], uxBMSetCheck, 1, 0)
	}
	if app.cfg.LLMServiceMode == serviceLaunch {
		procSendMessage.Call(p.launchChecks[1], uxBMSetCheck, 1, 0)
	}
	makeOne("BUTTON", "保存并应用", btn, 26, 613, 153, 38, uxSaveButton, false)
	makeOne("BUTTON", "高级参数 JSON", btn, 192, 613, 170, 38, uxAdvancedButton, false)
	makeOne("BUTTON", "关闭", btn, 578, 613, 102, 38, uxCloseButton, false)
	p.hint = makeOne("STATIC", "", stat, 27, 670, 658, 35, 0, false)
	p.switchTab(0)
	rounding := uint32(2)
	procDwmSetWindowAttribute.Call(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, uintptr(unsafe.Pointer(&rounding)), 4)
	procShowWindow.Call(hwnd, SW_SHOW)
	procSetForegroundWindow.Call(hwnd)
	return nil
}
func closeNativeSettings() {
	if uxActiveSettings != nil && uxActiveSettings.window != 0 {
		procDestroyWindow.Call(uxActiveSettings.window)
	}
}
