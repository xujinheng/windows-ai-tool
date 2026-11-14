# Windows installer

`VoiceRewrite.iss` builds a per-user installer for Windows 11 x64 using Inno Setup 6. It installs the application, shortcuts and an uninstaller without requiring administrator access.

User configuration under `%APPDATA%\VoiceRewrite` is preserved during uninstall.

After building `voice-rewrite/VoiceRewrite.exe`, run from the repository root:

```powershell
$version = '0.29.0'
& "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe" "/DAppVersion=$version" installer\VoiceRewrite.iss
```

CI also publishes the installer, portable ZIP and SHA-256 checksums. The installer is currently unsigned. See [download verification](../RELEASE-TRUST.md).
