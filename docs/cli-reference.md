# Detailed CLI behavior

For setup and basic usage, start with the [user guides](README.md#use-sweet-release).
For tag behavior, see [Version tags](version-tags.md).

## Package configuration

Run `sweet init repo` to create repository configuration at the Git root.
Run `sweet init package [dir]` to create a package manifest in an existing
directory (the current directory by default). Package initialization also creates
the `.sweet/` directory for release notes. Both commands preserve existing files.

New package manifests start with:

```yaml
version: 0.0.0
```

Add an optional display name, for example `name: Sweet Release`. It must be a
non-empty single-line string without surrounding whitespace. Names need not be
unique and never affect package selection, branch names, or version tags. When
omitted, the repository-relative path is the display name (`.` for a root package).
`sweet init package` leaves the name unset.

Display names appear in per-package PR titles, package headings in release PRs,
and new changelog titles (`# <display name> Changelog`). Existing changelog
introductions are preserved. CLI publishing reads the name from the tagged
manifest for the GitHub Release title; already published releases remain unchanged.
The workflow builds a current metadata reader before checking out older source
tags, so it can read manifests even when the tagged CLI predates this feature.

`sweet status --json` exposes the inventory for automation, including `path`,
optional `name`, `version`, `changes`, `bump`, `nextVersion`, and `problems`.
The default status table continues to show paths.

`sweet status --rev <tree-ish>` reads the plan from a Git commit, tag, or tree
object (resolved with `git rev-parse <tree-ish>^{tree}`) instead of the working
directory. Files come from `git ls-tree -r -l --full-tree` and one
`git cat-file --batch` process that reads the config, all manifests, and all
notes up front. No filters, hooks, index, work tree, or lazy fetches
(`GIT_NO_LAZY_FETCH=1`) are involved. Symlinks resolve within the tree one path
component at a time (at most 40 hops); links that leave the tree count as
missing files. Files over 1 MiB are file problems.
Output equals `status` in a checkout of that tree without Git filters (eol,
LFS, ident) and without symlinks that leave the tree. It works with and without
`--json`, from any directory, in bare repositories, and with `GIT_DIR` or
`GIT_WORK_TREE` set. Git failures (unresolvable revision, missing objects, no
repository) fail the command with Git's error output instead of becoming plan
problems.

`sweet status` shows a repository-wide inventory from any directory:

```text
PACKAGE   VERSION  BUMP   NEXT
pkg/api   1.2.3    minor  1.3.0
pkg/core  0.2.0
```

`BUMP` is the highest required bump from pending `.sweet/*.md` notes:
`major`, `minor`, `patch`, or `none`. Packages with no pending notes have blank
`BUMP` and `NEXT` cells.
`NEXT` applies that bump once to the manifest version. With no bump, the version
stays unchanged. A bump increments the selected version component, resets lower
components to zero, and removes build metadata. Manifest versions must be complete
semantic versions such as `1.2.3` or `2.0.0-beta.1`.

A prerelease version already covers bumps up to its own level: `1.3.0-beta.1`
covers minor and patch changes, `2.0.0-beta.1` covers all of them. Without a
manifest `prerelease` label, a prerelease version is finalized (`1.3.0-beta.1` →
`1.3.0`), even without pending notes; a larger bump moves the base instead.
With `prerelease: <label>`, Sweet produces prereleases:

| Current | Bump | Label | Next |
|---|---|---|---|
| `1.2.3` | minor | `beta` | `1.3.0-beta.0` |
| `1.3.0-beta.0` | patch or minor | `beta` | `1.3.0-beta.1` |
| `1.3.0-beta.1` | major | `beta` | `2.0.0-beta.0` |
| `1.3.0-beta.1` | any or none | `rc` | `1.3.0-rc.0` |
| `1.3.0-rc.0` | patch or minor | `beta` | problem: would not advance |

A stable version or current-channel prerelease without bump-relevant notes stays
unchanged. Switching to a label with lower precedence reports a manifest problem.

Notes with `breaking: true` require at least a minor bump for `0.x` versions and
a major bump for other versions. An explicit major type policy still takes
precedence for `0.x` packages. The manifest version is the starting point even
if the package has no previous release.

The inventory includes nested packages. It reads tracked and untracked files,
respects Git ignore rules for untracked files, and skips locally deleted files.
Only direct Markdown files in each package's `.sweet/` directory are notes.
Malformed notes, unknown types or properties, and invalid package manifests
appear in a `Problems` list with file paths. Affected packages show `invalid`
in the `BUMP` column and no next version; other packages still show their status.
The command succeeds when it can complete the inventory, even if it finds file
problems. Repository discovery and configuration failures still stop the command.
Package names are not required. Dependency propagation is not yet implemented.

`sweet init repo` starts `.sweet.config.yml` with:

```yaml
types:
  fix:
    bump: patch
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

Edit this mapping to configure the available change types, in picker order, for all
packages in the repository. Names must start with a lowercase letter and contain
only lowercase letters, digits, and hyphens. The mapping must be non-empty with unique names.
Missing configuration or an omitted `types` field uses the built-in defaults.
The old list format is no longer accepted.

Each type supports two settings:

- `bump`: `major`, `minor`, `patch`, or `none`. Omitting it means `none`.
- `section`: the changelog heading. An empty, null, or omitted section excludes
  this type from the changelog. It does not change the type's version bump.

For example, a type can trigger a patch without appearing in a changelog, or
appear under a heading without triggering a bump. Unknown settings and invalid
values produce configuration errors. `sweet status` uses these policies to
calculate versions. `sweet bump` uses the sections to generate changelog entries.

## Prepare a release

```sh
sweet bump                    # Choose pending releases interactively
sweet bump pkg/api pkg/web   # Prepare multiple packages from the repository root
sweet bump .                  # Prepare the package in the current directory
sweet bump --all              # Prepare all eligible packages from any directory
```

With no package arguments or `--all`, Sweet opens a searchable package picker.
Only valid packages with a pending version bump appear. Each row shows the
current version, next version, and bump. Type to fuzzy-filter, use the arrow
keys to move, and press Space to toggle one or more packages. Selections remain
selected when the filter changes. Press Enter to release the selected packages,
or the highlighted package if none are selected. Esc or Ctrl+C cancels without
changing files. Without a terminal, pass package paths or `--all`.
Package paths are relative to the current directory. `--all` cannot be combined
with package arguments.

For each selected package that needs a version bump, the command updates
`.sweet.yml`, creates or updates the package's `CHANGELOG.md`, and deletes the
consumed `.sweet/*.md` notes. It uses the same version calculation as `sweet status`.
The command changes local files; it does not commit, tag, publish, or create a PR.
Other version files are updated only when listed in the package's `version-files`
configuration, described below.

New changelogs start with `# <display name> Changelog`. Each release has a version heading,
with sections in repository configuration order. Types with the same section
share a heading. Notes appear in filename order within each section. Markdown
bodies appear as bullet items, with continuation lines indented to retain their
Markdown formatting and surrounding whitespace removed; breaking
notes receive a bold label. Types without a section are omitted from the
changelog but are still consumed. Existing introductions and release history
are retained, with the new release inserted before the first level-two heading.

When a release finalizes a prerelease, its changelog entry also merges the items
of all prerelease entries since the last stable version, newest first and breaking
changes first within each section. The prerelease entries remain in the history.
Entries are merged by their `###` section headings and top-level `- ` items.

Notes with `bump: none` remain pending until the package needs a release. That
release consumes all its pending notes, including those with no version effect.
Running the command again without new notes makes no changes.

The command validates all selected packages and prepares their file changes
before writing. Invalid notes or manifests in the selection stop the operation.
An unrelated invalid package does not block an explicitly selected package.
If a file operation fails, the command attempts to restore the affected files
and reports any restoration errors. This does not protect against process
interruption; review the Git diff before committing a release.

## Update project version files

Add `version-files` to a package's `.sweet.yml` to copy its next version into
project files during `sweet bump`. Sweet does not detect project languages or
run package managers. For example:

```yaml
version: 1.2.3
version-files:
  - path: package.json
    format: json
    key: version
  - path: chart.yaml
    format: yaml
    key: appVersion
  - path: pyproject.toml
    format: toml
    key: project.version
  - path: VERSION
    format: text
  - path: version.go
    format: annotated
```

Every entry requires `path` and `format`. Paths are relative to the package
directory; `../` can address shared files inside the repository. Paths cannot
leave the repository or pass through symlinks. Git files, Sweet manifests,
repository configuration, changelogs, and `.sweet/` notes cannot be targets.
Omit `version-files` or use an empty list when no other files need updates.

For `json`, `yaml`, and `toml`, `key` selects an existing string field through
mapping/table keys. Dots separate nested keys: `project.version` addresses
`version` inside `project`. Use a list for literal keys that contain dots:

```yaml
version-files:
  - path: metadata.json
    format: json
    key: ["my.project", version]
```

Selectors do not support arrays, wildcards, JSONPath, or XPath. TOML targets
can use tables, dotted keys, or inline tables. YAML targets must be explicit
plain or quoted strings; block scalars, tags, anchors, aliases, and inherited
merge fields are not supported as version targets.

With `format: text`, the entire file must contain a semantic version, with
optional surrounding whitespace. With `format: annotated`, place `sweet-version`
on each line that Sweet should update, usually in a comment:

```go
const Version = "1.2.3" // sweet-version
```

Each marked line must contain exactly one complete semantic version. All marked
lines are updated; unmarked lines are retained. An optional `v` prefix is retained
in annotated files. The annotation format treats the file as text and does not
parse the source language or comment syntax. Neither plaintext format accepts
`key`.

Version files must use UTF-8. Sweet replaces only the selected version token,
preserving surrounding content, comments, whitespace, and line endings. Existing
values must be complete semantic versions. They need not match `.sweet.yml`:
the package manifest is authoritative, and all targets receive its next version.
Missing files, missing keys, invalid values, or ambiguous annotations stop the
bump before it writes any release files or deletes notes.

Multiple entries and packages can update different fields in the same file.
Identical updates to one token are combined; conflicting updates fail before
writing. Version-file updates participate in the same validation and attempted
rollback as the rest of the release. Packages with no pending release do not
update their version files.

If the project needs lockfile regeneration, run its own command after a successful
bump, such as `pnpm install` at the repository root, and include those changes in
the release commit or PR. Sweet does not run post-bump scripts.

## Write release notes

Run `sweet change` to write a release note, with optional `--type` and `--stdin`
flags. Run `sweet` or `sweet --help` to show the available commands.

Run `sweet change [package]` with a package directory, or omit the argument to use
the current directory. The package must already contain a valid `.sweet.yml`.
If the current directory is not a package, Sweet opens a built-in Charm package picker
when standard error is a terminal. Type to fuzzy-filter, use the arrow keys, press
Enter to select, or Esc to cancel without creating a note. Without an interactive
picker, Sweet lists commands for the available packages. An explicit package
argument bypasses the picker. The picker also works with `--stdin`; it leaves
the piped Markdown untouched.
Next, Sweet opens the same searchable picker with the repository's configured types.
Pass `--type <type>` to skip this picker; this flag is required without an
interactive terminal. Cancelling either picker creates no file. No external
picker program is required.

The command creates `.sweet/<three-word-id>.md` with YAML frontmatter such as:

```markdown
---
type: fix
breaking: false
---

```

After type selection, a Charm prompt asks for a one-line description. Submit a
description to save it as the Markdown body, or leave it empty to open `$EDITOR`
with the generated frontmatter. Cancelling the prompt creates no file. An editor
is only required for an empty description. Editor commands can include arguments,
such as `code --wait`; they run through `sh`. Sweet waits for the editor to exit
and keeps the file if the editor fails.

```sh
sweet change apps/sweet
printf 'Add package discovery.\n' | sweet change apps/sweet --type feature --stdin
```

`--stdin` writes input verbatim as the Markdown body below the generated frontmatter
and skips the description prompt and editor. Supply body text, not a complete frontmatter document.
Existing notes are never overwritten. The command creates `.sweet/` if the
package does not yet have it. Notes require a `type`; the optional `breaking`
field defaults to false. Other frontmatter fields are rejected. The Markdown
body is preserved when notes are read for the status inventory.

## Publish GitHub releases

`sweet github release <tag> [assets...]` publishes a GitHub Release for an
existing Sweet tag through the `gh` CLI. `gh` reads `GH_TOKEN` and finds the
repository from `GH_REPO` or the Git remote. The tag selects the package whose
tag prefix it starts with, followed by a semantic version; the version need not
match the current manifest. Ambiguous or unknown tags fail before any request.

1. `gh release view` finds the release state. A missing release is created with
   `gh release create --verify-tag --draft --latest=false`, the title, and notes
   from `CHANGELOG.md` for that version (as `sweet changelog --version`). A
   missing changelog entry fails before creation. A draft is reused, so a rerun
   never creates a second release, and gets the same title and notes as a new
   release through `gh release edit --title --notes-file`. Drafts are not public,
   so a retried draft always carries the current changelog notes. A draft rerun
   therefore also needs a changelog entry for the version; if the draft was
   created with `--notes-file`, pass it again. A published
   release is left unchanged and the command stops here, unless
   `--replace-published` is given.
2. `gh release upload --clobber` uploads any assets, replacing same-name files.
3. Without `--draft`, `gh release edit --draft=false --prerelease=<p> --latest=<l>`
   publishes. With `--replace-published`, this also runs for an already
   published release and reapplies the computed state.

Published releases are skipped by default because one `gh release edit` sets
draft, prerelease, and Latest together: a published release already has its
state, and its assets may be live downloads. Even with `--replace-published`,
the title and notes of a published release are never replaced: they are public
and may have been edited by hand. Change them on GitHub or with `gh release edit`.

`prerelease` is true when the version has a prerelease suffix. Without `--latest`,
Latest is true only for a stable version of the main package (the root package,
or the only package) that no stable remote tag with the same prefix exceeds.
Sweet lists remote tags through the GitHub API, so the checkout needs no tags.
GitHub's `--latest=false` never demotes the current Latest release.
`--latest=true` for a prerelease is an error.

Flags: `--draft` (create and upload only), `--title` (default: `<name> <version>`
with a manifest name, otherwise the tag), `--notes-file` (notes for a new
or draft release instead of the changelog), `--latest[=false]`, and `--replace-published`.
Concurrent runs for the same missing release can create duplicate drafts;
serialize them per tag. `gh` deletes each same-name asset before uploading its
replacement, so with `--replace-published` a failed upload leaves that asset
missing from the live release until the next successful rerun.
