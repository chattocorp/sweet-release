# CLI publishing

The shared workflows create release PRs and push version tags. Publication is
the caller's responsibility. This repository's `.github/workflows/release-cli.yml`
runs on `sweet@*` tag pushes and publishes only the CLI.

The release workflow checks out the tag and uses `apps/sweet/go.mod` for Go setup.
GoReleaser builds archives in snapshot mode with `RELEASE_VERSION` supplied
by the workflow, which also supports prefixed release tags. Its release settings
do not publish anything. The workflow checks the Linux binary version and
SHA-256 checksums before publication.

Sweet extracts the release notes from the versioned changelog. The workflow
passes the notes, archives, and checksums to `gh release create --verify-tag`.
With assets, this command creates a draft, uploads the files, and then publishes.

Jobs for the same tag are serialized. A build failure stops publication; an
upload failure leaves an unpublished draft. Rerun the failed tag workflow to
retry. It rebuilds the assets, skips publication for an already published release,
and recreates a leftover draft before uploading again. The tag is kept.

Versions with prerelease suffixes are marked as prereleases. GitHub selects
Latest automatically. There is no separate promotion job.

## Local validation

Install tools with `mise install`, then run:

```sh
mise run release:check
mise run release:snapshot
```

Snapshots run Go tests and vet, then build Linux, macOS, and Windows archives for
amd64 and arm64 in the ignored `dist/` directory. Windows uses ZIP; other targets
use tar.gz. SHA-256 checksums accompany the archives. Local snapshots default to
`0.0.0-SNAPSHOT-<commit>`; CI sets `RELEASE_VERSION` to the package release version.
The linker overrides `main.version` for release builds; ordinary source builds
use `sweet.Version` from `apps/sweet/version.go`.

See the public [GitHub Actions guide](../apps/docs/src/content/docs/guides/github-actions.md)
for consumer inputs and examples.
