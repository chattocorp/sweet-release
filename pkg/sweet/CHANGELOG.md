# Sweet Release Changelog

## 0.2.0

### Fixes

- Validate semantic versions consistently when reading package manifests, including before creating release notes. Use one repository file inventory and one loaded configuration for release preparation.

### Features

- Use package display names in release headings and GitHub Release titles.

- Publish CLI binary archives and checksums after version tagging, with release notes from the changelog and retry support for incomplete uploads.

- Add `sweet status --json` for release automation.

- Add `sweet release-pr plan --details` to report planned release content and PR descriptions, and `sweet release-notes <changelog> --version <version>` to read a released version's notes. Release workflows use these commands to avoid reconstructing notes from Git diffs.

- **Breaking change.**

  Consolidate the CLI and core into one Sweet Release package at `pkg/sweet`,
  starting from the CLI version 0.1.0 and changelog. The Go module is now
  `github.com/chattocorp/sweet-release/pkg/sweet`; core imports use its `/core`
  subpackage. New releases use `pkg/sweet/v<version>` tags. Existing `pkg/cli/v`
  releases remain available and can still be published. The retired core changelog
  is retained as historical documentation.

## 0.1.0

### Fixes

- Rename `sweet list` to `sweet status` to show package paths and current versions.

### Features

- Add `sweet bump` to prepare releases for one or more package paths, all packages with `--all`, or pending packages selected in a fuzzy picker. The picker shows next versions and supports Space to toggle multiple packages. The command updates versions and changelogs and deletes consumed release notes.

- Add `sweet tags plan` and a GitHub workflow that creates annotated package tags after CI passes on main. Tag pushes are atomic and can be retried without moving existing release tags.

- Add `sweet release-pr plan` to produce a read-only JSON plan for release automation. The GitHub workflow creates or updates combined or per-package PRs and cleans up obsolete bot-created release PRs after successful preparation.

- Report the CLI package version in local builds and keep `version.go` synchronized with the package manifest through `sweet bump`. Release builds can still override the displayed version at link time.
