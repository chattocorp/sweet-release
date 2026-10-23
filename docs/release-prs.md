# Release PRs

The `Release PR` workflow runs on pushes to `main` or manual dispatch on `main`.
It lists packages on current `main` with `sweet status --json`, then uses independent
checkouts of that commit to run `sweet bump` and verify the changed files. It uses
`peter-evans/create-pull-request` to create or update PRs into `main`.
Configure PR policy in `.github/workflows/release-pr.yml`:

```yaml
env:
  RELEASE_PR_STRATEGY: per-package # or combined
```

Sweet no longer exposes `release-pr` or reads a PR strategy. Remove the obsolete
`release-pr` section from existing `.sweet.config.yml` files; it has no effect.
The workflow helper rejects strategies other than `per-package` and `combined`.

With `combined`, all packages with pending bumps share the `sweet/release` PR.
With `per-package`, each eligible package has its own PR. Its branch uses
`sweet/release-package-<path-slug>-<path-hash>` so root packages, nested paths,
and paths with spaces have stable, distinct branch names. Branch names do not
include versions; new notes and versions update the existing open PR.
Each job starts with a clean checkout and runs `sweet bump` for its selected
packages. It checks Git status, including untracked files, and creates a PR only
if the bump changed files. All inventory packages are included, so invalid
packages fail their jobs rather than silently disappearing from cleanup results.

The repository-owned `.github/scripts/release-pr.py` supplies the job matrix,
stable branch names, titles, descriptions, and per-job cleanup results. It uses
only Sweet's status, bump, and changelog extraction commands. There is no separate
release-planning API. Each bump validates its selection before writing.
Use combined mode for packages that share version targets; separate package jobs
do not detect or coordinate overlapping writes. Even different fields in one file
can cause PR merge conflicts until the next run regenerates changes from `main`.
Combined bumps still reject conflicting updates. Dependency propagation is not
implemented.

The PR contains updated manifests, changelogs, configured version files, and
deletions of consumed notes. Notes with no version effect alone do not create a PR.
Do not add a `branch-suffix`, which would create separate PR branches on each run.
After merging or closing a PR, a later release can create a new PR.
The PR description lists each package's target version and new changelog content,
without previous release history. After bumping, the workflow reads the new
versions with `sweet status --json` and extracts their sections with
`sweet release-notes`. Descriptions and cleanup results stay outside the checkout
so they cannot turn a no-op bump into a PR.

Each run regenerates changes from `main`; it does not bump previous release
branches again. Make corrections through `main`, since edits to generated branches
can be overwritten. Each successful job uploads its actual desired branch (or an
empty list for a no-op). After all PR jobs succeed, cleanup collects those results,
requires one result per job, and closes obsolete bot-created
release PRs and deletes their branches. This handles consumed notes, removed
packages, merged PRs, and switching strategies. Cleanup only acts on the reserved
branch names in this repository that have a bot-created PR targeting `main`;
human-created open PRs and fork branches are left alone. Branches without any
associated bot-created PR are also left alone. If `main` has changed during the
run, cleanup waits for the next run. After merging, successful CI creates version
[version tags](version-tags.md). CLI tags then publish binary archives.

One concurrency group serializes whole workflow runs without cancelling an active
update. At most four package jobs run concurrently, each with a separate checkout.
A bump or verification failure leaves that package's PR unchanged; other package
jobs can still succeed. Cleanup is skipped if any preparation job fails, so switching
strategies retains the old PRs until all replacements succeed. Rerun after a GitHub
failure to reconcile the PRs. A subsequent run always starts from current `main`.
The workflow rejects plans above GitHub's limit of 256 matrix entries.

The workflow uses `GITHUB_TOKEN` with `contents: write` and `pull-requests: write`.
Enable **Allow GitHub Actions to create and approve pull requests** under
Settings → Actions → General → Workflow permissions (organization policy must
also allow it). According to [GitHub's workflow trigger rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow),
PR workflows created by this token require a maintainer to approve their runs.
Use a GitHub App token in the PR action if unattended PR checks are needed.

This workflow dogfoods the CLI from source. Other projects can use the same
pattern with an installed `sweet` binary. Add project-specific commands, such
as `pnpm install`, between the bump and verification/PR steps to include lockfile
updates. The workflow helper supplies the job matrix; no Git history scan or
publishing task is needed.
