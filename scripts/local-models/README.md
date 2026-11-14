# Start two local llama.cpp servers for VoiceRewrite v19

Only one PowerShell script is needed. It launches two independent llama-server.exe processes (ASR and LLM); no runtime manager, auto-download, PID database, or API proxy.

1. Download a recent llama.cpp Windows build with Qwen3-ASR and Qwen3.5 support.
2. Edit the four paths at the top of `Start-VoiceRewrite-Models.ps1`: llama-server.exe, ASR GGUF, ASR mmproj GGUF, Qwen3.5 GGUF. You can point to your existing LM Studio GGUF files.
3. Stop LM Studio if it listens on port 1234, then run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\Start-VoiceRewrite-Models.ps1
```

The script opens two llama-server processes. Close their respective console windows to stop them. Starting a process does not guarantee the model has finished loading.

| Server | URL | Model settings |
| --- | --- | --- |
| Qwen3-ASR | `http://127.0.0.1:8080/v1` | `-ngl 99 -c 8192 -np 1`, matching `--mmproj`, model alias `whisper-1` |
| Qwen3.5-9B | `http://127.0.0.1:1234/v1` | `-ngl 32 -c 65536 -np 1 --threads 6 --batch-size 2048 --ubatch-size 512 --ctx-checkpoints 32 --flash-attn on --kv-unified --kv-offload --load-mode none --jinja --reasoning off` |

These values mirror the supplied LM Studio load settings. LM Studio's temperature=0 is a per-request inference setting, not a model-server startup option; VoiceRewrite supplies it. Some settings can behave differently between versions. If an option is unrecognized, use a current llama.cpp build. If GPU memory is insufficient, lower context size or GPU offload layers.

No real Windows model startup has been run as part of this change.
