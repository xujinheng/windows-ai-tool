# Local model servers

[Start-VoiceRewrite-Models.ps1](Start-VoiceRewrite-Models.ps1) starts separate `llama-server.exe` processes for speech recognition and text generation.

1. Download a Windows `llama.cpp` build with support for your models.
2. Set the executable, ASR GGUF, ASR `mmproj` and LLM GGUF paths at the top of the script.
3. Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\Start-VoiceRewrite-Models.ps1
```

| Service | Endpoint |
| --- | --- |
| ASR | `http://127.0.0.1:8080/v1` |
| LLM | `http://127.0.0.1:1234/v1` |

Both models must be available locally. Adjust GPU layers, context sizes and other server arguments in the script to suit your hardware.
