# VoiceRewrite v22 — keyboard access, result recovery, clear connection states

Build ID: `2026-11-14-native-ux-v22`.

## Functional improvements

- **Keyboard dictation.** Choose Mouse4/Mouse5 or a single F1–F24 key in Settings. Keyboard dictation uses the same hold-to-talk flow and releases using physical key state. The new binding is registered on application restart. Rewrite and dictation triggers cannot collide.
- **Keyboard navigable Settings.** Native `IsDialogMessageW` supports Tab and Shift+Tab focus traversal between input controls and buttons; Esc closes Settings. No WebView is required.
- **Recover the last response.** Tray > Recover latest result opens a copyable, non-activating result window containing the last dictated or generated text. Only one result remains in process memory; it is never written to disk or logs and is discarded on exit.
- **Connection verification.** Separate ASR/LLM checks now parse `/v1/models`, report available model IDs and warn if the configured ID is not in the returned list. A 200 response without a usable list is not falsely reported as a working speech or text model. Changing any connection field invalidates its prior probe result.
- **Security and reliability.** Reject API base URLs containing query strings (to avoid secrets in URL parameters); stop logging endpoint URLs; keep the user-scoped DPAPI API-key encryption from v20; restore native response cards without truncating large text.

## Design philosophy

VoiceRewrite stays invisible until invoked. The foreground editor is the primary workspace; the overlay is short-lived and non-activating. The app should never claim text was inserted merely because `SendInput` accepted a keystroke. Error recovery gives the user an explicit copyable result when the target changes.

The Win32 implementation keeps a clear separation between backend task state, per-session output, settings, and tray actions. The Windows desktop experience should be judged against keyboard, screen-reader, DPI and cross-application cases, not only CI compilation.

## Build command

```powershell
cd voice-rewrite
gofmt -l .  # must print nothing
go test ./...
go vet -unsafeptr=false ./...
go build -trimpath -ldflags='-H windowsgui -s -w' -o VoiceRewrite.exe .
```

The build and unit tests do not guarantee real Windows keyboard focus, UIA accessibility or target-editor paste behavior.
