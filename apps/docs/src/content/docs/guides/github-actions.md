---
title: GitHub Actions
description: Bump release PRs and run your repository's release policy.
---

Sweet's GitHub automation has two steps:

| Workflow | Purpose |
| --- | --- |
| `bump.yml` | Bump versions and create or update release PRs. |
| `release.yml` | Tag committed versions and optionally build assets and publish GitHub Releases. |

Both workflows use GitHub-hosted Ubuntu runners and download a published Sweet
binary by default. Consumers do not need Go or copies of Sweet's helper scripts.
The Sweet CLI has no GitHub release behavior. The reusable workflow owns the
optional GitHub lifecycle; callers keep their build configuration locally.

The examples use `@main` after these files have merged. For production, replace
`@main` with a full commit SHA. The automation revision and CLI release tag are
separate versions. There is no `actions-v1` tag yet. Internal action references use
GitHub's `$/` syntax, requiring runner 2.336.0 or newer.

## Bump

Create `.github/workflows/bump.yml` in your repository:

```yaml
name: Bump
on:
  push:
    branches: [main]
  workflow_dispatch:
permissions:
  contents: write
  pull-requests: write
jobs:
  bump:
    uses: chattocorp/sweet-release/.github/workflows/bump.yml@main
    with:
      sweet-version: apps/sweet/v0.2.0
      strategy: per-package
    secrets:
      release-token: ${{ secrets.RELEASE_PR_TOKEN }}
```

| Input | Default | Purpose |
| --- | --- | --- |
| `sweet-version` | `latest` | CLI release tag to install. |
| `base-branch` | `main` | Source branch and PR target. |
| `strategy` | `per-package` | Separate package PRs, or one `combined` PR. |
| `prepare-command` | Empty | Bash commands after a bump, before PR creation. |
| `pr-author` | Detected | Login used to identify PRs eligible for cleanup. |

The required `release-token` secret needs Contents and Pull requests write access.
For personal tokens, the workflow detects the user through GitHub's API. For an App
installation token, set `pr-author` to its bot login. For `GITHUB_TOKEN`, set it to
`github-actions[bot]` and enable the repository setting that permits Actions to
create PRs. Use a suitable personal or App token for unattended PR checks.

Cleanup only considers reserved release branches with PRs owned by that login,
in the same repository, targeting the configured base. Other authors' open PRs are
preserved. Changing the token owner leaves old PRs for manual review. Use one bump
workflow and one target branch per repository: branch names are shared.

Every package job starts at the same current base commit. Up to four jobs run
together. Cleanup waits for all jobs to succeed and stops if the base changed.
Rerun all jobs after a failure; cleanup needs complete results from one attempt.
Make corrections on the base branch because later runs replace generated changes.

Use `prepare-command: bash scripts/prepare-release.sh` to update lockfiles or run
checks. It runs at the repository root only when the bump changed files. Its changes
are included in the PR. `RELEASE_JOB` contains JSON with `branch`, `title`, and
`packages`. A failed command prevents that PR's update and cleanup. The script owns
its tool setup. Use `combined` when packages modify shared files.

## Release

Call `release.yml` after your normal checks pass:

```yaml
jobs:
  # Define your normal check job above this job.
  release:
    needs: check
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    permissions:
      contents: write
    uses: chattocorp/sweet-release/.github/workflows/release.yml@main
    with:
      base: ${{ github.event.before }}
      head: ${{ github.sha }}
      sweet-version: apps/sweet/v0.2.0
      release-command: bash scripts/release.sh
```

| Input | Default | Purpose |
| --- | --- | --- |
| `base` | Required | Commit before the push; all-zero SHA for a new branch. |
| `head` | Required | Exact pushed commit that passed checks. |
| `sweet-version` | `latest` | CLI release tag to install. |
| `push-tags` | `true` | Create and push package tags before running commands. |
| `release-command` | Empty | Bash command to run for each planned release. |
| `github-release` | `false` | Prepare notes and a draft, then upload assets and publish. |
| `github-latest` | `auto` | Let GitHub choose Latest, or explicitly use `true`/`false`. |
| `goreleaser` | Empty | Pinned GoReleaser version, such as `v2.18.2`, for archive builds. |
| `go-version-file` | `go.mod` | Go version file for GoReleaser builds. |
| `goreleaser-package` | Empty | Restrict GoReleaser to one package path; other packages publish notes only. |
| `release-assets` | Empty | Additional newline-separated file globs to upload. Each must match files. |

The caller owns triggers and check dependencies. Do not cancel or coalesce main-push
runs: every pushed range must reach release planning. Include the pushed SHA in any
CI concurrency group. The `releases` output is a JSON array with `package`, `version`,
`tag`, and `commit`, including matching existing tags on retries.

### Tags only

Omit `release-command` to create and push tags without running a publication job.
Tags are pushed atomically; conflicts fail without force pushes or partial updates.
If the push fails, no release commands run.

Tags pushed with `GITHUB_TOKEN` do not start tag-triggered workflows. To use those
workflows, pass a suitable GitHub App installation token or PAT as the optional
`release-token` secret. It authenticates both checkouts/tag pushes and `GH_TOKEN`
for commands. Without it, the workflow uses `github.token`.

GitHub does not emit tag push events when more than three tags are pushed together.
Sweet pushes the whole plan atomically, so use the command path for larger batches,
or use `push-tags: false` and let your command implement a different push strategy.
See GitHub's [workflow trigger rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)
and [push event limits](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#push).

### Publish GitHub Releases

Set `github-release: true` to verify the remote tag, read the package name and
changelog, and prepare a draft. After builds and uploads succeed, the workflow
publishes the draft. Prerelease versions produce GitHub prereleases. Tags must
exist remotely before draft preparation; normally leave `push-tags: true`.

For a Go CLI, add these inputs to the shared release call:

```yaml
      github-release: true
      goreleaser: v2.18.2
```

Keep build targets, linker flags, archive names, and checksums in the caller's
`.goreleaser.yml`. The workflow installs Go and the pinned GoReleaser binary,
then builds in snapshot mode without GoReleaser publishing. Configure the
snapshot version to use the version supplied by the workflow:

```yaml
snapshot:
  version_template: '{{ if index .Env "RELEASE_VERSION" }}{{ .Env.RELEASE_VERSION }}{{ else }}0.0.0-SNAPSHOT-{{ .ShortCommit }}{{ end }}'
changelog:
  disable: true
```

The uploader reads `dist/artifacts.json` and uploads Archive and Checksum
artifacts. At least one of each is required. It verifies the build version, source
commit, and archive hashes before upload. Other GoReleaser artifact types are not
uploaded automatically; use `release-assets` for additional files. GoReleaser's
release and changelog settings do not control the GitHub lifecycle.

For other build systems, use `release-command` to build files and `release-assets`
to select them. With no build or assets, the workflow publishes release notes only.
If both GoReleaser and a command are set, the command runs after GoReleaser and
before uploads, so it can verify binaries or prepare extra files.

Build or upload failures leave the draft unpublished. Rerunning the original CI
run refreshes the draft and replaces matching assets. Already-published releases
skip all builds, commands, uploads, and publication. A moved or deleted remote
tag stops mutations. Do not mutate the same release outside the workflow while
it is running; the concurrency lock only coordinates workflow jobs.

No tag-triggered publishing workflow is needed. The built-in `GITHUB_TOKEN` is
sufficient for this path, while Bump still needs a token that can trigger PR CI.

### Run a release command

The command runs from the repository root at the exact commit that introduced the
package version. A script referenced by a relative path must exist at that commit.
Sweet is installed, and these environment variables are available:

| Variable | Value |
| --- | --- |
| `RELEASE_PACKAGE` | Package directory, relative to the repository root. |
| `RELEASE_VERSION` | Released version. |
| `RELEASE_TAG` | Planned tag name. |
| `RELEASE_COMMIT` | Full commit SHA. |
| `GH_REPO` | Caller repository (`owner/name`). |
| `GH_TOKEN` | Release token, or the built-in GitHub token. |

The script installs any additional tools it needs. For example, it can select a
package build using `RELEASE_PACKAGE`, upload artifacts, or create a GitHub Release.
Without `github-release`, the command owns any publication policy. With it,
the workflow prepares the draft before the command and uploads/publishes after
the command succeeds; the command should only build or verify files.

Set `push-tags: false` when your command owns tagging or does not need tags. Sweet
still fetches existing tags and validates the plan, but creates no local or remote
tags. `RELEASE_TAG` remains the planned name; it need not exist.

Up to four release jobs run concurrently. Each job has a 30-minute timeout, and
commands for the same tag share the `sweet-github-release-<tag>` concurrency group.
A command or pipeline failure fails that job without cancelling other packages.
Rerun the original workflow to retry its original range. Existing tags do not
suppress commands, but published GitHub Releases do when `github-release` is enabled.
**Make your script safe to repeat** after a partial failure. Sweet cannot guarantee
exactly-once publication for arbitrary commands.

## Sweet's own release policy

This repository calls `bump.yml` from `prepare-releases.yml` and `release.yml` from
CI, using `sweet-version: source` to exercise unreleased CLI code. That development
option builds from the automation revision and requires Go.

Our caller enables `github-release` and GoReleaser for `apps/sweet`. Its optional
command checks the binary version and checksums. Other packages publish notes only.
There is no repository-specific GitHub lifecycle script. Published retries skip the build.
After a successful release run, a serialized job promotes the
highest stable CLI release to Latest. Rerun a failed release run to finish publication
and promotion.

The workflows share one internal bootstrap action under `.github/actions/`, which
installs Sweet and locates the internal scripts under `.github/scripts/`. Binary
downloads are checked against SHA-256 checksums and the executable's version.
These helpers are implementation details; consumers call `bump.yml` and `release.yml`.
