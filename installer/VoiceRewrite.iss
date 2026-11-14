#ifndef AppVersion
#define AppVersion "0.25.0"
#endif
#define AppName "VoiceRewrite"
#define AppExe "VoiceRewrite.exe"

[Setup]
AppId={{7E5BF57A-BC76-4A80-88DC-3BD00FE3C904}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=xujinheng
AppPublisherURL=https://github.com/xujinheng/windows-ai-tool
AppSupportURL=https://github.com/xujinheng/windows-ai-tool/issues
DefaultDirName={localappdata}\Programs\VoiceRewrite
DefaultGroupName=VoiceRewrite
UninstallDisplayIcon={app}\{#AppExe}
PrivilegesRequired=lowest
; Prevent upgrading or uninstalling while the tray process is running.
AppMutex=Local\VoiceRewrite_OneInstance_v1
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=output
OutputBaseFilename=VoiceRewrite-Setup-{#AppVersion}-x64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
DisableProgramGroupPage=yes
VersionInfoVersion={#AppVersion}

[Files]
Source: "..\voice-rewrite\VoiceRewrite.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\voice-rewrite\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\voice-rewrite\config.example.json"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\VoiceRewrite"; Filename: "{app}\{#AppExe}"
Name: "{autodesktop}\VoiceRewrite"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Run]
Filename: "{app}\{#AppExe}"; Description: "Launch VoiceRewrite"; Flags: nowait postinstall skipifsilent
