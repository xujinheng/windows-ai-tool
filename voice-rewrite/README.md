# VoiceRewrite — Windows Local Voice Edit

## 2026-11-13 performance refactor (threaded-input-v3)

**Mouse5 hold-to-talk remains the default.** The low-level XBUTTON hook now has its own dedicated Windows message thread, and slow clipboard copy/revalidation and HTTP requests are handled on background workers. Errors during ordinary operation update the orange tray indicator and log details instead of showing focus-stealing modal dialogs. The default selection-copy retry budget was reduced to 2 attempts × 750ms; **existing `config.json` files are never overwritten**.

See [ARCHITECTURE.md](ARCHITECTURE.md) for thread design and known clipboard-format limitations, and [MANUAL-TESTS.md](MANUAL-TESTS.md) for a desktop acceptance checklist. **This is a cross-compiled build; GUI, hook and real-microphone behavior require Windows 11 testing**.


## Default control: hold Mouse5 (second side button)

Select text in a Windows application, **hold the second mouse side button (Mouse5 / XBUTTON2)** and speak your editing instruction, then **release** it. The application transcribes speech with your configured local ASR service and rewrites the selected text with a local text LLM.

- Default `record_mode`: `hold`
- Default `hotkey_record`: `Mouse5` (Windows XBUTTON2). If the physical layout is reversed, use `Mouse4` (XBUTTON1).
- Default `hotkey_exit`: `Ctrl+Alt+F10`
- The selected mouse button's normal browser forward/back action is suppressed while VoiceRewrite runs. The other side button works as usual.
- F-key (`F9`) hold and Ctrl+Alt+F9 toggle modes remain configurable, but are **not** the default.
- Mouse side button support uses the native Windows `WH_MOUSE_LL` hook, not Python or a mouse-vendor utility.

### System tray

The notification-area icon is always present while running. Gray = idle; red = recording; yellow = ASR; blue = rewriting; green = success; orange = error. Hover the icon for a text status. **Right-click** the icon for:

- **编辑 config.json** — open editable local config
- **查看日志** — open the local troubleshooting log
- **退出 VoiceRewrite** — fully exit and release the mouse-button hook

The chosen mouse side button is consumed by VoiceRewrite across applications. You regain the original button behavior when you exit. If your mouse-vendor software maps a physical side button to a keyboard shortcut instead of exposing XBUTTON events, set that key as the configured hotkey or disable the vendor remapping.

### Configuration

Each runtime folder has its own `config.json` beside the EXE. On first start, an absent file is created with defaults. Existing `config.json` files are **never overwritten**. To switch an existing installation from F9/toggle to the new default, edit these two keys and restart:

```json
"record_mode": "hold",
"hotkey_record": "Mouse5"
```

Keep your existing ASR and LLM service URLs and other settings. `config.example.json` contains all supported fields without personal computer paths, usernames, or API keys. Use `Mouse4` for the other side button. Pause/Break is not suitable for hold mode.

### First functional test

1. Start Qwen3-ASR OneClick (`http://127.0.0.1:8081/v1`) and the LM Studio local text-model server (`http://127.0.0.1:1234/v1`).
2. Exit all older VoiceRewrite instances; only one tray icon should exist. Launch the new EXE in the folder holding your `config.json`.
3. Select editable text in Notepad and hold Mouse5 while saying "改得简洁一些", then release.
4. The icon should transition **red → yellow → blue → green**. If it remains red, check the log for mouse button press/release and timeout events.
5. Right-click the icon and choose **退出 VoiceRewrite** to restore normal Mouse5 behavior.

The log file is located in `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log` and records the build ID plus mode/hotkey on startup, not the selected text, dictated instruction, or audio data.

**Validation limits:** Go unit tests, mock HTTP integration tests, and Windows x64 cross-compilation can be run in a Linux environment. Real mouse hook delivery, tray status, microphone input and clipboard replacement must be verified on Windows 11 hardware.

---

## Previous keyboard-mode notes (optional)

# VoiceRewrite

Native Windows app: select some text, press the configured hotkey, dictate a revision instruction, press the hotkey again, and let a text LLM rewrite the selection.

## Setup

1. Run an ASR server with OpenAI-compatible `/v1/audio/transcriptions` (for example the OneClick launcher).
2. Run an OpenAI-compatible **text** chat model in LM Studio, Ollama/llama.cpp or another supported server. ASR-only models cannot rewrite text.
3. Build `VoiceRewrite.exe` or download the executable from GitHub Actions. Place VoiceRewrite in a **different folder** than OneClick.
4. On first launch VoiceRewrite creates `config.json`; edit its `asr_base_url`, `llm_base_url`, `text_model`, API keys, and hotkeys as needed. You may start with `config.example.json`.
5. Keyboard shortcuts remain available through `config.json` for users who prefer them.

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

- `"record_mode": "toggle"`: press your configured hotkey to start; press again to stop (previous default).
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

## 2026-11 reliability fixes

The push-to-talk flow now starts recording immediately on keydown but waits until **after the key is released** before copying selected text. This avoids Ctrl+C being sent while the hotkey itself is still held. The WinMM buffer recycler no longer clears the WAVEHDR `WHDR_PREPARED` state and reports requeue failures rather than silently dropping audio. Tray icon changes are handled on the Windows UI thread. Worker goroutine panics are logged and surfaced as errors where recoverable.

To test, keep an ASR server configured in `asr_base_url` and a loaded text-instruct model accessible via `llm_base_url`. Select a phrase in Notepad, hold F9 for 2–3 seconds while speaking an edit instruction, and release. The icon should transition from red to yellow to blue, then green on success (or orange if a step fails).

For diagnostics, if the icon disappears or the rewrite fails, inspect the log **locally** using:

```powershell
Get-Content "$env:LOCALAPPDATA\VoiceRewrite\VoiceRewrite.log" -Tail 60
Get-Process VoiceRewrite* -ErrorAction SilentlyContinue
```

Do not publish log contents without removing any private paths, hostnames, or identifiers. Audio and selected-text payloads are not logged by this program; step names and character counts are logged instead.

The Windows tray and WinMM paths require real Windows 11 validation. A successful cross-compile or mock HTTP test alone cannot verify physical hotkey release or actual microphone device behavior.

## Diagnostic build: hold-watch-v2

The startup log now contains `build=2026-11-13-holdwatch-v2`, `mode=hold`, and the selected recording hotkey. If the log does not contain that build ID, a previous EXE is still running.

In hold mode, a separate keyboard-state watcher detects physical key release and posts a message back to the Windows UI thread. After releasing F9, the tray icon switches from red to processing. If the key release cannot be detected by the configured maximum recording duration, recording is canceled with a visible error rather than staying red forever.

Use PowerShell to check the running executable location and version:

```powershell
Get-CimInstance Win32_Process | Where-Object { $_.Name -like '*VoiceRewrite*' } | Select-Object ProcessId, ExecutablePath, CommandLine
Get-Content "$env:LOCALAPPDATA\VoiceRewrite\VoiceRewrite.log" -Encoding UTF8 -Tail 35
```

This build is cross-compiled and unit-tested on Linux; real Windows keyboard, microphone and tray functionality requires desktop testing.

## Tray context-menu fix (v4)

The tray callback now uses a custom registered Win32 window procedure rather than a STATIC window. Shell notifications are forwarded to the UI message queue as WM_TRAY_EVENT, supporting sent and posted notifications. A separate tiny top-level toolwindow is foregrounded before TrackPopupMenu, and a WM_NULL is posted after closing for reliable repeated menus. The right-click menu contains **Edit config.json**, **View log**, and **Exit VoiceRewrite**. Local logs record menu-request and choice events without logging any user text. Build stamp: `2026-11-13-tray-menu-v4`. Real Windows desktop verification is still required.
