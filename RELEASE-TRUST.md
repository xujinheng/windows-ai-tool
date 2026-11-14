# Windows downloads

CI builds a portable ZIP and an Inno Setup installer. Tagged preview releases include SHA-256 checksums and GitHub artifact provenance attestations.

Verify a downloaded artifact with the published checksum. For tag builds with attestations, use:

```bash
gh attestation verify PATH --repo xujinheng/windows-voice-rewrite
```

The executable and installer currently lack Authenticode publisher signatures, so Windows SmartScreen may warn.

The repository does not currently have a project-wide license. Releases are provided as unsigned preview builds.
