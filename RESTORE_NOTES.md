# Source recovery and version history

Recovered from release ZIP archives provided by the repository owner:

- v16: managed Qwen3-ASR llama-server support, preserved in the v16 Git commit/branch.
- v17: optional managed ASR + LLM services, preserved in the v17 Git commit/branch.
- v19: deliberately **client-only**, connects to independently running ASR and LLM APIs.

The v19 main tree **does not include v17's managed-service Go source**, by design.
Earlier version history and documentation remain accessible in Git, with README-V10…V17 notes also preserved.
`v18` source was not present in the supplied archives and was not invented.

The local PowerShell launcher is separate from the v19 client to keep v19 client-only.
Release binaries, GGUF weights, credentials, and proprietary runtimes were deliberately not pushed.

Windows GUI and multi-app interaction need real-device acceptance testing.
No project-wide license is assumed by this recovery.
