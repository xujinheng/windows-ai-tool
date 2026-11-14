# Product design contract — VoiceRewrite

## Product promise

Speak in any editor. Dictation inserts at the caret; explicit spoken rewrite commands transform a selection; questions show an answer without modifying the original. Do not make users learn a new editor, log format or model runtime.

## Interaction map

| State | Human-facing copy | Actions | Must not happen |
| --- | --- | --- | --- |
| Ready | Tray icon only | Hold configured shortcut | Steal focus, record silently |
| Recording | Audio level, elapsed time, cancel | Release to submit / X to cancel | Cover active caret or alter clipboard |
| Transcribing | Recognizing speech | Cancel | Show a fake progress percentage |
| Rewriting | Processing selection | Cancel | Replace based solely on LLM intent classification |
| Applying | Applying text | Cancel before commit | Paste into a changed foreground editor |
| Preview | Full result, Copy, optional Apply | Copy / dismiss | Truncate text or hide output after failure |
| Recover | Last result from this session | Copy | Persist transcript by default |
| Error | Plain-language action | Open Settings / recover result | Unqualified success check mark |

## Interface principles

- One family of typography, controls, spacing and focus states across settings, result cards, overlay and tray. Use Windows 11 native conventions where possible; avoid gratuitous custom-drawn UI.
- Settings uses grouped ASR / LLM connections with **independent** status and a third shortcut section. An HTTP response is not proof of actual transcription.
- Cursor and active editor take priority. The overlay must not focus itself. Output cards can be opened non-activating and support complete text copy.
- Error messages name the broken stage and next action; network exceptions and status codes are not the entire user interface.
- Keyboard users must reach every settings control. `Tab`, `Shift+Tab`, `Escape`, and standard editable text shortcuts must work without third-party components.
- Privacy is the default: no recording on idle, no transcript persistence by default, keys encrypted at rest with Windows DPAPI, and endpoint URLs/credentials excluded from diagnostic logs.

## Implementation boundaries

The Win32 Go app remains client-only for ASR and LLM, with explicit API base URLs; a separate OneClick can manage Qwen3-ASR. UI tasks are owned by the message thread, long-running jobs use goroutines, and asynchronous results are fenced by task generations and target identity. The Windows clipboard is *not* currently OLE multi-format-safe; do not advertise complete preservation.

## Engineering checklist

- Changes must leave `gofmt -l .` empty on Windows, and `go test ./...`, `go vet -unsafeptr=false ./...`, Windows GUI build passing.
- Live changes to shortcut registration require a full startup/restart boundary; do not label them applied instantly.
- Always show explicitly what data will be sent to third-party endpoints and keep raw input/audio out of logs.
- Document screen-reader, keyboard-only, high DPI and cross-app behavior as verified only when real desktop testing actually occurs.

## Reference implementations and guidance

- [Dictum](https://github.com/Vkandil/dictum): hold/toggle shortcuts, HUD, escape/cancel behavior and app-aware dictation.
- [freewispr](https://github.com/x26prakhar/freewispr): minimal tray-first Windows dictation, configurable keyboard input and onboarding.
- [Microsoft Windows app best practices](https://learn.microsoft.com/en-us/windows/apps/get-started/best-practices): native layout, interaction, windowing and shell integration.
- [Windows 11 design resources](https://learn.microsoft.com/en-us/windows/apps/design/downloads/): typography, controls, focus and accessibility reference.

References inform design direction, not evidence that any particular Windows UI flow has been tested in this app.
