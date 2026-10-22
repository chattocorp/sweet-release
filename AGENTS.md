# Agent guide

Sweet Release is a Go CLI for releasing packages from Markdown change files.
Keep [README.md](README.md) concise and focused on features and the user workflow.
Keep development instructions and detailed behavior in this file.

## Issues

We use **Means** for issue tracking. Issues live in the Git-tracked `issues/`
collection. Follow the
[sweet-issues skill](.agents/skills/sweet-issues/SKILL.md) when finding, creating,
or updating issues. Read `issues/means.schema.json` before writing and use
`--path issues` from the repository root; Means does not search parent directories.

## Development

The repository uses a Go workspace with two modules:

- `pkg/cli`: the `sweet` CLI, command output, and command integration tests.
- `pkg/core`: package discovery, manifest parsing, and initialization.

Each module has its own `.sweet.yml` and version. The root holds shared
`.sweet.config.yml` configuration and `go.work`; it is not a release package.
The CLI imports the core module. A local replacement in its `go.mod` also lets
module commands resolve core when workspace mode is disabled.

With the Go version from `mise.toml` installed, run these commands at the root:

```sh
go run ./pkg/cli/cmd/sweet status
go test ./pkg/cli/... ./pkg/core/...
go vet ./pkg/cli/... ./pkg/core/...
go build -o bin/sweet ./pkg/cli/cmd/sweet
```

The equivalent Mise tasks are `mise run sweet -- status`, `mise run check`, and
`mise run build`. Use `mise run deploy` to install `sweet` into `~/.local/bin`.
The root has no Go module, so use the module patterns above instead of `go test ./...`.

GitHub Actions runs CI on branch pushes and pull requests. It checks Go formatting,
runs both modules' tests with race detection, runs `go vet`, and builds the CLI
on Linux. You can also start the CI workflow manually from the Actions tab.
To run the additional checks locally, use `gofmt -l pkg` (no output means
formatting is correct) and `go test -race ./pkg/cli/... ./pkg/core/...`.

## Releases

### Release PRs

The `Release PR` workflow runs on pushes to `main` or manual dispatch on `main`.
It checks out current `main`, runs `sweet bump --all`, verifies the changed tree,
and uses `peter-evans/create-pull-request` to create or update one PR from
`sweet/release` into `main`. All packages with pending bumps are included.
The PR contains updated manifests, changelogs, configured version files, and
deletions of consumed notes. Notes with no version effect alone do not create a PR.
The fixed `sweet/release` branch reuses the open PR on each run. Do not add a
`branch-suffix`, which would create separate PR branches. After merging or closing
the PR, a later release can create a new PR.
The PR description lists each package's target version and new changelog content,
without previous release history. The workflow collects added changelog lines
from `git diff` after `sweet bump`. It marks new changelogs with intent-to-add
so first releases also appear in the diff, and writes the description outside
the checkout.

Each run regenerates the changes from `main`; it does not bump the previous
release branch again. Make corrections through `main`, since edits to the
generated branch can be overwritten. When there is no diff, the action creates
no PR and closes an obsolete open PR. It also removes an unused release branch.
After a release PR is merged, the next run either cleans up its branch or
prepares the next pending release. Merging the PR does not tag or publish.

One concurrency group serializes writers without cancelling an active update.
A bump or verification failure stops before the PR action and leaves any
existing PR unchanged. If GitHub fails after the branch is pushed, rerun the
workflow to reconcile the PR. A subsequent run always starts from current main.

The workflow uses `GITHUB_TOKEN` with `contents: write` and `pull-requests: write`.
Enable **Allow GitHub Actions to create and approve pull requests** under
Settings → Actions → General → Workflow permissions (organization policy must
also allow it). According to [GitHub's workflow trigger rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow),
PR workflows created by this token require a maintainer to approve their runs.
Use a GitHub App token in the PR action if unattended PR checks are needed.

This workflow dogfoods the CLI from source. Other projects can use the same
pattern with an installed `sweet` binary. Add project-specific commands, such
as `pnpm install`, between the bump and verification/PR steps to include lockfile
updates. No release-plan command, Git history scan, or publishing task is needed.

### Existing CLI publishing

The release workflow and `.goreleaser.yml` still reference the old `packages/`
directory. These paths must be changed to `pkg/` before snapshot builds or
publishing can succeed. The following describes the intended release workflow.

Install the tools with `mise install`, then run these commands from the root:

```sh
mise run release:check
mise run release:snapshot
```

The snapshot runs tests and static checks, then creates archives and SHA-256
checksums in `dist/` without publishing. It builds `sweet` for Linux, macOS,
and Windows on amd64 and arm64. Windows archives use ZIP; the others use tar.gz.
The binary's `--version` output contains the release or snapshot version.
Local builds default to `cli.Version` in `pkg/cli/version.go`. The CLI package's
`version-files` entry updates its annotated constant during `sweet bump`.
The existing `-X main.version=...` linker option overrides the displayed version
for release and snapshot builds.

To publish a CLI release, update `pkg/cli/.sweet.yml` to the intended
version and commit it with the release changes. Once that commit and the release
workflow are on GitHub, tag the commit and push the tag. For example:

```sh
git tag -a v0.1.0 -m "Release sweet v0.1.0"
git push origin v0.1.0
```

The release workflow uses GoReleaser to run checks, build the archives, and
publish a GitHub Release with checksums and a generated changelog. It uses the
repository's built-in `GITHUB_TOKEN`. Tags such as `v0.1.0-rc.1` create prereleases.

These `v*` tags version the CLI binaries. They do not publish the nested Go
modules or change package manifests automatically. Go module releases need
path-prefixed tags, such as `pkg/core/v0.1.0`, which do not trigger this
workflow.

## Package configuration

Run `sweet init repo` to create repository configuration at the Git root.
Run `sweet init package [dir]` to create a package manifest in an existing
directory (the current directory by default). Package initialization also creates
the `.sweet/` directory for release notes. Both commands preserve existing files.

New package manifests start with:

```yaml
version: 0.0.0
```

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
components to zero, and removes prerelease and build suffixes. Manifest versions
must be complete semantic versions such as `1.2.3` or `2.0.0-beta.1`.

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
sweet bump pkg/core pkg/cli   # Prepare multiple packages from the repository root
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

New changelogs start with `# Changelog`. Each release has a version heading,
with sections in repository configuration order. Types with the same section
share a heading. Notes appear in filename order within each section. Markdown
bodies appear as bullet items, with continuation lines indented to retain their
Markdown formatting and surrounding whitespace removed; breaking
notes receive a bold label. Types without a section are omitted from the
changelog but are still consumed. Existing introductions and release history
are retained, with the new release inserted before the first level-two heading.

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

Run `sweet` to start the `sweet change` workflow. You can also pass `--type` and
`--stdin` directly to `sweet`. Use `sweet --help` to show the available commands.

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
sweet change pkg/core
printf 'Add package discovery.\n' | sweet change pkg/cli --type feature --stdin
```

`--stdin` writes input verbatim as the Markdown body below the generated frontmatter
and skips the description prompt and editor. Supply body text, not a complete frontmatter document.
Existing notes are never overwritten. The command creates `.sweet/` if the
package does not yet have it. Notes require a `type`; the optional `breaking`
field defaults to false. Other frontmatter fields are rejected. The Markdown
body is preserved when notes are read for the status inventory.
