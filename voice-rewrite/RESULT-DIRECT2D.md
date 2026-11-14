# Direct2D + DirectWrite: production AI result window

This implementation is **connected to the real VoiceRewrite result path** in
`voice-rewrite/popup_windows.go`. It is not an isolated mockup or new settings
page. It contains no web engine, Node.js, UI framework, or external Go module.

## Runtime model

- The existing main Go/Win32 message pump, low-latency tray, microphone and AI
  request logic are unchanged.
- When a result is ready, `showAnswerPopup` creates the same result HWND and
  constructs a `resultRenderer` on the UI thread.
- Direct2D paints the flat light/dark surfaces, rounded reading panel, subtle
  separator and brand. DirectWrite draws the Chinese action title and labels.
- A real `EDIT` HWND displays **the untruncated result**; selection, Ctrl+C,
  scroll and its caret/keyboard behavior remain managed by Windows.
- Three real `BUTTON` HWNDs offer **Apply**, **Copy**, and **Close**, with
  keyboard focus and an explicit Escape handler.
- `Apply` calls the unchanged `answerPopup.apply()` and
  `replaceSelectionWithGuard` to check the original editor focus/selection,
  epoch and task state before any text replacement. The UI never substitutes a
  direct, unguarded clipboard/paste operation.
- Answer-only and stale-target scenarios omit the Apply action while retaining
  a fully copyable result.
- Close releases Direct2D, DirectWrite, GDI and HWND resources. Windows resize
  and D2DERR_RECREATE_TARGET also release/rebuild device-dependent resources.
- If native graphics initialization is unavailable, GDI draws a visible
  fallback heading/card while the same Win32 text and buttons stay usable.

## Design scope

This is the compact **result/rewrite surface**, not a dashboard, command
center, advanced-settings shell, or a replacement for the limited settings.
The design uses restrained typographic hierarchy, subtle lilac accents,
one reading panel and two prominent actions.

## Verification

The `direct2d-result.yml` Windows workflow:
1. Runs all production Go tests and compiles the **actual application**.
2. Opens the real result window with only fixed visual-preview data.
3. Checks the frameless HWND, native Copy/Close and conditionally Apply.
4. Captures real Windows pixels for light rewrite, dark rewrite, answer-only
   and long-form rewrite.
5. Records a process working-set and private-memory snapshot.

Preview windows use fixed text and a deliberately inert Apply; no microphone,
API, original editor or persisted configuration is touched by CI.

### Remaining release checks

- Manually verify mouse-driven window drag, button keyboard focus, screen
  reader announcements, Ctrl+C, text selection and long-scroll positioning.
- Test Windows 10/11, Chinese IME, DPI changes and HDR/non-default fonts.
- Exercise real rewrite + guarded application into browsers, native fields and
  unsupported/inaccessible target editors.
- Report **full app** background/active/result-window memory on the same
  Windows workstation as the 40.4 MB baseline; preview process measurements
  cannot establish the delta.

Do not merge the draft solely on the basis of CI screenshots.
