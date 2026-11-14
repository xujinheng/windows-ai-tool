# VoiceRewrite Windows visual acceptance

The application remains a native **Go + Win32 + GDI+** executable. We test the **actual executable** on GitHub-hosted Windows desktops using real HWND bounds and `Graphics.CopyFromScreen`, not a browser mockup.

## Automated screenshots

Run the `VoiceRewrite visual matrix (real Windows pixels)` GitHub Action workflow. The artifact `VoiceRewrite-v29-real-windows-visual-matrix` contains PNGs for these cases:

- Native settings window in simulated light and dark themes.
- Settings at internally simulated 125%, 150% and 200% UI scaling.
- 200% settings window before and after scrolling both axes to their ends.
- AI answer card in light mode and dark mode with long text.
- The layered GDI+ recording pill in a 200% preview.

The recording preview does **not** activate microphone hardware or call ASR/LLM; the actual GDI+ window and its real controls are used. The long result card contains synthetic fixture text.

CI fails if a visual window is outside the virtual display, if screenshots are missing/very small, if dark and light frames are byte-identical, or if **Save / Advanced / Close** is clipped after scrolling at 200%.

## Coverage limitations

**Internal scale overrides are not the same as changing Windows OS DPI.** The 125–200% screenshot cases use `VOICEREWRITE_VISUAL_SCALE` inside preview mode on the same 1024×768 desktop. Native system scaling, user font scaling, mixed-DPI monitors and DPI transition events still require an actual interactive Windows setup.

The GitHub-hosted runner presently supplies only one virtual display. The code routes windows to a specific monitor, but real multi-monitor physical placement and unplug/replug transitions need a multi-display Windows test host; CI does not establish that coverage.

The screenshot job validates geometry and appearance. Separate interaction testing is still required for keyboard navigation, screen-reader announcements, clipboard round trips and text replacement in other applications.

Never represent these screenshots as an actual audio-transcription or AI-service end-to-end test.
