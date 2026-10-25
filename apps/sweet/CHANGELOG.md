# Sweet Release Changelog

## 0.4.0

### Fixes

- Keep changelog source attribution inline after the final paragraph of multi-paragraph prose notes. Preserve paragraph breaks and keep attribution separate for structured Markdown such as code blocks, lists, and tables. ([#48](https://github.com/chattocorp/sweet-release/pull/48))

## 0.3.0

### Fixes

- Select the package automatically when `sweet bump` runs in a repository with only one package, without opening the package picker. ([#42](https://github.com/chattocorp/sweet-release/pull/42))

### Features

- ⚠️ **BREAKING CHANGE:** Release tags now default to `<directory-name>@<version>`. Root packages keep
  `v<version>`. Set `tag-prefix` in a package's `.sweet.yml` to override the literal
  text before its version, or use the old path followed by `/v` to keep existing
  naming. `sweet doctor` warns about duplicate prefixes and tag names; other
  repository commands reject these collisions before making changes. ([#40](https://github.com/chattocorp/sweet-release/pull/40))

## 0.2.0

### Fixes

- ⚠️ **BREAKING CHANGE:** Show help when running `sweet` without a command. Use `sweet change` to write
  release notes, including with `--type` and `--stdin`. ([#24](https://github.com/chattocorp/sweet-release/pull/24))
- Validate semantic versions consistently when reading package manifests, including before creating release notes. Use one repository file inventory and one loaded configuration for release preparation. ([#17](https://github.com/chattocorp/sweet-release/pull/17))
- List breaking changes first in each changelog section, preserving the order within breaking and non-breaking notes. ([#31](https://github.com/chattocorp/sweet-release/pull/31))
- Format breaking changes with an inline ⚠️ **BREAKING CHANGE:** label so the description follows the warning on the same line. ([#38](https://github.com/chattocorp/sweet-release/pull/38))
- Keep PR and commit links inline when a wrapped release-note paragraph has a line
  that starts with inline code. ([#36](https://github.com/chattocorp/sweet-release/pull/36))
- Keep commit and PR links inline for release notes whose paragraphs wrap across source lines. ([#35](https://github.com/chattocorp/sweet-release/pull/35))

### Features

- ⚠️ **BREAKING CHANGE:** Replace `sweet release-notes <file>` with `sweet changelog [package]`.
  Select a package directory instead of a changelog file. With no package argument,
  use the current package or choose one interactively. Show the latest changelog
  release by default, or select one with `--version <version>`. The output remains
  the release body without its version heading. The old command is removed. ([#23](https://github.com/chattocorp/sweet-release/pull/23))
- ⚠️ **BREAKING CHANGE:** Replace `sweet tags plan` with `sweet git tag`. Create missing annotated local
  tags for current committed releases at their version-introduction commits. Use
  `--dry-run` to preview, `--json` for automation, and `--base` with `--head` to
  select an explicit commit range. Tags are never pushed or overwritten. ([#21](https://github.com/chattocorp/sweet-release/pull/21))
- ⚠️ **BREAKING CHANGE:** Remove `sweet release-pr plan` and the `release-pr` configuration setting.
  Release PR automation now uses `sweet status --json` and `sweet bump`, with
  PR policy owned by the workflow. Existing `release-pr` settings have no effect
  and can be removed. ([#22](https://github.com/chattocorp/sweet-release/pull/22))
- ⚠️ **BREAKING CHANGE:** Consolidate the CLI and core into one Sweet Release package at `apps/sweet`,
  starting from the CLI version 0.1.0 and changelog. The Go module is now
  `github.com/chattocorp/sweet-release/apps/sweet`; core imports use its `/core`
  subpackage. New releases use `apps/sweet/v<version>` tags. Existing `pkg/sweet/v` and `pkg/cli/v`
  releases remain available and can still be published. The retired core changelog
  is retained as historical documentation. ([#17](https://github.com/chattocorp/sweet-release/pull/17))
- Link changelog items to their introducing PR or commit using local Git history, with an optional repository URL override. ([#33](https://github.com/chattocorp/sweet-release/pull/33))
- Use package display names in release headings and GitHub Release titles. ([#17](https://github.com/chattocorp/sweet-release/pull/17))
- Publish CLI binary archives and checksums after version tagging, with release notes from the changelog and retry support for incomplete uploads. ([#17](https://github.com/chattocorp/sweet-release/pull/17))
- Add `sweet status --json` for release automation. ([#17](https://github.com/chattocorp/sweet-release/pull/17))
- Add `sweet changelog [package]` to read the latest released changes, or use
  `--version <version>` to select a release. Release workflows use the changelog to describe releases without
  reconstructing notes from Git diffs. ([#17](https://github.com/chattocorp/sweet-release/pull/17))

## 0.1.0

### Fixes

- Rename `sweet list` to `sweet status` to show package paths and current versions.

### Features

- Add `sweet bump` to prepare releases for one or more package paths, all packages with `--all`, or pending packages selected in a fuzzy picker. The picker shows next versions and supports Space to toggle multiple packages. The command updates versions and changelogs and deletes consumed release notes.

- Add `sweet tags plan` and a GitHub workflow that creates annotated package tags after CI passes on main. Tag pushes are atomic and can be retried without moving existing release tags.

- Add `sweet release-pr plan` to produce a read-only JSON plan for release automation. The GitHub workflow creates or updates combined or per-package PRs and cleans up obsolete bot-created release PRs after successful preparation.

- Report the CLI package version in local builds and keep `version.go` synchronized with the package manifest through `sweet bump`. Release builds can still override the displayed version at link time.
