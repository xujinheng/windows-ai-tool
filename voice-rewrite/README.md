# VoiceRewrite v26 — Windows native voice writing assistant

VoiceRewrite is a native Windows 11 Go application. Hold **Mouse4**, or configure a standalone F-key in Settings, to dictate at the caret; hold **Mouse5** after selecting text to rewrite, translate, or ask a question. VoiceRewrite is a **client**, not a model installer: ASR and LLM endpoints are configured separately.

## First run

1. Run `VoiceRewrite.exe`. The settings center opens on first launch; later open it from the tray.
2. Set **ASR** endpoint (for example `http://127.0.0.1:8080/v1`), model ID (`whisper-1` for OneClick), and API Key when required.
3. Set **LLM** endpoint (for example `http://127.0.0.1:1234/v1`), model ID and optional API Key.
4. Test each service independently. The test reads the API model list and checks for your configured model ID where available. This is not an ASR transcription test; dictate into Notepad to confirm audio support.
5. Select the system-default or a specific microphone and configure Mouse4/Mouse5 or an F1–F24 dictation shortcut. Changes to the hotkey take effect after restarting VoiceRewrite. Click **Save**. API Keys are encrypted under the signed-in Windows account via DPAPI before being written to disk. A copied encrypted config cannot be decrypted by a different Windows account.

## Everyday use

- **Mouse4** or your configured **F-key** held: record, release to transcribe and attempt to insert. If the target changes or paste fails, the complete recognized text appears in a copyable result card instead of writing into another app.
- **Mouse5** held with selected text: a clear edit command attempts a guarded replacement; explanation and ambiguous commands open a preview.
- **Result card**: copy the complete result; optionally apply it back to the original selection after validation. Long results are scrollable and not truncated.
- **Tray**: pause/resume background interception, recover the most recent result (memory only, never saved to disk), settings, recent error and recovery guidance, diagnostic log, close answer, exit. Pausing returns Mouse4/Mouse5 to normal browser/OS behavior.

## Architecture

- Go 1.23, standard library and Win32, one native Windows GUI executable. v23+ separates configuration, ASR, LLM, intent routing, recording, clipboard insertion, and the Windows task worker into dedicated modules.
- The settings center and result card share Windows light/dark appearance, including live theme refresh when system colors change.
- Each task owns a cancelable context: pressing Cancel fences results *and cancels outstanding HTTP I/O*.
- Long recordings are split into 25-second-or-shorter PCM chunks near quieter audio windows, then transcribed sequentially and rejoined without dropping source samples.
- Connection pooling reuses HTTP transport, redirects are not followed, and server error bodies are not echoed in the UI/logs.
- ASR: OpenAI-compatible `POST /audio/transcriptions`; handles `{"text":"..."}`, JSON strings and text/plain, strips Qwen3-ASR language markers.
- LLM: OpenAI-compatible `POST /chat/completions`. ASR and LLM may use different remote/local servers.
- Selected text is untrusted model input, not a source of instructions.
- Clipboard/SendInput insertion is **best effort**; sending Ctrl+V does not prove that a third-party application accepted it. A conservative multi-format HGLOBAL snapshot restores accessible text, HTML, RTF, and file-list data when clipboard ownership has not changed; uncopyable GDI/owner formats are never overwritten. Dictation falls back to Unicode SendInput when preservation cannot be guaranteed. This is **not** complete Windows OLE clipboard fidelity.
- `PRODUCT-DESIGN.md` describes the product interaction contract; `ARCHITECTURE-V24.md` explains module boundaries, and `EXPERIENCE-V24.md` records the latest release.

## Build

```powershell
cd voice-rewrite
go test ./...
go vet -unsafeptr=false ./...  # Windows API callbacks use OS-provided uintptr pointers
go build -trimpath -ldflags='-H windowsgui -s -w' -o VoiceRewrite.exe .
```

Run real Windows 11 interactive tests before claiming compatibility across WeChat, VS Code, Chromium, DPI and multi-monitor environments. Avoid publishing API credentials or actual audio/transcript content.

See the repository root for the restored **Qwen3-ASR OneClick** launcher. The project still requires an owner-approved license before redistribution terms are clarified.

## Installation and distribution

On every commit GitHub Actions builds a portable ZIP and an **unsigned per-user Inno Setup installer** with Start-menu shortcut and uninstaller. It preserves the private config and DPAPI secrets under the user profile when uninstalled. An installer is not equivalent to a signed or Microsoft Store-approved application; Windows SmartScreen may warn until the owner arranges code signing.

See `../installer/README.md` and `EXPERIENCE-V24.md` for release boundaries. Tag-triggered GitHub releases are **pre-releases**, not automatically declared stable. Native password-style Win32 input controls are excluded from global dictation and selection operations; browser/Electron password controls still need separate native automation auditing. Technical logs rotate at 4 MiB with three backups; microphone content and selection text are not intentionally logged.

## v25 additions

- Settings allows choosing a Windows recording input by device name (changes require app restart). The `audio_device_id` config defaults to -1 (system default).
- The tray's manual “检查更新” queries GitHub Releases only when clicked. No update package is silently downloaded or executed.
- Password-sensitive focus detection uses Win32 and Microsoft UI Automation `IsPassword` on a separate COM worker. Missing accessibility support can cause browser-style surfaces to reject automatic input. This check is best effort, not a guarantee of detecting every sensitive field.

See [v25 development notes](EXPERIENCE-V25.md). This is an **unsigned development build**, not a production-certified application.


## Session result history and real ASR capability check (v26)

- The tray keeps up to **10 recent text results** in memory only. Use “最近 10 次结果” to reopen an entry without rewriting the original editor; use “清空本次会话历史” to erase the in-memory list. Results are not saved to disk. Menu entries display only category and time, never dictated text. If the process exits, history is lost. An individual result larger than 4 MiB is excluded from the recovery cache, but remains fully available in its original card.
- The ASR “检测” action now sends **one second of generated silent WAV** to the configured `POST /v1/audio/transcriptions` endpoint using the actual multipart request format and model ID. This catches servers that list a model but reject audio with HTTP 415. It uploads no microphone recordings. A successful silent-file request is **not** proof of microphone quality or transcription accuracy; follow with real dictation. LLM checks still inspect the configured model via `/v1/models`.
