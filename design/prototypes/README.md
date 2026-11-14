# VoiceRewrite Studio — Raycast × Linear design exploration

This is an **interactive, self-contained HTML concept**, not a shipped Windows GUI.
Open `voice-rewrite-studio.html` in a modern browser to explore the experience.
No network, CDN, dependency, or real model/audio/clipboard-insertion access is needed.

## Intent

Move beyond the current “Win32 API form with rounded boxes” visual model.

- **Raycast-inspired:** task-first navigation, discoverable Ctrl+K command palette,
  fast jump to an action, keyboard-centric interactions, small predictable controls.
- **Linear-inspired:** subdued sidebar, calm information hierarchy, restrained
  color, fewer visible boundaries, consistent typographic and spacing tokens.
- **VoiceRewrite's own identity:** an original subtle lilac/rose waveform mark,
  Chinese-first copy, speak → review → apply/copy as the product's primary flow.

Original designs are referenced for **principles**, not duplicated as assets or
trademarked interface elements.

## What to try

1. Switch **概览 / 语音听写 / AI 写作 / 快捷键 / 外观与显示 / 高级选项**
   using the left navigation.
2. Press **Ctrl+K** (or Cmd+K) to search commands, type and use arrow keys/Enter.
3. Open **结果预览** to see the reading-first AI result card.
4. Toggle “原输入位置已不可用”; **应用到原文** becomes unavailable,
   but copy remains accessible.
5. Try the recording pill and dark/light theme switch.
6. Edit a configuration field or bind a shortcut. Everything is **demo-only**.

All interaction test values are simulated. The screenshot/HTML is **not an
accurate reflection of current API connectivity**, does not persist settings,
and does not modify any Windows application.

## Integration recommendation

Treat the prototype as a visual and interaction specification. Before replacing
the Windows implementation, decide on a UI technology and constraints:

- Keep the low-latency tray, keyboard hooks, audio pipeline, model requests,
  focus guards, and clipboard-sensitive insertions in the existing Go core.
- Separate the **settings workspace** from a **small result/recording surface**;
  never launch a large dashboard for a momentary edit.
- Consider WebView2 for a modern settings workspace if packaging and memory
  impact are acceptable; otherwise a genuinely reusable custom native controls
  layer is required. Evaluate startup time, DPI, accessibility, IME, and OS
  compatibility before choosing.
- Preserve real model-verification semantics, keyboard navigation, screen-reader
  labels, high-DPI behavior, dark theme, transparent error states, and copying
  when a target is no longer safe to modify.

## References

- [Raycast Settings](https://manual.raycast.com/settings)
- [Raycast v2](https://manual.raycast.com/new-in-v2)
- [Linear: A calmer interface for a product in motion](https://linear.app/now/behind-the-latest-design-refresh)
- [Linear UI refresh](https://linear.app/changelog/2026-03-12-ui-refresh)
