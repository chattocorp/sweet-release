---
type: refactor
breaking: true
---

Rename `sweet release-notes` to `sweet changelog extract <file> --version <version>`.
The command retains the same output and validation. Update scripts to use the
new command; the old name is no longer available.
