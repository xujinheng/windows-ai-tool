# VoiceRewrite · Windows voice dictation and AI rewrite

Native Go desktop utility for Windows 11 x64. Two mouse buttons, one tray app.

| Gesture | Operation |
| --- | --- |
| Hold Mouse4, speak, release | Dictate text at the current caret (Handy-style) |
| Select text, hold Mouse5, speak an edit command | Rewrite / translate and attempt to replace the selection |
| Select text, hold Mouse5, ask a question | Show a small AI answer window |

Current `main` source is the **v13 core with the ASR default port updated to 8080**. Newer experimental v14–v16 source distributions are currently packaged separately; do not infer from this README that their runtime features are already committed to `main`.

## Local model services

- ASR: `http://127.0.0.1:8080/v1`, OpenAI-compatible `POST /audio/transcriptions`; Qwen3-ASR output prefix cleanup is performed by VoiceRewrite.
- LLM: `http://127.0.0.1:1234/v1`, e.g. LM Studio.

The former `oneclick/` utility was retired. **The v13 code in this repository does not launch the ASR model server**: please start a compatible ASR server separately. v16 experiments include an optional integrated local launcher, but that code is not yet part of this main-branch source snapshot.

No GGUF weights, llama.cpp/CUDA executables, or proprietary runtime files are redistributed by this repository.

## Build

```powershell
cd voice-rewrite
go test ./...
go vet ./...
go build -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .
```

See [VoiceRewrite README](voice-rewrite/README.md), [architecture](voice-rewrite/ARCHITECTURE.md), [Windows acceptance tests](voice-rewrite/CROSS_APP_ACCEPTANCE.md), and [third party notes](voice-rewrite/THIRD_PARTY_NOTICES.md).

## Status and licensing

This is an **experimental** tool. Cross-application clipboard insertion must be verified on Windows; some rich-text clipboard formats are not fully preserved. The repo owner should choose and add a clear LICENSE before presenting this as a redistributable open-source release.
