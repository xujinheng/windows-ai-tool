# Local llama.cpp services for VoiceRewrite v19

Windows PowerShell 5.1+ launcher for **two independent** `llama-server.exe` processes.
No GGUF files or llama.cpp executables are distributed here.

## VoiceRewrite endpoints

- Qwen3-ASR: `http://127.0.0.1:8080/v1`, model alias `whisper-1`, requires its matching audio mmproj.
- Qwen3.5-9B: `http://127.0.0.1:1234/v1`, model alias `qwen3.5`.

Set `llamaServer`, `asr.model`, `asr.mmproj`, and `llm.model` in `voice-servers.json` to your actual paths (the script can try to locate LM Studio GGUF files). These sample configs intentionally contain **no personal machine paths or API secrets**.

```powershell
# Preview full executable commands without starting anything
.\Start-VoiceRewrite-Models.ps1 -DryRun
# Start both API endpoints
.\Start-VoiceRewrite-Models.ps1
# Verify endpoints (a health check is not an actual transcription test)
.\Start-VoiceRewrite-Models.ps1 -Action Test
# Stop only script-owned processes
.\Start-VoiceRewrite-Models.ps1 -Action Stop
```

## Qwen3.5-9B settings copied from screenshots

- Context: 65536 tokens; GPU offload layers: 32; threads: 6
- Logical batch: 2048; physical batch: 512; concurrency: 1
- Flash Attention: on; unified KV: on; KV GPU offload: on
- Context checkpoints: 32; memory mapped model: off (`--load-mode none`)
- Thinking: disabled (`--reasoning off`); `--jinja` enabled

The screenshot's temperature 0 is an **inference-time request setting**, not a persistent model-load flag; VoiceRewrite controls temperature in its API requests. The LM Studio setting `Truncate Middle` and model-specific automatic settings are not fully equivalent to llama-server flags. These defaults are based on screenshots, not a claim of bit-for-bit identical inference.

The ASR launcher retains its earlier v16 defaults (`-ngl 99`, `-c 8192`, `-np 1`).
**Use a current llama.cpp CUDA build** with Qwen3-ASR support and the server flags shown above. If long context causes out-of-memory, reduce `llm.ctxSize` and/or `llm.gpuLayers`.

The script uses only `127.0.0.1` and does not stop an unrelated listener on the configured ports. Logs and process state are generated inside a `runtime/` folder, which should remain untracked.
