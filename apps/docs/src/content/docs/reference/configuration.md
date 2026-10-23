---
title: Configuration
description: Configure change types, packages, and version file updates.
---

## Repository configuration

`sweet init repo` creates `.sweet.config.yml` at the Git root:

```yaml
release-pr:
  strategy: per-package
types:
  fix:
    bump: minor
    section: Fixes
  feature:
    bump: minor
    section: Features
  chore:
    bump: none
  refactor:
    bump: none
  performance:
    bump: patch
    section: Performance
```

Types appear in picker and changelog order. Type names start with a lowercase
letter and contain only lowercase letters, digits, and hyphens.

| Setting | Values | When omitted |
| --- | --- | --- |
| `bump` | `major`, `minor`, `patch`, `none` | `none` |
| `section` | Changelog heading | Exclude from the changelog |

Types with the same section share a heading. An empty or null section also
excludes the type from the changelog without changing its bump.

Missing repository configuration or an omitted `types` field uses the built-in
types above. An omitted release PR strategy uses `combined`; repository
initialization explicitly writes `per-package`. Unknown settings within
`release-pr`, invalid strategies, and invalid type definitions are errors.

## Package manifest

Each package needs `.sweet.yml`:

```yaml
name: api
display-name: My API
version: 1.2.3
version-files:
  - path: package.json
    format: json
    key: version
```

`version` must be a complete semantic version. The optional `name` starts with a
lowercase letter and contains only lowercase letters, digits, and hyphens.
When omitted, it defaults to the repository-relative path (`.` for the root).
Path defaults retain their slashes and spaces. `sweet init package` leaves both
name fields unset.

The optional `display-name` is a non-empty single-line string without surrounding
whitespace. Changelogs and release titles use `display-name`, then `name`, then
the path. Display labels need not be unique. Doctor checks effective package names,
including conflicts between explicit names and path defaults.

Use `sweet change --package api` or `sweet bump --package api` to select by name
from any directory. Repeat `--package` with bump to select several packages.
Name selection rejects ambiguous names, including readable names in manifests
with invalid versions or other invalid fields. The selected manifest must be valid.
Positional arguments remain directory
paths; do not combine them with `--package` or combine `--package` with `--all`.
Tags and release branch names still use package directories.

Previously, `name` held a display label. Move that value to `display-name` and
optionally add a machine-readable `name`. This is a configuration change; labels
are not automatically converted to identifiers.

## Version files

Version targets are optional. Paths are relative to the package; `../` can
address shared files inside the repository. Paths cannot leave the repository
or pass through symlinks. Git files and Sweet's own release files cannot be
targets.

| Format | Target | Example key |
| --- | --- | --- |
| `json` | Existing string field | `version` |
| `yaml` | Explicit plain or quoted string | `appVersion` |
| `toml` | Existing string field | `project.version` |
| `text` | Entire file containing a semantic version | Omit `key` |
| `annotated` | Version on each line marked `sweet-version` | Omit `key` |

Dotted keys select nested mapping or table fields. Use a list for literal keys
that contain dots, such as `key: ["my.project", version]`. Arrays and wildcard
selectors are not supported.

For annotated source files:

```go
const Version = "1.2.3" // sweet-version
```

Each marked line must contain exactly one complete semantic version. All marked
lines are updated, and an optional `v` prefix is preserved.

Targets must be UTF-8 and contain valid semantic versions. Their current values
need not match the manifest. Sweet replaces only the selected tokens, retaining
comments, formatting, and line endings. Missing or ambiguous targets stop the
bump before files are written.
