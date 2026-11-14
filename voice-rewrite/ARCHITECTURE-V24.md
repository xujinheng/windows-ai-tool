# v24 modular architecture

VoiceRewrite stays a single native Windows executable. It is strictly an ASR/LLM **client**; process management stays in external OneClick/llama.cpp.

## Module boundaries

| Module | Responsibility |
| --- | --- |
| `config.go`, `config_store.go`, `secrets_windows.go` | Defaults, private configuration, DPAPI-at-rest API keys |
| `http_client.go` | Shared connection pool, deny redirects, reject credential-bearing URLs, sanitized HTTP errors |
| `asr.go`, `audio_chunks.go` | Audio transcription API, Qwen3-ASR cleanup, long-recording segmentation; keep every original PCM sample |
| `llm.go`, `intent.go` | Model discovery, selected-text processing, non-destructive intent routing |
| `task_epoch.go` | Session generation + cancellation context; stale tasks cannot publish results |
| `windows.go`, `task_worker_windows.go` | Windows message loop, UI ownership, task execution; no network work inside mouse hook callbacks |
| `recording_windows.go` | WinMM audio device and capture thread |
| `clipboard_windows.go` | Selected-text capture, foreground/focus/selection guarding, synthetic paste |
| `appearance_windows.go`, `settings_windows.go`, `popup_windows.go` | Shared system appearance and native window surfaces |

## Critical behavioral guarantees

1. `taskEpoch.start()` cancels previous task I/O; `taskEpoch.shutdown()` cancels the current task. External-editor writes still require fresh focus and selection checks. Cancellation is not a substitute for write validation.
2. Only explicitly spoken edit/translation commands can authorize cross-app writes. The model's returned `intent` alone does not.
3. ASR splitting has no audio-data loss; the backend may infer each chunk separately. It cannot guarantee perfect linguistic continuity at chunk boundaries. This is a deliberate trade-off against unknown long-audio limits.
4. ASR and LLM HTTP clients never follow 301/302/307/308 redirects; audio and text stay on the configured host. Provider error bodies may include private data, so the app exposes only service + status.
5. v24 uses conservative movable-HGLOBAL clipboard snapshots and sequence-fenced restoration; GDI, owner-rendered formats and full OLE/UI Automation support remain outside the backup scope. Dictation switches to Unicode SendInput for unsafe clipboard types.
6. Windows settings and result windows respond to theme/setting changes without restarting.

## Testing

Run `gofmt -l .` (must print no files), `go test ./...`, `go test -race ./...`, `go vet -unsafeptr=false ./...`, and Windows x64 cross-compile. The unsafeptr exception is limited to OS-owned Win32 callback and memory pointers. Physical Windows input, dark-theme contrast, DPI and multimodal provider behavior remain real-device checks.

## Additional privacy fences

Native EDIT/RichEdit controls marked as password inputs are blocked from dictation and selection capture. Browser/Electron password inputs are not assumed to be detectable through raw Win32 HWND styles; this remains a security limitation. Technical logs rotate on startup and avoid logging microphone samples, clipboard backup data, or API secrets.
