---
type: feature
breaking: true
---

Release tags now default to `<directory-name>@<version>`. Root packages keep
`v<version>`. Set `tag-prefix` in a package's `.sweet.yml` to override the literal
text before its version, or use the old path followed by `/v` to keep existing
naming. `sweet doctor` warns about duplicate prefixes and tag names; other
repository commands reject these collisions before making changes.
