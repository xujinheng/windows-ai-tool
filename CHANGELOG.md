# Changelog

## v22 — Native Windows experience
- Configure a single F1–F24 key for push-to-talk dictation alongside the existing mouse-side-button flow. Keyboard shortcut changes require restart.
- Add Tab/Shift+Tab navigation and Esc-to-close in Settings.
- Recover the most recent dictated/generated result from the system tray without persisting history.
- Diagnose configured model ID from OpenAI-compatible `/v1/models` results, invalidate probes after edits, and show more actionable feedback.
- Protect API keys through Windows DPAPI, sanitize endpoint logging, and preserve complete result-card text.
- Package a portable Windows ZIP with SHA-256 verification alongside the existing EXE artifact.\n- Keep CI read-only toward source branches: no bot commits of rebuilt binaries, which previously caused concurrent-push failures.

## v21 — Everyday-use reliability
- Pause/resume tray interception and restore mouse side buttons when paused.
- Preserve dictation output in a copyable card if focus changes or insertion fails.
- Normalize Go source line endings to LF on Windows CI.

## v20 — UX foundation
- Separate connection status, resizable settings window, copyable untruncated results, guarded rewrite routing and meaningful errors.

## v19 — Client-only architecture
- Keep ASR and LLM as separate configurable HTTP API services. The client does not manage model processes.

**Scope:** Version notes describe source-level behavior. GUI accessibility, DPI layout and cross-app insertion require real Windows interactive testing.
