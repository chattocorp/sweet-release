# Changelog

## 0.2.0

### Features

- Support optional package display names for changelogs and release PR titles. Paths remain package identities; duplicate names are allowed.

## 0.1.0

### Features

- Plan path-based version tags from committed package version changes. Preserve each release commit in multi-commit pushes, skip unchanged and initial `0.0.0` versions, and reject conflicting or reused tags.

- Configure combined or per-package release PRs with `release-pr.strategy`. New repositories select per-package PRs; existing configurations retain combined behavior. Plan stable release branches and validate shared version targets before preparing PRs.

- Update explicit version targets in JSON, YAML, TOML, and plaintext files during release preparation. Preserve surrounding content, combine updates to shared files, and validate all targets before writing release files or consuming notes.

- Prepare package releases from pending notes, with changelog sections, version updates, and consumed-note deletion. Validate selected packages before writing and attempt to restore affected files when a file operation fails.
