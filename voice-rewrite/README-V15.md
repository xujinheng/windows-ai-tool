# VoiceRewrite v15 — OneClick ASR cleanup + native settings + UX

Build: `2026-11-14-ux-asr-prefix-v15`

## What's new

1. **OneClick language metadata cleanup in VoiceRewrite.** `cleanASR` uses the same constrained language delimiter pattern as `oneclick/main.go`: `language <name><asr_text>`. It removes each tag, including multiple bilingual segments; it does not simply remove arbitrary strings between `language` and `<asr_text>`. Bare leading `<asr_text>` is also accepted. JSON `{ "text": "..." }`, JSON string, and real `text/plain` HTTP responses are supported. Malformed JSON is never pasted as text.
2. **Native Windows settings window.** Right-click the tray icon → `设置与连接检测…`; edit Qwen3-ASR URL + model, LM Studio URL + text model; test ASR (`GET /health`, as provided by OneClick) and LLM (`GET /models`) without blocking the UI; save and apply model settings immediately. Advanced parameters remain available through the JSON button. Settings does not show API keys or log recorded text.
3. **Clearer recording-mode indicator.** Mouse4 status uses the original rose waveform, Mouse5 uses soft lavender. Both retain the same GDI+ compact pill, DPI handling, cancellation, transcript processing and result popup.
4. **Window polish.** Settings uses system-native editable controls and font, Windows 11 DWM rounded corners, a monitor-centered window and an explicit connection status line. It is still a single Windows Go executable, not Electron/WebView.

## Core flows (unchanged)

- Mouse4 hold → release: **dictation**, no selected text required, ASR → guarded paste at cursor.
- Mouse5 hold → release: selected-text **rewrite/translation** through local LLM, or **explanation** in the AI result card.
- ASR API default `http://127.0.0.1:8081/v1` and LM Studio default `http://127.0.0.1:1234/v1`.

## Important boundaries

- OneClick/llama.cpp launch logic was **not** moved into VoiceRewrite; only transcription response cleanup and parsing were integrated. Continue running an ASR server separately. No new GPU inference engine is embedded.
- The ASR health probe is a **server readiness probe**, not proof that the microphone or chosen ASR model transcribes correctly. An ASR endpoint without `/health` shows a clear unverified status.
- The standard clipboard and SendInput flows remain best effort. `SendInput` success cannot prove that WeChat, VS Code, or a browser actually inserted text.
- As with v14, Windows 11 GUI appearance and focus interactions require real desktop testing. Go tests and Windows cross-compilation prove buildability, not UI correctness.

## Test plan

1. Quit v14 or earlier, place v15 GUI EXE beside your current `config.json`, start it.
2. Tray right click → Settings: verify Chinese labels, no controls clipped at Windows 100/125/150%, and Close returns to tray.
3. Test ASR with OneClick running: check HTTP 200 / model readiness; shut down OneClick and verify nonblocking failure message. Test LM Studio `/v1/models`.
4. Change test ASR/LLM address, Save. Reopen to confirm value is persisted. Restore original address. Test Mouse4 dictation and Mouse5 rewrite and query.
5. With Qwen3-ASR returning language metadata tags from one or more languages, no `language ...<asr_text>` prefixes should end up pasted or sent as an edit instruction.
6. Confirm Mouse4 is rose and Mouse5 is lavender during recording; UI must not steal focus when the status bar appears.

## Licensing

This v15 behavior was reimplemented in Go from the **user's own** MIT/stdlib OneClick repository (`oneclick/main.go`); no external Tauri UI code was copied. Handy Remote is used as a visual and interaction reference only.
