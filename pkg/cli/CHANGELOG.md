# Changelog

## 0.1.0

### Fixes

- Rename `sweet list` to `sweet status` to show package paths and current versions.

### Features

- Add `sweet bump` to prepare releases for one or more package paths, all packages with `--all`, or pending packages selected in a fuzzy picker. The picker shows next versions and supports Space to toggle multiple packages. The command updates versions and changelogs and deletes consumed release notes.

- Report the CLI package version in local builds and keep `version.go` synchronized with the package manifest through `sweet bump`. Release builds can still override the displayed version at link time.
