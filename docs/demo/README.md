# Product demonstration

The [30-second demo](voicerewrite-demo.mp4) shows dictation (Mouse4), selected-text editing and questions (Mouse5). The README displays the [animated preview](voicerewrite-preview.webp).

The editor text and spoken commands are scripted examples. The recording indicator is captured from VoiceRewrite on Windows.

Regenerate the files with Python, Pillow and FFmpeg:

```bash
python scripts/render-demo-video.py --assets docs/ui --output docs/demo
```

The [demo video workflow](../../.github/workflows/demo-video.yml) runs when the UI screenshots change.
