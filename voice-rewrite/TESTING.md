# Windows testing

Run the Go test suite and Windows GUI build before merging:

```powershell
cd voice-rewrite
gofmt -l .
go test ./...
go vet -unsafeptr=false ./...
go build -trimpath -ldflags="-H windowsgui -s -w" -o VoiceRewrite.exe .
```

## Interactive acceptance checklist

Use disposable text and a Windows 11 desktop with ASR and LLM services configured.

- [ ] Fresh launch: configure both API endpoints, microphone and shortcut; save and reopen Settings.
- [ ] Mouse4: dictate into Notepad, VS Code and a browser text field; release to insert.
- [ ] Mouse5: select text and request a rewrite or translation; confirm only the intended selection changes.
- [ ] Mouse5: select read-only text and ask a question; confirm the answer card opens without modifying the source.
- [ ] Result card: copy the full answer, scroll long text, close with the button, title bar and Escape.
- [ ] Switch editor or selection while an API request is pending; stale output must not replace newer text.
- [ ] Cancel an active task; confirm network work stops and nothing is inserted.
- [ ] Test clipboard contents containing text, HTML, rich text, file lists and unsupported formats.
- [ ] Test native password controls and browser password fields; confirm input is blocked where detected.
- [ ] Try connection failures: invalid key, unreachable host, missing endpoint, invalid model ID.
- [ ] Verify the tray, pause/resume, session history and exit restore mouse-button behavior.
- [ ] Test at 100%, 125%, 150% and 200% system display scaling, including two monitors with different scaling.
- [ ] Test fresh installation, upgrade and uninstall; user settings should survive uninstall.

## Automated screenshots

[Windows UI screenshots](VISUAL-TESTING.md) documents the GitHub Actions visual checks. Screenshot previews use deterministic example data; interactive scenarios above require manual verification.

A test checklist is not evidence that the listed scenario passed.
