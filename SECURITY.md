# Security policy

This is a source-available Windows desktop utility undergoing hardening, not a production-certified security product.

Do not put real API Keys, audio recordings, text selections, clipboard data, user-specific paths, or private logs into public GitHub issues. To report sensitive vulnerabilities, use GitHub's private vulnerability reporting feature **if it is enabled** for the repository; otherwise contact the maintainer through a private channel listed on the maintainer's profile before sharing sensitive details.

Security boundaries:

- Only user-configured ASR/LLM endpoints receive task content. HTTP redirects are denied. Remote API operators are outside this application's trust boundary.
- API Keys use Windows DPAPI at rest, bound to the signed-in user's protection context. Config and temporary files should never be committed.
- Results are inserted only after rechecking foreground-window identity; editor acceptance cannot always be verified.
- Clipboard HGLOBAL snapshots and ownership fences are best effort for supported formats; GDI/OLE/owner-rendered formats cannot be guaranteed.
- Native EDIT/RichEdit password controls are excluded; browser/Electron password detection is incomplete.
- Unsigned installers are development previews; release signing, source audits and real-device testing remain outstanding.

See `voice-rewrite/EXPERIENCE-V24.md` for current technical limitations.
