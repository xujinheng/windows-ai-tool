# Windows 11 acceptance checklist (manual; not yet executed)

- Fresh profile: launch EXE in Downloads with no existing config. Verify `%APPDATA%\\VoiceRewrite\\config.json` created, setup shown, both pages navigable at 100/125/150/200% display scaling.
- Connect ASR localhost/remote; connect text localhost/remote; confirm changing URLs/IDs and API keys survives restart and never launches model processes.
- Select shared llama-server.exe and compatible ASR+mmproj and LLM GGUF; ensure 8080/8082 children appear, load, respond to API, and disappear when GUI exits. Check normal exit and crash cleanup separately.
- Mix launch ASR/connect LLM and connect ASR/launch LLM; save during model loading and when busy. Verify no externally launched process is killed.
- Intentionally use duplicate ports, remote URL in launch mode, missing GGUF, wrong model type, missing CUDA DLL, unsupported ASR endpoint; check the UI and log messages.
- Mouse4 dictation in Notepad, WeChat, VS Code, Chrome and a browser contenteditable. Mouse5 selected-text replacement and question card in same apps. Check cancellation, clipboard restore, focus switching, no wrong-window paste.
- Test CPU-only llama-server, available GPU VRAM, two large models, load time, GPU OOM, shutdown/restart and whether subprocesses leak.
- Verify status bar position/anti-aliasing/timers, answer card interaction and file picker modal focus with DPI/multiple monitors.
