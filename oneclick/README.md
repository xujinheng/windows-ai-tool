# Qwen3-ASR OneClick

A lightweight Windows launcher for the official `llama.cpp` server, with an OpenAI-compatible `/v1/audio/transcriptions` proxy that cleans Qwen3-ASR metadata tags. Handles **any language** (e.g. `language English<asr_text>`, `language Chinese<asr_text>`), not only Chinese.

## Setup

1. Get the Windows CUDA build of `llama.cpp` and matching CUDA runtime DLLs from its [official releases](https://github.com/ggml-org/llama.cpp/releases).
2. Place `Qwen3-ASR-OneClick.exe` beside `llama-server.exe`, or configure `llama_server_path`.
3. Download compatible Qwen3-ASR main GGUF and `mmproj` GGUF from the **same converter/repository**. Set `model_path` and `mmproj_path` in `config.json`. Relative paths are relative to `config.json`; absolute paths work locally but must never be committed.
4. Launch once to generate `config.json` if absent, then edit it and restart. You can also copy `config.example.json` to `config.json` beforehand.
5. Start the executable. It launches `llama-server.exe` automatically, waits for its `/health`, and serves the cleaned audio API.

The default listening address for Handy Remote is `http://127.0.0.1:8081/v1`. The internal backend is `http://127.0.0.1:8080`.

If CUDA is missing, the launcher refuses to silently fall back to CPU unless `allow_cpu` is `true` in `config.json`. To connect to an existing server, set `attach_to_existing_server` to `true`.

**Security:** Keep loopback binding for personal use. Don't expose the unauthenticated audio endpoint on a public network. The tool does not download models or libraries automatically. All paths, ports and GPU parameters are configurable in `config.json`.
