# VoiceRewrite v24 — distribution and user-data protection

## Implemented

- Conservative clipboard snapshot before dictation/selected-text capture, preserving accessible movable-HGLOBAL clipboard formats (including standard Unicode, HTML, RTF and file-list formats when provided as HGLOBAL).
- Fail closed before modifying a non-copyable clipboard format (bitmap/palette/metafile/owner-rendered/oversized/private handles). Dictation uses direct UTF-16 SendInput instead, preserving the original clipboard. This fallback is not guaranteed to work in every editor.
- Clipboard sequence ownership fences: never restore the old content after a different process/user writes to clipboard. Original formats stay in memory only and are never logged or persisted.
- Selection replacement no longer copies AI results to the clipboard when the target selection/focus changes. It shows a copyable result card.
- Settings and result windows respond to system light/dark theme changes live (no application restart).
- Windows per-user Inno Setup installer with uninstall and optional desktop shortcut, portable ZIP and SHA-256. CI runs Linux race tests and Windows checks. Preview releases only on version tags.

## Not yet guaranteed

- Full COM/OLE clipboard fidelity, delayed-rendered source objects, and native GDI handles; selection operations refuse unsafe clipboard formats.
- Verified insertion in all browsers/remote desktops/editors, Windows UI Automation accessibility, signed binaries, auto-update, and real-machine acceptance.
- Formal redistribution license: the owner must explicitly select and approve a LICENSE.

Do not label these unsigned, untested installers as production-stable.
