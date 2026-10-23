---
type: feature
breaking: true
---

Replace `sweet tags plan` with `sweet git tag`. Create missing annotated local
tags for current committed releases at their version-introduction commits. Use
`--dry-run` to preview, `--json` for automation, and `--base` with `--head` to
select an explicit commit range. Tags are never pushed or overwritten.
