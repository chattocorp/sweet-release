---
title: GitHub Actions
description: Bump release PRs and run your repository's release policy.
---

Sweet's GitHub automation has two steps:

| Workflow | Purpose |
| --- | --- |
| `bump.yml` | Bump versions and create or update release PRs. |
| `release.yml` | Find committed releases, optionally push tags, and run your release command. |

Both workflows use GitHub-hosted Ubuntu runners and download a published Sweet
binary by default. Consumers do not need Go or copies of Sweet's helper scripts.
Your release command owns building, uploading, deploying, and publication.
The release workflow does not create GitHub Releases automatically.

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
There are no implicit draft or publication steps after it finishes.

Set `push-tags: false` when your command owns tagging or does not need tags. Sweet
still fetches existing tags and validates the plan, but creates no local or remote
tags. `RELEASE_TAG` remains the planned name; it need not exist.

Up to four release jobs run concurrently. Each job has a 30-minute timeout, and
commands for the same tag share the `sweet-github-release-<tag>` concurrency group.
A command or pipeline failure fails that job without cancelling other packages.
Rerun the original workflow to retry its original range. Existing tags do not
suppress commands: **make your script safe to repeat**. Sweet cannot guarantee
exactly-once publication for arbitrary commands.

## Sweet's own release policy

This repository calls `bump.yml` from `prepare-releases.yml` and `release.yml` from
CI, using `sweet-version: source` to exercise unreleased CLI code. That development
option builds from the automation revision and requires Go.

Our `release-command` runs `.github/scripts/release-package.sh`. It prepares a GitHub
Release draft, builds and uploads CLI assets with GoReleaser when applicable, and
publishes. Other packages publish notes only. Published retries skip the build.
After a successful release run, a serialized job promotes the
highest stable CLI release to Latest. Rerun a failed release run to finish publication
and promotion.

The workflows share one internal bootstrap action under `.github/actions/`, which
installs Sweet and locates the internal scripts under `.github/scripts/`. Binary
downloads are checked against SHA-256 checksums and the executable's version.
These helpers are implementation details; consumers call `bump.yml` and `release.yml`.
