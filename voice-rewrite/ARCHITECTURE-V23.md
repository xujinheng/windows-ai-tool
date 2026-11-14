# v23 modular architecture

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
5. Clipboard ownership checks currently cover **plain text only**. Rich clipboard and editor insertion verification require a separate Windows OLE/UI Automation implementation. Do not advertise full clipboard restoration.
6. Windows themes are read when a native settings/result window opens; changes while the window is already open are not yet live-updated.

## Testing

Run `gofmt -l .` (must print no files), `go test ./...`, `go test -race ./...`, `go vet -unsafeptr=false ./...`, and Windows x64 cross-compile. The unsafeptr exception is limited to OS-owned Win32 callback and memory pointers. Physical Windows input, dark-theme contrast, DPI and multimodal provider behavior remain real-device checks.
