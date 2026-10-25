# Version tags

On pushes to `main`, CI calls `.github/workflows/tag.yml` after all checks
succeed. The shared workflow builds Sweet from its own revision by default,
checks out the exact pushed SHA, fetches tags, and invokes
`.github/scripts/tag-releases.sh` with the push's before and after SHAs.
It creates and pushes package tags and returns their metadata as JSON.
Separate workflows triggered by those tags own builds and publication.
Pull requests, other branches,
and manual CI dispatches do not create tags. Each main push has its own concurrency
group so a later push cannot cancel a range before tagging. Separate runs can push
different tags concurrently; a competing write to the same tag fails without force.

`sweet git tag` creates missing annotated local tags for package versions committed
at `HEAD`. It follows first-parent history to find the commit that introduced each
current version; it does not backfill older versions or use working files. It skips
`0.0.0` and retains matching existing tags, including lightweight tags. Conflicting
tags stop the operation before creation. Full history is required; shallow clones
must fetch with `--unshallow` first. Fetch remote tags before running the command
to include them in conflict checks. Sweet does not fetch or push tags itself.

Use `sweet git tag --dry-run` to preview without writing. Add `--json` for the
machine-readable plan. Its `exists` field describes tags present before creation.
An operational Git failure can leave some local tags created; rerun to complete
the remaining tags. Tags are never moved or overwritten.

For an explicit commit range, use
`sweet git tag --base <before-sha> --head <after-sha> --dry-run --json`.
Both range flags are required together. This prints a JSON array with
`package`, `version`, `name`, `commit`, and `exists` fields. It reads committed
manifests, not working files, and makes no Git changes or network requests. Fetch
remote tags before invoking it if existing-tag checks must include remote state.
Use a full-history checkout. The base must be on the head's first-parent history;
rewritten or unrelated histories are rejected.

Each version change produces `<directory-name>@<version>` at the commit that introduces
it to the first-parent history. A root package uses `v<version>`. For example,
`pkg/core` at `0.1.0` gets `core@0.1.0`. A merge commit is compared with its first
parent, so intermediate versions on an unmerged feature branch are not released.
Multiple release commits within one push retain their individual tag targets.
CI verifies the pushed tip; it does not separately test every intermediate commit.

Unchanged versions, pending notes, deleted packages, and `0.0.0` versions produce
no tags. A newly added manifest above `0.0.0` is eligible. For an all-zero base
(initial branch creation), only the head snapshot is considered; older versions
are not backfilled. Manual version edits on main are treated like prepared releases.
Tag names must be valid Git refs. A package can set `tag-prefix` in `.sweet.yml`
to override the literal text before its version.

The tag job invokes `sweet git tag --base <before-sha> --head <after-sha> --json`,
so Sweet validates the range and creates missing annotated tags. The job pushes
one tag at a time: GitHub does not create push events when more than three tags
are pushed together. Matching existing tags are retained, including lightweight
tags. A tag targeting another commit or a version reused within the range stops
the operation. Tags are never moved or overwritten. If a remote rejects a tag,
earlier pushes remain. Rerunning the original CI run safely retries
the original range after a failure; later pushes do not automatically backfill it.

The job requires a personal or GitHub App token with Contents write access,
passed as `release-token`. This repository uses `RELEASE_PR_TOKEN`.
Tags pushed with `GITHUB_TOKEN` do not start other push-triggered workflows.
See [CLI publishing](publishing.md) and
[GitHub's workflow trigger rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow).
The local tests use temporary repositories and a bare remote to verify annotated
tags, reruns, conflicts, push rejection, and recovery without contacting GitHub.
