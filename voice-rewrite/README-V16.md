# VoiceRewrite v16 — Public Beta foundations

- Mouse4: hold to dictate; Mouse5: hold to edit or explain selected text.
- Default ASR URL: `http://127.0.0.1:8080/v1`. Default LLM URL: `http://127.0.0.1:1234/v1`.
- Built-in Qwen3-ASR metadata prefix cleaning: `language English<asr_text>` etc. Supports JSON and text/plain responses. No intermediate OneClick proxy is needed.
- Optional integrated CUDA server launcher: if a server already listens at the configured local address, VoiceRewrite attaches without managing it. Otherwise it looks for `llama-server.exe`, main Qwen3-ASR GGUF, and matching `mmproj` in the EXE folder or `models/`, then starts `llama-server` in a background process. It stops **only** the process it launched, when VoiceRewrite exits. Set `auto_start_asr: false` to disable this.
- Qwen3-ASR GGUF and mmproj MUST be from a matching converter/repository and use the appropriate architecture. CUDA runtime DLLs must be available to llama-server; no models/CUDA binaries are included in this release.
- An ordinary Qwen3-ASR setup that already runs on 8080 works without launcher files, provided its endpoint accepts `/v1/audio/transcriptions` with JSON response.
- The native settings UI provides separate ASR/LLM connection checks; advanced JSON is still available.
- These features have unit tests and cross-compiled Windows binaries; real Windows 11 setup and multi-app input must still be validated. This is an experimental pre-release, not a guaranteed plug-and-play application.
- OneClick's original repository directory is obsolete once the source is synced: it had two roles, launcher and metadata proxy; VoiceRewrite now implements both required behaviors directly (launcher + local response cleanup, no extra proxy).
- For distribution: add a user-selected OSS license, verify provenance of included assets, write end-user installation instructions, and test with a clean Windows machine (without preinstalled ASR / LM Studio).
