---
type: feature
breaking: true
---

Replace `sweet release-notes <file>` with `sweet changelog [package]`.
Select a package directory instead of a changelog file. With no package argument,
use the current package or choose one interactively. Show the latest changelog
release by default, or select one with `--version <version>`. The output remains
the release body without its version heading. The old command is removed.
