# CLI publishing

After checks pass, CI calls the shared `.github/workflows/release.yml` with
`github-release: true`. The GitHub workflow owns tags, release notes, drafts,
asset uploads, and publication. The Sweet CLI only supplies package metadata,
changelogs, and local Git tags; it does not call GitHub.

Our caller enables GoReleaser for `apps/sweet` and selects
`apps/sweet/go.mod` for Go setup. Other packages publish notes only.
GoReleaser builds archives in snapshot mode with `RELEASE_VERSION` supplied
by the workflow, which also supports prefixed release tags. Its release settings
do not publish anything. The caller's verification command checks the Linux
binary version and SHA-256 checksums before the shared uploader runs.

The shared workflow checks out each exact release commit, verifies the remote
tag, and prepares a draft from its package name and versioned changelog.
The internal build action installs GoReleaser through its official action and
checks the recorded version, commit, and archive hashes. It returns a JSON list
of Archive and Checksum paths selected from `dist/artifacts.json`, plus custom
assets. The GitHub uploader only receives that list of files and rechecks the
remote tag and draft state. Publication runs only after every preceding step succeeds.

Jobs for the same tag are serialized. A build/upload failure leaves the draft
unpublished. Rerun the original CI run to preserve its commit range: retries reuse
matching tags, refresh drafts, and replace partial uploads. Published releases
skip all builds and mutations. API errors fail the job. A moved or deleted tag
stops release mutations. There is no arbitrary historical-tag publishing workflow.

Our caller sets `github-latest` to false. After all release jobs succeed, a
serialized repository-specific promotion job selects the highest published stable
CLI version across current and historical prefixes. It excludes drafts,
prereleases, and non-CLI packages, so retrying an older release cannot replace
a newer Latest release. Rerunning CI also retries failed promotion.

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
