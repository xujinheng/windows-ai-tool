# VoiceRewrite v11 — one Handy-like status bar for both mouse buttons

The **same compact, native Win32/GDI Status Bar** now appears for both flows:

| Key | Purpose | Status bar steps | Delivery |
| --- | --- | --- | --- |
| Mouse4 (XBUTTON1) | Direct dictation without selecting text | recording (live mic level and elapsed time) → transcribing → sent/error | Ctrl+V at the caret |
| Mouse5 (XBUTTON2) | Selected-text voice rewrite, translation or AI question | recording (same waveform and timer) → reading selection → recognizing instruction → AI processing → replacing or result shown | Replace selection, or open an answer card for questions/fallback |

The overlay uses one drawing implementation and one lifecycle for both shortcuts. The Win32 timer/paint WndProc remains on the application's UI thread, so background ASR/LLM requests do not block paint or mouse hooks. The status window uses `WS_EX_NOACTIVATE`, staying out of the target editor's focus. A matching epoch is required for each queued UI status message, so an older task cannot overwrite the newer task's status bar. Clicking the status bar's X cancels the **active** task of either flow; clicking X after the status is completed only closes the bar and does not invalidate an already-open answer card. A successful result shows for ~1.5s and fades by closing; the result card stays open separately.

### What did *not* change

- Mouse4 remains ASR-only, not an LLM command.
- Mouse5 retains the v10 edit-first selected-text routing, clipboard verification and fallback result card.
- `config.json` and the local ASR/LM Studio service endpoints remain unchanged.
- The overlay remains a native visual adaptation inspired by `Hyroniem/handy-remote`, not an embedded Tauri/WebView component and not a pixel-identical Handy copy.

### How to test on Windows 11

1. Close older VoiceRewrite versions and Handy, then start this v11 EXE next to your existing `config.json`. Startup log must say `build=2026-11-14-unified-status-v11`.
2. Open Notepad or WeChat, focus the insertion point, hold **Mouse4** for 2–3 seconds. The Handy-like waveform/timer should appear; release, watch `正在转写…`, and verify that the transcript is inserted at the caret.
3. Select a line in VS Code, hold **Mouse5**, say `请写成英文`, and release. The **same** waveform/timer should appear, followed by `读取选区…`, `识别指令…`, `AI 处理中…` and `正在替换…`, ending in `已发送改写`. Verify the original text is actually replaced (not just that `SendInput` was accepted).
4. In a browser, select a term and use **Mouse5** to ask `这是什么意思？`. The status bar should report `已显示结果` while the independent answer card opens; the bar then disappears and the card remains usable.
5. Press the overlay's X during Mouse4 recording and during Mouse5 recording/processing. In both cases no result should be pasted. Press X after `已显示结果` and confirm it closes only the status pill, not the result card.
6. Confirm the bar does not steal focus, and switching to another window while the model processes does not paste the result into that other window.

**Testing limit:** Go tests (`go test ./...`), race detector, vet, and Windows x64 cross-build pass locally. Windows 11 mouse-hook, GDI rendering and external-app paste behavior still require real-desktop acceptance.

### Attribution

The compact status bar's style and behavior are inspired by Handy Remote's MIT-licensed `RecordingOverlay.tsx` / `.css`, copyright 2025 CJ Pais. This application reimplements the overlay in Go/Win32 without embedding source UI assets; see `THIRD_PARTY_NOTICES.md`.
