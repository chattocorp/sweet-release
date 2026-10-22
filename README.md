# Sweet Release

Release packages from Markdown change files. Initialization and package listing
are implemented; release calculation is still in development.

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

`sweet status` shows package paths and versions from anywhere in the repository.
It includes nested packages and respects Git ignore rules for untracked files.
Package names are not required. Dependency metadata is not yet implemented.

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
values produce configuration errors. These policies are loaded and validated;
release calculation and changelog generation are not yet implemented.

## Write release notes

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
Existing notes are
never overwritten. The command creates `.sweet/` if the package does not yet
have it. Release calculation is not yet implemented.
