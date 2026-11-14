# Repository instructions for coding agents

This repository represents the **current product**, not the history of how it was developed.

## Final-state only

- Implement the requested end state directly. Do not leave transitional implementations, parallel versions, temporary adapters, compatibility shims, deprecated paths, fallback code for previous implementations, or scaffolding for abandoned approaches.
- Do not preserve behavior, interfaces, configuration formats, names, or workflows merely for backward compatibility with earlier versions of this repository. There is no existing user base to protect. When a design changes, update all affected call sites and remove the superseded implementation.
- Keep one authoritative implementation per feature. Remove unused code, obsolete tests, dead assets, outdated scripts, and duplicate workflows. Update current tests to describe the intended behavior.
- Git history is the only place for old implementations, migration stages, experiments, and intermediate decisions. The latest `main` branch must not contain a narrative or residue of prior versions.

## Documentation and comments

- Write documentation only for the product as it exists now: what it does, how to use it, how to build it, and operational constraints that matter.
- Do not create versioned notes, development diaries, progress reports, retrospective comparisons, change summaries, or files such as `README-V*.md`, `EXPERIENCE-V*.md`, `ARCHITECTURE-V*.md`, or `RESTORE_NOTES.md`.
- Do not explain that a feature was “added,” “fixed,” “improved,” or “no longer” behaves like an earlier version. Describe the current behavior directly.
- Do not include conversational artifacts: user instructions, rejected alternatives, self-congratulation, claims about how much work an agent completed, or explanations of why an old approach was discarded.
- Code comments should explain a non-obvious invariant, constraint, or safety property of the **current** implementation. Do not annotate code with iteration numbers, prototype history, or old-versus-new comparisons.
- Avoid marketing claims and defensive statements about technologies or techniques not used. State only facts that help a reader understand or operate the current system.

## Making changes

1. Inspect the current implementation and identify the desired final design.
2. Modify the existing authoritative code and documentation in place.
3. Delete superseded code and documents in the same change; do not keep both implementations.
4. Update tests, build scripts, UI assets, and README references to match the final state.
5. Run relevant checks and report their results in the pull request or conversation, **not** as a progress narrative committed to the repository.

Do not compromise current correctness, security, or required external protocol behavior while removing historical baggage. Such constraints must be justified by present functionality, never by compatibility with a previous internal version.
