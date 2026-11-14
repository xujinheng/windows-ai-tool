# VoiceRewrite v21: Everyday-use controls and build hardening

## Behavior

- **Pause/Resume from the tray**: a single click toggles assistant interception. While paused, the low-level mouse hook forwards XBUTTON1/XBUTTON2 normally; no model server is stopped. Pausing is rejected while a recording or inference task is active. The paused state has its own notification-area icon and tooltip.
- **No lost dictation on focus change**: if the target editor is no longer foreground when ASR completes, the full text appears in a native copyable result card. It is not written into an arbitrary different window.
- **No lost dictation on paste failure**: the full transcript is similarly available in a copyable card. This is a fallback, not a claim that clipboard formats are fully preserved or that insertion succeeded.

## Source discipline

- `.gitattributes` pins Go files to LF on Windows CI. Git for Windows can otherwise CRLF-convert all Go files, making `gofmt -l .` flag *every* file even though the repository is formatted.
- CI tests, vet and native Windows cross-compilation remain mandatory. Real Windows interactive UI testing remains separate.

## Known limitations

- There is no complete OLE clipboard snapshot/restore for images, HTML, files and arbitrary formats.
- Some rich web editors and high-integrity applications cannot be safely auto-pasted.
- Modern responsive Win32 settings are not equivalent to a full UI Automation accessibility audit.
