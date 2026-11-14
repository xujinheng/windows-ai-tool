# Contributing

The Windows application is in `voice-rewrite/` and the ASR launcher is in `oneclick/`. Go 1.23 or newer is required.

```powershell
cd voice-rewrite
gofmt -l .
go test ./...
go vet -unsafeptr=false ./...
go build -trimpath -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .
```

Changes to UI or input handling should include the relevant [Windows interaction checks](voice-rewrite/TESTING.md). Keep the mouse hook and UI thread free of network and model work. Preserve the original editor's focus and clipboard data when possible.

Never commit credentials, microphone recordings, selected user text, diagnostic logs or local model paths. See [security](SECURITY.md).

VoiceRewrite is licensed under [BSD 3-Clause](LICENSE). By contributing, you confirm that you have the right to submit your work under this license. Preserve existing copyright notices and third-party attributions.
