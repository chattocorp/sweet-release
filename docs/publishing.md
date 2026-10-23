# CLI publishing

After tags are pushed, the tag workflow calls `.github/workflows/release.yml`
for each software release in the plan. Only `apps/sweet` and historical
`pkg/sweet` and `pkg/cli`
releases build binaries; docs and retired core releases do not.
Each job checks out the exact tag commit and verifies it against the planner's
commit. Jobs for the same tag are serialized. Publishing failures leave tags
intact; rerunning CI retries publishing, including tags that already exist.

The workflow keeps its own build configuration and script when checking out an
older tag. It selects `apps/sweet/go.mod` for current source,
`pkg/sweet/go.mod` for the previous unified layout, root `go.mod`
for the intermediate layout, and `pkg/cli/go.mod` for historical source after
checkout. The retained check script runs tests and vet for each layout. The tag
selects the build directory, manifest, and changelog: `apps/sweet` for new tags,
and `pkg/sweet` or `pkg/cli` for historical tags. You can publish or retry
an existing CLI tag from main manually:

```sh
gh workflow run release.yml --ref main -f tag=apps/sweet/v0.1.0
```

GoReleaser's free edition builds archives in snapshot mode with an explicit
release version, avoiding its Pro-only support for prefixed tags. GitHub CLI
publishes the artifacts against the existing package tag. No extra
tags are created. The job checks the Linux binary's version and archive checksums
before upload. Release notes come from that version's CLI changelog section.

`sweet changelog [package] --version <version>` reads one release body from
the package's `CHANGELOG.md`. The package is a directory path relative to the current
directory and must contain a valid `.sweet.yml`. With no argument, the command uses
the current package or offers the package picker when a terminal is available.
Without a terminal, pass a package path if the current directory is not a package.
Without `--version`, it selects the first semantic-version heading in changelog
order, skipping headings such as `Unreleased` and headings in fenced code blocks.
It does not use pending notes or the manifest version to select the latest entry.
It excludes the version heading and earlier history,
preserves the body, and ignores headings inside fenced code blocks. A missing
version, missing changelog, or changelog without released versions is an error.
Explicit packages need no repository configuration. Publishing
uses the current metadata binary to read the tagged changelog, so older source
tags do not need to support this command.

Uploads go to a draft release, which is published only after all uploads succeed.
A retry replaces assets in an unfinished draft; an already published release is
left unchanged. API errors stop the job. Prerelease versions produce GitHub
prereleases. The workflow does not move GitHub's Latest designation, so publishing
an older tag cannot replace a newer release's designation.

Install the tools with `mise install`, then run these commands from the root:

```sh
mise run release:check
mise run release:snapshot
```

The snapshot runs tests and static checks, then creates archives and SHA-256
checksums in `dist/` without publishing. It builds `sweet` for Linux, macOS,
and Windows on amd64 and arm64. Windows archives use ZIP; the others use tar.gz.
The binary's `--version` output contains the release or snapshot version.
Local builds default to `sweet.Version` in `apps/sweet/version.go`. The Sweet package's
`version-files` entry updates its annotated constant during `sweet bump`.
The `-X main.version=...` linker option overrides the displayed version
for release and snapshot builds.

Local snapshots use `0.0.0-SNAPSHOT-<commit>` and never publish. CI supplies
`SWEET_RELEASE_VERSION` from the CLI tag for release archives. Both builds run
tests and static checks. Publishing uses the repository's `GITHUB_TOKEN` with
`contents: write`; no additional credential is required.
