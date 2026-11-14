# v17 – Universal model connections and managed GGUF engines

Build: `2026-11-14-universal-models-v17`

## New

- Independently choose `connect` or `launch` for ASR and LLM, with distinct API base URLs and model identifiers.
- Choose one `llama-server.exe` file via native Win32 file chooser; choose separate ASR/LLM GGUF files and optional ASR mmproj file.
- Two independent managed subprocesses on dedicated loopback ports. Default ASR endpoint 8080; selecting managed LLM changes the stock LM Studio 1234 endpoint to 8082.
- A Windows Job Object guards each successfully assigned child, with per-app ownership tracking and shutdown. Processes started independently by users are never killed or modified.
- Check paths, role-specific model compatibility requirements, invalid ports, duplicate bound ports and conflicting listeners before launching. Process output goes to the diagnostic log; failed startup is reported in settings.
- API key is supported in the native UI; for managed local llama-server it is inherited through the child environment, not exposed as a command line switch.
- User configuration moved to `%APPDATA%\\VoiceRewrite\\config.json`, so software can be installed under read-only directories.
- Core Mouse4 dictation / Mouse5 rewrite and status-bar/answer popup flows remain unchanged.

## Not provided

- GGUF inference/CUDA runtime is not bundled. User supplies their own llama.cpp binary and model files.
- No guarantee that every OpenAI-like service supports the audio or chat endpoint.
- No Windows desktop real-device visual, clipboard, or GPU runtime validation in this build host.
- No automatic model download, signature validation, installer, or auto-updater.

See `README.md` for first-run instructions and `service_launch_test.go` for plan validation tests.
