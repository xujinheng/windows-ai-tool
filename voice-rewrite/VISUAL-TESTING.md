# UI screenshots

The [Windows UI screenshots workflow](../.github/workflows/visual-matrix.yml) builds and starts `VoiceRewrite.exe` on a Windows runner, captures the settings window, result card and recording indicator, and checks their bounds.

The workflow covers light/dark themes, long result text, settings scrolling and internal 125–200% UI scale overrides. It publishes README images under `docs/ui/`. The [demo video workflow](../.github/workflows/demo-video.yml) generates the walkthrough from those images.

The screenshot inputs use example text and preview states. Internal scale overrides do not test Windows system DPI changes. Physical multi-monitor behavior, keyboard accessibility and end-to-end dictation are covered by the [interactive checklist](TESTING.md).
