# CLI publishing

After checks pass, CI calls `.github/workflows/release.yml` with
`release-command: bash .github/scripts/release-package.sh`. The workflow pushes
tags and runs that command for each planned release at its exact commit.
The script owns draft creation, asset uploads, and publication. Only `apps/sweet` and historical
`pkg/sweet` and `pkg/cli`
releases build binaries; other packages publish GitHub Release notes without assets.
The shared workflow does not create GitHub Releases itself.
Each job checks out the exact release commit and verifies it against the planner's
commit. Jobs for the same tag are serialized. Publishing failures leave tags
intact; rerunning CI retries publishing, including tags that already exist.

Normal release commands use scripts at the release commit. The manual
`publish-cli.yml` workflow retains its current build configuration and scripts
when checking out an older tag, then calls the same release script. It selects `apps/sweet/go.mod` for current source,
`pkg/sweet/go.mod` for the previous unified layout, root `go.mod`
for the intermediate layout, and `pkg/cli/go.mod` for historical source after
checkout. The retained check script runs tests and vet for each layout. The tag
selects the build directory, manifest, and changelog: `apps/sweet` for new tags,
and `pkg/sweet` or `pkg/cli` for historical tags. You can publish or retry
an existing CLI tag from main manually:

```sh
gh workflow run publish-cli.yml --ref main -f tag=apps/sweet/v0.1.0
```

GoReleaser's free edition builds archives in snapshot mode with an explicit
release version, avoiding its Pro-only support for prefixed tags. The build script
uploads artifacts to the existing draft. `actions/github-release` owns draft
creation, release notes, and final publication. No extra
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
uses the shared action's current metadata binary to read the tagged changelog, so older source
tags do not need to support this command.

The repository script uses the `actions/github-release` helper to check state,
skip published releases, and prepare a draft from the package metadata at the
verified tag commit. Command jobs install pinned GoReleaser with `go install` if
it is not already available; manual jobs install it with the GoReleaser action.
The build script calls the same state helper before building and
again before uploading, and refuses to upload without an existing draft.
The action's publish phase runs only after all builds and uploads succeed. It
requires only the tag and commit; it reads prerelease status from GitHub. Remote
tag commits are checked before release mutations, so a moved/deleted remote tag
cannot silently pass validation using a stale local tag.
A retry replaces assets in an unfinished draft; an already published release is
left unchanged. The `published` output skips rebuilding on retries. API errors
stop the job. Prerelease versions produce GitHub
prereleases. After the release workflow succeeds, a separate job sets GitHub's Latest designation to
the highest published stable CLI version. It compares semantic versions across
`apps/sweet/v`, `pkg/sweet/v`, and `pkg/cli/v` tags. Drafts, prereleases, and other
packages are excluded. Equal versions prefer the current package prefix, then
`pkg/sweet`, then `pkg/cli`; build metadata does not affect version precedence,
and equal-version tags within a prefix use lexical order as a deterministic tie-break.

Promotion jobs share a repository-wide concurrency group and read releases only
after acquiring it. Each job reconciles all published CLI releases, so publishing
an older tag cannot displace a newer stable version. Promotion also runs when the
publisher skips an existing release. A partial release failure delays promotion
until a successful retry. If promotion fails, rerun the release workflow;
it retries promotion without rebuilding or replacing published assets.

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
