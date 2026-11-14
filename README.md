# VoiceRewrite v19 (client-only)

Windows 11 x64 native Go desktop voice dictation and AI rewrite tool.

This source snapshot was restored from the v19 user-provided Windows release archive.

Read [the detailed README](voice-rewrite/README.md) for usage, API configuration, privacy and limitations.

## Development

```powershell
cd voice-rewrite
go test ./...
go vet ./...
go build -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .
```

This version has not been validated in a real interactive Windows desktop environment as part of this recovery.
There is no project-wide LICENSE file: add a user-approved license before advertising redistribution terms.

## Separate model services

For local GGUF models, use [the PowerShell two-server launcher](scripts/local-models/README.md) without changing the client-only v19 application.
