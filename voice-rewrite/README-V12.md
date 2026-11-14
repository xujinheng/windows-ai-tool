# VoiceRewrite v12 — Status Bar Polish

Build ID: `2026-11-14-status-polish-v12`

## What changed from v11

The two workflows are **unchanged**:

- **Mouse4** hold/release: local ASR direct dictation at the current caret (no selection or LLM needed).
- **Mouse5** hold/release: selected-text speech-driven rewrite or AI explanation. Explicit transformations still try guarded inline replacement; questions show the result card.

Only the small Handy-like recording/working overlay was redesigned:

1. The overlay is now a native **per-pixel-alpha layered Win32 window** (`WS_EX_LAYERED` + `UpdateLayeredWindow`). No `SetWindowRgn` clipping and no GDI jagged capsule edges.
2. Its graphics come from Windows **GDI+ anti-aliased rendering** into a top-down premultiplied 32-bit DIB. This ships with Windows 11; no WebView2, Node, browser runtime, or extra DLL download.
3. Rounded 9-bar waveform based on mic RMS level; compact anti-aliased loading spinner and status colors.
4. Separate recording-timer (`00:02`) and vector close icon lanes; no `0:02x` overlap.
5. Uses per-monitor DPI and creates the overlay under its own temporary `PER_MONITOR_AWARE_V2` context so 125%/150%/200% displays render at native resolution. The rest of VoiceRewrite retains its existing DPI context.
6. Preserves the topmost, no-activate, taskbar-hidden window and close-to-cancel behavior; normal application focus should not be stolen.
7. One overlay for both Mouse4 and Mouse5. No new route/model/network changes.

### Basis and attribution

Visual design / interaction reference: [Hyroniem/handy-remote](https://github.com/Hyroniem/handy-remote), in particular `src/overlay/RecordingOverlay.{tsx,css}` and `src-tauri/src/overlay.rs`. Original Handy is MIT, copyright (c) 2025 CJ Pais. This is a native-Go adaptation, **not** a pasted Tauri/React component; pixel-perfect identity is not claimed. See `THIRD_PARTY_NOTICES.md`.

### Testing

Executed on the build host:

- `go test -count=1 ./...`
- `go test -race -count=1 ./...`
- `go vet ./...`
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ...`
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ...`

**Not run on actual Windows 11** in this environment. The GDI+/layered-window path requires testing at 100%, 125%, 150% and 200% display scaling. Run the included WinTests EXE with `VOICE_REWRITE_NATIVE_STATUS_TEST=1` in an interactive desktop session, then conduct the manual checks below.

### Manual checks

1. Quit **all** old VoiceRewrite and Handy processes to avoid overlapping hooks.
2. Put the v12 EXE beside the existing `config.json` (keep the ASR and LLM endpoints).
3. Mouse4: hold to speak in Notepad/WeChat; watch the smooth waveform and `00:xx` timer; release; verify transcription is pasted.
4. Mouse5: select text in VS Code/Chrome; hold to speak a rewrite instruction; see the same waveform, then the `识别指令…`, `AI 处理中…`, `正在替换…` states; ensure original text is replaced.
5. Mouse5: select read-only webpage text, ask for an explanation; expect the small status and separate answer card.
6. During recording/processing, click the little X; verify it cancels safely and the app remains available.
7. Repeat on 125%, 150%, and 200% displays, including moving the mouse between monitors and checking the pill doesn't steal input focus.
8. Verify the log contains `build=2026-11-14-status-polish-v12` at `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log`.

If a layered bar is unexpectedly blank or missing in your real Windows test, the debug EXE (console build) and log can help isolate the GDI+/DPI issue. The old v11 package can be used as a rollback.
