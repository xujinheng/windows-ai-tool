//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"
)

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

// Restore every snapshotted clipboard format when the sequence is still ours.
// Never overwrite a later clipboard change made by the user or another app.
func restoreSelectionClipboardIfOwned(hwnd uintptr, sel selectedText) {
	if sel.ClipboardBackup != nil {
		_ = sel.ClipboardBackup.restoreIfOwned(hwnd)
		return
	}
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
	ctx, cancel := context.WithTimeout(app.epoch.contextFor(generation), 220*time.Second)
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
		// A result card preserves the transcript without requiring the user to
		// guess what happened to their clipboard after a focus change.
		app.rememberResult("听写结果", text)
		app.showAnswer("听写结果 · 未自动插入", text, currentCursorPoint(), nil, false, generation)
		app.postHandyPhase(statusFallback, generation)
		return
	}
	start = time.Now()
	if err = insertDictationText(app.Window, expectedWindow, text, func() bool { return app.epoch.valid(generation) }); err != nil {
		app.fail(err)
		app.rememberResult("听写结果", text)
		app.showAnswer("听写结果 · 请手动复制", text, currentCursorPoint(), nil, false, generation)
		app.postHandyPhase(statusFallback, generation)
		return
	}
	app.rememberResult("听写结果", text)
	app.logger.Printf("stage=paste flow=dictation ms=%d chars=%d status=unverified", time.Since(start).Milliseconds(), utf8.RuneCountInString(text))
	app.setStatus(traySuccess)
	app.postHandyPhase(statusSuccess, generation)
}

// Dictation preserves multi-format clipboard contents. If a format cannot be
// backed up safely, use direct Unicode input rather than changing clipboard.
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
	if nativePasswordFieldFocused(target) {
		return errors.New("密码输入框不接受自动听写，请使用普通输入方式")
	}
	backup, err := snapshotClipboard(hwnd)
	if err != nil {
		// High-quality clipboard support must never destroy a bitmap, virtual
		// file, or an application's private object merely to paste text.
		return sendUnicodeText(text, func() bool { return valid() && isSameForegroundWindow(target) })
	}
	if err = setClipboardText(hwnd, text); err != nil {
		return err
	}
	backup.markOwned(clipboardSeq())
	defer backup.restoreIfOwned(hwnd)
	if !valid() || !isSameForegroundWindow(target) {
		return errors.New("写入前窗口已变化，已取消听写")
	}
	if err = sendCombo('V'); err != nil {
		return err
	}
	time.Sleep(250 * time.Millisecond)
	if err = backup.restoreIfOwned(hwnd); err != nil {
		return fmt.Errorf("已触发粘贴，但剪贴板恢复失败: %w", err)
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
	ctx, cancel := context.WithTimeout(app.epoch.contextFor(generation), 240*time.Second)
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
	app.rememberResult("AI "+answer.Intent, answer.Result)
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

// Dispatch popup paint, command and close events through the Win32 message loop.
