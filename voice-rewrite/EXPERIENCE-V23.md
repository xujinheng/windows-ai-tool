# VoiceRewrite v23 — architecture and everyday reliability

This release replaces oversized source files with independent configuration, ASR, LLM, intent, WinMM capture, clipboard and task worker modules. App behavior is maintained at the API and shortcut layers, with improvements to cancellation, long-form speech and native theme integration.

## What changed

- Native Windows settings and answer card read the system light/dark preference, use one shared color palette and ask DWM for the corresponding title bar. Settings are shorter and have resizable width.
- Each task now has its own `context.Context`. Canceling a session tears down its pending model HTTP requests instead of merely hiding UI and waiting for provider timeout. Tests assert the old task context is canceled.
- Dictations over 25 seconds split at low-energy boundaries. Segment outputs are combined in chronological order; every PCM sample is retained. A failed segment is identified by position.
- Shared HTTP transport pools connections; redirects are blocked, and sensitive provider error bodies are not shown or logged. URL validation disallows userinfo/query/fragment.
- Added regression tests for PCM continuity, actual transcription of multiple chunks, cancel propagation, redirect rejection and URL validation.

## Technical limitations

This is a Go and Windows-target build verification, not proof of correctness on a live Windows 11 desktop. The app does not yet provide all-format clipboard restoration, streaming transcription, a signed installer, a fully accessible Windows UI, or dynamic live-theme updates. Those require focused native implementations and hardware tests. No model runtimes or weights are bundled.

## Inspiration, not copied source

- [Handy](https://github.com/cjpais/Handy): separated keyboard/tray/audio layers and cross-platform robustness.
- [Dictum](https://github.com/Vkandil/dictum): input cancellation, long recordings and OS-level integrations.
- [Microsoft Windows app design](https://learn.microsoft.com/en-us/windows/apps/design/): Fluent interface consistency and coherent feedback.
