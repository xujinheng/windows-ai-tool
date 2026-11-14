# VoiceRewrite v20 — Windows native voice writing assistant

VoiceRewrite is a native Windows 11 Go application. Hold **Mouse4** to dictate at the caret; hold **Mouse5** after selecting text to rewrite, translate, or ask a question. VoiceRewrite is a **client**, not a model installer: ASR and LLM endpoints are configured separately.

## First run

1. Run `VoiceRewrite.exe`. The settings center opens on first launch; later open it from the tray.
2. Set **ASR** endpoint (for example `http://127.0.0.1:8080/v1`), model ID (`whisper-1` for OneClick), and API Key when required.
3. Set **LLM** endpoint (for example `http://127.0.0.1:1234/v1`), model ID and optional API Key.
4. Test each service independently. The test probes API connectivity only. To confirm audio transcription, dictate into Notepad.
5. Click **Save**. API Keys are encrypted under the signed-in Windows account via DPAPI before being written to disk. A copied encrypted config cannot be decrypted by a different Windows account.

## Everyday use

- **Mouse4** held: record, release to transcribe and attempt to insert. If the target changes, the operation stops rather than writing into another app.
- **Mouse5** held with selected text: a clear edit command attempts a guarded replacement; explanation and ambiguous commands open a preview.
- **Result card**: copy the complete result; optionally apply it back to the original selection after validation. Long results are scrollable and not truncated.
- **Tray**: settings, recent error and recovery guidance, diagnostic log, close answer, exit.

## Architecture

- Go 1.23, standard library and Win32, one native Windows GUI executable.
- ASR: OpenAI-compatible `POST /audio/transcriptions`; handles `{"text":"..."}`, JSON strings and text/plain, strips Qwen3-ASR language markers.
- LLM: OpenAI-compatible `POST /chat/completions`. ASR and LLM may use different remote/local servers.
- Selected text is untrusted model input, not a source of instructions.
- Clipboard/SendInput insertion is **best effort**; sending Ctrl+V does not prove that a third-party application accepted it. Rich clipboard formats are not fully preserved yet.
- `EXPERIENCE-V20.md` documents interaction behavior, privacy, limitations and follow-up work.

## Build

```powershell
cd voice-rewrite
go test ./...
go vet -unsafeptr=false ./...  # Windows API callbacks use OS-provided uintptr pointers
go build -trimpath -ldflags='-H windowsgui -s -w' -o VoiceRewrite.exe .
```

Run real Windows 11 interactive tests before claiming compatibility across WeChat, VS Code, Chromium, DPI and multi-monitor environments. Avoid publishing API credentials or actual audio/transcript content.

See the repository root for the restored **Qwen3-ASR OneClick** launcher. The project still requires an owner-approved license before redistribution terms are clarified.
