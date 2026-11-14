# VoiceRewrite v25 — Windows native usability and privacy

This is a substantial development iteration, **not** a production certification.

## Changes delivered

1. **Microphone selector:** the native settings window lists WinMM recording inputs and the system default. The selected `audio_device_id` persists; the app must be restarted for device or shortcut changes.
2. **Privacy:** a single dedicated COM MTA worker checks the focused UI Automation element's `CurrentIsPassword`. The UI caller waits at most 800 ms. On browser-like windows an unavailable provider causes the operation to be rejected.
3. **Update UX:** a tray command checks the public GitHub Releases feed over HTTPS on user request. The response is limited to 512 KiB and tags are ordered with SemVer. Only the fixed GitHub Releases page may be opened, after approval; no EXE is automatically executed.
4. **Installer:** the existing per-user Inno Setup package detects VoiceRewrite's running named mutex before install/uninstall.

## Limits

- Windows 11 live tests for password fields, WeChat/VS Code insertion, microphone changes, DPI and installer upgrade have **not** been performed in this environment.
- UI Automation providers may report wrong information or hang. Timeouts limit the caller but not a stalled COM provider; after that, opaque browser windows remain blocked.
- Full OLE clipboard fidelity (including owner-rendered formats), OS UI Automation accessibility for VoiceRewrite's own custom controls, code-signing, trustworthy automatic update, and an owner-selected open-source license are not finished.
- A successful Go build or GitHub Actions job does not establish real desktop correctness.

## Verification

`gofmt`, unit tests, Go race checking, static checking, Windows cross-compilation and GitHub Windows build/install-package CI. See the Actions run for the current commit.
