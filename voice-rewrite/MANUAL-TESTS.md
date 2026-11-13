# Windows 11 acceptance checklist — Intent Popup v6

1. Exit all old VoiceRewrite processes. Ensure the latest startup log shows `build=2026-11-13-intent-popup-v6`.
2. In Notepad, select some text; hold Mouse5 and say `请写成英文`; release. Confirm the translated text replaces selection, not a popup (if the focused control is reported as a native editable control).
3. On a web page, select a technical term; say `这是什么意思`. Confirm a nonempty explanation popup appears near the pointer and the webpage remains unchanged.
4. Close the popup with title-bar X; repeat and close with bottom Close button; repeat and choose Copy and Close; repeat and use tray Close AI Result Window.
5. Select read-only webpage text; say `翻译成中文`. Confirm the translation is shown in a copyable popup, not pasted into the webpage.
6. Test a browser textarea. It can be classified as unknown and show a preview; this is a deliberate safety limitation.
7. Open and exit the tray menu, verify the Mouse5 button returns to its usual functionality after program exit.
8. Check the log for `stage=selection`, `stage=asr`, `stage=llm`, `intent`, `route` and `popup` events. No selected or dictated user content is logged.
9. Test ASR errors, tiny recordings, focus switching, and rapid double presses; verify no freezing and no empty popup.
10. To exercise the native popup lifecycle with Go tests on an interactive Windows desktop, set `VOICE_REWRITE_NATIVE_POPUP_TEST=1` and run `go test -run TestNativePopupPaintAndClose -v`.

## v7 acceptance (requires an interactive Windows 11 desktop)

1. Close old VoiceRewrite processes, launch v7 and check build ID in log.
2. In Notepad select Chinese text, say `请写成英文`, release Mouse5. Expect direct replacement, no card.
3. In Chrome/Edge inside an editable ChatGPT draft textbox, select text, say `请写成英文` or `润色一下`. Expect direct paste when validation succeeds; otherwise a card with `应用到原文`.
4. Select static article text in the browser, say `这是什么意思`. Expect answer card, no editing, and no `应用到原文` button.
5. Select static article text, say `改得简洁一点`. The guarded paste probe may fail; expect a preview rather than a false success. Revisit this scenario after an accessibility-based editability check is implemented.
6. Click card `复制结果`, `关闭`, top-right X, Esc, and tray `关闭 AI 结果窗口` independently.
7. Click `应用到原文` after switching selection/window and confirm it refuses to overwrite the wrong content.
8. Check the card on multi-monitor / different DPI, narrow or long result texts, and inspect `route=...` and `stage=paste` log entries.

Do not assume successful go test or Windows cross-compilation verifies browser contenteditable behavior.
