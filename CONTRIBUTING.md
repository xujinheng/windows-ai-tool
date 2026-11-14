# Contributing

VoiceRewrite is a Windows-first desktop voice tool; changes are judged by actual interaction behavior rather than extra UI decorations or version bumps.

## Build

Use Go 1.23 or newer. The client lives in `voice-rewrite/` and the independently buildable ASR launcher lives in `oneclick/`.

```powershell
cd voice-rewrite
gofmt -l .    # must print nothing
go test ./...
go vet -unsafeptr=false ./...
go build -trimpath -ldflags '-H windowsgui -s -w' -o VoiceRewrite.exe .
```

The `unsafeptr` analyzer is disabled for OS-owned Win32 callbacks/GlobalLock pointers; other vet analyzers remain enabled. CI executes the same Windows checks. Before submitting UI changes, run the cases listed in `voice-rewrite/WINDOWS-RELEASE-CHECKLIST.md` on a real interactive Windows session and clearly distinguish performed testing from test plans.

## Design and code review

Read `voice-rewrite/PRODUCT-DESIGN.md`. Do not steal focus from the target editor, paste into a different window, lose a complete result, or claim a paste was verified when only a key event was sent. Keep network activity off the low-level mouse-hook thread. Test changes to keyboard shortcuts, retries and state transitions.

Never include personal audio, selected text, API keys, clipboard dumps, local model paths, or `config.json` in issues or commits. Example configs must remain non-sensitive. Binary model files and llama.cpp/CUDA runtimes are not distributed by this repository.

The owner has not yet selected a repository-wide license. Do not assume redistribution or contribution licensing terms from repository visibility alone.
