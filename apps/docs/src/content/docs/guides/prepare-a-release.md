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

## Choose packages

```sh
sweet bump                   # Open a searchable package picker
sweet bump pkg/api pkg/web   # Select packages by directory
sweet bump .                 # Select the current package
sweet bump --all             # Select all eligible packages
```

Paths are relative to the current directory. In the picker, use Space to select
packages and Enter to prepare them. With no selection, Enter prepares the
highlighted package. Without a terminal, supply paths or `--all`.

For each selected package with a pending bump, Sweet updates `.sweet.yml`,
configured version files, and `CHANGELOG.md`. It then deletes all consumed notes,
including notes with no version effect. Existing changelog history is preserved.

Changelog items have no blank lines between them. Paragraphs and other Markdown
formatting within each note are preserved.

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

## Tag committed releases

```sh
sweet git tag --dry-run
sweet git tag
```

The command creates annotated local tags for the current committed package
versions. Tags use `<package-directory>/v<version>`, or `v<version>` for a root
package. Each tag targets the commit that introduced the version on first-parent
history, even if other commits followed it. Uncommitted changes, older versions,
and `0.0.0` versions are excluded.

Matching existing tags are retained. A tag at a different commit is an error;
Sweet never moves or overwrites tags. Use a full-history checkout and fetch remote
tags first if they must be included in conflict checks. Sweet does not fetch,
push, or publish anything. If Git fails after creating some tags, rerun the
command to create the remaining tags.

For automation, add `--json` to print the plan. The `exists` field reports whether
each tag existed before creation. To select version changes within a commit range,
pass both `--base <commit>` and `--head <commit>`. Combine these with `--dry-run`
to inspect the range without creating tags.

## Read released changes

```sh
sweet changelog apps/sweet
sweet changelog apps/sweet --version 0.2.0
sweet changelog
```

Package paths are relative to the current directory. With no argument, Sweet uses
the current package or offers a package picker in a terminal. In scripts, supply
a package path if the current directory is not a package.

Sweet reads the package's `CHANGELOG.md`. By default it shows the first released
version in changelog order, skipping `Unreleased` and fenced examples. This is
independent of pending notes and the version in `.sweet.yml`. Use `--version` to
select a specific entry. Output contains the release body without its version
heading. A missing package, changelog, or release produces an error.

## Automate release pull requests

Use `sweet status --json` to list packages. For each package, start from a clean
checkout of the same base commit, run `sweet bump <package>`, and inspect the Git
working tree, including new files. Create or update a PR only when files change.
For a combined release, run `sweet bump --all` in one clean checkout instead.

Your automation owns PR branches, titles, descriptions, and cleanup. Sweet has
no `release-pr` command or PR strategy setting. Extract descriptions from the
updated changelogs with `sweet changelog <package> --version <version>`.

For a working automation example, see this repository's
[Release PR workflow](https://github.com/chattocorp/sweet-release/blob/main/.github/workflows/release-pr.yml).
It uses `RELEASE_PR_STRATEGY: per-package` in the workflow; set it to `combined`
for one PR. Each package's PR description heading contains its display name (or
path) and new version, such as `Sweet Release 0.2.0`, followed by its release notes.
Use combined mode for packages that share a version target, since
separate PRs do not coordinate shared writes. The workflow verifies changed
checkouts and cleans up obsolete PRs only after all jobs succeed. Make corrections
through `main`, since the next run can overwrite edits on generated branches.
