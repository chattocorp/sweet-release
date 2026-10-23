# Development

The repository has two independently versioned Sweet packages: the Go CLI in
`apps/sweet` and the Astro and Starlight documentation site in `apps/docs`.

The Go module in `apps/sweet` contains:

- `cmd/sweet`: the CLI and command integration tests.
- `core`: package discovery, release planning, and file updates.
- `version.go`: the CLI version, updated by the package manifest.

The module path is `github.com/chattocorp/sweet-release/apps/sweet`. Core imports
use `github.com/chattocorp/sweet-release/apps/sweet/core`. The root `go.work`
contains this module. The root is not a release package.

The unified package starts from CLI version 0.1.0 and retains the CLI changelog.
The retired core changelog in `apps/sweet/core/CHANGELOG.md` is historical only.
New tags use `apps/sweet/v<version>`, which also versions the nested Go module.
Existing `pkg/sweet/v`, `pkg/cli/v`, and `pkg/core/v` tags retain their original
source layouts. Moving a manifest above `0.0.0` to a new directory makes it
eligible for a tag at that path, even if its version is unchanged.

With the Go version from `mise.toml` installed, run these commands at the root:

```sh
go run ./apps/sweet/cmd/sweet status
go test ./apps/sweet/...
go vet ./apps/sweet/...
go build -o bin/sweet ./apps/sweet/cmd/sweet
```

The equivalent Mise tasks are `mise run sweet -- status`, `mise run check`, and
`mise run build`. Use `mise run deploy` to install `sweet` into `~/.local/bin`.
Run all Go checks from the repository root.

GitHub Actions runs CI on branch pushes and pull requests. It checks Go formatting,
runs all tests with race detection, runs `go vet`, and builds the CLI
on Linux. You can also start the CI workflow manually from the Actions tab.
To run the additional checks locally, use `gofmt -l apps/sweet` (no output means
formatting is correct) and `go test -race ./apps/sweet/...`.

## Documentation site development

Install the tools with `mise install`. The site uses Node 24 and pnpm, with its own
`package.json` and `pnpm-lock.yaml` in `apps/docs`; no root JavaScript workspace is needed.
From the repository root:

```sh
mise run docs:install
mise run docs:dev
mise run docs:check
mise run docs:build
mise run docs:preview
```

The equivalent pnpm commands are `pnpm --dir apps/docs install --frozen-lockfile` and
`pnpm --dir apps/docs run <dev|check|build|preview>`. Content lives in
`apps/docs/src/content/docs/`. Navigation is explicit in `apps/docs/astro.config.mjs`;
add new pages there. Use relative links between content pages. Starlight provides
search, a table of contents, and light and dark themes.

CI checks and builds the docs alongside Go checks before tagging releases.
Static output goes to `apps/docs/dist/`. Hosting is not configured yet. Set Astro's
`site` (and `base` for subpath hosting) when choosing a deployment target, and
check navigation, the favicon, and search under that URL.

The site's `.sweet.yml` controls its independent version and updates
`package.json`. After `sweet bump apps/docs`, run
`pnpm --dir apps/docs install --lockfile-only --no-frozen-lockfile --ignore-scripts` and commit
the lockfile with the release. The Release PR workflow does this and verifies
the site automatically for docs releases. Docs tags use `apps/docs/v<version>`
and do not publish CLI binaries.

## Issues

We use **Means** for issue tracking. Issues live in the Git-tracked `issues/`
collection. Follow the
[sweet-issues skill](../.agents/skills/sweet-issues/SKILL.md) when finding, creating,
or updating issues. Read `issues/means.schema.json` before writing and use
`--path issues` from the repository root; Means does not search parent directories.
