# VoiceRewrite v10 — Handy Remote Mouse4 + VoiceRewrite Mouse5

## Usage

- **Mouse4 (XBUTTON1)** — press and hold to dictate, release to transcribe and insert at the current caret. No selection is needed. Uses Qwen3-ASR alone; **never calls the text LLM**.
- **Mouse5 (XBUTTON2)** — unchanged VoiceRewrite selected-text workflow: select text, hold Mouse5, speak an edit/translation command (paste result), or ask a question (answer card).
- **Single tray application and single recording engine**, one mouse hook for both buttons. Pressing the other mouse key during an active recording is ignored; only the matching release ends that recording.
- Mouse4 displays a compact 40px high, centered, rounded status bar with animated level visualization, elapsed time, transcribing feedback and an X to cancel. Close the bar with X; the next task is available once the previous worker is drained.
- The status bar is **a native Win32/GDI visual adaptation** of Handy Remote's Tauri CSS/React overlay, **not a byte-for-byte or pixel-perfect copy**. It does not yet support Handy's live streaming transcript panel, themes, full keyboard accessibility, or its exact transition effects.

## Configuration

Keep your existing `config.json` in the EXE directory. v10 supplies a default for any missing `dictation_hotkey`:

```json
{
  "hotkey_record": "Mouse5",
  "dictation_hotkey": "Mouse4",
  "record_mode": "hold",
  "asr_base_url": "http://127.0.0.1:8081/v1",
  "llm_base_url": "http://127.0.0.1:1234/v1"
}
```

The above is an excerpt, not a complete replacement config. The ASR server must be running for either mode; LM Studio is needed only for Mouse5. On a network server, point `asr_base_url` to the trusted server `/v1` root.

## Expected behavior and limits

- First close all earlier VoiceRewrite versions and Handy if Handy has already claimed the same mouse shortcut.
- When Mouse4 finishes, the output text is inserted at the active caret with the same Ctrl+V injection mechanism as v9. If the original top-level foreground window changed while ASR ran, v10 **does not auto-insert**: it copies the generated text to the clipboard and reports an error.
- The result of SendInput cannot prove the target editor accepted the paste; the log calls the status **unverified**, not verified.
- The clipboard implementation still preserves only a prior text value. Image, HTML and file-list clipboard formats are NOT currently round-tripped, so back up any important clipboard data before testing. This limitation is inherited from v9.
- v10 does not automatically start ASR or LM Studio services. The design stays a single GUI EXE, with existing local model backends.
- Automated Go tests and Windows cross-compilation do not verify physical mouse keys or Win11 UI rendering. See `MANUAL-TESTS.md` for manual acceptance.

## Open source acknowledgement

UI behavior and sizing were studied in [Hyroniem/handy-remote](https://github.com/Hyroniem/handy-remote), especially `src/overlay/RecordingOverlay.tsx`, `RecordingOverlay.css`, and `src-tauri/src/overlay.rs`. Handy Remote is MIT licensed, copyright 2025 CJ Pais. See `THIRD_PARTY_NOTICES.md`. The actual Win32/GDI status overlay in VoiceRewrite is newly implemented in Go and does not embed its React/Tauri assets.

Build ID: `2026-11-14-dual-mouse-handy-v10`.
