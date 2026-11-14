# VoiceRewrite

Native Windows 11 client for push-to-talk dictation and AI-assisted text editing.

## Setup

1. Run `VoiceRewrite.exe`. Open **Settings** from the system tray if the window is not already shown.
2. Set the **ASR endpoint**, model ID and optional API key. The endpoint must support `POST /v1/audio/transcriptions`.
3. Set the **LLM endpoint**, model ID and optional API key. The endpoint must support `POST /v1/chat/completions`.
4. Choose a microphone and dictation shortcut. Mouse4 is the default; F1–F24 can be configured.
5. Test both connections, save the settings and restart after changing the input device or shortcut.

Example endpoints are `http://127.0.0.1:8080/v1` for ASR and `http://127.0.0.1:1234/v1` for LLM. Configure and start the services independently.

The ASR connection test submits a generated silent WAV to the transcription endpoint. Confirm microphone recognition by dictating into an editor.

## Controls

| Action | Behavior |
| --- | --- |
| Hold **Mouse4** for 250 ms, speak, release | Transcribe and insert text at the cursor |
| Select text, hold **Mouse5** for 250 ms, speak, release | Rewrite, translate or ask a question |
| Result card | Copy output or apply a replacement when available |
| Tray menu | Open Settings, pause hotkeys, view recent results, check for updates, exit |

A short Mouse4/Mouse5 click performs its normal browser Back/Forward action. The recording indicator and tone appear when a hold is recognized.

Editing a selection requires the original editor and selection to remain valid. If insertion cannot be completed safely, the text is shown in a copyable result card.

Up to ten recent results are available from the tray during the current session. They are not written to disk. Use **Clear session history** or exit to discard them.

## Build

Requires Go 1.23 or newer. On Windows:

```powershell
cd voice-rewrite
go test ./...
go vet -unsafeptr=false ./...
go build -trimpath -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .
```

See [architecture](ARCHITECTURE.md), [Windows testing](TESTING.md), [privacy](../PRIVACY.md) and [installer instructions](../installer/README.md).
