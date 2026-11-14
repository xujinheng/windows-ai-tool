# Windows release trust model

VoiceRewrite intentionally does **not** download or execute updates without the user opening the official GitHub Releases page. Official source builds are currently unsigned Windows executables. This is an engineering limitation, not a reason to suppress security warnings.

## Version-tag build provenance

The `build.yml` workflow generates signed GitHub artifact provenance attestations (`actions/attest`) for the portable ZIP, the Inno Setup installer and Qwen3-ASR OneClick when a `v*` tag is pushed. Run `gh attestation verify PATH --repo xujinheng/windows-ai-tool` with a current GitHub CLI to check a downloaded release artifact. Also verify its published SHA-256 checksum independently.

**Build provenance does not certify that an application is malware-free** and is not an Authenticode publisher signature.

## Requirements before stable public distribution

- A trusted code-signing identity and a documented protected signing process for installer and executable.
- A user-approved open-source license (the repository does not yet have one).
- Real Windows 11 usability, screen-reader, input, clipboard and installer upgrade/uninstall verification on clean machines.
- A documented rollback and vulnerability response process.

Until those requirements are complete, distribution should remain clearly labeled unsigned prerelease/development builds.
