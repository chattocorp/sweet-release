---
title: GitHub Actions
description: Two shared workflows for release PRs, previews, tags, and publication.
---

Sweet provides two public reusable workflows:

| Workflow | Responsibility |
| --- | --- |
| `bump.yml` | Create and update release PRs from pending change notes. |
| `release.yml` | Preview committed releases on PRs; tag, build, and publish on main pushes. |

The Sweet CLI handles versions, changelogs, and local Git tags. GitHub PRs,
release drafts, uploads, and publication belong to these workflows, not the CLI.

## Version the integration once

Each workflow revision includes a tested default CLI release, currently
`sweet@0.3.0`. Most consumers only need to pin the workflow revision.
`sweet-version` remains an override for testing or independent CLI upgrades.

The examples use `@main` for initial setup. For production, select a published
immutable `workflows-vX.Y.Z` tag or its full commit SHA, and use that revision
for both workflows. Workflow tags are published separately from CLI releases;
the first such tag must be published after this workflow version is merged.
Do not pin an unmerged PR commit: Actions may stop resolving it after a squash
merge and branch deletion.

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
| `sweet-version` | `sweet@0.3.0` | Override the tested CLI release. |
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

## Preview and release with one call

In a CI workflow triggered by PRs and pushes to `main`, call the shared workflow
after your checks. A Go CLI consumer needs:

```yaml
jobs:
  # Define your normal check job above this job.
  release:
    needs: check
    permissions:
      contents: write
    uses: chattocorp/sweet-release/.github/workflows/release.yml@main
    with:
      github-release: true
      goreleaser: v2.18.2
```

No duplicate PR planner call, commit-range expressions, or CLI version input is
needed. In the default `mode: auto`:

- A PR compares its current base with GitHub's test merge commit and runs a
  read-only preview. This also works when the PR branch is behind main; preview
  commit IDs are provisional and main pushes are planned again. It never tags,
  builds, uploads, or publishes, even when publication/build inputs are enabled.
- A push to `main` uses that push's original before/after SHAs and executes the
  plan. Only committed package version changes produce releases.
- Other events fail with instructions to supply an explicit mode and, when
  needed, an explicit commit range.

The caller still owns triggers, permissions, and dependencies. Preserve every
main-push run: do not cancel or coalesce them. Include the pushed SHA in the
caller's main CI concurrency group. Obsolete PR runs may be canceled.

### Execution stages

1. **Plan:** Read committed metadata with `sweet git tag --dry-run`. This job
   has read-only repository permissions and makes no tag or GitHub writes.
2. **Tag:** Validate all planned tags, create missing annotated tags, and push
   the entire set atomically. Conflicts never force-overwrite existing tags.
3. **Release matrix:** Run one job per planned release at its exact commit,
   with up to four jobs concurrently and a per-tag lock.

The matrix consumes the original plan; tagging does not discover a different
set of releases. A failed tag job prevents all publication jobs. An empty plan
skips tagging and execution.

### Release inputs

| Input | Default | Purpose |
| --- | --- | --- |
| `mode` | `auto` | Event-aware behavior; `plan` previews, `release` executes. |
| `base`, `head` | From event | Override both with full SHAs for an explicit range. |
| `sweet-version` | `sweet@0.3.0` | Override the tested CLI release. |
| `push-tags` | `true` | Push the planned tags in execution mode. |
| `github-release` | `false` | Prepare notes/draft, upload assets, and publish. |
| `github-latest` | `auto` | Let GitHub decide Latest, or specify `true`/`false`. |
| `goreleaser` | Empty | Optional pinned GoReleaser version. |
| `go-version-file` | `go.mod` | Go version file for GoReleaser. |
| `goreleaser-package` | Empty | Restrict GoReleaser to one package; others publish notes only. |
| `release-command` | Empty | Optional build/verification command, after GoReleaser if enabled. |
| `release-assets` | Empty | Additional newline-separated file globs; each must match files. |

The optional `release-token` secret defaults to `GITHUB_TOKEN`. The read-only
planner always uses the built-in token; the custom token authenticates tag and
publication jobs. With GitHub publication enabled, no tag-triggered workflow or
personal token is needed for release execution.

The `releases` output is a JSON array containing `package`, `version`, `tag`,
and `commit`, including matching existing tags on retries.

For explicit recovery or historical tests, set `mode: release` and pass the
original full `base` and `head` SHAs. `push-tags: false` disables only tagging,
not execution; GitHub publication then requires matching tags already on the
remote. Use `mode: plan` to guarantee a preview of the complete configuration.

### Build configuration

GoReleaser is an optional builder, not the GitHub publisher. Keep targets,
linker flags, archives, and checksums in the consumer's `.goreleaser.yml`.
The adapter installs Go and the pinned GoReleaser binary, runs a snapshot build,
and verifies its source commit, version, and archive hashes. Set:

```yaml
snapshot:
  version_template: '{{ if index .Env "RELEASE_VERSION" }}{{ .Env.RELEASE_VERSION }}{{ else }}0.0.0-SNAPSHOT-{{ .ShortCommit }}{{ end }}'
changelog:
  disable: true
```

The adapter selects Archive and Checksum entries from `dist/artifacts.json`;
at least one of each is required. Select other outputs with `release-assets`.

For other build systems, supply `release-command` and `release-assets`.
Commands run from the repository root at the exact release commit and own extra
tool setup. The script must exist at that commit. Both builders return the same
internal JSON list of asset paths; the publisher does not parse build-tool metadata.
With no builder or assets, the workflow can publish release notes only.

Commands receive `RELEASE_PACKAGE`, `RELEASE_VERSION`, `RELEASE_TAG`,
`RELEASE_COMMIT`, `GH_REPO`, and `GH_TOKEN`. Sweet is installed.
With `github-release: true`, commands should build/verify files; the workflow
handles publication. Without it, commands may implement their own deployment
policy and must be safe to repeat.

### Tags only

Leave `github-release`, `goreleaser`, and `release-command` unset to push tags
without publication. If another workflow reacts to those tags, supply a suitable
App token or PAT through `release-token`: tags pushed with `GITHUB_TOKEN` do not
start push-triggered workflows. GitHub also suppresses tag-push events when more
than three tags are pushed together; use the direct publication path for larger batches.

### Results and retries

Job summaries show the planned releases, preview/no-op outcomes, publication links,
already-published skips, and the failed stage with retry instructions.

Build/upload failures leave drafts unpublished. Rerun the original CI run to retain
its commit range. Matching tags and drafts are reused, partial uploads are replaced,
and published releases skip builds and mutations. API errors and moved/deleted tags
stop publication. Jobs have a 30-minute execution limit and do not cancel other
packages on failure. Do not modify the same release outside the workflow while it
runs; its concurrency lock only coordinates workflow jobs.

## Maintaining this integration

The two public entry points stay `bump.yml` and `release.yml`. Internal actions
and scripts separate planning/tagging, building, and GitHub publication. Their
paths and outputs are not separate supported consumer APIs.

The repository's **Publish workflow version** maintenance workflow is manually
dispatched from `main` with a version such as `1.0.0`. It requires successful
main CI for the exact dispatched commit and creates an immutable
`workflows-v1.0.0` tag. It does not create a CLI release or change GitHub Latest.
Existing tags cannot be moved. Publish the tag after merging, then pin consumers
to that tag or its merged commit SHA.

When changing the CLI default, update both workflow defaults and the bootstrap
default together, verify the integration, then publish a new workflow version.
Consumers retain the old tested combination until they update their pin.

Sweet itself uses `sweet-version: source` to test unreleased CLI code. Its caller
selects GoReleaser for `apps/sweet`, verifies the resulting binary, and separately
promotes the highest stable CLI release to Latest.
