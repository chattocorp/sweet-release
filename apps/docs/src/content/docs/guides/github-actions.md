---
title: GitHub Actions
description: Install Sweet and reuse release PR and tag workflows.
---

Sweet provides a setup action and two reusable workflows. The workflows run on
GitHub-hosted Ubuntu runners. They download a published Sweet binary by default;
consumers do not need Go or copies of Sweet's helper scripts.

| Workflow | Purpose |
| --- | --- |
| `release-pr.yml` | Prepare version bumps and create or update release PRs. |
| `release.yml` | Tag committed versions and create GitHub Releases. |

The examples use `@main` after these files have merged. For production, replace
`@main` with a full commit SHA. The action revision and CLI release tag are separate
versions. There is no `actions-v1` tag yet. Internal action references use GitHub's
`$/` syntax to select the same automation revision, requiring runner 2.336.0 or newer.

## Install the CLI

```yaml
steps:
  - uses: actions/checkout@v7
  - uses: chattocorp/sweet-release/actions/setup@main
    with:
      version: apps/sweet/v0.2.0
  - run: sweet status --json
```

`version` accepts an exact GitHub release tag or `latest` (the default). The action
resolves `latest` once, downloads the archive and `checksums.txt`, verifies its
SHA-256 checksum, and checks the executable's version before adding it to `PATH`.
Outputs are `version` and `tag`. Checksums detect damaged downloads; they are not
an independent signature.

Linux, macOS, and Windows archives are supported for amd64 and arm64. The runner
must supply Bash, Python 3, and the GitHub CLI. `token` defaults to `github.token`.
The installer does not require a checkout.

## Create release PRs

Create `.github/workflows/release-pr.yml` in your repository:

```yaml
name: Release PRs
on:
  push:
    branches: [main]
  workflow_dispatch:
permissions:
  contents: write
  pull-requests: write
jobs:
  releases:
    uses: chattocorp/sweet-release/.github/workflows/release-pr.yml@main
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
For personal tokens, the workflow detects the user through GitHub's API. For an
App installation token, set `pr-author` to its bot login, such as
`my-release-app[bot]`. For `GITHUB_TOKEN`, set it to `github-actions[bot]` and enable
the repository setting that permits Actions to create PRs. Use a suitable personal
or App token for unattended PR checks.

Cleanup only considers reserved release branches with PRs owned by that login,
in the same repository, targeting the configured base. Open PRs from other authors
are preserved. Changing the token owner leaves old PRs for manual review. Use one
release PR workflow and one target branch per repository: branch names are shared.

Every package job starts at the same current base commit. Up to four jobs run
together. Cleanup waits for all jobs to succeed and stops if the base changed.
Rerun all jobs after a failure; cleanup needs complete results from one attempt.
Make corrections on the base branch because later runs replace generated changes.

### Update lockfiles and verify changes

Keep project commands in a script in your repository:

```yaml
with:
  strategy: combined
  prepare-command: bash scripts/prepare-release.sh
```

The script runs at the repository root only when `sweet bump` changed files.
`RELEASE_JOB` is JSON with `branch`, `title`, and `packages`. The script can install
tools, update lockfiles, and run checks. Its changes are included in the PR. A
nonzero exit stops that package's PR update and prevents cleanup. The workflow
does not set up Node, Go, or package managers for the hook. Use trusted base-branch
scripts. Use `combined` when packages modify shared files.

## Create releases after checks pass

Call the release workflow from your existing CI workflow:

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
```

`base` and `head` are required. The caller owns triggers and check dependencies.
Do not cancel or coalesce main-push runs: every pushed range must reach tagging.
Include the pushed SHA in any CI concurrency group.

The workflow checks out `head` with full history, fetches tags, and pushes the
planned tags atomically. Conflicts fail without force pushes. It then checks out
each release's exact commit and creates a GitHub Release with the package's name,
version, and changelog section. Prerelease versions produce GitHub prereleases.
By default, it publishes these releases without assets. Reruns include matching
existing tags; already published releases remain unchanged.

Set `draft: true` alongside `base` and `head` if downstream jobs must upload assets
first. Failed builds or uploads then leave a draft for the next attempt.

The `releases` output is a JSON array containing `package`, `version`, `tag`, and
`commit`. With `draft: true`, pass it to a downstream publishing job:

```yaml
publish:
  needs: release
  if: needs.release.outputs.releases != '[]'
  runs-on: ubuntu-latest
  permissions:
    contents: write
  concurrency:
    group: sweet-github-release-${{ matrix.tag }}
    cancel-in-progress: false
  strategy:
    fail-fast: false
    matrix:
      include: ${{ fromJSON(needs.release.outputs.releases) }}
  steps:
    - uses: actions/checkout@v7
      with:
        ref: ${{ matrix.commit }}
        fetch-depth: 0
    - uses: chattocorp/sweet-release/actions/github-release@main
      id: status
      with:
        phase: status
        tag: ${{ matrix.tag }}
        commit: ${{ matrix.commit }}
    - name: Build and upload assets
      if: steps.status.outputs.state == 'draft'
      env:
        GH_TOKEN: ${{ github.token }}
        PACKAGE: ${{ matrix.package }}
        VERSION: ${{ matrix.version }}
        TAG: ${{ matrix.tag }}
      run: bash scripts/upload-release-assets.sh
    - uses: chattocorp/sweet-release/actions/github-release@main
      with:
        phase: publish
        tag: ${{ matrix.tag }}
        commit: ${{ matrix.commit }}
```

Your script builds assets and uploads them to the existing draft, for example
with `gh release upload "$TAG" dist/*.tar.gz --clobber`. Sweet publishes only after
the script succeeds. Keep the per-tag concurrency group around the full build,
upload, and publication sequence. Tags pushed with `GITHUB_TOKEN` do not trigger
another push workflow, so connect publishing through `needs`. See GitHub's
[workflow trigger rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow).

### GitHub Release action

`actions/github-release` has three phases. Each requires `tag` and a full `commit`
SHA. Check out that commit with full history and an `origin` remote pointing to
the target repository.

| Phase | Behavior | Additional inputs |
| --- | --- | --- |
| `status` | Read state without installing Sweet or changing the release. | None |
| `draft` | Create or refresh release notes at the tag commit. | `package`, `version`; optional `sweet-version` |
| `publish` | Publish an existing draft after asset jobs succeed. | Optional `latest` |

The `state` output is `missing`, `draft`, or `published`. The reusable workflow
already prepares drafts, so downstream asset jobs only need `status` and `publish`.
Standalone/manual jobs can call `draft` when `status` returns `missing`.

The action rejects mismatched checkouts, local tags, and remote tags. It checks the
remote tag again immediately before a release mutation. A moved or deleted tag
stops publication. These checks are not a lock on remote Git refs; protect release
tags against deletion and force updates to prevent changes during API requests.
The draft phase verifies the manifest version and reads the changelog at that
commit, even when a push contains several releases.

The `published` output is `true` for an already published release, allowing asset
jobs to skip rebuilding. Published releases and their assets are left unchanged
on retries. API errors fail the action; they do not count as missing releases.
State lookup queries the requested tag, rather than listing all releases. A missing
release is distinct from an inaccessible repository or API error. `token` defaults
to `github.token`; mutations need Contents write access.

Sweet defaults to `latest: 'false'`. To designate a release as GitHub's Latest,
pass `latest: 'true'` to the publish phase. The action checks the existing release's
prerelease flag; the caller does not repeat its version. Prereleases cannot be Latest. This is
an explicit selection, not a highest-version comparison. Monorepos should define
which package owns Latest and serialize any repository-wide selection policy.

## Developing Sweet's automation

This repository uses the shared workflows with `sweet-version: source` to test
unreleased CLI behavior. That development option builds from the automation
revision and requires Go. Consumers should use a binary release. The local
`publish-cli.yml` workflow builds and uploads Sweet binaries with GoReleaser between the shared draft
and publish steps. Documentation releases publish their notes without assets.

CI also runs integration tests with a real Sweet binary, temporary Git repositories
and a bare remote. A stateful fake GitHub CLI exercises fresh drafts, uploads,
publication, retries, API failures, and moved/deleted tags without publishing test
releases. Hosted smoke checks separately exercise real read-only GitHub API calls.
