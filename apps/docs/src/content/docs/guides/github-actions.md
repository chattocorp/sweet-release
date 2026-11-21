---
title: GitHub Actions
description: Shared workflows for PR validation, release PRs, and version tags.
---

Sweet provides three reusable workflows:

| Workflow | Responsibility |
| --- | --- |
| `doctor.yml` | Validate pending releases in pull requests. |
| `bump.yml` | Create and update release PRs from pending change notes. |
| `tag.yml` | Tag package versions committed by merged release PRs. |

Publishing is yours: fan out over the tags that `tag.yml` returns, in the same
run, and build, upload, or deploy each package however it needs.
[`sweet github release`](#publish-a-github-release) handles GitHub Releases.

## Version the integration

The examples use `@main`. To pin, use a published immutable `workflows-vX.Y.Z`
tag or a full commit SHA, and use the same revision for all workflows. By
default the workflows build the Sweet CLI from that same revision
(`sweet-version: source`), so the workflows and CLI always match. Set a CLI
release tag or `latest` to install a prebuilt binary instead.

Internal action references use GitHub's `$/` syntax and require runner 2.336.0
or newer. The shared workflows use GitHub-hosted Ubuntu runners.

## Doctor

Add this job to your PR CI, or create `.github/workflows/doctor.yml`:

```yaml
name: Validate releases
on:
  pull_request:
permissions:
  contents: read
jobs:
  doctor:
    uses: chattocorp/sweet-release/.github/workflows/doctor.yml@main
```

Pin `@main` to the same workflow tag or full commit SHA as `bump.yml` and
`tag.yml`, using a revision that includes `doctor.yml`. The default
`sweet-version: source` builds the CLI from that revision. Older binaries,
including Sweet 0.2.0, do not validate pending notes with `doctor`.

The workflow checks the PR merge checkout with `sweet doctor`. It needs no
release token and does not change files. Make this job a required status check
to block merges with invalid releases. If your CI already installs Sweet, add
a step with `run: sweet doctor` instead.

| Input | Default | Purpose |
| --- | --- | --- |
| `sweet-version` | `source` | CLI to use: `source`, a release tag, or `latest`. |

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

### Prereleases

With [`prerelease`](/reference/configuration/#prereleases) set in a package
manifest, its release PR prepares the next prerelease, such as
`chore: release api 1.3.0-beta.0`. Notes merged before the PR keep updating that
same version; the counter advances only after the PR is merged. Changing the label
on the base branch, for example to `rc`, produces a PR without new notes. Removing
it produces the final `1.3.0` PR, whose description lists all changes since the
last stable release.

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

The optional `release-token` secret needs Contents write access and defaults to
`GITHUB_TOKEN`. That is all you need when you
[publish in the same run](#publish-releases). Tags pushed with `GITHUB_TOKEN` do
not trigger other workflows, so pass a personal or GitHub App token only if
separate workflows run on the pushed tags.

| Output | Content |
| --- | --- |
| `tags` | JSON array with one entry per tag in the range. |
| `packages` | JSON object from package path (`.` for the root) to its newest entry. |

Each entry looks like this:

```json
{
  "package": "apps/cli",
  "version": "1.3.0",
  "tag": "cli@1.3.0",
  "commit": "<sha>",
  "prerelease": false,
  "highest": true
}
```

`prerelease` is true for versions such as `1.3.0-beta.0`. `highest` is true when
the version is the highest stable version among the package's tags, including
older ones, so a rerun after a newer release reports `false`. Use it for
per-package channels such as a Docker `latest` tag. It is not GitHub's
repository-wide Latest release; `sweet github release` decides that. `packages` returns `null`
for a package without a tag in this run; if a push releases a package twice,
`tags` lists both and `packages` keeps the newer one.

## Publish releases

Publish from the same workflow run that tags. Pass each tag to a reusable
workflow in your repository. No personal token is needed:

```yaml
jobs:
  tag:
    # As above.

  # One job per tag, for packages that publish the same way.
  release:
    needs: tag
    if: needs.tag.outputs.tags != '[]'
    strategy:
      matrix:
        release: ${{ fromJSON(needs.tag.outputs.tags) }}
    permissions:
      contents: write
    uses: ./.github/workflows/release.yml
    with:
      tag: ${{ matrix.release.tag }}
      commit: ${{ matrix.release.commit }}

  # One job for a specific package. A job-level if cannot read matrix values,
  # so select the package from the packages output instead.
  cli:
    needs: tag
    if: fromJSON(needs.tag.outputs.packages)['apps/cli'] != null
    permissions:
      contents: write
    uses: ./.github/workflows/release-cli.yml
    with:
      tag: ${{ fromJSON(needs.tag.outputs.packages)['apps/cli'].tag }}
      commit: ${{ fromJSON(needs.tag.outputs.packages)['apps/cli'].commit }}
```

The called workflow checks out `inputs.commit`, builds, and publishes. Rerunning
a failed job retries the publication for that tag. Keep the
"call `tag.yml` after checks pass" order, so only checked commits are released.

If you would rather trigger separate workflows with `on: push: tags`, pass a
personal or GitHub App token as `release-token` (for example
`release-token: ${{ secrets.RELEASE_PR_TOKEN }}`).

### Publish a GitHub Release

`sweet github release <tag> [assets...]` publishes a GitHub Release for a Sweet
tag. A called workflow could end like this:

```yaml
on:
  workflow_call:
    inputs:
      tag: { type: string, required: true }
      commit: { type: string, required: true }
permissions:
  contents: write
concurrency:
  group: github-release-${{ inputs.tag }}
  cancel-in-progress: false
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          ref: ${{ inputs.commit }}
      - uses: chattocorp/sweet-release/.github/actions/bootstrap@main
      # Build your assets into dist/ here.
      - run: sweet github release "$TAG" dist/*
        env:
          TAG: ${{ inputs.tag }}
          GH_TOKEN: ${{ github.token }}
```

The command:

1. Creates a draft release for the existing tag, with notes from the package's
   changelog entry for that version, or reuses an existing draft and refreshes
   its title and notes the same way. A published release is left unchanged
   (see below).
2. Uploads the assets, replacing files with the same name.
3. Publishes the release, marks prerelease versions as prereleases, and sets
   whether it is the repository's Latest release.

GitHub has one Latest release per repository. Sweet makes a release Latest only
when it is the highest stable version of the root package, or of the only
package in the repository. Other packages' releases never become Latest
automatically. Pass `--latest` or `--latest=false` to decide yourself.

Reruns are safe. They reuse a draft, refresh its title and notes, and replace
its assets. An already published
release is left unchanged: no uploads and no edits, because publication already
set its prerelease and Latest state in one step. Pass `--replace-published` to
upload to a published release anyway and reapply that state; its title and
notes stay as they are. `gh` deletes each
old asset before uploading its replacement, so a failed upload leaves that file
missing from the live release until a rerun succeeds. Serialize jobs for the same tag, as above, so two runs do not create
two drafts. Use `--draft` to create and upload without publishing, then run the
command again without it to publish. `--title` and `--notes-file` replace the
default title (the package `name` and version, or the tag) and notes for a new
or draft release. The command needs the [`gh` CLI](https://cli.github.com/), which
GitHub-hosted runners include, and a token with Contents write access in
`GH_TOKEN`.

## Maintaining this integration

The public entry points are `bump.yml` and `tag.yml`. Internal actions and
scripts are not separate supported consumer APIs.

The repository's **Publish workflow version** maintenance workflow is manually
dispatched from `main` with a version such as `1.0.0`. It requires successful
main CI for the exact dispatched commit and creates an immutable
`workflows-v1.0.0` tag. Existing tags cannot be moved. Publish the tag after
merging, then pin consumers to that tag or its merged commit SHA.

Sweet itself calls `tag.yml` from its CI with a personal token. Its
`release-cli.yml` workflow runs on each `sweet@*` tag, builds GoReleaser archives,
and publishes them with `sweet github release`.
