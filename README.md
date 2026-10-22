# Sweet Release

Release packages from Markdown change files. Initialize packages, write release
notes, inspect pending version bumps with `sweet status`, and prepare releases
with `sweet bump`.

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
To run the additional checks locally, use `gofmt -l packages` (no output means
formatting is correct) and `go test -race ./packages/cli/... ./packages/core/...`.

## Releases

Install the tools with `mise install`, then run these commands from the root:

```sh
mise run release:check
mise run release:snapshot
```

The snapshot runs tests and static checks, then creates archives and SHA-256
checksums in `dist/` without publishing. It builds `sweet` for Linux, macOS,
and Windows on amd64 and arm64. Windows archives use ZIP; the others use tar.gz.
The binary's `--version` output contains the release or snapshot version.

To publish a CLI release, update `packages/cli/.sweet.yml` to the intended
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
path-prefixed tags, such as `packages/core/v0.1.0`, which do not trigger this
workflow. Prefixed release tags in GoReleaser itself require
[GoReleaser Pro](https://goreleaser.com/customization/monorepo/).

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
Other version files, such as `package.json`, are not updated.

New changelogs start with `# Changelog`. Each release has a version heading,
with sections in repository configuration order. Types with the same section
share a heading. Notes appear in filename order within each section. Markdown
bodies retain their formatting, with surrounding whitespace removed; breaking
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
