# VoiceRewrite v26 — session recovery and verifiable server compatibility

## Problems addressed

1. A single recovery slot made consecutive dictations easy to lose. A bounded in-memory history now retains up to 10 entries, with an aggregate 4 MiB ceiling. Full outputs are never truncated merely to fit the history. The user can open or clear entries from the native Windows tray menu. Each menu label is **type + time only** to avoid exposing speech or selected text to taskbar menu observers. Closing the process erases history.
2. Some text APIs provide `/v1/models` and list Qwen3-ASR but reject multipart audio transcription (HTTP 415). ASR settings now test the same `POST /v1/audio/transcriptions` route, model ID, and multipart WAV format the client actually uses. The probe sends one second of local silence and reports 200, 400, 401/403, 404/405, 415, 503 and network errors separately. A 200 response from silence confirms only protocol acceptance, **not** transcription accuracy.
3. Tagged prereleases now carry build provenance attestations for the portable ZIP, installer and original OneClick binary. The end-user application still does not auto-execute updates.

## Design and privacy decisions

- Results remain entirely in process memory; no persistent dictation history, telemetry or hidden transcript previews. This consciously avoids retaining sensitive voice content between sessions.
- The tray and result card use native Win32 controls and keep the original text editor untouched when browsing recovered results.
- GitHub provenance attestations are distinct from commercial Authenticode signing; users should not mistake one for the other.

## Verification

Local `gofmt`, `go test`, `go test -race`, `go vet -unsafeptr=false`, and Windows amd64 cross-compilation are expected. Windows desktop focus/clipboard/recording interactions and signature validation require an actual Windows test station.
