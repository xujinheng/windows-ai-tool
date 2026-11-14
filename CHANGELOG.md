## v0.25.0 — Windows interaction hardening (development)

- Added microphone selection in Settings using Windows WinMM enumeration, persisting `audio_device_id` (-1 means system default).
- Added UI Automation `CurrentIsPassword` checks on a separate COM worker, with conservative browser behavior when the provider is unavailable.
- Added a user-triggered GitHub release check (SemVer, strict HTTPS, response limits, no unattended EXE downloads).
- Guarded the installer upgrade using the process's existing named mutex.
- Expanded tests for update ordering, Windows input structures and UI privacy; normal CI format/test/build still applies.

**Not production certified:** no clean-machine Windows 11 accessibility/clipboard/device validation, no signed installer, no automatic signed updates, and the repository still needs an owner-approved LICENSE.

# Changelog

## v24 — Clipboard fidelity, native theme refresh, packaging and privacy

- Multi-format in-memory clipboard HGLOBAL snapshots, guarded restoration, and Unicode input fallback for dictation when backup is unsafe.
- Selection results remain in a copyable card instead of overwriting the clipboard after target focus/selection changes.
- Live light/dark refresh in settings and answer card; native Windows password-style edit fields excluded from global recording and rewriting.
- Technical logs rotate at 4 MiB and retain three backups.
- Per-user Inno Setup installer with uninstall, checksums, GitHub version-tag preview releases, and Linux race-testing job.
- Installer is unsigned; native desktop and full OLE/UI Automation fidelity remain open work.

## v23 — Core architecture

- Split Windows UI, recording, selection, clipboard, ASR, LLM, configuration and task execution into focused Go files.
- Propagate cancellation to network requests, chunk long recordings at quieter audio windows, and block cross-host HTTP redirects.
- Unified initial Windows light/dark palettes and improved remote API error reporting.

## v22 — Native Windows everyday use

- Configurable F1–F24 hold-to-dictate, keyboard-accessible settings and in-session result recovery.
- Model-ID-aware connection diagnostics, DPAPI credentials, and complete result card text.
- Portable ZIP with SHA-256 and CI no longer auto-commits generated binaries.

## v21 — Reliability

- Pause/resume global mouse-button interception from the tray.
- Copyable fallback card if dictation cannot safely insert; normalize Go source line endings.

## v20 — UX baseline

- Resizable settings window, separated connection status, safer rewrite authorization and actionable errors.

## v19 — Client-only architecture

- ASR and LLM are independently configured HTTP API endpoints; models run outside the client.

Version notes describe source-level work; completed builds do not replace real Windows desktop usability testing.
