---
type: feature
breaking: true
---

Use optional machine-readable package names, defaulting to repository-relative
paths. Move existing human-readable `name` values to `display-name`. Display
labels fall back to the name, then the path. Select packages with `--package`
in `sweet change` and `sweet bump`. JSON inventories expose effective names and
optional `displayName` labels; detailed release plans expose both names and labels.
The status table shows effective package names instead of directory paths.
