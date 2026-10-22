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
