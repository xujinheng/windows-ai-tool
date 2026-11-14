# VoiceRewrite v20: Experience Foundation

> Scope: first real design-system and reliability refactor. This is not a claim of completed Windows desktop acceptance testing.

## Interaction model

- **Mouse4**: hold to dictate at the caret; releasing starts ASR and insertion.
- **Mouse5**: select text, hold and speak an explicit editing/translation request; a deterministic phrase gate permits guarded replacement. Questions, summaries, and uncertain requests stay as non-destructive previews, even if the LLM independently calls the response `edit`.
- **Results**: native, non-activating output card, no character truncation, and full clipboard copy. The edit widget's Win32 text limit is increased before writing so long generated text remains available.
- **Errors**: users can inspect the latest actionable recovery message from the tray instead of opening an implementation log. The detailed log remains available separately. 415, authentication, connectivity, timeouts and changed edit targets have explicit guidance.
- **Connections**: separate independent diagnostics for ASR and LLM, so an asynchronous result from one service cannot hide the other. Only reports HTTP connectivity unless an actual task is performed.
- **Security**: on Windows, API keys saved through Settings are protected by user-scoped DPAPI in the on-disk config (`dpapi:v1:`); the process keeps only decrypted values in memory. Legacy plaintext config keys remain loadable for migration by saving Settings. The config file is written via a private temp file and atomic rename.

## UI foundations

- Native Win32 settings window is resizable, and child layout reflows horizontally in 96-DPI logical units.
- Independent service status lines, grouped controls, stronger headings, clearer primary/secondary actions, a compact tray error entry.
- The recording HUD remains non-activating and retains the earlier accurate `paste sent` vs `insertion verified` semantics.

## Technical boundaries and follow-up

- The desktop core is native Go/Win32 without Electron. No managed model process is introduced in v20; OneClick stays independent.
- Unit tests, Windows vet and cross-compilation prove source integrity, not real display quality, cross-app success or runtime compatibility.
- Follow-up work: desktop accessibility audit, complete clipboard IDataObject snapshot and restore, signed installer/automatic upgrade, theme switching, keyboard-only alternative to mouse side buttons, end-to-end microphone and GPU testing.
- Avoid exposing real API keys, transcripts or clipboard payloads in issue reports and screenshots.
