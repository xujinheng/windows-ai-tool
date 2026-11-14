# VoiceRewrite — native Windows voice writing

VoiceRewrite is a Windows 11 x64 desktop voice dictation and AI rewrite tool,
implemented in Go and Win32. It connects to separately configured OpenAI-compatible
**speech-to-text (ASR)** and **text-generation (LLM)** APIs. It is not a model manager.

## What it does

- **Hold Mouse4** (or configure F1–F24) and dictate. Release to transcribe and attempt text insertion at the original caret.
- **Select text, hold Mouse5** and speak an edit, translation, or question. Explicit edits may replace the selection after guarded validation; questions open a copyable result card.
- Pause or resume hotkey interception, inspect last error, restore up to 10 recent results in this session, and configure both services through the native tray settings.

## Install and run

Download an artifact from [GitHub Actions](https://github.com/xujinheng/windows-ai-tool/actions) after a successful build:

- `VoiceRewrite-installer-windows-x64`: per-user Inno Setup installer, Start-menu shortcut, optional desktop shortcut, and uninstall.
- `VoiceRewrite-portable-windows-x64`: portable ZIP + SHA-256 checksum.
- `Qwen3-ASR-OneClick-Windows-x64`: independent ASR launcher.

**The installer is unsigned.** Windows SmartScreen may warn. The owner must arrange verified publisher signing and interactive Windows testing before claiming production-stable distribution. There is no automatic update client.

First run opens ASR and LLM settings. Default examples are `http://127.0.0.1:8080/v1` (ASR) and `http://127.0.0.1:1234/v1` (LLM); these URLs do **not** start servers automatically. See [usage and limitations](voice-rewrite/README.md).

### Qwen3-ASR OneClick

The restored original [OneClick source](oneclick/) builds separately and can launch a compatible `llama-server.exe` with ASR GGUF and mmproj. The OneClick original API is `8081`; if keeping VoiceRewrite's ASR default `8080`, use the [v19-compatible preset](oneclick/config.voicerewrite-v19.example.json), placing the OneClick proxy at `8080` and llama.cpp at `8082`. OneClick does not bundle any models, CUDA libraries, or llama.cpp binary.

## Privacy and safety

VoiceRewrite sends audio and selected text only to the configured API services, which may be remote. DPAPI encrypts API Keys locally. Clipboard backups remain in memory and are restored only while the app still owns the clipboard sequence. GDI, delayed-rendered and private OLE clipboard objects cannot be guaranteed to survive automatic selection actions, which are refused when they cannot be backed up. Native password-style edit controls are blocked; browser password detection is not complete. Text insertion in third-party editors remains best effort. Read [PRIVACY.md](PRIVACY.md) and [SECURITY.md](SECURITY.md).

## Source, contribution and release

The native Go application is in [`voice-rewrite/`](voice-rewrite/); the independent launcher is in [`oneclick/`](oneclick/). See [`CONTRIBUTING.md`](CONTRIBUTING.md), [`CHANGELOG.md`](CHANGELOG.md), and [`installer/`](installer/). CI checks Go formatting, Linux race tests, Windows tests, vet and cross-builds, and generates portable ZIP and installer artifacts. Git tags beginning with `v` generate **preview releases**, not automatically stable releases.

**Repository license is not decided.** Public availability of this source does not grant a redistribution license; the owner must choose one before presenting it as licensed open-source software.

## v25 Windows usability & privacy

The tray can check the GitHub Releases feed on demand (no silent installer downloads). The Settings window can select an input microphone, and a bounded Microsoft UI Automation worker checks the focused element's password status in addition to native password styles. See [v25 details](voice-rewrite/EXPERIENCE-V25.md).

### v26 in-memory recovery and functional ASR checks

The native tray offers a memory-only list of the last 10 dictation/rewrite results and a clear-history command. It deliberately shows no transcript text in menu labels. The ASR connection check sends a generated silent WAV through the actual transcription endpoint, identifying HTTP 415 and unsupported routes rather than mistaking `/models` for audio capability. See [v26 notes](voice-rewrite/EXPERIENCE-V26.md).
