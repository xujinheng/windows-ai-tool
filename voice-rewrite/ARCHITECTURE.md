# VoiceRewrite: input/clipboard architecture (threaded-input-v3)

This is a Windows desktop automation prototype. The design intentionally separates:

1. **Dedicated low-level mouse-hook OS thread** (`mouse_windows.go`). It owns the `WH_MOUSE_LL` hook and a private `GetMessageW` message pump, never runs clipboard or HTTP work, suppresses only the chosen XBUTTON, and posts `WM_SIDE_DOWN`/`WM_SIDE_UP` to the UI window. `WM_QUIT` is posted to the same thread to uninstall the hook.
2. **UI/main message thread** (`windows.go`). It owns the tray icon, hotkey registrations, recording state transitions and small WinMM start/stop requests. It does not poll the clipboard, call ASR/LLM HTTP endpoints, or block on the recorder's completion channel during normal operation.
3. **Background operations** (`windows.go`, `main.go`). On release, immediately mark busy and show the yellow tray state, then copy the selected text on a worker with a bounded retry budget. Audio readback, ASR, text-model rewrite and final revalidation/paste all run off the UI and hook threads.

## State machine

```
Idle -> Mouse5 down -> Recording (red)
Recording -> Mouse5 up -> Processing (yellow) -> Rewriting (blue) -> Success (green) -> Idle
                                  |                  |               |
                                  +-- failure -------+---------------+--> Error (orange) -> Idle
```

The existing `record_mode=toggle` remains available, and even its initial clipboard capture is now offloaded from the UI thread. `Mouse5`/`hold` remains the default for fresh configuration files.

## Performance and troubleshooting

The log contains `build=2026-11-13-threaded-input-v3` and stage measurements (`stage=selection`, `stage=asr`, `stage=llm`, `stage=paste`, milliseconds). Audio and selected document contents are never logged by the app. HTTP errors may contain provider-supplied diagnostics; logs stay on the user's computer and are excluded from Git.

The default selection copy budget is 2 attempts, each up to 750ms, plus a short settle/backoff. **If migrating from an older install, existing local `config.json` values are preserved:** edit `selection_copy_attempts` and `selection_copy_timeout_ms` manually to use the new defaults.

Microsoft's guidance for `WH_MOUSE_LL` explicitly recommends a dedicated thread that quickly forwards events. Reference: https://learn.microsoft.com/en-us/windows/win32/winmsg/lowlevelmouseproc

## Known limitations

- This version still uses clipboard-based `Ctrl+C`/`Ctrl+V` because broad, tested UI Automation selection support is not implemented yet. Some editors, protected windows, browser surfaces or elevated applications may not expose a usable selection. Failure is nonmodal; review logs from the tray menu.
- Only **plain-text clipboard contents** can be restored. **Rich text, files, images and complex clipboard formats are not preserved** by this implementation. Restoration occurs only if clipboard sequence number indicates no intervening external changes. Avoid using this tool when preserving those formats is essential.
- Windows 11 desktop/physical mouse/IME behavior cannot be fully validated by Linux cross-compilation. Check `MANUAL-TESTS.md` after downloading a build.
- A device driver that remaps the chosen side button to another input might bypass `XBUTTON2` events. You can switch to `Mouse4` or a function key in `config.json`.
- Although the hook is isolated, WinMM APIs and selected-app clipboard copy can still be delayed by OS or device drivers. Such delays should now affect **the operation** rather than freezing the desktop's mouse hook/tray UI.
