# Windows 11 smoke-test checklist (requires a real Windows desktop)

Close all previous VoiceRewrite EXEs before testing. Keep your local `config.json` and do not upload it.

**Fresh defaults:** `record_mode=hold`, `hotkey_record=Mouse5`, `selection_copy_attempts=2`, `selection_copy_timeout_ms=750`.

1. Confirm the running version from the right-click tray -> View log: `build=2026-11-13-threaded-input-v3`.
2. Open Notepad, select one sentence, hold the *second* mouse side key for two seconds and say a rewrite instruction. Observe red while held, yellow on release, blue while rewriting, and green after replacement.
3. Repeat in a standard browser textarea and a rich-text editor. Verify the selected text remains in the same window, and the UI/tray remain responsive even when a target refuses Ctrl+C.
4. Try without any selection. Expect orange status and a logged error, **not a modal popup**. Other mouse movement and clicking must remain responsive.
5. Right-click tray -> Edit `config.json`, View log, Exit. Confirm the bound side button returns to normal browser forward/back navigation after Exit.
6. Test switching foreground windows mid-recording: the tool should abort replacement. Test user changing the selection during ASR/LLM: the tool must not replace a different selection; it should leave the rewrite text on clipboard for manual paste.
7. Test 10 repeated short recordings in Notepad. Check there is never a stuck red indicator and there is one start and finish log per event.
8. Confirm audio service (`asr_base_url`) and text service (`llm_base_url`) are reachable. From PowerShell use `Invoke-RestMethod http://127.0.0.1:1234/v1/models` for LM Studio if configured on that address.
9. Verify Windows Settings > Privacy & security > Microphone permits desktop apps if recordings are empty.

View diagnostic log:

```powershell
Get-Content "$env:LOCALAPPDATA\VoiceRewrite\VoiceRewrite.log" -Encoding UTF8 -Tail 60
```

PowerShell listing of running EXEs:

```powershell
Get-CimInstance Win32_Process | Where-Object { $_.Name -like '*VoiceRewrite*' } | Select-Object ProcessId, ExecutablePath
```

Do not upload screenshots containing a full local path or raw configuration with private API keys.
