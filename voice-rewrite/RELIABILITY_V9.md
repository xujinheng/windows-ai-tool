# VoiceRewrite v9 reliability changes

Release date: 2026-11-13.

## Core interaction unchanged

Mouse5 hold/release. Local ASR and LLM. Clear edit, polish, shorten, and translate requests still attempt direct Ctrl+V replacement in WeChat, VS Code, browser inputs, and Notepad. Questions/explanations still use the answer card.

## Engineering additions

- Named Win32 mutex: second v9 instance is rejected. Older builds do not share this mutex, so exit v8 and earlier manually before using v9.
- Session epoch: every capture has a numeric generation. A stale worker cannot show a popup or initiate a paste after shutdown or a newer capture; the popup Apply button also checks its session and closed state.
- Best-effort plain-text clipboard recovery on early worker failures; sequence-number guard avoids overwriting an intervening user copy.
- When the paste verification probe performs Ctrl+C again, record its clipboard sequence to prevent accidental loss of plain-text restore.
- Explicit `paste_status=unverified` log: SendInput acknowledging a paste is not reliable confirmation that an editor accepted the text.

## Limitations

- Still uses keyboard/clipboard simulation, not a complete UI Automation editor adapter.
- The app cannot fully preserve images, file lists, HTML, or all clipboard formats. This is not solved in v9.
- Clipboard sequence numbers do not prove exclusive write ownership under adversarial scheduling.
- A stale task already in the middle of a `SendInput` call cannot be reversed atomically; generation checks happen before the input step.
- No Windows 11 physical mouse, clipboard, WeChat, VS Code, or Chrome interaction test was possible in the build environment.

## Recommended acceptance procedure

1. Close **all** previously launched VoiceRewrite processes.
2. Run v9 in the same folder as your existing `config.json`; do not overwrite your configuration.
3. Verify `started; build=2026-11-13-reliability-v9` in `%LOCALAPPDATA%\VoiceRewrite\VoiceRewrite.log`.
4. WeChat: select short text, say "改得简洁一点", verify replacement.
5. VS Code: select one line, say "翻译成英文", verify replacement only of the selected range.
6. Chrome/Edge ChatGPT textarea: select text, say "请写成英文", verify replacement.
7. Select a web article term, say "这是什么意思", verify popup contents, close button, Esc and tray exit.
8. Start v9 twice; second instance should display an already-running error and exit.
9. Test prior text-only clipboard recovery when ASR is unavailable; then test copying something new while LLM is busy and ensure it is not overwritten.
10. If you need to preserve a clipboard image or file list, do **not** rely on v9; this remains an explicit known limitation.

No raw selected text or voice audio should be included when sharing diagnostic logs.
