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

const buildID = "2026-11-13-tray-menu-v4"

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	winmm                          = syscall.NewLazyDLL("winmm.dll")
	procRegisterHotKey             = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey           = user32.NewProc("UnregisterHotKey")
	procGetMessage                 = user32.NewProc("GetMessageW")
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
	WM_HOTKEY       = 0x0312
	WM_TIMER        = 0x0113
	RECORD_TIMER_ID = 3001
	WM_HOLD_RELEASE = 0x8000 + 50
	WM_HOLD_TIMEOUT = 0x8000 + 51
	WM_TOGGLE_READY = 0x8000 + 56
	HOTKEY_RECORD   = 100
	HOTKEY_EXIT     = 101
	MOD_ALT         = 0x0001
	MOD_CTRL        = 0x0002
	MOD_NOREPEAT    = 0x4000
	MOD_SHIFT       = 0x0004
	VK_F9           = 0x78
	VK_PAUSE        = 0x13
	VK_F10          = 0x79
	VK_CONTROL      = 0x11
	VK_MENU         = 0x12
	VK_SHIFT        = 0x10
	INPUT_KEYBOARD  = 1
	KEYEVENTF_KEYUP = 0x0002
	CF_UNICODETEXT  = 13
	GMEM_MOVEABLE   = 0x0002
	WAVE_FORMAT_PCM = 1
	WAVE_MAPPER     = ^uintptr(0)
	WHDR_DONE       = 0x00000001
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
	rec *audioRecorder
	sel selectedText
}

type selectedText struct {
	Text                 string
	OriginalClipboard    string
	HadOriginalClipboard bool
	TargetWindow         uintptr
}
type appState struct {
	Window uintptr
	logger *log.Logger
	cfg    Config
	busy   atomic.Bool
	status atomic.Int32
	trayMu sync.Mutex
	tray   *trayIndicator
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

func runApp() {
	runtime.LockOSThread() // RegisterHotKey posts to the registering thread's message queue.
	defer runtime.UnlockOSThread()
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
	app := &appState{Window: hwnd, logger: l, cfg: cfg}
	selectedMouseButton, usingMouse := mouseSideButton(cfg.HotkeyRecord)
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
	defer func() {
		app.trayMu.Lock()
		indicator.close()
		app.tray = nil
		app.trayMu.Unlock()
	}()
	l.Printf("started; build=%s mode=%s hotkey=%s ASR=%s LLM=%s (model=%s)", buildID, cfg.RecordMode, cfg.HotkeyRecord, cfg.ASRBaseURL, cfg.LLMBaseURL, cfg.TextModel)
	// The tray icon is the UI. Do not steal focus with an instructional MessageBox.
	// Install the hook on its own locked OS thread; the UI remains independent.
	if usingMouse {
		mouseHook, hookErr := installSideMouseHook(hwnd, selectedMouseButton)
		if hookErr != nil {
			winErr(hookErr.Error())
			return
		}
		defer mouseHook.close()
		l.Printf("mouse hook installed: button=%d", selectedMouseButton)
	}
	var active *audioRecorder
	var selected selectedText
	toggleResult := make(chan togglePrepareResult, 1)
	var holdTargetWindow uintptr
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
		if cfg.RecordMode == "hold" {
			go processRecordedHold(app, rec, holdTargetWindow)
		} else {
			go processRecorded(app, rec, selected)
		}
	}

	for {
		var msg winMSG
		rv, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(rv) <= 0 {
			break
		}
		if msg.Message == WM_TOGGLE_READY {
			result := <-toggleResult
			active = result.rec
			selected = result.sel
			app.busy.Store(false)
			app.setStatus(trayRecording)
			l.Printf("recording started, mode=toggle, selected characters=%d", utf8.RuneCountInString(selected.Text))
			continue
		}
		if msg.Message == WM_TRAY_UPDATE {
			state := trayState(app.status.Load())
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
			if err := shellOpenLocalFile(configPath()); err != nil {
				app.fail(err)
			}
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
				app.fail(errors.New("未收到录音按键或鼠标侧键的松开事件，已自动结束。请检查是否有鼠标驱动将侧键映射为其他按键。"))
			}
			continue
		}
		if usingMouse && msg.Message == WM_SIDE_UP {
			if cfg.RecordMode == "hold" && active != nil {
				l.Print("mouse side button released; completing recording")
				finish()
			}
			continue
		}
		if usingMouse && msg.Message == WM_SIDE_DOWN {
			msg.Message = WM_HOTKEY
			msg.WParam = HOTKEY_RECORD
		}
		if msg.Message != WM_HOTKEY {
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
				win, _, _ := procGetForegroundWindow.Call()
				if win == 0 {
					app.fail(errors.New("没有可用的编辑窗口"))
					continue
				}
				rec, e := startRecorder(cfg.MaxRecordSeconds)
				if e != nil {
					app.fail(e)
					continue
				}
				holdTargetWindow = win
				active = rec
				app.setStatus(trayRecording)
				holdSession++
				holdWatch = make(chan struct{})
				if usingMouse {
					// Mouse release is delivered by the low-level hook. Only the
					// deadline guard is necessary (no F-key polling).
					go watchMouseHoldTimeout(hwnd, holdSession, holdWatch, time.Duration(cfg.MaxRecordSeconds+2)*time.Second, l)
				} else {
					go watchHoldRelease(hwnd, recordVK, holdSession, holdWatch, time.Duration(cfg.MaxRecordSeconds+2)*time.Second, l)
				}
				beep(900, 60)
				l.Printf("recording started, mode=hold hotkey=%s", cfg.HotkeyRecord)
				continue
			}
			// Legacy toggle mode also prepares its selection on a worker.
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
					app.fail(err)
					app.busy.Store(false)
					return
				}
				rec, err := startRecorder(cfg.MaxRecordSeconds)
				if err != nil {
					app.fail(err)
					app.busy.Store(false)
					return
				}
				toggleResult <- togglePrepareResult{rec: rec, sel: sel}
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

// The selection must be copied AFTER releasing the held shortcut. This function
// runs off the UI and low-level mouse-hook threads, and enforces a small time budget.
func processRecordedHold(app *appState, rec *audioRecorder, expectedWindow uintptr) {
	started := time.Now()
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
	// We already own the busy flag and error recovery: avoid a second outer worker.
	processAudioAndRewrite(app, rec, sel)
}

func processRecorded(app *appState, rec *audioRecorder, sel selectedText) {
	defer app.busy.Store(false)
	defer func() {
		if reason := recover(); reason != nil {
			app.logger.Printf("worker panic: %v\n%s", reason, debug.Stack())
			app.fail(fmt.Errorf("录音处理线程发生错误：%v", reason))
		}
	}()
	processAudioAndRewrite(app, rec, sel)
}

func processAudioAndRewrite(app *appState, rec *audioRecorder, sel selectedText) {
	app.logger.Print("worker: waiting for recording buffer")
	r := <-rec.result
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
	app.logger.Printf("stage=asr ms=%d", time.Since(startedASR).Milliseconds())
	if e != nil {
		app.fail(e)
		return
	}
	app.setStatus(trayRewriting)
	app.logger.Printf("ASR instruction decoded; chars=%d", utf8.RuneCountInString(instruction))
	startedLLM := time.Now()
	rewritten, e := rewrite(ctx, app.cfg, sel.Text, instruction)
	app.logger.Printf("stage=llm ms=%d", time.Since(startedLLM).Milliseconds())
	if e != nil {
		app.fail(e)
		return
	}
	app.logger.Printf("worker: rewrite generated %d characters; attempting paste", utf8.RuneCountInString(rewritten))
	startedPaste := time.Now()
	e = replaceSelection(app.Window, sel, rewritten, app.cfg)
	app.logger.Printf("stage=paste ms=%d", time.Since(startedPaste).Milliseconds())
	if e != nil {
		app.fail(e)
		return
	}
	app.setStatus(traySuccess)
	beep(1050, 110)
	app.logger.Printf("rewrite success; resulting chars=%d", utf8.RuneCountInString(rewritten))
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
	if e != nil {
		// Do not blindly overwrite the clipboard if another app/user changed it.
		return sel, e
	}
	sel.Text = text
	return sel, nil
}
func replaceSelection(hwnd uintptr, sel selectedText, replacement string, cfg Config) error {
	if !waitKeysReleased(3 * time.Second) {
		return errors.New("请松开 Ctrl/Alt 等快捷键后再试")
	}
	fg, _, _ := procGetForegroundWindow.Call()
	valid, _, _ := procIsWindow.Call(sel.TargetWindow)
	if valid == 0 || fg != sel.TargetWindow {
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
	if e = setClipboardText(hwnd, replacement); e != nil {
		return e
	}
	ownedSeq := clipboardSeq()
	if e = sendCombo('V'); e != nil {
		return e
	}
	// Small grace interval for applications that read the clipboard asynchronously.
	time.Sleep(350 * time.Millisecond)
	if sel.HadOriginalClipboard && clipboardSeq() == ownedSeq {
		if e := setClipboardText(hwnd, sel.OriginalClipboard); e != nil {
			return fmt.Errorf("改写已粘贴，但恢复文本剪贴板失败: %w", e)
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
