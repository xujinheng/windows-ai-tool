# Architecture

VoiceRewrite is a native Go/Win32 application. Its main components are:

| Component | Source | Responsibility |
| --- | --- | --- |
| Input and recording | `mouse_windows.go`, `recording_windows.go`, `microphone_windows.go` | Detect hold/release and capture PCM audio |
| Task execution | `task_worker_windows.go`, `task_epoch.go` | Coordinate asynchronous work, cancellation and target checks |
| ASR | `asr.go`, `audio_chunks.go` | Transcribe audio through the configured API |
| Text processing | `llm.go`, `intent.go` | Classify spoken instructions and generate text |
| Text insertion | `clipboard_windows.go`, `edit_target.go`, `unicode_input_windows.go` | Apply text to an eligible editor |
| User interface | `settings_windows.go`, `popup_windows.go`, `handy_status_windows.go`, `tray_windows.go` | Settings, results, recording indicator and tray menu |
| Configuration | `config.go`, `config_store.go`, `secrets_windows.go` | Persist settings and protect API keys |

## Execution

Windows input hooks and the UI message loop remain responsive while audio, network requests and text processing run in background tasks. Results are associated with a task generation and the original target window; stale tasks cannot apply text to a different target.

The ASR client uses `/v1/audio/transcriptions`; the LLM client uses `/v1/chat/completions`. Long recordings are split into bounded audio chunks. Text selections are treated as untrusted data when sent to the LLM.

## Input safety

Automatic replacement checks the editor and selection before inserting text. Clipboard restoration is conditional on ownership and available formats. Unsupported clipboard formats and inaccessible editors may require manual copy from the result card.

Known native password fields are excluded. Windows UI Automation augments password-field detection in other applications; accessibility providers may be incomplete.

## Data

Settings are stored under `%APPDATA%\VoiceRewrite`; API keys use DPAPI. Logs are stored under `%LOCALAPPDATA%\VoiceRewrite`. Recent text results and clipboard snapshots remain in process memory.

For setup, see [README](README.md). For verification, see [TESTING.md](TESTING.md).
