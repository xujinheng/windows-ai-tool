# Windows installer

`VoiceRewrite.iss` builds a **per-user** Inno Setup installer for Windows 11 x64.
It installs `VoiceRewrite.exe`, a Start-menu shortcut, an optional desktop
shortcut, and an uninstaller. No administrator privileges are requested.

The user's `%APPDATA%/VoiceRewrite/config.json` and DPAPI-protected API keys
are stored outside the install directory and are **preserved on uninstall**.
The installer never installs CUDA, llama.cpp, ASR models, or OneClick.

On GitHub Actions, the Windows build compiles the Go executable first, then
Inno Setup generates `VoiceRewrite-Setup-<version>-x64.exe` and its SHA-256.
The installer is currently **unsigned**, so Windows SmartScreen may warn.
Authenticode signing requires a publisher-owned certificate or a supported
cloud-signing service; do not treat a successful build as trusted distribution.

To package locally after building Go, install Inno Setup 6 and run:

```powershell
& "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe" /DAppVersion=0.24.0 installer\VoiceRewrite.iss
```

Do not create a public stable release or claim Store eligibility without
publisher signing, native desktop validation, and a reviewed project LICENSE.
