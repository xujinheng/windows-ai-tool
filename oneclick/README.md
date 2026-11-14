# Qwen3-ASR OneClick

Windows launcher for `llama-server.exe` with an OpenAI-compatible audio transcription endpoint.

## Configure

1. Download `llama.cpp` for Windows from [official releases](https://github.com/ggml-org/llama.cpp/releases).
2. Obtain a compatible Qwen3-ASR GGUF and matching `mmproj` GGUF.
3. Place `Qwen3-ASR-OneClick.exe` beside `llama-server.exe` or set `llama_server_path` in `config.json`.
4. Copy `config.example.json` to `config.json`, then set the model paths.
5. Run the launcher.

Default proxy endpoint: `http://127.0.0.1:8081/v1`. Default backend: `http://127.0.0.1:8080`.

For VoiceRewrite's default ASR endpoint (`http://127.0.0.1:8080/v1`), start from [config.voicerewrite.example.json](config.voicerewrite.example.json), which uses port 8082 for the backend.

The launcher can attach to an existing server. GPU acceleration is configurable; CPU fallback requires `allow_cpu: true`. Keep the unauthenticated audio API bound to loopback.
