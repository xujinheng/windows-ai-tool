//go:build windows

package main

import (
	"bytes"
	"context"
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
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

const buildID = "2026-11-14-public-beta-v16"

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
	Anchor               trayPoint
}
type appState struct {
	Window uintptr
	logger *log.Logger
	cfg    Config
	busy   atomic.Bool
	epoch  taskEpoch
	status atomic.Int32
	trayMu sync.Mutex
	tray   *trayIndicator
	popups chan popupMessage
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
	a.setStatus(trayError) // non-modal: tooltip + log via tray menu
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
	if !dictationMouse || dictationButton == selectedMouseButton {
		winErr("dictation_hotkey 必须是未被改写占用的 Mouse4 或 Mouse5")
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
	if !usingMouse && recordMods == exitMods && recordVK == exitVK {
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
	if rv, _, _ := procRegisterHotKey.Call(0, HOTKEY_EXIT, exitMods, exitVK); rv == 0 {
		winErr("Cannot register exit hotkey: " + cfg.HotkeyExit)
		return
	}
	defer procUnregisterHotKey.Call(0, HOTKEY_EXIT)
	indicator, err := newTrayIndicator(hwnd)
	if err != nil {
		winErr("无法创建系统托盘图标: " + err.Error())
		return
	}
	app.tray = indicator
	stopASR := startManagedASR(app.cfg, l)
	defer stopASR()
	defer func() {
		app.trayMu.Lock()
		indicator.close()
		app.tray = nil
		app.trayMu.Unlock()
	}()
	l.Printf("started; build=%s mode=%s hotkey=%s ASR=%s LLM=%s (model=%s)", buildID, cfg.RecordMode, cfg.HotkeyRecord, cfg.ASRBaseURL, cfg.LLMBaseURL, cfg.TextModel)
	if firstRun {
		l.Print("first start: opening setup and service checks")
		if err := showNativeSettings(app); err != nil {
			l.Printf("first start settings: %v", err)
		}
	}
	// The tray icon is the UI. Do not steal focus with an instructional MessageBox.
	// Install the hook on its own locked OS thread; the UI remains independent.
	if usingMouse || dictationMouse {
		mouseHook, hookErr := installSideMouseHook(hwnd, selectedMouseButton, dictationButton)
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
		if app.busy.Load() || active != nil {
			beep(320, 90)
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
			go watchHoldRelease(hwnd, recordVK, holdSession, holdWatch, time.Duration(cfg.MaxRecordSeconds+2)*time.Second, l)
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
				app.setStatus(trayIdle)
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
		case HOTKEY_RECORD:
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
func watchMouseHoldTimeout(hwnd, session uintptr, canceled <-chan struct{}, timeout time.Duration, logger *log.Logger) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-canceled:
		return
	case <-timer.C:
		logger.Printf("mouse hold timed out; session=%d", session)
		if ok, _, err := procPostMessage.Call(hwnd, WM_HOLD_TIMEOUT, session, 0); ok == 0 {
			logger.Printf("ERROR: cannot post mouse timeout: %v", err)
		}
	}
}

func watchHoldRelease(hwnd, vk, session uintptr, canceled <-chan struct{}, timeout time.Duration, logger *log.Logger) {
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	start := time.Now()
	releasedTicks := 0
	polls := 0
	logger.Printf("hold watcher started; session=%d", session)
	for {
		select {
		case <-canceled:
			return
		case <-deadline.C:
			logger.Printf("hold watcher deadline reached; session=%d", session)
			if ok, _, err := procPostMessage.Call(hwnd, WM_HOLD_TIMEOUT, session, 0); ok == 0 {
				logger.Printf("ERROR: cannot post hold timeout: %v", err)
			}
			return
		case <-tick.C:
			pressedValue, _, _ := procGetAsyncKeyState.Call(vk)
			polls++
			if polls == 40 || polls%80 == 0 {
				logger.Printf("hold watcher state; session=%d down=%t elapsed=%s", session, pressedValue&0x8000 != 0, time.Since(start).Truncate(time.Millisecond))
			}
			if releaseDebounced(pressedValue&0x8000 != 0, &releasedTicks) {
				logger.Printf("hold watcher detected key release; session=%d elapsed=%s", session, time.Since(start).Truncate(time.Millisecond))
				if ok, _, err := procPostMessage.Call(hwnd, WM_HOLD_RELEASE, session, 0); ok == 0 {
					logger.Printf("ERROR: cannot post hold release: %v", err)
				}
				return
			}
		}
	}
}

// Restore only pre-existing plain text, and only when no other clipboard
// write happened after our most recent known Ctrl+C. Non-text data is NOT
// preserved by this helper; see the manual test checklist for that limitation.
func restoreSelectionClipboardIfOwned(hwnd uintptr, sel selectedText) {
	if mayRestoreTextClipboard(sel.HadOriginalClipboard, sel.CopyClipboardSeq, clipboardSeq()) {
		_ = setClipboardText(hwnd, sel.OriginalClipboard)
	}
}

// Mouse4 never performs Ctrl+C or model-based intent routing. Handy-style
// dictation inserts raw ASR output at the captured window's current caret.
func processDictationHold(app *appState, rec *audioRecorder, expectedWindow uintptr, generation uint64) {
	defer app.busy.Store(false)
	defer func() {
		if x := recover(); x != nil {
			app.logger.Printf("dictation panic: %v", x)
			app.fail(fmt.Errorf("听写处理失败: %v", x))
			app.postHandyPhase(statusError, generation)
		}
	}()
	r := <-rec.result
	if !app.epoch.valid(generation) {
		return
	}
	if r.err != nil || len(r.pcm) < 3200 {
		if r.err == nil {
			r.err = errors.New("语音太短，无法转写")
		}
		app.fail(r.err)
		app.postHandyPhase(statusError, generation)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 95*time.Second)
	defer cancel()
	start := time.Now()
	text, err := transcribe(ctx, app.cfg, r.pcm)
	if !app.epoch.valid(generation) {
		return
	}
	app.logger.Printf("stage=asr flow=dictation ms=%d", time.Since(start).Milliseconds())
	if err != nil {
		app.fail(err)
		app.postHandyPhase(statusError, generation)
		return
	}
	if !app.epoch.valid(generation) {
		return
	}
	// Focus may have changed while ASR ran: never insert in another app.
	if !isSameForegroundWindow(expectedWindow) {
		if copyErr := setClipboardText(app.Window, text); copyErr != nil {
			app.fail(fmt.Errorf("原窗口已切换，听写结果复制失败：%w", copyErr))
			app.postHandyPhase(statusError, generation)
		} else {
			app.fail(errors.New("原窗口已切换：听写结果已复制到剪贴板，请手动粘贴"))
			app.postHandyPhase(statusFallback, generation)
		}
		return
	}
	start = time.Now()
	if err = insertDictationText(app.Window, expectedWindow, text, func() bool { return app.epoch.valid(generation) }); err != nil {
		app.fail(err)
		app.postHandyPhase(statusError, generation)
		return
	}
	app.logger.Printf("stage=paste flow=dictation ms=%d chars=%d status=unverified", time.Since(start).Milliseconds(), utf8.RuneCountInString(text))
	app.setStatus(traySuccess)
	app.postHandyPhase(statusSuccess, generation)
}

// Dictation does not require a selected text. Restore only clipboard text we
// still own; previous rich formats are not yet preserved by the v9 backend.
func insertDictationText(hwnd, target uintptr, text string, valid func() bool) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("听写结果为空")
	}
	if !waitKeysReleased(2 * time.Second) {
		return errors.New("请松开快捷键后重试")
	}
	if !valid() || !isSameForegroundWindow(target) {
		return errors.New("写入前窗口已变化，已取消听写")
	}
	old, existed, err := getClipboardText(hwnd)
	if err != nil {
		return err
	}
	if err = setClipboardText(hwnd, text); err != nil {
		return err
	}
	owned := clipboardSeq()
	if !valid() || !isSameForegroundWindow(target) {
		if mayRestoreTextClipboard(existed, owned, clipboardSeq()) {
			_ = setClipboardText(hwnd, old)
		}
		return errors.New("写入前窗口已变化，已取消听写")
	}
	if err = sendCombo('V'); err != nil {
		if mayRestoreTextClipboard(existed, owned, clipboardSeq()) {
			_ = setClipboardText(hwnd, old)
		}
		return err
	}
	time.Sleep(250 * time.Millisecond)
	if mayRestoreTextClipboard(existed, owned, clipboardSeq()) {
		if err = setClipboardText(hwnd, old); err != nil {
			return fmt.Errorf("已触发粘贴，但剪贴板恢复失败: %w", err)
		}
	}
	return nil // SendInput accepted; insertion cannot be proven in all editors.
}

// The selection must be copied AFTER releasing the held shortcut. This function
// runs off the UI and low-level mouse-hook threads, and enforces a small time budget.
func processRecordedHold(app *appState, rec *audioRecorder, expectedWindow uintptr, anchor trayPoint, generation uint64) {
	started := time.Now()
	if !app.epoch.valid(generation) {
		rec.Stop()
		go func() { <-rec.result }()
		return
	}
	defer app.busy.Store(false)
	defer func() {
		if reason := recover(); reason != nil {
			app.logger.Printf("hold worker panic: %v\n%s", reason, debug.Stack())
			app.fail(fmt.Errorf("录音处理线程发生错误：%v", reason))
		}
	}()
	if !isSameForegroundWindow(expectedWindow) {
		rec.Stop()
		go func() { <-rec.result }()
		app.fail(errors.New("编辑窗口已经变化，取消自动替换"))
		return
	}
	sel, err := captureSelection(app.Window, app.cfg)
	sel.Anchor = anchor
	defer restoreSelectionClipboardIfOwned(app.Window, sel)
	app.logger.Printf("stage=selection mode=hold ms=%d", time.Since(started).Milliseconds())
	if err == nil && sel.TargetWindow != expectedWindow {
		err = errors.New("选区所属窗口已变化")
	}
	if err == nil && utf8.RuneCountInString(sel.Text) > app.cfg.MaxSelectedRunes {
		err = fmt.Errorf("选中文字超过 %d 字限制", app.cfg.MaxSelectedRunes)
	}
	if err != nil {
		go func() { <-rec.result }()
		app.fail(err)
		return
	}
	app.logger.Printf("selection captured; chars=%d", utf8.RuneCountInString(sel.Text))
	app.postHandyPhase(statusProcessing, generation)
	if !app.epoch.valid(generation) {
		return
	}
	// We already own the busy flag and error recovery: avoid a second outer worker.
	processAudioAndRewrite(app, rec, sel, generation)
}

func processRecorded(app *appState, rec *audioRecorder, sel selectedText, generation uint64) {
	defer app.busy.Store(false)
	defer restoreSelectionClipboardIfOwned(app.Window, sel)
	defer func() {
		if reason := recover(); reason != nil {
			app.logger.Printf("worker panic: %v\n%s", reason, debug.Stack())
			app.fail(fmt.Errorf("录音处理线程发生错误：%v", reason))
		}
	}()
	processAudioAndRewrite(app, rec, sel, generation)
}

func processAudioAndRewrite(app *appState, rec *audioRecorder, sel selectedText, generation uint64) {
	if !app.epoch.valid(generation) {
		return
	}
	app.logger.Print("worker: waiting for recording buffer")
	r := <-rec.result
	if !app.epoch.valid(generation) {
		return
	}
	if r.err != nil {
		app.fail(r.err)
		return
	}
	if len(r.pcm) < 3200 {
		app.fail(errors.New("录音太短（至少约 0.1 秒）；请再试一次"))
		return
	}
	app.logger.Printf("audio capture complete; %.2f seconds", float64(len(r.pcm))/32000)
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	startedASR := time.Now()
	instruction, e := transcribe(ctx, app.cfg, r.pcm)
	if !app.epoch.valid(generation) {
		return
	}
	app.logger.Printf("stage=asr ms=%d", time.Since(startedASR).Milliseconds())
	if e != nil {
		app.fail(e)
		return
	}
	app.setStatus(trayRewriting)
	app.postHandyPhase(statusRewriting, generation)
	app.logger.Printf("ASR instruction decoded; chars=%d", utf8.RuneCountInString(instruction))
	startedLLM := time.Now()
	answer, e := completeSelectedTask(ctx, app.cfg, sel.Text, instruction)
	if !app.epoch.valid(generation) {
		return
	}
	app.logger.Printf("stage=llm ms=%d intent=%s", time.Since(startedLLM).Milliseconds(), answer.Intent)
	if e != nil {
		app.fail(e)
		return
	}
	// The selected text is the user's editing surface regardless of its Win32
	// class (WeChat/VS Code/Chrome may expose only a renderer HWND). The voice
	// command remains the primary signal. If a model mislabels a clear edit as
	// an answer, regenerate as plain replacement text before touching the editor.
	if (clearlyRequestsEdit(instruction) || clearlyRequestsTranslation(instruction)) &&
		answer.Intent != "edit" && answer.Intent != "translate" {
		app.logger.Printf("intent conflict; voice requires edit but model returned %s; retrying rewrite", answer.Intent)
		fixed, rewriteErr := rewrite(ctx, app.cfg, sel.Text, instruction)
		if rewriteErr != nil {
			app.fail(fmt.Errorf("模型误判意图，重新生成改写文字失败: %w", rewriteErr))
			return
		}
		if !app.epoch.valid(generation) {
			return
		}
		answer.Result = fixed
		answer.Intent = "edit"
	}
	editable := isSafelyEditableControl(sel.TargetWindow)
	browserSurface := isBrowserEditorSurface(sel.TargetWindow)
	action := routeTask(answer, instruction, editable, browserSurface)
	app.logger.Printf("route=%s intent=%s native_editable=%t browser_surface=%t voice_edit=%t voice_translate=%t", action, answer.Intent, editable, browserSurface, clearlyRequestsEdit(instruction), clearlyRequestsTranslation(instruction))
	allowApply := action == "replace" || action == "try_replace" || answer.Intent == "edit" || answer.Intent == "translate"
	if action == "replace" || action == "try_replace" {
		app.postHandyPhase(statusApplying, generation)
		startedPaste := time.Now()
		// All unknown controls receive the same pre/post paste checks as a browser.
		e = replaceSelectionWithGuard(app.Window, sel, answer.Result, app.cfg, action == "try_replace", func() bool { return app.epoch.valid(generation) })
		app.logger.Printf("stage=paste ms=%d", time.Since(startedPaste).Milliseconds())
		if e == nil {
			app.setStatus(traySuccess)
			app.postHandyPhase(statusSuccess, generation)
			beep(1050, 110)
			app.logger.Printf("paste_status=unverified intent=%s result_chars=%d (SendInput accepted, editor insertion not provable)", answer.Intent, utf8.RuneCountInString(answer.Result))
			return
		}
		app.logger.Printf("auto replacement failed; preview instead: %v", e)
	}
	title := "AI 解释"
	switch answer.Intent {
	case "edit":
		title = "改写预览"
	case "summarize":
		title = "AI 摘要"
	case "translate":
		title = "翻译结果"
	case "ambiguous":
		title = "AI 回答"
	}
	selectionCopy := sel
	app.showAnswer(title, answer.Result, sel.Anchor, &selectionCopy, allowApply, generation)
	app.setStatus(traySuccess)
	app.logger.Printf("popup result queued; chars=%d", utf8.RuneCountInString(answer.Result))
}

// Standard Windows message dispatch is essential for interactive popup controls.
// In the v5 prototype the queue was drained without this call, so WM_PAINT,
// WM_COMMAND and WM_CLOSE were never delivered to the popup WndProc.
func dispatchNativeMessage(msg *winMSG) {
	if shouldClosePopupForKey(*msg) {
		closeActiveAnswer()
		return
	}
	procTranslateMessage.Call(uintptr(unsafe.Pointer(msg)))
	procDispatchMessage.Call(uintptr(unsafe.Pointer(msg)))
}

func waitKeysReleased(max time.Duration) bool {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		keys := []uintptr{VK_CONTROL, VK_MENU, VK_SHIFT, recordVirtualKey}
		down := false
		for _, k := range keys {
			v, _, _ := procGetAsyncKeyState.Call(k)
			if v&0x8000 != 0 {
				down = true
				break
			}
		}
		if !down {
			time.Sleep(60 * time.Millisecond)
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func sendCombo(letter uint16) error {
	keys := [4]winINPUT{
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: VK_CONTROL}},
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: letter}},
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: letter, Flags: KEYEVENTF_KEYUP}},
		{Type: INPUT_KEYBOARD, Key: winKEYBDINPUT{Vk: VK_CONTROL, Flags: KEYEVENTF_KEYUP}},
	}
	if unsafe.Sizeof(keys[0]) != 40 {
		return fmt.Errorf("INPUT layout invalid: %d", unsafe.Sizeof(keys[0]))
	}
	n, _, e := procSendInput.Call(uintptr(len(keys)), uintptr(unsafe.Pointer(&keys[0])), unsafe.Sizeof(keys[0]))
	runtime.KeepAlive(keys)
	if n != 4 {
		return fmt.Errorf("SendInput 失败(%d/4): %v。管理员权限的目标程序可能不接受普通权限模拟键盘", n, e)
	}
	return nil
}
func clipboardSeq() uintptr { n, _, _ := procGetClipboardSequenceNumber.Call(); return n }
func openClipboard(hwnd uintptr) error {
	for i := 0; i < 20; i++ {
		ok, _, _ := procOpenClipboard.Call(hwnd)
		if ok != 0 {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("无法打开剪贴板（其他程序可能正在占用）")
}
func getClipboardText(hwnd uintptr) (string, bool, error) {
	if e := openClipboard(hwnd); e != nil {
		return "", false, e
	}
	defer procCloseClipboard.Call()
	h, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if h == 0 {
		return "", false, nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return "", false, errors.New("读取剪贴板失败")
	}
	defer procGlobalUnlock.Call(h)
	// Limit reads to the actual allocation instead of assuming a huge buffer.
	allocationSize, _, _ := procGlobalSize.Call(h)
	if allocationSize < 2 || allocationSize > 8*1024*1024 {
		return "", false, errors.New("剪贴板文本长度异常")
	}
	data := unsafe.Slice((*uint16)(unsafe.Pointer(p)), int(allocationSize/2))
	n := 0
	for n < len(data) && data[n] != 0 {
		n++
	}
	if n == len(data) {
		return "", false, errors.New("剪贴板文本长度异常")
	}
	text := string(utf16.Decode(data[:n]))
	return text, true, nil
}
func setClipboardText(hwnd uintptr, s string) error {
	if e := openClipboard(hwnd); e != nil {
		return e
	}
	defer procCloseClipboard.Call()
	chars := utf16.Encode([]rune(s))
	chars = append(chars, 0)
	size := uintptr(len(chars) * 2)
	h, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if h == 0 {
		return errors.New("剪贴板内存分配失败")
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return errors.New("无法锁定剪贴板内存")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(p)), len(chars)), chars)
	procGlobalUnlock.Call(h)
	if ok, _, _ := procEmptyClipboard.Call(); ok == 0 {
		procGlobalFree.Call(h)
		return errors.New("无法清空剪贴板")
	}
	if out, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, h); out == 0 {
		procGlobalFree.Call(h)
		return errors.New("无法写入剪贴板")
	}
	return nil
}

func clipboardCopy(hwnd uintptr, timeout time.Duration) (string, error) {
	old := clipboardSeq()
	if err := sendCombo('C'); err != nil {
		return "", err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if clipboardSeq() != old {
			s, ok, e := getClipboardText(hwnd)
			if e == nil && ok && strings.TrimSpace(s) != "" {
				return s, nil
			}
		}
		time.Sleep(35 * time.Millisecond)
	}
	return "", errors.New("未获取到选中文字。请先在原窗口选中文字再按快捷键；某些受保护或管理员窗口无法自动复制")
}
func isSameForegroundWindow(target uintptr) bool {
	if target == 0 {
		return false
	}
	foreground, _, _ := procGetForegroundWindow.Call()
	valid, _, _ := procIsWindow.Call(target)
	return valid != 0 && foreground == target
}

// Copy only while the original editor is foreground. Some applications update the
// clipboard asynchronously, so retry without crossing application boundaries.
func copySelectionWithRetry(hwnd, target uintptr, cfg Config) (string, error) {
	time.Sleep(35 * time.Millisecond) // Let mouse key-up and selection settle.
	var lastErr error
	for i := 0; i < cfg.SelectionCopyAttempts; i++ {
		if !isSameForegroundWindow(target) {
			return "", errors.New("编辑窗口已切换，已停止自动复制，避免读取其他窗口的文字")
		}
		text, err := clipboardCopy(hwnd, time.Duration(cfg.SelectionCopyTimeoutMS)*time.Millisecond)
		if !isSameForegroundWindow(target) {
			return "", errors.New("复制期间编辑窗口发生切换，已停止自动复制")
		}
		if err == nil && strings.TrimSpace(text) != "" {
			return text, nil
		}
		lastErr = err
		if i+1 < cfg.SelectionCopyAttempts {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
		}
	}
	return "", fmt.Errorf("复制选中文字失败（已尝试 %d 次）：%w", cfg.SelectionCopyAttempts, lastErr)
}

func captureSelection(hwnd uintptr, cfg Config) (selectedText, error) {
	var sel selectedText
	sel.TargetWindow, _, _ = procGetForegroundWindow.Call()
	if sel.TargetWindow == 0 {
		return sel, errors.New("找不到当前编辑窗口")
	}
	old, exists, e := getClipboardText(hwnd)
	if e != nil {
		return sel, e
	}
	sel.OriginalClipboard = old
	sel.HadOriginalClipboard = exists
	text, e := copySelectionWithRetry(hwnd, sel.TargetWindow, cfg)
	sel.CopyClipboardSeq = clipboardSeq()
	if e != nil {
		// Do not blindly overwrite the clipboard if another app/user changed it.
		return sel, e
	}
	sel.Text = text
	return sel, nil
}
func replaceSelection(hwnd uintptr, sel selectedText, replacement string, cfg Config) error {
	return replaceSelectionWithVerification(hwnd, sel, replacement, cfg, false)
}

// Unknown editor types (Chrome/Edge/WeChat/VS Code and more) cannot be
// classified reliably via Win32 HWND. First verify the selected text and
// foreground window; then paste and check whether the same original selection
// remains. If it does, the surface likely refused the paste, so show a preview.
// This remains a heuristic: it cannot prove an application's content changed.
func replaceSelectionWithVerification(hwnd uintptr, sel selectedText, replacement string, cfg Config, verifyBrowser bool) error {
	return replaceSelectionWithGuard(hwnd, sel, replacement, cfg, verifyBrowser, nil)
}

func replaceSelectionWithGuard(hwnd uintptr, sel selectedText, replacement string, cfg Config, verifyBrowser bool, valid func() bool) error {
	if valid != nil && !valid() {
		return errors.New("任务已过期，禁止覆盖原文")
	}
	if !waitKeysReleased(3 * time.Second) {
		return errors.New("请松开 Ctrl/Alt 等快捷键后再试")
	}
	fg, _, _ := procGetForegroundWindow.Call()
	windowValid, _, _ := procIsWindow.Call(sel.TargetWindow)
	if windowValid == 0 || fg != sel.TargetWindow {
		if e := setClipboardText(hwnd, replacement); e != nil {
			return e
		}
		return errors.New("编辑窗口已经切换；为了避免覆盖错误位置，改写内容已复制到剪贴板，请返回原窗口手动粘贴。")
	}
	// Re-check selection; otherwise Ctrl+V might insert into a different place.
	actual, e := copySelectionWithRetry(hwnd, sel.TargetWindow, cfg)
	if e != nil || actual != sel.Text {
		if e2 := setClipboardText(hwnd, replacement); e2 != nil {
			return e2
		}
		return errors.New("原来的选中文字已经变化；为避免误覆盖，结果已复制到剪贴板。请手动粘贴。")
	}
	if valid != nil && !valid() {
		restoreSelectionClipboardIfOwned(hwnd, selectedText{
			HadOriginalClipboard: sel.HadOriginalClipboard, OriginalClipboard: sel.OriginalClipboard,
			CopyClipboardSeq: clipboardSeq(),
		})
		return errors.New("任务已过期，禁止覆盖原文")
	}
	if !isSameForegroundWindow(sel.TargetWindow) {
		return errors.New("粘贴前编辑窗口已变化，已取消覆盖")
	}
	if e = setClipboardText(hwnd, replacement); e != nil {
		return e
	}
	ownedSeq := clipboardSeq()
	if valid != nil && !valid() {
		restoreSelectionClipboardIfOwned(hwnd, selectedText{HadOriginalClipboard: sel.HadOriginalClipboard, OriginalClipboard: sel.OriginalClipboard, CopyClipboardSeq: ownedSeq})
		return errors.New("任务已过期，禁止粘贴")
	}
	if e = sendCombo('V'); e != nil {
		return e
	}
	// Small grace interval for applications that read the clipboard asynchronously.
	time.Sleep(270 * time.Millisecond)
	if verifyBrowser {
		if !isSameForegroundWindow(sel.TargetWindow) {
			return errors.New("粘贴后窗口切换，无法验证结果")
		}
		// A post-paste Ctrl+C itself may overwrite the clipboard; track its
		// sequence as part of our own operation instead of losing the backup.
		beforeProbe := clipboardSeq()
		stillSelected, verifyErr := clipboardCopy(hwnd, 470*time.Millisecond)
		afterProbe := clipboardSeq()
		if verifyErr == nil && afterProbe != beforeProbe {
			ownedSeq = afterProbe
		}
		if verifyErr == nil && stillSelected == sel.Text {
			if mayRestoreTextClipboard(sel.HadOriginalClipboard, ownedSeq, clipboardSeq()) {
				_ = setClipboardText(hwnd, sel.OriginalClipboard)
			}
			return errors.New("原始文字仍被选中，可能是只读内容；改为预览")
		}
		if afterProbe != beforeProbe && verifyErr != nil {
			// Unknown clipboard writer: never overwrite that new value.
			ownedSeq = 0
		}
	}
	if mayRestoreTextClipboard(sel.HadOriginalClipboard, ownedSeq, clipboardSeq()) {
		if e := setClipboardText(hwnd, sel.OriginalClipboard); e != nil {
			return fmt.Errorf("改写已发送，但恢复纯文本剪贴板失败: %w", e)
		}
	}
	return nil
}

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
func startRecorder(maxSec int) (*audioRecorder, error) {
	f := waveFormatEx{FormatTag: WAVE_FORMAT_PCM, Channels: 1, SamplesPerSec: 16000, AvgBytesPerSec: 32000, BlockAlign: 2, BitsPerSample: 16}
	var h uintptr
	rc, _, _ := procWaveInOpen.Call(uintptr(unsafe.Pointer(&h)), WAVE_MAPPER, uintptr(unsafe.Pointer(&f)), 0, 0, 0)
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
