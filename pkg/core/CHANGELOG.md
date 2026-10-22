# Changelog

## 0.1.0

### Features

- Update explicit version targets in JSON, YAML, TOML, and plaintext files during release preparation. Preserve surrounding content, combine updates to shared files, and validate all targets before writing release files or consuming notes.

- Prepare package releases from pending notes, with changelog sections, version updates, and consumed-note deletion. Validate selected packages before writing and attempt to restore affected files when a file operation fails.
