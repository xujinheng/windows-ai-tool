# VoiceRewrite v13 — Status Bar alignment & high-DPI clipping fix

Build ID: `2026-11-14-status-align-v13`.

**User-visible fix:** The v12 rounded bar was chopped off at the right, and the controls (waveform, clock, X, and success checkmark) became displaced at high display scale. These bugs were in the Win32 rendering layer, not the dictation/rewriting pipeline.

## Root causes and fixes

1. **Double-scaling geometry**: v12 calculated a *physical-pixel* DIB width and then passed it as a *logical* width to painter primitives, where it was multiplied by the DPI factor again. All painting coordinates now stay in 96-DPI logical units and are converted exactly once inside `handyPainter`; the bitmap and HWND width are calculated independently using `handyPhysicalSize`.
2. **Stale native HWND size**: v12 changed bitmap size from recording (218) to working (246) without synchronizing the native HWND. v13 calls `SetWindowPos` with `SWP_NOACTIVATE|SWP_NOZORDER` on transitions, preserving the text editor's focus.
3. **Overlapping lanes and drifting text**: The recording layout now uses a padded left indicator, 9-waveform lane, separated `00:xx` clock, and a dedicated right-side X. The processing label uses a consistent logical rectangle with centered baseline via GDI+ `StringFormat`.
4. **Corrupted success glyph**: v12 used a Chinese-font checkmark glyph drawn inside a too-small rectangle; v13 draws a vector checkmark directly inside the green status circle.
5. **Click target at high DPI**: Convert the physical mouse x-coordinate back to logical before checking the cancel-button lane.
6. **Keep anti-aliasing**: Status continues to use native GDI+, per-pixel alpha, soft shadow, and `UpdateLayeredWindow`. No WebView, Rust, or external runtime.

## Scope

**Mouse4 remains ASR direct dictation** at the caret; **Mouse5 remains selected-text rewrite / question**. ASR/LLM endpoints, shortcuts, recorder, model API, clipboard and paste logic are unchanged from v12.

## Test results and caveats

Build-host checks: `go test ./...`, `go test -race ./...`, `go vet ./...`, Windows x64 GUI/debug/test-binary builds. Ordinary layout tests cover 100/125/150/175/200% scaling and spacing. The Windows interactive test, which is opt-in via `VOICE_REWRITE_NATIVE_STATUS_TEST=1`, also verifies that the native HWND actually resizes on recording → transcribing.

**Windows 11 visual display, input focus, hover/cancel, and multimonitor DPI still require real desktop verification.** This package is not a claim of pixel-perfect Handy UI or that the screenshot has been reproduced on a live Windows machine.

To install, quit old VoiceRewrite first. Put the executable next to the existing `config.json` and keep that config. Look for `build=2026-11-14-status-align-v13` in `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log`.
