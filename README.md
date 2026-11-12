# Windows AI Tools

Two small, local-first utilities for Windows 11. Written in Go (standard library); no Python/pip required at runtime.

| Tool | Purpose | Source |
| --- | --- | --- |
| Qwen3-ASR OneClick | Starts a local `llama-server.exe` running Qwen3-ASR, provides a Handy-compatible ASR API, removes `language English<asr_text>` / `language Chinese<asr_text>` metadata | [`oneclick/`](oneclick/) |
| VoiceRewrite | Select text in another Windows application, use a configurable hotkey to dictate an editing instruction, transcribe it locally, ask a local text LLM to rewrite the selected text, and paste it back safely | [`voice-rewrite/`](voice-rewrite/) |

## Configuration / privacy

**No personal Windows usernames, machine paths, internal IP addresses or API keys are stored in this repository.** This public repository tracks only `config.example.json` templates. Each executable creates its own editable **`config.json`** alongside the executable on first launch. `.gitignore` excludes `config.json`, local model files, binaries, and logs. Never commit your real config if it contains a private path or API token.

The programs do not contain telemetry. They send requests only to the endpoints you configure. The examples use `127.0.0.1` (loopback), so audio and text stay on the local computer. If you point an endpoint at another machine or a cloud service, your data is sent there.

**Keep the two tools in separate runtime folders** because each uses its own `config.json` schema.

## Build (Go 1.23+)

```powershell
cd oneclick
go test ./...
go build -o Qwen3-ASR-OneClick.exe .

cd ../voice-rewrite
go test ./...
go build -o VoiceRewrite.exe .
```

The [GitHub Actions workflow](.github/workflows/build.yml) builds Windows x64 EXEs and uploads them as workflow artifacts. CUDA libraries, GGUF model weights, and `llama-server.exe` are **not included**. Acquire those separately from their original providers.

The programs have automated Go unit/integration tests, but compatibility with every Windows application, microphone, and GPU has not been verified. VoiceRewrite uses clipboard-based copy/paste and may not preserve rich clipboard formats.

## Source projects

These are independent companion utilities; neither package redistributes or modifies the upstream [Handy](https://github.com/cjpais/Handy), [handy-remote](https://github.com/Hyroniem/handy-remote), or [llama.cpp](https://github.com/ggml-org/llama.cpp) binaries. See upstream projects for their licenses.
