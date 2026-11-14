# Privacy and data handling

VoiceRewrite is designed to work with ASR and LLM servers selected by the user. It does not automatically discover cloud accounts, start model servers, or implement telemetry.

| Data | Used for | Storage |
| --- | --- | --- |
| Microphone audio | Sent to the configured ASR endpoint for transcription | In memory while the task runs; not intentionally written to disk |
| Selected text and voice command | Sent to the configured LLM endpoint | In memory while the task runs |
| Last result | Tray recovery during this session | Memory only; lost on exit |
| API Key | Authentication with configured endpoint | User config, encrypted with Windows DPAPI |
| Endpoint, model and shortcut settings | Client operation | User config in `%APPDATA%\VoiceRewrite` |
| Technical diagnostic log | Error diagnosis | `%LOCALAPPDATA%\VoiceRewrite` with 4 MiB rotation and three backups |
| Clipboard backup | Prevent format loss during automatic input | Memory only, sequence-fenced restoration |

**Remote endpoint warning:** If a remote provider is configured, that provider receives the relevant audio or text and applies its own retention policy. VoiceRewrite cannot control the remote service. API probes may send the configured model ID. Do not configure an untrusted server.

VoiceRewrite refuses known native Windows password-style edit controls. Browser/Electron password controls are not comprehensively detected. The implementation does not provide full fidelity for Windows OLE delayed rendering or every private clipboard type. Unsafe selection operations fail closed; dictation may use simulated Unicode keystrokes without modifying the clipboard.

To delete local settings, exit VoiceRewrite and remove `%APPDATA%\VoiceRewrite`. To delete logs, remove `%LOCALAPPDATA%\VoiceRewrite`. The uninstaller intentionally preserves these user-owned directories unless removed separately.

## v25 update and focus checks

A GitHub update check occurs **only** when the user selects “检查更新” in the tray. The request to `api.github.com` includes no audio, transcripts, API keys, or user configuration; GitHub necessarily receives ordinary network request metadata. The app opens the repository's fixed HTTPS Releases page only after confirmation and does not download or execute any installer. Focused input controls are additionally checked using Windows UI Automation `CurrentIsPassword`; providers may be incomplete, and this is not a universal protection guarantee.

## Session recovery data (v26)

Up to 10 text results totaling no more than 4 MiB can be retained **in process memory only** for recovery via the system tray. They are not written to configuration, logs or telemetry. The tray menu contains only item type and clock time. Use the **clear current session history** menu item to discard them, or exit VoiceRewrite. Results too large for the cache are not silently truncated. The ASR connection check uploads a locally generated **silent audio test file**, not microphone data, to the user-configured ASR API.
