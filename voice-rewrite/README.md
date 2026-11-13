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

## Clipboard timing: intermittent selection failures

If the first hotkey press sometimes says no selected text was found, the Windows application may be responding to the simulated Ctrl+C too slowly. The tool now retries while checking that the original editor remains foreground.

In your local `config.json`, use: `"selection_copy_attempts": 3` and `"selection_copy_timeout_ms": 1800`. If the target app is particularly slow, try increasing the timeout to 3000; values are limited to 500–5000 ms and 1–5 attempts. The copy retry never intentionally sends a copy command to a different foreground window.

The recording shortcut can also be set to `"hotkey_record": "Pause"` (or a Ctrl+Alt+F-key combination). The exit hotkey defaults to `Ctrl+Alt+F10`. Restart VoiceRewrite after changing config.

## Push-to-talk / press-and-hold recording

VoiceRewrite supports two recording modes through its per-program `config.json`:

- `"record_mode": "toggle"`: press your configured hotkey to start; press again to stop (backward-compatible default).
- `"record_mode": "hold"`: **hold down an unmodified function key** to record; **release** it to stop and run the rewrite. Example: `"hotkey_record": "F9"`.

Example additional settings inside your existing `config.json`:

```json
"record_mode": "hold",
"hotkey_record": "F9",
"hotkey_exit": "Ctrl+Alt+F10"
```

Use a physical F1–F24 key that sends ordinary press AND release events. `Pause/Break` is **not supported in hold mode** because many Windows keyboards do not reliably report key release. Modifier combinations (`Ctrl+Alt+F9`) remain supported in **toggle** mode, but are not supported in hold mode since keeping Ctrl+Alt pressed can interfere with reading selected text via Ctrl+C. You can switch back to toggle mode at any time.

When you press the hold key, recording starts before the application copies the selected text; on release it submits audio to the configured ASR/LLM services. For applications that map F9 to their own command, choose a different F key. Existing local `config.json` files are **not overwritten**; restart VoiceRewrite after changing it.

The key-release detection uses a short Windows timer that polls the physical key state. This has been cross-compiled for Windows but requires real-keyboard testing; accessibility/remote-desktop keyboards can behave differently.

## Windows system tray state icon

VoiceRewrite stays in the **notification area / system tray** (usually near the clock, sometimes under the `^` hidden-icons menu). Its microphone icon changes color to show the current state:

| Color | State |
| --- | --- |
| Gray | Idle, ready to capture selected text |
| Red | Recording (hold your F-key, or use the configured toggle hotkey) |
| Yellow | Converting speech into an editing instruction |
| Blue | Asking the text model to rewrite the selection |
| Green | Rewrite succeeded; returns to gray after about 4.5 seconds |
| Orange | Error; returns to gray after about 4.5 seconds |

Hover over the icon to see the current state. **Right-click → 退出 VoiceRewrite** to quit. The icon is removed when the program exits, and is re-added if Windows Explorer restarts. The program does not bring any floating status window to the foreground during recording. It retains the original hotkeys and the local `config.json` settings.

**Windows 11 tip:** Windows decides whether new tray icons appear next to the clock or inside the hidden-icons section. If the indicator is under `^`, open Settings → Personalization → Taskbar → Other system tray icons (wording may vary) and enable VoiceRewrite, or drag the icon to the visible tray.

The tray API is provided by the native Windows Shell. No additional Go modules or third-party libraries are required.
## Recording / tray reliability fix

The hold-to-talk flow starts microphone recording immediately on the F-key press, but waits until **after the key is released** before copying selected text. This avoids sending Ctrl+C while the recording hotkey itself remains held. The WinMM recycler preserves the WAVEHDR `WHDR_PREPARED` flag, and reports requeue failures. Tray icon changes are handled on the Windows UI thread instead of a worker thread. Background panics are logged where recoverable.

Test: Start your configured ASR and text Instruct model server, select text in Notepad, hold F9 for 2–3 seconds, speak an editing instruction and release. The icon should change red → yellow → blue → green (or orange on error).

For diagnostics:

```powershell
Get-Process VoiceRewrite* -ErrorAction SilentlyContinue
Get-Content "$env:LOCALAPPDATA\VoiceRewrite\VoiceRewrite.log" -Tail 60
```

Logs may contain model names and configured endpoint addresses. Remove any private details before sharing. This build has passed Go tests and Windows x64 cross-compilation, but has **not been tested on an actual Windows keyboard or microphone**.
