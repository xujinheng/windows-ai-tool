# VoiceRewrite v22 — Native Windows experience overhaul

Windows 11 x64 native Go desktop voice dictation and AI rewrite tool.

VoiceRewrite v22 extends the native Windows UI with configurable F-key dictation, keyboard-accessible settings, session-only result recovery, model-ID-aware connection diagnostics, and safer configuration/logging. See [the v22 release notes](voice-rewrite/EXPERIENCE-V22.md) and [product design contract](voice-rewrite/PRODUCT-DESIGN.md).

Read [the detailed README](voice-rewrite/README.md) for usage, API configuration, privacy and limitations.

## Local ASR with restored OneClick

The original [Qwen3-ASR OneClick](oneclick/README.md) source (previously deleted from main) has been restored unmodified from Git history. The Windows build now produces `Qwen3-ASR-OneClick-Windows-x64.exe`, uploads it as a CI artifact. The originally restored EXE remains in [dist/](dist/), while newer compiled versions are obtained from the CI Artifact without automated commits to main.

OneClick is a small launcher + proxy, **not** an all-in-one model/runtime installer. Supply a compatible `llama-server.exe`, CUDA DLLs and matching Qwen3-ASR model/MMProj GGUF locally. Do not upload model weights, credentials, or user-specific config.

**Port compatibility:** Original OneClick defaults to proxy `127.0.0.1:8081`, backend `127.0.0.1:8080`. VoiceRewrite v22 expects ASR at `127.0.0.1:8080/v1`. To keep VoiceRewrite unchanged, copy [config.voicerewrite-v19.example.json](oneclick/config.voicerewrite-v19.example.json) next to the OneClick executable as `config.json` and edit GGUF and llama-server paths. That preset uses a proxy at 8080 and backend at 8082. Alternatively keep original OneClick ports and configure VoiceRewrite's ASR URL as `http://127.0.0.1:8081/v1`.

Qwen3.5 text generation may continue using LM Studio or a separate compatible llama-server on port 1234; the optional [two-server script](scripts/local-models/README.md) is not required for OneClick.

## Windows portable build

Each successful main-branch Windows build uploads a `VoiceRewrite-portable-windows-x64` Artifact containing `VoiceRewrite-windows-x64.zip` and its `SHA256SUMS.txt`. The ZIP includes `VoiceRewrite.exe`, an example config and the user guide. Extract to a normal directory before launch; user configuration is written to the Windows user profile rather than the extracted directory. The separate OneClick build is available as a different CI Artifact. The restored historical EXE is also in `dist/Qwen3-ASR-OneClick-Windows-x64.exe`.

## Development

```powershell
cd voice-rewrite
go test ./...
go vet -unsafeptr=false ./...
go build -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .

cd ../oneclick
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w" -o Qwen3-ASR-OneClick-Windows-x64.exe .
```

No model weights or llama.cpp runtime binaries are redistributed by this repository. This version has not been tested on the owner's interactive Windows desktop as part of recovery. There is no project-wide LICENSE: choose and add a license before advertising redistribution terms.
