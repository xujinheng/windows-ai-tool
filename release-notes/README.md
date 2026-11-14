# Release notes

Each public version must have a hand-written `release-notes/vX.Y.Z.md` file before tagging and publishing. The release workflow reads that version's file from `main` and **fails** when it is absent or empty.

Write for end users: what the program does, what changed in their experience, what to download, and any important requirements. Do **not** paste automatically generated pull request lists, commit history, build/rebuild notes, CI steps, internal hashes, or contributor metadata into the public description.

The release job uses `body_path` with `generate_release_notes: false`. Updating a version's release notes requires changing its version-specific Markdown file and intentionally running the release workflow.
