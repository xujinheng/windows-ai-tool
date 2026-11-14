## v8 edit-first acceptance checklist

1. WeChat compose box: select plain text, say "请写成英文". Expect replaced text, not a card.
2. VS Code editor: select a sentence, say "润色一下". Expect Ctrl+V replacement.
3. ChatGPT / Chrome contenteditable input: select text, say "请写成英文". Expect direct replacement if the selection remains valid.
4. Read-only webpage article: select a term, say "这是什么意思". Expect an answer card, no paste.
5. For read-only article, explicitly say "请写成英文". Expect guarded paste attempt followed by preview if the selection is unchanged. This is heuristic; use only test text.
6. Switch focus during LLM processing: must NOT overwrite the wrong editor.
7. Context menu, Escape, Copy, Close and Apply buttons remain functional.
8. Check `route=try_replace` and `stage=paste` timing; native controls use `route=replace`.

# Windows 11 acceptance checklist — Intent Popup v6

1. Exit all old VoiceRewrite processes. Ensure the latest startup log shows `build=2026-11-13-intent-popup-v6`.
2. In Notepad, select some text; hold Mouse5 and say `请写成英文`; release. Confirm the translated text replaces selection, not a popup (if the focused control is reported as a native editable control).
3. On a web page, select a technical term; say `这是什么意思`. Confirm a nonempty explanation popup appears near the pointer and the webpage remains unchanged.
4. Close the popup with title-bar X; repeat and close with bottom Close button; repeat and choose Copy and Close; repeat and use tray Close AI Result Window.
5. Select read-only webpage text; say `翻译成中文`. Confirm the translation is shown in a copyable popup, not pasted into the webpage.
6. Test a browser textarea. It can be classified as unknown and show a preview; this is a deliberate safety limitation.
7. Open and exit the tray menu, verify the Mouse5 button returns to its usual functionality after program exit.
8. Check the log for `stage=selection`, `stage=asr`, `stage=llm`, `intent`, `route` and `popup` events. No selected or dictated user content is logged.
9. Test ASR errors, tiny recordings, focus switching, and rapid double presses; verify no freezing and no empty popup.
10. To exercise the native popup lifecycle with Go tests on an interactive Windows desktop, set `VOICE_REWRITE_NATIVE_POPUP_TEST=1` and run `go test -run TestNativePopupPaintAndClose -v`.

## v7 acceptance (requires an interactive Windows 11 desktop)

1. Close old VoiceRewrite processes, launch v7 and check build ID in log.
2. In Notepad select Chinese text, say `请写成英文`, release Mouse5. Expect direct replacement, no card.
3. In Chrome/Edge inside an editable ChatGPT draft textbox, select text, say `请写成英文` or `润色一下`. Expect direct paste when validation succeeds; otherwise a card with `应用到原文`.
4. Select static article text in the browser, say `这是什么意思`. Expect answer card, no editing, and no `应用到原文` button.
5. Select static article text, say `改得简洁一点`. The guarded paste probe may fail; expect a preview rather than a false success. Revisit this scenario after an accessibility-based editability check is implemented.
6. Click card `复制结果`, `关闭`, top-right X, Esc, and tray `关闭 AI 结果窗口` independently.
7. Click `应用到原文` after switching selection/window and confirm it refuses to overwrite the wrong content.
8. Check the card on multi-monitor / different DPI, narrow or long result texts, and inspect `route=...` and `stage=paste` log entries.

Do not assume successful go test or Windows cross-compilation verifies browser contenteditable behavior.

## v10 Dual Side Buttons / Handy-style Dictation (MANUAL TESTS, NOT YET VERIFIED)

1. In WeChat compose area, put the caret at the end with NO selected text. Hold Mouse4, speak "你好今天下午开会", release. Confirm ASR text is inserted at caret, with no LLM call or answer card.
2. Repeat Mouse4 in VS Code and ChatGPT input field. Confirm no selection is needed and same target window remains active.
3. Select a passage in WeChat, VS Code and browser; hold Mouse5 and say "翻译成英文". Confirm original selected text is replaced (v9 core regression check).
4. Select a technical term in a web article; Mouse5 "这是什么意思". Confirm answer appears in card and web document is not modified.
5. Hold Mouse4, speak, release; confirm pill shows waveform/timer and then spinner, then success or error, and auto-hides. Click X while recording to cancel.
6. During Mouse4, try Mouse5: must not switch modes or stop current recording. Release the original Mouse4: completes Mouse4 only. Reverse test for Mouse5.
7. During Mouse4 ASR processing, change foreground window: result must NOT automatically insert into new app (copy-to-clipboard fallback).
8. Close all old VoiceRewrite/Handy instances before starting. Ensure only one low-level hook is active.
9. Revisit image / rich clipboard limitation and verify whether current text-only clipboard preservation impacts your workflow.
10. Review `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log` for `flow=dictation`, `stage=asr`, and `stage=paste` (unverified).

Native status lifecycle test: `VOICE_REWRITE_NATIVE_STATUS_TEST=1 go test -run TestHandyStatusLifecycle` on an interactive Windows desktop.

## v11 regression: unified Handy-style status bar

- Mouse4: recording pill with 9 reactive bars/timer, ASR spinner, then sent/error and auto-dismiss.
- Mouse5 edit: **same pill** from the first press, then selection, ASR instruction, AI, paste, sent/error; selected text still replaced in WeChat/VS Code/Chrome.
- Mouse5 question: same pill, answer card appears, `已显示结果` then pill dismisses; card remains open and closable.
- X while either key is recording / worker is processing: cancels the correct task without inserting stale text.
- X after success / answer: closes only the pill, not the model answer card.
- Press other side button while recording: no second recording / status overlay; only matching release finishes.
- Switch focus mid-processing: must not paste into another app; status eventually dismisses with error.
- Multi-monitor and DPI 125%/150%: bar stays within current work area and must not steal focus.

## v12 smooth Status Bar acceptance (not yet executed on Windows 11)

- **S01**: Mouse4 + Mouse5 show the **same** status bar while recording.
- **S02**: 9 waveform bars have rounded anti-aliased edges; the capsule silhouette has no obvious stepped cutout.
- **S03**: The `00:xx` timer is clearly separated from the X icon, including when elapsed time exceeds one minute.
- **S04**: `识别指令…`, `AI 处理中…`, `正在替换…` states render readable Chinese glyphs rather than blanks/squares.
- **S05**: At 100%, 125%, 150%, 200% scaling, pill/labels are crisp, correctly positioned, and not clipped.
- **S06**: Overlay remains no-activate; typing and paste are still sent to the original editor.
- **S07**: Clicking X while recording/processing cancels; X after completion only dismisses the status, not an answer card.
- **S08**: Mouse5 auto-replace still works in WeChat, VS Code, Chrome and does not unexpectedly revert to popup-only.
