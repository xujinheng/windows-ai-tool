# VoiceRewrite: 30-second workflow demonstration

The [README video](voicerewrite-demo.mp4) gives a first-time visitor a direct view of the product interaction:

1. **Mouse4 (lower side button):** hold to dictate and release to type at the current caret.
2. **Mouse5 (upper side button):** select text, hold, and say a rewrite instruction; the workflow may replace the original selection after validation.
3. **Mouse5 (upper side button):** select text and ask a question instead; a copyable AI answer card appears without changing the selection.

**Evidence and limitations:** This is an **illustrated, scripted product demonstration**, not a recording of real mouse hardware, an actual microphone, or real ASR/LLM responses. Its recording pill is composited from a real screenshot captured from VoiceRewrite.exe on Windows. The demonstrated editor, sentences, selection and voice-command captions are deliberately synthetic. The underlying UI captures are maintained under [../ui/](../ui/).

To regenerate on Linux/Windows with `ffmpeg` and `Pillow>=11`:

```bash
python scripts/render-demo-video.py --assets docs/ui --output docs/demo
```

The [demo-video.yml](../../.github/workflows/demo-video.yml) workflow updates `voicerewrite-demo.mp4`, `voicerewrite-preview.gif` and `poster.jpg` when source UI screenshots change. A stale run will not overwrite assets from a newer main commit.
