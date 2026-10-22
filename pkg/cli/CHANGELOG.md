# Changelog

## 0.1.0

### Fixes

- Rename `sweet list` to `sweet status` to show package paths and current versions.

### Features

- Add `sweet bump` to prepare releases for one or more package paths, all packages with `--all`, or pending packages selected in a fuzzy picker. The picker shows next versions and supports Space to toggle multiple packages. The command updates versions and changelogs and deletes consumed release notes.

- Add `sweet tags plan` and a GitHub workflow that creates annotated package tags after CI passes on main. Tag pushes are atomic and can be retried without moving existing release tags.

- Add `sweet release-pr plan` to produce a read-only JSON plan for release automation. The GitHub workflow creates or updates combined or per-package PRs and cleans up obsolete bot-created release PRs after successful preparation.

- Report the CLI package version in local builds and keep `version.go` synchronized with the package manifest through `sweet bump`. Release builds can still override the displayed version at link time.
