# VoiceRewrite · macOS-inspired visual language v2

VoiceRewrite is a **quiet writing companion**, not a developer API demo. The product
experience should support **speak → review → apply or copy** without distraction.

## Principles

1. **Content first.** Recognition and AI output carry the visual weight; controls are
   calm, compact and predictable.
2. **One family.** Settings, result card, and recording pill share neutral surfaces,
   the berry accent, Chinese/Latin typography, spacing and soft corners.
3. **Human language.** Explain service connections with “语音转文字 / 智能文字处理”
   rather than making protocol terminology the page headline. Retain API fields
   for configuring supported external/local endpoints.
4. **Confidence without false promises.** Never suggest a provider is connected
   before probing, a paste was verified when it was not, or the client is always
   local/private when the user configured remote services.
5. **Native access.** Keep real Win32 EDIT/COMBOBOX controls for IME, selection,
   keyboard navigation and accessibility. GDI paints framing rather than
   replacing text inputs with inaccessible imitation controls.

## Palette

| Role | Light RGB | Dark RGB |
| --- | --- | --- |
| Window background | `#F6F7FA` | `#171B23` |
| Surface | `#FFFFFF` | `#232934` |
| Field | `#F8F9FB` | `#2C3340` |
| Main text | `#1F2937` | `#F4F6F9` |
| Supporting text | `#6B7889` | `#A8B4C3` |
| Border | `#E1E6ED` | `#3B4654` |
| Primary action | `#B84B78` | `#E477A0` |

Use Microsoft YaHei UI for mixed Chinese and Latin text in native windows,
especially read-only EDIT results, to avoid unwanted serif fallback.
Suggested typographic hierarchy: title 24–26, section 19–20, body 16–18,
helper 13–14 logical px. Work on an 8px rhythm with 10–18px rounded surfaces.

## Interaction

- **Settings:** a compact, centered three-tab preference switcher: **语音输入**,
  **AI 写作**, and **快捷键**. Show one logical category at a time instead of
  stacking every server field. Keep the Save action pinned to the footer.
  At narrow/high-DPI sizes, field labels stack over controls and only the
  content pane scrolls vertically. No horizontal scrollbar is required.
- **Result card:** a small branded mark → prominent result title → honest
  safety status → border-light, selectable and scrollable reading pane → thin
  divider and compact footer actions. The complete result is always available
  for copying. Writing to another application continues to
  require explicit user intent and existing selection/focus guards.
- **Recording pill:** stay compact; prioritize waveform/timer and state transitions,
  not additional buttons, labels or settings.

## Release acceptance

Review screenshots of real Windows UI in light and dark themes at 100/125/150/200%
scaling. Test on an interactive Windows 11 desktop with Notepad, WeChat, VS Code,
Chrome, Chinese IME, Tab/Shift+Tab, focus switching, screen readers and external
ASR/LLM endpoints. CI screenshots are visual evidence, not proof of real-world
accessibility or cross-app editing reliability.

**Platform approach:** this is Windows software inspired by Apple’s visual
hierarchy, spacing, progressive disclosure, and restrained surfaces—not a
Mac title-bar clone or a simulated Liquid Glass layer. Real Win32 controls,
IME, and native window behaviors remain intact.

Reference patterns: [Apple HIG Layout](https://developer.apple.com/design/human-interface-guidelines/layout),
[Designing for macOS](https://developer.apple.com/design/human-interface-guidelines/designing-for-macos),
and [Materials](https://developer.apple.com/design/human-interface-guidelines/materials).

**Known limitations:** real Windows accessibility, custom scrollbar appearance,
focus/IME behavior and DPI/monitor transitions still need interactive testing.
Automated screen captures are visual regression evidence, not production sign-off.
