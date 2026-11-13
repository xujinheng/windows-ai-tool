# VoiceRewrite v6: Windows threading, intent and popup architecture

## Processing

- Dedicated Windows `WH_MOUSE_LL` hook OS thread: forwards Mouse5 press/release events; it must not block on clipboard or network I/O.
- UI thread: maintains the tray state and main `GetMessageW` loop. The app handles its own posted app messages, and forwards **every other window message** to `TranslateMessage` and `DispatchMessageW`; these are required to paint and close popup windows and deliver child `WM_COMMAND` notifications.
- Worker goroutines: record PCM, capture selected text after Mouse5 release, transcribe via local Qwen3-ASR, use local LLM exactly once to classify intent and generate output, and optionally attempt safe replacement.

## Routing

One response: `{"intent":"edit|translate|answer|summarize|ambiguous","result":"..."}`. The model never controls whether text may be written. A deterministic instruction guard and a native editable-control check must BOTH authorize replacement. A translation command like `请写成英文` is a transformation: auto-replace when safe, popup otherwise. Requests for explanations/summaries always use the popup. Browser article and unknown editor control selection never gets auto-pasted into.

The popup is a native nonmodal Win32 top-level window containing a UTF-16 read-only EDIT and two buttons (Copy and Close), with an explicit `WM_CLOSE` and `WM_COMMAND` handler. The central message pump dispatches its events. Mouse position is captured when dictation ends; results are rendered later without taking keyboard focus. If a popup has focus, Escape closes it. Tray menu can close it at any time.

## Known limitations

- No general-purpose UI Automation / IAccessible implementation; browser editable text may show preview instead of auto-replace. This avoids injecting Ctrl+V into a page read-only selection.
- Clipboard restore is only plain text and is conditional on clipboard sequence staying unchanged. Image/rich-text clipboard preservation is not yet implemented.
- The native popup lifecycle test is opt-in because it requires an interactive Windows desktop. Cross-compilation does **not** establish native GUI correctness.
- Pasted speech and selected text remain local to the configured servers; never log content; log `intent`, `route`, timings and status instead.
