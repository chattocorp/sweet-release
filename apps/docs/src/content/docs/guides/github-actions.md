---
title: GitHub Actions
description: Shared workflows for release PRs and version tags.
---

Sweet provides two reusable workflows, both run on pushes to `main`:

| Workflow | Responsibility |
| --- | --- |
| `bump.yml` | Create and update release PRs from pending change notes. |
| `tag.yml` | Tag package versions committed by merged release PRs. |

Publishing is yours: add workflows that run on the pushed tags and build,
upload, or deploy each package however it needs.

## Version the integration

The examples use `@main`. To pin, use a published immutable `workflows-vX.Y.Z`
tag or a full commit SHA, and use the same revision for both workflows. By
default the workflows build the Sweet CLI from that same revision
(`sweet-version: source`), so the workflows and CLI always match. Set a CLI
release tag or `latest` to install a prebuilt binary instead.

Internal action references use GitHub's `$/` syntax and require runner 2.336.0
or newer. The shared workflows use GitHub-hosted Ubuntu runners.

## Bump

Create `.github/workflows/bump.yml` in the consumer:

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
    secrets:
      release-token: ${{ secrets.RELEASE_PR_TOKEN }}
```

| Input | Default | Purpose |
| --- | --- | --- |
| `base-branch` | `main` | Source branch and PR target. |
| `strategy` | `per-package` | Separate package PRs, or one `combined` PR. |
| `sweet-version` | `source` | CLI to use: `source`, a release tag, or `latest`. |
| `prepare-command` | Empty | Commands after a bump, before PR creation. |
| `pr-author` | Detected | Login that owns this integration's release PRs. |

The required `release-token` needs Contents and Pull requests write access.
Use a personal or GitHub App token so the generated PRs trigger CI. Personal-token
owners are detected; for an App token set `pr-author` to its bot login.
With `GITHUB_TOKEN`, set `pr-author: github-actions[bot]` and enable Actions PR
creation in repository settings, but generated PRs will not start normal CI.

The workflow reuses reserved release branches and only cleans up PRs owned by
the configured author, in the same repository, against the configured base.
Use one bump workflow and one target branch per repository. Changing token owners
leaves older PRs for manual review.

Up to four package jobs run together from the same base commit. Cleanup waits
for complete successful results and skips if the base branch moved.
After a failure, rerun all jobs; make release-content corrections on the base
branch because later runs replace generated PR changes.

`prepare-command` runs at the repository root only when a bump changed files.
It receives `RELEASE_JOB` JSON with `branch`, `title`, and `packages`, and owns
any extra tool setup. Its changes join the release PR. Use `strategy: combined`
when packages share lockfiles or other generated files.

## Tag

Call `tag.yml` from your main CI after your checks pass:

```yaml
on:
  push:
    branches: [main]
  pull_request:

jobs:
  # Define your check jobs above this job.
  tag:
    needs: check
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    permissions:
      contents: write
    uses: chattocorp/sweet-release/.github/workflows/tag.yml@main
    secrets:
      release-token: ${{ secrets.RELEASE_PR_TOKEN }}
```

The job fetches remote tags and runs `sweet git tag` for the pushed commit range,
from the push's `before` SHA to its pushed SHA. Each committed package version
change gets an annotated tag at the commit that introduced it. It then pushes the
tags one at a time, because GitHub creates no push events when more than three
tags are pushed together.

Matching existing tags are kept, conflicting tags stop the job before any tags are
created, and tags are never moved or overwritten. If a push fails partway, rerun
the original CI run: already-pushed tags are no-ops and the rest are pushed.
Preserve every main-push run (do not cancel or coalesce them), because each one
owns its commit range.

| Input | Default | Purpose |
| --- | --- | --- |
| `sweet-version` | `source` | CLI to use: `source`, a release tag, or `latest`. |

The required `release-token` needs Contents write access. Use a personal or
GitHub App token: tags pushed with `GITHUB_TOKEN` do not trigger other workflows.
The `tags` output is a JSON array of `package`, `version`, `tag`, and `commit`.

## Release from tags

Tags use `<directory-name>@<version>`, or `v<version>` for a root package, unless
the package sets `tag-prefix`. Trigger a release workflow on the tags you want to
publish. For example, to publish a Go CLI in `apps/cli` with GitHub Release notes
from its changelog:

```yaml
name: Release CLI
on:
  push:
    tags: ['cli@*']
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    env:
      GH_TOKEN: ${{ github.token }}
      TAG: ${{ github.ref_name }}
    steps:
      - uses: actions/checkout@v7
      - uses: chattocorp/sweet-release/.github/actions/bootstrap@main
      - run: |
          version=${TAG#cli@}
          # Build your assets into dist/ here.
          sweet changelog --version "$version" -- apps/cli > notes.md
          gh release create "$TAG" dist/* --verify-tag --notes-file notes.md
```

Release workflows should be safe to rerun; for example, skip publication when
`gh release view "$TAG"` shows a published release.

## Maintaining this integration

The public entry points are `bump.yml` and `tag.yml`. Internal actions and
scripts are not separate supported consumer APIs.

The repository's **Publish workflow version** maintenance workflow is manually
dispatched from `main` with a version such as `1.0.0`. It requires successful
main CI for the exact dispatched commit and creates an immutable
`workflows-v1.0.0` tag. Existing tags cannot be moved. Publish the tag after
merging, then pin consumers to that tag or its merged commit SHA.

Sweet itself calls `tag.yml` from its CI. Its `release-cli.yml` workflow
publishes the CLI from each `sweet@*` tag with GoReleaser archives.
