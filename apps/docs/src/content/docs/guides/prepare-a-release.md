---
title: Prepare a release
description: Inspect pending changes and update selected package versions.
---

## Inspect pending releases

```sh
sweet status
sweet status --json
```

Status shows all packages from any directory in the repository. The highest
required bump among a package's pending notes determines its next version.
Invalid manifests or notes appear in the problems list.

The table shows package names, falling back to repository-relative paths when
names are omitted. Display labels do not replace names in the table.
JSON includes each package's effective `name`
(the explicit name or path default) and its optional `displayName` label, along
with version, pending changes, expected bump, and problems. Status reports file
problems without failing; use doctor when automation needs a failing exit code.

## Check repository consistency

```sh
sweet doctor
sweet doctor --json
```

Doctor reports the same manifest and note problems as status, plus duplicate
package names. Omitted names default to repository-relative paths; doctor also
finds conflicts between these defaults and explicit names. Display labels need
not be unique. Each duplicate report lists the conflicting manifests.

The command works from any directory in the repository and changes no files.
It exits with code 0 when no problems are found, or code 2 for problems or a
repository/configuration error. `--json` prints an array of `path` and `message`
objects, or `[]` when there are no problems. Repository/configuration errors are
reported on standard error. Version-file target contents are checked during bump
planning, not by doctor.

## Choose packages

```sh
sweet bump                   # Open a searchable package picker
sweet bump pkg/api pkg/web   # Select packages by directory
sweet bump .                 # Select the current package
sweet bump --all             # Select all eligible packages
sweet bump --package api     # Select by package name from any directory
```

Paths are relative to the current directory. In the picker, use Space to select
packages and Enter to prepare them. With no selection, Enter prepares the
highlighted package. Without a terminal, supply paths, `--package`, or `--all`.

Name selectors use the [package name](../../reference/configuration/#package-manifest)
from any repository directory. Select multiple names by repeating the flag:

```sh
sweet bump --package api --package web
```

When a name is omitted in the manifest, its repository-relative path is the name,
for example `sweet bump --package pkg/api`. Unknown or ambiguous names fail before
writing. Do not combine `--package` with positional paths or `--all`.

For each selected package with a pending bump, Sweet updates `.sweet.yml`,
configured version files, and `CHANGELOG.md`. It then deletes all consumed notes,
including notes with no version effect. Existing changelog history is preserved.

Sweet validates selected packages and prepares all edits before writing. An
unrelated invalid package does not block an explicit selection. If a file
operation fails, Sweet attempts to restore affected files. Review the Git diff
before committing.

## Update lockfiles and publish

After a bump, run any package-specific commands needed to update lockfiles.
Sweet does not run package managers or post-bump scripts. Commit the generated
changes, then tag and publish through your project's release process.

Running `sweet bump` again without new notes makes no changes. Dependency
propagation between packages is not implemented.

## Plan release pull requests

```sh
sweet release-pr plan --details
```

This read-only command returns JSON with stable branch names, titles, selected
packages, and rendered release descriptions. It does not contact GitHub or create
pull requests. Set `release-pr.strategy` to `per-package` for separate PRs or
`combined` for a single release PR.

With `--details`, each release includes its effective `name`, resolved
`displayName`, package `path`, target `version`, and new `changelog` entry.
PR headings use the display label; branch names continue to use package paths.

For a working automation example, see this repository's
[Release PR workflow](https://github.com/chattocorp/sweet-release/blob/main/.github/workflows/release-pr.yml).
It regenerates release branches from `main`, applies `sweet bump`, verifies the
result, and creates or updates PRs. Make corrections through `main`, since the
next run can overwrite edits on generated branches.
