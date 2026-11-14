# VoiceRewrite v14 — Experience Polish (2026-11-14)

Build ID: `2026-11-14-experience-polish-v14`.

This release focuses on **presentation and truthful status feedback**, not additional AI functionality. Mouse4 still dictates at the caret using local ASR only; Mouse5 still rewrites/translates a selected text or displays answers.

## UI changes

- Compact native layered status pill: 232 × 50 logical units while recording, 252 × 50 for other phases (previously 250 × 58 / 268 × 58).
- GDI+ per-pixel antialiasing, a single consistent 96-DPI logical coordinate system, and monitor-aware physical bitmap/window dimensions. Works without a WebView2 dependency.
- Native-layer opacity animation: 170 ms entrance, 160 ms subtle transition fade, 240 ms exit fade near the end of the 1.75 s terminal display. Each new session resets animation cleanly.
- Nine narrow round-tipped microphone bars, with exponential smoothing (`0.72*old + 0.28*new`) to reduce jitter. Dedicated elapsed timer / cancel lanes.
- Distinguish result states. A sent paste keystroke is **not verified insertion**. Consequently the message is now `已发送粘贴指令` with a neutral-blue icon rather than claiming `已发送文字` or `已发送改写` with a green success check.
- If the dictation target changes after ASR, copy text to clipboard and display `已复制 · 请手动粘贴` **only after the copy succeeds**; otherwise report an error. It is never safe to paste automatically into a new window.
- The result card is more compact for short explanations (480 px wide; adaptive height 224/280/360/450), and its primary button uses a restrained rose accent coordinated with the status bar.

## Source-level acceptance

Run:

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-H windowsgui -s -w' -o VoiceRewrite.exe .
```

The tests cover state-label accuracy, transition opacity timing, waveform smoothing/bounds, popup sizing and status lanes at 100%, 125%, 150%, 175%, and 200% scale. A separate, **opt-in interactive Windows** test exists for the native HWND lifecycle (`VOICE_REWRITE_NATIVE_STATUS_TEST=1`).

## Installation and limitation

Quit any running older VoiceRewrite, put v14 EXE next to your existing `config.json`, launch. Do not change the `Mouse4` / `Mouse5` bindings. v13 still works as a rollback.

**Important:** Go tests and Windows cross-compilation were run in a Linux environment, not a physical Windows 11 desktop. Actual input compatibility, focus behavior, animation smoothness, per-monitor scaling, and popup appearance still require real Windows testing. Clipboard handling is still text-only; preservation of arbitrary clipboard formats is **not yet implemented**. The response `SendInput accepted` does **not** prove inserted text.

Sources studied (design concepts, no source code copied):
- Handy Remote: https://github.com/Hyroniem/handy-remote/blob/main/src/overlay/RecordingOverlay.tsx
- OpenLess: https://github.com/Open-Less/openless/blob/beta/openless-all/app/src/components/OpenLessCapsule.tsx
- OpenLess fallback UX: https://github.com/Open-Less/openless/blob/beta/openless-all/app/src/components/InsertFallbackCard.tsx

The third-party notice remains at `THIRD_PARTY_NOTICES.md`.
