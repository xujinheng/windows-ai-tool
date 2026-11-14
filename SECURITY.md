# Security

Report vulnerabilities through GitHub private vulnerability reporting when available, or contact the maintainer privately. Do not include sensitive recordings, selected text, clipboard data, API keys or local logs in public issues.

- ASR and LLM requests go to the configured endpoints. HTTP redirects are rejected.
- API keys are encrypted with Windows DPAPI in the current user's context.
- Text replacement checks the foreground target before applying output.
- Clipboard restoration depends on the formats and ownership available through Windows APIs.
- Native password controls and supported UI Automation password fields are excluded from automatic input. Detection across third-party applications is not universal.

The Windows installer is unsigned. Verify downloaded checksums and [build provenance](RELEASE-TRUST.md) where available. See [Privacy](PRIVACY.md).
