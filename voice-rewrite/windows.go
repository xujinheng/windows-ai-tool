//go:build windows

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

var buildID = "0.24.0-dev" // overwritten by -ldflags -X at build time

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	winmm                          = syscall.NewLazyDLL("winmm.dll")
	procRegisterHotKey             = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey           = user32.NewProc("UnregisterHotKey")
	procGetMessage                 = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessage            = user32.NewProc("DispatchMessageW")
	procSetTimer                   = user32.NewProc("SetTimer")
	procKillTimer                  = user32.NewProc("KillTimer")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procIsWindow                   = user32.NewProc("IsWindow")
	procSendInput                  = user32.NewProc("SendInput")
	procGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procMessageBox                 = user32.NewProc("MessageBoxW")
	procCreateWindow               = user32.NewProc("CreateWindowExW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procGlobalAlloc                = kernel32.NewProc("GlobalAlloc")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalFree                 = kernel32.NewProc("GlobalFree")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
	procCreateMutex                = kernel32.NewProc("CreateMutexW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procBeep                       = kernel32.NewProc("Beep")
	procWaveInOpen                 = winmm.NewProc("waveInOpen")
	procWaveInPrepareHeader        = winmm.NewProc("waveInPrepareHeader")
	procWaveInUnprepareHeader      = winmm.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer            = winmm.NewProc("waveInAddBuffer")
	procWaveInStart                = winmm.NewProc("waveInStart")
	procWaveInStop                 = winmm.NewProc("waveInStop")
	procWaveInReset                = winmm.NewProc("waveInReset")
	procWaveInClose                = winmm.NewProc("waveInClose")
)

const (
	WM_HOTKEY        = 0x0312
	WM_TIMER         = 0x0113
	RECORD_TIMER_ID  = 3001
	WM_HOLD_RELEASE  = 0x8000 + 50
	WM_HOLD_TIMEOUT  = 0x8000 + 51
	WM_TOGGLE_READY  = 0x8000 + 56
	WM_SHOW_ANSWER   = 0x8000 + 61
	WM_APPLY_RESULT  = 0x8000 + 65
	WM_STATUS_UPDATE = 0x8000 + 70
	WM_STATUS_CANCEL = 0x8000 + 71
	HOTKEY_RECORD    = 100
	HOTKEY_EXIT      = 101
	HOTKEY_DICTATION = 102
	MOD_ALT          = 0x0001
	MOD_CTRL         = 0x0002
	MOD_NOREPEAT     = 0x4000
	MOD_SHIFT        = 0x0004
	VK_F9            = 0x78
	VK_PAUSE         = 0x13
	VK_F10           = 0x79
	VK_CONTROL       = 0x11
	VK_MENU          = 0x12
	VK_SHIFT         = 0x10
	INPUT_KEYBOARD   = 1
	KEYEVENTF_KEYUP  = 0x0002
	CF_UNICODETEXT   = 13
	GMEM_MOVEABLE    = 0x0002
	WAVE_FORMAT_PCM  = 1
	WAVE_MAPPER      = ^uintptr(0)
	WHDR_DONE        = 0x00000001
)

type winMSG struct {
	Hwnd     uintptr
	Message  uint32
	_        uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	X        int32
	Y        int32
	LPrivate uint32
}
type winKEYBDINPUT struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}
type winINPUT struct {
	Type uint32
	_    uint32
	Key  winKEYBDINPUT
	_    [8]byte
}
type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}
type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

type togglePrepareResult struct {
	rec        *audioRecorder
	sel        selectedText
	generation uint64
}

type selectedText struct {
	Text                 string
	OriginalClipboard    string
	HadOriginalClipboard bool
	TargetWindow         uintptr
	CopyClipboardSeq     uintptr // clipboard after our simulated Ctrl+C
	ClipboardBackup      *clipboardSnapshot
	Anchor               trayPoint
}
type appState struct {
	Window          uintptr
	logger          *log.Logger
	cfg             Config
	busy            atomic.Bool
	epoch           taskEpoch
	status          atomic.Int32
	trayMu          sync.Mutex
	tray            *trayIndicator
	popups          chan popupMessage
	lastErrorMu     sync.Mutex
	lastError       string
	lastResultMu    sync.Mutex
	lastResult      string
	lastResultTitle string
	paused          atomic.Bool
}

// Shell notification APIs and icon lifetime are owned by the Windows UI thread.
// Worker goroutines update only atomic state and post a message to that thread.
// This prevents tray-icon manipulation concurrently with window shutdown.
func (a *appState) setStatus(s trayState) {
	a.status.Store(int32(s))
	a.logger.Printf("status=%s", trayInfo(s).label)
	if ok, _, err := procPostMessage.Call(a.Window, WM_TRAY_UPDATE, 0, 0); ok == 0 {
		a.logger.Printf("failed to queue tray update: %v", err)
	}
}

// Worker threads never mutate native windows. All phase updates are owned by
// the UI message pump and tagged with the recording epoch to discard late events.
func (a *appState) postHandyPhase(phase uintptr, generation uint64) {
	if a.epoch.valid(generation) {
		procPostMessage.Call(a.Window, WM_STATUS_UPDATE, phase, uintptr(generation))
	}
}

func (a *appState) showAnswer(title, body string, anchor trayPoint, selection *selectedText, allowApply bool, generation uint64) {
	if !a.epoch.valid(generation) {
		return
	}
	msg := popupMessage{title: title, body: body, anchor: anchor, app: a, selection: selection, allowApply: allowApply, generation: generation}
	select {
	case a.popups <- msg:
	default:
		a.logger.Print("popup queue full; dropping result")
		return
	}
	if ok, _, err := procPostMessage.Call(a.Window, WM_SHOW_ANSWER, 0, 0); ok == 0 {
		a.logger.Printf("cannot show popup: %v", err)
	}
}

func (a *appState) rememberResult(title, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	a.lastResultMu.Lock()
	a.lastResultTitle, a.lastResult = title, value
	a.lastResultMu.Unlock()
}

func winErr(msg string) {
	title, _ := syscall.UTF16PtrFromString("VoiceRewrite - 语音改写")
	content, _ := syscall.UTF16PtrFromString(msg)
	procMessageBox.Call(0, uintptr(unsafe.Pointer(content)), uintptr(unsafe.Pointer(title)), 0x00000030|0x00040000)
	runtime.KeepAlive(content)
	runtime.KeepAlive(title)
}
func beep(freq, dur uintptr) { procBeep.Call(freq, dur) }

func (a *appState) fail(e error) {
	if e == nil {
		return
	}
	a.logger.Printf("ERROR: %v", e)
	a.lastErrorMu.Lock()
	a.lastError = friendlyError(e)
	a.lastErrorMu.Unlock()
	a.setStatus(trayError) // nonmodal; available via tray's last-error command
}

var hotkeyPattern = regexp.MustCompile(`(?i)^Ctrl\+Alt\+(Shift\+)?F([1-9]|1[0-9]|2[0-4])$`)
var plainFKeyPattern = regexp.MustCompile(`(?i)^F([1-9]|1[0-9]|2[0-4])$`)

func parseHotkey(s string) (uintptr, uintptr, error) {
	if match := plainFKeyPattern.FindStringSubmatch(strings.TrimSpace(s)); match != nil {
		num, _ := strconv.Atoi(match[1])
		return MOD_NOREPEAT, uintptr(0x70 + num - 1), nil
	}
	if strings.EqualFold(strings.TrimSpace(s), "Pause") || strings.EqualFold(strings.TrimSpace(s), "Break") {
		return MOD_NOREPEAT, VK_PAUSE, nil
	}
	match := hotkeyPattern.FindStringSubmatch(strings.TrimSpace(s))
	if match == nil {
		return 0, 0, fmt.Errorf("invalid hotkey %q: use F9, Pause, Ctrl+Alt+F9 or Ctrl+Alt+Shift+F9", s)
	}
	num, _ := strconv.Atoi(match[2])
	mods := uintptr(MOD_CTRL | MOD_ALT | MOD_NOREPEAT)
	if match[1] != "" {
		mods |= MOD_SHIFT
	}
	return mods, uintptr(0x70 + num - 1), nil
}

var recordVirtualKey uintptr = VK_F9

// A named Windows mutex prevents two app instances from swallowing the same
// mouse side-button. We hold the handle until process exit.
func singleInstanceGuard() (func(), error) {
	name, err := syscall.UTF16PtrFromString("Local\\VoiceRewrite_OneInstance_v1")
	if err != nil {
		return nil, err
	}
	handle, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	runtime.KeepAlive(name)
	if handle == 0 {
		return nil, fmt.Errorf("CreateMutexW: %w", callErr)
	}
	if callErr == syscall.Errno(183) {
		procCloseHandle.Call(handle)
		return nil, errors.New("VoiceRewrite 已在运行，请先从托盘退出旧版本（或在任务管理器结束旧进程）")
	}
	return func() { procCloseHandle.Call(handle) }, nil
}

func runApp() {
	runtime.LockOSThread() // RegisterHotKey posts to the registering thread's message queue.
	defer runtime.UnlockOSThread()
	releaseInstance, singleErr := singleInstanceGuard()
	if singleErr != nil {
		winErr(singleErr.Error())
		return
	}
	defer releaseInstance()
	logDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "VoiceRewrite")
	if logDir == "VoiceRewrite" {
		logDir = os.TempDir()
	}
	_ = os.MkdirAll(logDir, 0700)
	f, e := os.OpenFile(filepath.Join(logDir, "VoiceRewrite.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		f = os.Stderr
	} else {
		defer f.Close()
	}
	l := log.New(f, "", log.LstdFlags)
	defer func() {
		if rec := recover(); rec != nil {
			l.Printf("PANIC: %v\n%s", rec, debug.Stack())
			winErr(fmt.Sprint("程序错误: ", rec))
		}
	}()
	_, configBefore := os.Stat(configPath())
	firstRun := os.IsNotExist(configBefore)
	cfg, e := loadConfig()
	if e != nil {
		winErr(e.Error())
		return
	}
	hwnd, e := createTrayCallbackWindow()
	if e != nil {
		winErr("无法初始化 Windows 托盘消息窗口：" + e.Error())
		return
	}
	defer procDestroyWindow.Call(hwnd)
	app := &appState{Window: hwnd, logger: l, cfg: cfg, popups: make(chan popupMessage, 4)}
	defer app.epoch.shutdown()
	defer closeActiveAnswer()
	defer closeNativeSettings()
	defer closeHandyStatus()
	activeHandyAppWindow = hwnd
	selectedMouseButton, usingMouse := mouseSideButton(cfg.HotkeyRecord)
	dictationButton, dictationMouse := mouseSideButton(cfg.DictationHotkey)
	var dictationVK uintptr
	if !dictationMouse {
		mods, key, parseErr := parseHotkey(cfg.DictationHotkey)
		if parseErr != nil || mods != MOD_NOREPEAT || key == VK_PAUSE {
			winErr("听写快捷键请选择 Mouse4、Mouse5 或 F1–F24 单独功能键")
			return
		}
		dictationVK = key
	}
	if dictationMouse && usingMouse && dictationButton == selectedMouseButton {
		winErr("听写和改写不能占用同一个鼠标侧键")
		return
	}
	if cfg.RecordMode != "hold" && !dictationMouse {
		winErr("键盘听写快捷键当前只支持按住说话模式（record_mode=hold）")
		return
	}
	var recordMods, recordVK uintptr
	if usingMouse {
		if cfg.RecordMode != "hold" {
			winErr("Mouse4/Mouse5 目前仅支持 record_mode=hold")
			return
		}
	} else {
		recordMods, recordVK, e = parseHotkey(cfg.HotkeyRecord)
		if e != nil {
			winErr(e.Error())
			return
		}
	}
	exitMods, exitVK, e := parseHotkey(cfg.HotkeyExit)
	if e != nil {
		winErr(e.Error())
		return
	}
	if (!usingMouse && recordMods == exitMods && recordVK == exitVK) ||
		(!dictationMouse && recordVK != 0 && dictationVK == recordVK) ||
		(!dictationMouse && exitMods == MOD_NOREPEAT && exitVK == dictationVK) {
		winErr("Record and exit hotkeys must differ")
		return
	}
	if cfg.RecordMode == "hold" && !usingMouse && (recordMods != MOD_NOREPEAT || recordVK == VK_PAUSE) {
		winErr("按住录音模式请使用 F1~F24 等单独的功能键，例如 F9。Pause/Break 通常没有可靠的松开事件，Ctrl+Alt 组合键会干扰复制选中文字。")
		return
	}
	if !usingMouse {
		recordVirtualKey = recordVK
		if rv, _, _ := procRegisterHotKey.Call(0, HOTKEY_RECORD, recordMods, recordVK); rv == 0 {
			winErr("Cannot register record hotkey: " + cfg.HotkeyRecord + " (already in use?)")
			return
		}
		defer procUnregisterHotKey.Call(0, HOTKEY_RECORD)
	}
	if !dictationMouse {
		if rv, _, _ := procRegisterHotKey.Call(0, HOTKEY_DICTATION, MOD_NOREPEAT, dictationVK); rv == 0 {
			winErr("无法注册听写快捷键：" + cfg.DictationHotkey + "（可能已被其他程序占用）")
			return
		}
		defer procUnregisterHotKey.Call(0, HOTKEY_DICTATION)
	}
	if rv, _, _ := procRegisterHotKey.Call(0, HOTKEY_EXIT, exitMods, exitVK); rv == 0 {
		winErr("Cannot register exit hotkey: " + cfg.HotkeyExit)
		return
	}
	defer procUnregisterHotKey.Call(0, HOTKEY_EXIT)
	indicator, err := newTrayIndicator(hwnd, &app.paused)
	if err != nil {
		winErr("无法创建系统托盘图标: " + err.Error())
		return
	}
	app.tray = indicator
	defer func() {
		app.trayMu.Lock()
		indicator.close()
		app.tray = nil
		app.trayMu.Unlock()
	}()
	l.Printf("started; build=%s mode=%s hotkey=%s dictation=%s", buildID, cfg.RecordMode, cfg.HotkeyRecord, cfg.DictationHotkey)
	if firstRun {
		l.Print("first start: opening setup and service checks")
		if err := showNativeSettings(app); err != nil {
			l.Printf("first start settings: %v", err)
		}
	}
	// The tray icon is the UI. Do not steal focus with an instructional MessageBox.
	// Install the hook on its own locked OS thread; the UI remains independent.
	if usingMouse || dictationMouse {
		mouseHook, hookErr := installSideMouseHook(hwnd, selectedMouseButton, dictationButton, &app.paused)
		if hookErr != nil {
			winErr(hookErr.Error())
			return
		}
		defer mouseHook.close()
		l.Printf("mouse hook installed: rewrite=%d dictation=%d", selectedMouseButton, dictationButton)
	}
	var active *audioRecorder
	var selected selectedText
	toggleResult := make(chan togglePrepareResult, 1)
	var holdTargetWindow uintptr
	var holdButton uint16
	var holdIsDictation bool
	var holdGeneration uint64
	var selectedGeneration uint64
	var holdSession uintptr
	var holdWatch chan struct{}
	stopHoldWatch := func() {
		if holdWatch != nil {
			close(holdWatch)
			holdWatch = nil
		}
	}
	defer stopHoldWatch()
	finish := func() {
		if active == nil {
			return
		}
		if cfg.RecordMode == "hold" {
			stopHoldWatch()
		}
		rec := active
		active = nil
		rec.Stop()
		app.busy.Store(true) // no second operation until the worker is finished
		app.setStatus(trayTranscribing)
		l.Print("recording stop requested; selection/model processing moved to worker")
		if cfg.RecordMode == "hold" && holdIsDictation {
			app.postHandyPhase(statusProcessing, holdGeneration)
			go processDictationHold(app, rec, holdTargetWindow, holdGeneration)
		} else if cfg.RecordMode == "hold" {
			app.postHandyPhase(statusSelection, holdGeneration)
			go processRecordedHold(app, rec, holdTargetWindow, currentCursorPoint(), holdGeneration)
		} else {
			sel := selected
			sel.Anchor = currentCursorPoint()
			go processRecorded(app, rec, sel, selectedGeneration)
		}
	}

	// One capture engine, two explicitly chosen workflows. Never infer dictation
	// from Ctrl+C: VS Code may copy an entire line without any selection.
	startHold := func(dictation bool, button uint16) {
		if app.paused.Load() || app.busy.Load() || active != nil {
			beep(320, 90)
			return
		}
		if uxActiveSettings != nil && isSameForegroundWindow(uxActiveSettings.window) {
			// Never dictate into VoiceRewrite's own settings form.
			return
		}
		win, _, _ := procGetForegroundWindow.Call()
		if win == 0 {
			app.fail(errors.New("没有可用的目标窗口"))
			return
		}
		rec, e := startRecorder(cfg.MaxRecordSeconds)
		if e != nil {
			app.fail(e)
			return
		}
		holdTargetWindow = win
		holdButton = button
		holdIsDictation = dictation
		holdGeneration = app.epoch.start()
		active = rec
		app.setStatus(trayRecording)
		holdSession++
		holdWatch = make(chan struct{})
		if button != 0 { // mouse button release delivered by low level hook
			go watchMouseHoldTimeout(hwnd, holdSession, holdWatch, time.Duration(cfg.MaxRecordSeconds+2)*time.Second, l)
		} else {
			vk := recordVK
			if dictation {
				vk = dictationVK
			}
			go watchHoldRelease(hwnd, vk, holdSession, holdWatch, time.Duration(cfg.MaxRecordSeconds+2)*time.Second, l)
		}
		// Identical Handy-style pill for both flows; label/phase is mode-specific.
		statusMode := "改写"
		if dictation {
			statusMode = "听写"
		}
		showHandyStatus(rec, statusMode, statusRecording, holdGeneration)
		beep(900, 60)
		l.Printf("recording started, mode=hold flow=%s button=%d", map[bool]string{true: "dictation", false: "rewrite"}[dictation], button)
	}

	for {
		var msg winMSG
		rv, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(rv) <= 0 {
			break
		}
		if msg.Message == WM_STATUS_UPDATE {
			if app.epoch.valid(uint64(msg.LParam)) && activeHandyStatus != nil && activeHandyStatus.owner == uint64(msg.LParam) {
				updateHandyStatus(msg.WParam)
			}
			continue
		}
		if msg.Message == WM_STATUS_CANCEL {
			if !app.epoch.valid(uint64(msg.WParam)) || activeHandyStatus == nil || activeHandyStatus.owner != uint64(msg.WParam) {
				continue // a cancel click queued by a previous session
			}
			if isHandyTerminalPhase(activeHandyStatus.phase) {
				closeHandyStatus() // close a completed status, not the answer card
				continue
			}
			app.epoch.start() // cancel either Mouse4 or Mouse5 before paste
			if active != nil {
				stopHoldWatch()
				rec := active
				active = nil
				rec.Stop()
				go func() { <-rec.result }()
			}
			closeHandyStatus()
			app.setStatus(trayIdle)
			continue
		}
		if msg.Message == WM_APPLY_RESULT {
			if activeAnswer != nil && activeAnswer.window != 0 && msg.WParam == activeAnswer.window {
				popup := activeAnswer
				if popup.applyResult != nil {
					select {
					case err := <-popup.applyResult:
						if err == nil {
							l.Print("popup apply: selection replaced")
							closeActiveAnswer()
						} else {
							l.Printf("popup apply failed: %v", err)
							popup.setNote("无法安全替换，请使用复制结果")
						}
					default:
					}
				}
			}
			continue
		}
		if msg.Message == WM_SHOW_ANSWER {
			select {
			case answer := <-app.popups:
				if !app.epoch.valid(answer.generation) {
					l.Print("discarded stale popup result")
					continue
				}
				if err := showAnswerPopup(answer); err != nil {
					l.Printf("popup window: %v", err)
					app.fail(err)
				} else {
					app.postHandyPhase(statusAnswer, answer.generation)
					l.Printf("popup native window displayed; chars=%d", utf8.RuneCountInString(answer.body))
				}
			default:
			}
			continue
		}
		if msg.Message == WM_TOGGLE_READY {
			result := <-toggleResult
			if !app.epoch.valid(result.generation) {
				result.rec.Stop()
				go func() { <-result.rec.result }()
				continue
			}
			selectedGeneration = result.generation
			active = result.rec
			selected = result.sel
			app.busy.Store(false)
			app.setStatus(trayRecording)
			l.Printf("recording started, mode=toggle, selected characters=%d", utf8.RuneCountInString(selected.Text))
			continue
		}
		if msg.Message == WM_TRAY_UPDATE {
			state := trayState(app.status.Load())
			if state == trayError && activeHandyStatus != nil && !isHandyTerminalPhase(activeHandyStatus.phase) && app.epoch.valid(activeHandyStatus.owner) {
				updateHandyStatus(statusError)
			}
			// Always run Shell_NotifyIconW on the thread that owns the tray window.
			if err := indicator.notify(state, false); err != nil {
				l.Printf("tray state update failed: %v", err)
			}
			procKillTimer.Call(0, TRAY_RESET_TIMER_ID)
			if state == traySuccess || state == trayError {
				procSetTimer.Call(0, TRAY_RESET_TIMER_ID, 4500, 0)
			}
			continue
		}
		if msg.Message == WM_TIMER && msg.WParam == TRAY_RESET_TIMER_ID {
			procKillTimer.Call(0, TRAY_RESET_TIMER_ID)
			state := trayState(app.status.Load())
			if state == traySuccess || state == trayError {
				if app.paused.Load() {
					app.setStatus(trayPaused)
				} else {
					app.setStatus(trayIdle)
				}
			}
			continue
		}
		if msg.Message == WM_TRAY_EVENT {
			event := uint32(msg.LParam & 0xffff)
			if trayMenuRequested(event) {
				l.Printf("tray: context menu requested event=0x%04x", event)
				command, menuErr := indicator.contextMenu()
				if menuErr != nil {
					l.Printf("tray: menu failed: %v", menuErr)
				} else {
					l.Printf("tray: menu dismissed command=%d", command)
				}
			}
			continue
		}
		if msg.Message == WM_TRAY_CONFIG {
			if err := showNativeSettings(app); err != nil {
				app.fail(err)
			}
			continue
		}
		if msg.Message == WM_TRAY_CLOSE_POPUP {
			closeActiveAnswer()
			l.Print("popup closed through tray menu")
			continue
		}
		if msg.Message == WM_TRAY_TOGGLE_PAUSE {
			if app.busy.Load() {
				winErr("当前任务仍在处理；结束后可暂停语音操作。")
				continue
			}
			newPaused := !app.paused.Load()
			app.paused.Store(newPaused)
			if newPaused {
				app.setStatus(trayPaused)
			} else {
				app.setStatus(trayIdle)
			}
			continue
		}
		if msg.Message == WM_TRAY_LAST_RESULT {
			app.lastResultMu.Lock()
			title, value := app.lastResultTitle, app.lastResult
			app.lastResultMu.Unlock()
			if value == "" {
				winErr("当前会话还没有可恢复的语音结果。")
			} else {
				app.showAnswer(title+" · 历史结果", value, currentCursorPoint(), nil, false, app.epoch.current())
			}
			continue
		}
		if msg.Message == WM_TRAY_LAST_ERROR {
			app.lastErrorMu.Lock()
			latest := app.lastError
			app.lastErrorMu.Unlock()
			if latest == "" {
				latest = "目前没有记录到错误。"
			}
			winErr(latest)
			continue
		}
		if msg.Message == WM_TRAY_LOG {
			if err := shellOpenLocalFile(filepath.Join(logDir, "VoiceRewrite.log")); err != nil {
				app.fail(err)
			}
			continue
		}
		if indicator.taskbarCreated != 0 && msg.Message == indicator.taskbarCreated {
			if err := indicator.notify(trayState(app.status.Load()), true); err != nil {
				l.Printf("tray re-add failed: %v", err)
			}
			continue
		}
		if msg.Message == WM_TRAY_EXIT || (msg.Message == WM_HOTKEY && msg.WParam == HOTKEY_EXIT) {
			closeHandyStatus()
			if active != nil {
				stopHoldWatch()
				active.Stop()
				go func(rec *audioRecorder) { <-rec.result }(active)
			}
			l.Print("stopped via exit action")
			return
		}
		if msg.Message == WM_HOLD_RELEASE {
			if cfg.RecordMode == "hold" && active != nil && msg.WParam == holdSession {
				l.Print("hold key released; completing recording")
				finish()
			}
			continue
		}
		if msg.Message == WM_HOLD_TIMEOUT {
			if cfg.RecordMode == "hold" && active != nil && msg.WParam == holdSession {
				stopHoldWatch()
				rec := active
				active = nil
				rec.Stop()
				go func() { <-rec.result }()
				l.Print("ERROR: hold watcher exceeded maximum duration without observing key release")
				app.postHandyPhase(statusError, holdGeneration)
				app.fail(errors.New("未收到录音按键或鼠标侧键的松开事件，已自动结束。请检查是否有鼠标驱动将侧键映射为其他按键。"))
			}
			continue
		}
		if msg.Message == WM_SIDE_UP {
			if cfg.RecordMode == "hold" && active != nil && uint16(msg.WParam) == holdButton {
				l.Printf("mouse button %d released; completing recording", holdButton)
				finish()
			}
			continue
		}
		if msg.Message == WM_SIDE_DOWN {
			button := uint16(msg.WParam)
			switch sideButtonFlow(button, selectedMouseButton, dictationButton) {
			case "dictation":
				startHold(true, button)
			case "rewrite":
				startHold(false, button)
			}
			continue
		}
		if msg.Message != WM_HOTKEY {
			// All popup paint, close, mouse, keyboard and child WM_COMMAND
			// messages MUST be dispatched to their registered WndProc.
			// The v5 implementation silently discarded them, leaving blank,
			// impossible-to-close windows.
			dispatchNativeMessage(&msg)
			continue
		}
		switch msg.WParam {
		case HOTKEY_EXIT:
			if active != nil {
				stopHoldWatch()
				active.Stop()
				go func(rec *audioRecorder) { <-rec.result }(active)
			}
			l.Print("stopped via exit hotkey")
			return
		case HOTKEY_DICTATION:
			if !app.paused.Load() {
				startHold(true, 0)
			}
			continue
		case HOTKEY_RECORD:
			if app.paused.Load() {
				continue
			}
			if app.busy.Load() {
				beep(320, 90)
				continue
			}
			if active != nil {
				if cfg.RecordMode == "toggle" {
					finish()
				}
				continue
			}
			// Hold mode records immediately and does NOT touch the clipboard
			// while the hotkey is physically held. Capture is performed on release.
			if cfg.RecordMode == "hold" {
				startHold(false, selectedMouseButton)
				continue
			}
			// Legacy toggle mode also prepares its selection on a worker.
			generation := app.epoch.start()
			app.busy.Store(true)
			app.setStatus(trayTranscribing)
			go func() {
				if !waitKeysReleased(2500 * time.Millisecond) {
					app.fail(errors.New("请松开快捷键后重试"))
					app.busy.Store(false)
					return
				}
				begin := time.Now()
				sel, err := captureSelection(hwnd, cfg)
				l.Printf("stage=selection mode=toggle ms=%d", time.Since(begin).Milliseconds())
				if err == nil && utf8.RuneCountInString(sel.Text) > cfg.MaxSelectedRunes {
					err = fmt.Errorf("选中文字超过 %d 字限制", cfg.MaxSelectedRunes)
				}
				if err != nil {
					restoreSelectionClipboardIfOwned(hwnd, sel)
					app.fail(err)
					app.busy.Store(false)
					return
				}
				rec, err := startRecorder(cfg.MaxRecordSeconds)
				if err != nil {
					restoreSelectionClipboardIfOwned(hwnd, sel)
					app.fail(err)
					app.busy.Store(false)
					return
				}
				if !app.epoch.valid(generation) {
					restoreSelectionClipboardIfOwned(hwnd, sel)
					rec.Stop()
					go func() { <-rec.result }()
					app.busy.Store(false)
					return
				}
				toggleResult <- togglePrepareResult{rec: rec, sel: sel, generation: generation}
				if ok, _, _ := procPostMessage.Call(hwnd, WM_TOGGLE_READY, 0, 0); ok == 0 {
					rec.Stop()
					go func() { <-rec.result }()
					app.fail(errors.New("无法通知 UI 线程准备录音"))
					app.busy.Store(false)
				}
			}()
		}
	}
}

// watchHoldRelease polls physical key state on its own goroutine instead of relying
// on WM_TIMER messages sharing the GUI message queue. The main UI thread owns all
// state transitions, after a WM_APP message is posted to its hidden window.
// The deadline is a second defense against indefinitely stuck red tray status.

var procIsDialogMessage = user32.NewProc("IsDialogMessageW")

func dispatchNativeMessage(msg *winMSG) {
	if uxActiveSettings != nil && uxActiveSettings.window != 0 {
		if uxSettingsKeyMessage(*msg) {
			return
		}
		if handled, _, _ := procIsDialogMessage.Call(uxActiveSettings.window, uintptr(unsafe.Pointer(msg))); handled != 0 {
			return
		}
	}
	if shouldClosePopupForKey(*msg) {
		closeActiveAnswer()
		return
	}
	procTranslateMessage.Call(uintptr(unsafe.Pointer(msg)))
	procDispatchMessage.Call(uintptr(unsafe.Pointer(msg)))
}
