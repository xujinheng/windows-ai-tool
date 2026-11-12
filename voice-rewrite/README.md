# VoiceRewrite

Native Windows app: select some text, press the configured hotkey, dictate a revision instruction, press the hotkey again, and let a text LLM rewrite the selection.

## Setup

1. Run an ASR server with OpenAI-compatible `/v1/audio/transcriptions` (for example the OneClick launcher).
2. Run an OpenAI-compatible **text** chat model in LM Studio, Ollama/llama.cpp or another supported server. ASR-only models cannot rewrite text.
3. Build `VoiceRewrite.exe` or download the executable from GitHub Actions. Place VoiceRewrite in a **different folder** than OneClick.
4. On first launch VoiceRewrite creates `config.json`; edit its `asr_base_url`, `llm_base_url`, `text_model`, API keys, and hotkeys as needed. You may start with `config.example.json`.
5. Default shortcut: select text, **Ctrl+Alt+F9** to start speaking, **Ctrl+Alt+F9** to stop; **Ctrl+Alt+F10** exits. The configured shortcuts may use `Ctrl+Alt+F1..F24` or `Ctrl+Alt+Shift+F1..F24`.

The code makes a WAV in memory, sends it to ASR, and sends **only the selected text and recognized instruction** to the configured text-model API. It does not send anything to any other service. It checks the target window and selected text before replacing; if either changed, the result goes to clipboard instead. The clipboard integration is plain-text oriented; avoid relying on restoration of rich clipboard formats.

## Configuration

All user-specific choices are in `config.json`, next to the EXE. Default loopback addresses are placeholders, not the user's LAN or home directory. `asr_api_key` and `llm_api_key` are separate and empty by default. Keep real credentials out of git.

Logs are written under the local user profile's application-data directory at runtime; the repository includes no logs.
