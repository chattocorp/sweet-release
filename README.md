# Sweet Release

Release packages from Markdown change files. Initialization and package listing
are implemented; release calculation is still in development.

## Development

The repository uses a Go workspace with two modules:

- `packages/cli`: the `sweet` CLI, command output, and command integration tests.
- `packages/core`: package discovery, manifest parsing, and initialization.

Each module has its own `.sweet.yml` and version. The root holds shared
`.sweet.config.yml` configuration and `go.work`; it is not a release package.
The CLI imports the core module. A local replacement in its `go.mod` also lets
module commands resolve core when workspace mode is disabled.

With the Go version from `mise.toml` installed, run these commands at the root:

```sh
go run ./packages/cli/cmd/sweet list
go test ./packages/cli/... ./packages/core/...
go vet ./packages/cli/... ./packages/core/...
go build -o bin/sweet ./packages/cli/cmd/sweet
```

The equivalent Mise tasks are `mise run sweet -- list`, `mise run check`, and
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
directory (the current directory by default). Both commands preserve existing files.

New package manifests start with:

```yaml
version: 0.0.0
```

`sweet list` lists package paths and versions from anywhere in the repository.
It includes nested packages and respects Git ignore rules for untracked files.
Package names are not required. Dependency metadata is not yet implemented.
