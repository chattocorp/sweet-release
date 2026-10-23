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
name: My API
version: 1.2.3
version-files:
  - path: package.json
    format: json
    key: version
```

`version` must be a complete semantic version. The optional `name` must be a
non-empty single-line string without surrounding whitespace. It supplies a
display name for changelogs and releases. Paths still identify packages and
determine tag names. Without a name, the repository-relative path is used.

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
