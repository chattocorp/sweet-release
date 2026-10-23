---
title: Configuration
description: Configure change types, packages, and version file updates.
---

## Repository configuration

`sweet init repo` creates `.sweet.config.yml` at the Git root:

```yaml
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
types above. Invalid type definitions are errors. Pull request policy belongs to
your automation, not Sweet's configuration. The former `release-pr` setting is
no longer used and can be removed from existing configuration files.

### Changelog source repository

By default, Sweet infers changelog source links from the GitHub `origin` remote
(HTTPS, SSH URL, or `git@github.com:owner/repo.git` syntax). Set `repository` in
`.sweet.config.yml` to override the inferred URL:

```yaml
repository: https://github.com/owner/repo
```

The value must be an HTTPS repository URL without credentials, query parameters,
or a fragment. A trailing `.git` is removed. Explicit URLs can name
GitHub-compatible hosts with `/owner/repo`, `/pull/number`, and `/commit/sha`
routes. Other Git hosts are not inferred automatically. This setting only controls
source links; it does not configure release PR automation. See
[links to commits and pull requests](/guides/release-notes/#links-to-commits-and-pull-requests)
for history requirements and PR subject patterns.

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
display name for changelogs and releases. Paths still identify packages.
Without a name, the repository-relative path is used for display.

### Release tags

Tags default to `<directory-name>@<version>`: `apps/api` at version `1.2.3`
produces `api@1.2.3`. A root package defaults to `v1.2.3`.
The optional `tag-prefix` replaces all text before the version:

```yaml
version: 1.2.3
tag-prefix: service-api@
```

This produces `service-api@1.2.3`. An empty string produces just `1.2.3`.
Prefixes must produce valid Git tag names. The display `name` does not affect tags.
To keep the previous path-based format, set `tag-prefix: apps/api/v`.
Existing tags are never renamed or removed; changing the prefix can create a new
tag for an already released version when you run `sweet git tag`.

Each package must have a unique tag prefix, even if its current version differs
from other packages. Run `sweet doctor` to report configuration problems and
duplicate prefixes as warnings. Other repository commands fail on duplicate
prefixes before making changes. Set a unique `tag-prefix` in each conflicting
package's `.sweet.yml` to resolve the warning.

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
